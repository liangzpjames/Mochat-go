package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"jiyi/mochat-go/internal/dashboard"
)

// TestDashboardAccessIntegration deliberately uses the database after migration 0127.
// The fixture is supplied by the desktop acceptance harness; creating shadow tables here
// would only prove a parallel schema and would not exercise MySQLStore or production FKs.
func TestDashboardAccessIntegration(t *testing.T) {
	db := openDashboardIntegrationDB(t)
	fixture := dashboardIntegrationFixtureFromEnv(t)
	assertDashboard0127Applied(t, db)
	ctx := context.Background()

	// All writes are named fixture rows and are removed on exit. No non-fixture row is touched.
	cleanup := func() {
		_, _ = db.ExecContext(ctx, `UPDATE mc_rbac_role SET status=? WHERE tenant_id=? AND id=?`, fixture.OriginalRoleStatus, fixture.TenantID, fixture.RoleID)
		_, _ = db.ExecContext(ctx, `DELETE FROM mochat_go_dashboard_user_permissions WHERE tenant_id=? AND user_id=?`, fixture.TenantID, fixture.TargetUserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM mochat_go_dashboard_user_roles WHERE tenant_id=? AND user_id=?`, fixture.TenantID, fixture.TargetUserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM mochat_go_dashboard_role_permissions WHERE tenant_id=? AND role_id=?`, fixture.TenantID, fixture.RoleID)
		_, _ = db.ExecContext(ctx, `DELETE FROM mochat_go_dashboard_permission_audits WHERE tenant_id=? AND request_id LIKE 'task9-%'`, fixture.TenantID)
	}
	t.Cleanup(cleanup)

	// Production composite FKs reject a relation whose tenant does not own either endpoint.
	if _, err := db.ExecContext(ctx, `INSERT INTO mochat_go_dashboard_user_roles(tenant_id,user_id,role_id) VALUES (?,?,?)`, fixture.OtherTenantID, fixture.TargetUserID, fixture.RoleID); err == nil {
		t.Fatal("cross-tenant dashboard role relation unexpectedly accepted")
	}

	permissionID := fixture.PermissionID
	if _, err := db.ExecContext(ctx, `INSERT INTO mochat_go_dashboard_role_permissions(tenant_id,role_id,permission_id,data_scope) VALUES (?,?,?,?)`, fixture.TenantID, fixture.RoleID, permissionID, dashboard.DataScopeSelf); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO mochat_go_dashboard_role_permissions(tenant_id,role_id,permission_id,data_scope) VALUES (?,?,?,?)`, fixture.TenantID, fixture.RoleID, permissionID, dashboard.DataScopeTenant); err == nil {
		t.Fatal("production unique role-permission relation accepted duplicate")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO mochat_go_dashboard_user_roles(tenant_id,user_id,role_id) VALUES (?,?,?)`, fixture.TenantID, fixture.TargetUserID, fixture.RoleID); err != nil {
		t.Fatal(err)
	}

	store := NewMySQLStore(db)
	profile, err := dashboard.NewDashboardAccessService(store).Resolve(ctx, fixture.TargetUserID, fixture.CorpID)
	if err != nil {
		t.Fatal(err)
	}
	if !containsEffectivePermission(profile.EffectivePermissions, fixture.PermissionCode) {
		t.Fatalf("production service did not resolve role permission %q", fixture.PermissionCode)
	}

	// Disable the real role and verify its contribution disappears while a direct grant remains.
	if _, err := db.ExecContext(ctx, `UPDATE mc_rbac_role SET status=2 WHERE tenant_id=? AND id=?`, fixture.TenantID, fixture.RoleID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO mochat_go_dashboard_user_permissions(tenant_id,user_id,permission_id,effect,data_scope) VALUES (?,?,?,?,?)`, fixture.TenantID, fixture.TargetUserID, permissionID, "allow", dashboard.DataScopeSelf); err != nil {
		t.Fatal(err)
	}
	profile, err = dashboard.NewDashboardAccessService(store).Resolve(ctx, fixture.TargetUserID, fixture.CorpID)
	if err != nil || !containsEffectivePermission(profile.EffectivePermissions, fixture.PermissionCode) {
		t.Fatalf("direct grant was not retained after role disable: err=%v", err)
	}

	// The real aggregate version is compare-and-incremented by the production admin store.
	var version uint64
	if err := db.QueryRowContext(ctx, `SELECT dashboard_access_version FROM mc_user WHERE tenant_id=? AND id=?`, fixture.TenantID, fixture.TargetUserID).Scan(&version); err != nil {
		t.Fatal(err)
	}
	_, err = store.ReplaceUserDashboardAccess(ctx, dashboard.ReplaceUserDashboardAccessCommand{TenantID: fixture.TenantID, ActorUserID: fixture.ActorUserID, ActorName: fixture.ActorName, TargetUserID: fixture.TargetUserID, ExpectedVersion: version, RequestID: "task9-integration"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.ReplaceUserDashboardAccess(ctx, dashboard.ReplaceUserDashboardAccessCommand{TenantID: fixture.TenantID, ActorUserID: fixture.ActorUserID, ActorName: fixture.ActorName, TargetUserID: fixture.TargetUserID, ExpectedVersion: version, RequestID: "task9-stale"}); err != dashboard.ErrDashboardAccessAdminConflict {
		t.Fatalf("stale expectedVersion error=%v", err)
	}

	// Audit insertion is part of the same transaction; the store's rollback contract is covered
	// by the production fake-transaction tests and verified here by the resulting audit row.
	var audits int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_dashboard_permission_audits WHERE tenant_id=? AND request_id IN ('task9-integration','task9-stale')`, fixture.TenantID).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if audits != 1 {
		t.Fatalf("expected one committed audit and no stale audit, got %d", audits)
	}
}

type dashboardIntegrationFixture struct {
	TenantID           int    `json:"tenantId"`
	OtherTenantID      int    `json:"otherTenantId"`
	ActorUserID        int    `json:"actorUserId"`
	TargetUserID       int    `json:"targetUserId"`
	RoleID             int    `json:"roleId"`
	PermissionID       int64  `json:"permissionId"`
	CorpID             int    `json:"corpId"`
	PermissionCode     string `json:"permissionCode"`
	ActorName          string `json:"actorName"`
	OriginalRoleStatus int    `json:"originalRoleStatus"`
}

func dashboardIntegrationFixtureFromEnv(t *testing.T) dashboardIntegrationFixture {
	raw := os.Getenv("MOCHAT_GO_DASHBOARD_FIXTURE_JSON")
	if raw == "" {
		t.Fatal("MOCHAT_GO_DASHBOARD_FIXTURE_JSON is required when MOCHAT_GO_MYSQL_INTEGRATION_DSN is set; provide isolated 0127 fixture IDs")
	}
	var fixture dashboardIntegrationFixture
	if err := json.Unmarshal([]byte(raw), &fixture); err != nil {
		t.Fatalf("invalid dashboard fixture JSON: %v", err)
	}
	if fixture.PermissionCode == "" || fixture.ActorName == "" {
		t.Fatal("dashboard fixture must include permissionCode and actorName")
	}
	if fixture.OriginalRoleStatus == 0 {
		fixture.OriginalRoleStatus = 1
	}
	return fixture
}

func assertDashboard0127Applied(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, table := range []string{"mochat_go_dashboard_permissions", "mochat_go_dashboard_permission_resources", "mochat_go_dashboard_user_roles", "mochat_go_dashboard_role_permissions", "mochat_go_dashboard_user_permissions", "mochat_go_dashboard_permission_audits"} {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?`, table).Scan(&n); err != nil || n != 1 {
			t.Fatalf("0127 migration is not applied: table %s missing", table)
		}
	}
}

func openDashboardIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN")
	if dsn == "" {
		t.Skip("SKIP: MOCHAT_GO_MYSQL_INTEGRATION_DSN is not set; isolated MariaDB DSN is required")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	return db
}

func containsEffectivePermission(items []dashboard.EffectivePermission, code string) bool {
	for _, item := range items {
		if item.Code == code {
			return true
		}
	}
	return false
}
