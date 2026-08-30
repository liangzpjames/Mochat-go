package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/integrationtestdb"
	migrationtestharness "jiyi/mochat-go/internal/migration/testharness"
)

// TestDashboardAccessIntegration creates a throwaway schema from an admin DSN, applies the
// production 0127 migration, and exercises MySQLStore against only named fixture rows.
func TestDashboardAccessIntegration(t *testing.T) {
	db := openDashboardIntegrationDB(t)
	fixture := dashboardIntegrationFixture{TenantID: 1, OtherTenantID: 2, ActorUserID: 100, TargetUserID: 101, RoleID: 10, OtherRoleID: 11, CorpID: 7, PermissionCode: "dashboard.index", ActorName: "task9 actor", OriginalRoleStatus: 1}
	assertDashboard0127Applied(t, db)
	if err := db.QueryRow(`SELECT id FROM mochat_go_dashboard_permissions WHERE code=?`, fixture.PermissionCode).Scan(&fixture.PermissionID); err != nil {
		t.Fatal(err)
	}
	var secondPermissionID int64
	if err := db.QueryRow(`SELECT id FROM mochat_go_dashboard_permissions WHERE code='dashboard.chat.v2_all'`).Scan(&secondPermissionID); err != nil {
		t.Fatal(err)
	}
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
	var backfilled int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_dashboard_user_roles WHERE tenant_id=? AND user_id=? AND role_id=?`, fixture.TenantID, fixture.TargetUserID, fixture.RoleID).Scan(&backfilled); err != nil || backfilled != 1 {
		t.Fatalf("0127 role backfill=%d err=%v", backfilled, err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO mochat_go_dashboard_role_permissions(tenant_id,role_id,permission_id,data_scope) VALUES (?,?,?,?)`, fixture.TenantID, fixture.OtherRoleID, secondPermissionID, dashboard.DataScopeDepartment); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO mochat_go_dashboard_user_roles(tenant_id,user_id,role_id) VALUES (?,?,?)`, fixture.TenantID, fixture.TargetUserID, fixture.OtherRoleID); err != nil {
		t.Fatal(err)
	}

	store := NewMySQLStore(db)
	created, err := store.CreateDashboardRole(ctx, dashboard.CreateDashboardRoleCommand{TenantID: 1, ActorUserID: 100, ActorName: "task9 actor", Name: "task9 role", Status: 1, RequestID: "task9-role-create"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := store.UpdateDashboardRole(ctx, dashboard.UpdateDashboardRoleCommand{TenantID: 1, ActorUserID: 100, ActorName: "task9 actor", RoleID: created.ID, Name: "task9 role updated", ExpectedVersion: created.Version, RequestID: "task9-role-update"})
	if err != nil || updated.Name != "task9 role updated" {
		t.Fatalf("role CRUD update=%+v err=%v", updated, err)
	}
	if err := store.DeleteDashboardRole(ctx, dashboard.DeleteDashboardRoleCommand{TenantID: 1, ActorUserID: 100, ActorName: "task9 actor", RoleID: created.ID, ExpectedVersion: updated.Version, RequestID: "task9-role-delete"}); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteDashboardRole(ctx, dashboard.DeleteDashboardRoleCommand{TenantID: 1, ActorUserID: 100, ActorName: "task9 actor", RoleID: fixture.RoleID, ExpectedVersion: 1, RequestID: "task9-member-delete"}); err != dashboard.ErrDashboardAccessRoleHasMembers {
		t.Fatalf("member role delete err=%v", err)
	}
	// Migration 0176 intentionally widens audit request IDs to 128 characters so
	// callback reconciliation can preserve every valid Idempotency-Key byte.
	// Keep the rollback contract at the current database boundary, not the old
	// 96-character limit.
	longRequest := strings.Repeat("x", 129)
	if _, err := store.UpdateDashboardRole(ctx, dashboard.UpdateDashboardRoleCommand{TenantID: 1, ActorUserID: 100, ActorName: "task9 actor", RoleID: fixture.OtherRoleID, Name: "must rollback", ExpectedVersion: 1, RequestID: longRequest}); err == nil {
		t.Fatal("oversized audit request unexpectedly committed")
	}
	var unchanged string
	if err := db.QueryRowContext(ctx, `SELECT name FROM mc_rbac_role WHERE tenant_id=? AND id=?`, fixture.TenantID, fixture.OtherRoleID).Scan(&unchanged); err != nil || unchanged != "role-b" {
		t.Fatalf("audit failure did not rollback role name=%q err=%v", unchanged, err)
	}
	profile, err := dashboard.NewDashboardAccessService(store).Resolve(ctx, fixture.TargetUserID, fixture.CorpID)
	if err != nil {
		t.Fatal(err)
	}
	if !containsEffectivePermission(profile.EffectivePermissions, fixture.PermissionCode) {
		t.Fatalf("production service did not resolve role permission %q", fixture.PermissionCode)
	}
	if !containsEffectivePermission(profile.EffectivePermissions, "dashboard.chat.v2_all") {
		t.Fatal("second enabled role permission missing from union")
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
	indexPermission, indexOK := findEffectivePermission(profile.EffectivePermissions, fixture.PermissionCode)
	chatPermission, chatOK := findEffectivePermission(profile.EffectivePermissions, "dashboard.chat.v2_all")
	if !indexOK || !hasSourceType(indexPermission, dashboard.PermissionSourceDirect) || hasSourceType(indexPermission, dashboard.PermissionSourceRole) {
		t.Fatal("disabled role source was not removed while direct source remained")
	}
	if !chatOK || !hasSourceType(chatPermission, dashboard.PermissionSourceRole) {
		t.Fatal("enabled second role source disappeared")
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
	OtherRoleID        int    `json:"otherRoleId"`
	PermissionID       int64  `json:"permissionId"`
	CorpID             int    `json:"corpId"`
	PermissionCode     string `json:"permissionCode"`
	ActorName          string `json:"actorName"`
	OriginalRoleStatus int    `json:"originalRoleStatus"`
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
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN"))
	if dsn == "" {
		t.Skip("SKIP: MOCHAT_GO_MYSQL_INTEGRATION_DSN is not set; isolated MariaDB/MySQL DSN is required")
	}
	isolated := integrationtestdb.NewIsolated(t, dsn)
	db := isolated.DB
	root := filepath.Join("..", "..")
	evidence, err := migrationtestharness.NewControlledEvidence(fmt.Sprintf("dashboard-access-%d", currentStoreIntegrationSequence.Add(1)))
	if err != nil {
		t.Fatal(err)
	}
	if err := migrationtestharness.ApplyThrough(context.Background(), db, root, "0126_phase3_final_providers", evidence); err != nil {
		t.Fatal(err)
	}
	createDashboardAccessFixture(t, db)
	if err := migrationtestharness.ApplyThrough(context.Background(), db, root, "0127_dashboard_page_rbac", evidence); err != nil {
		t.Fatal(err)
	}
	if err := migrationtestharness.ApplyLatest(context.Background(), db, root, evidence); err != nil {
		t.Fatal(err)
	}
	return db
}

func createDashboardAccessFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	statements := []string{
		`INSERT INTO mc_tenant(id,name,status) VALUES (1,'tenant-1',1),(2,'tenant-2',1)`,
		`INSERT INTO mc_user(id,tenant_id,name,phone,password,status,isSuperAdmin) VALUES (100,1,'actor','13800000100','!fixture',1,1),(101,1,'target','13800000101','!fixture',1,0),(201,2,'other','13800000201','!fixture',1,0)`,
		`INSERT INTO mc_rbac_role(id,tenant_id,name,remarks,status,operate_id,operate_name,data_permission) VALUES (10,1,'role-a','',1,100,'actor','[]'),(11,1,'role-b','',1,100,'actor','[]')`,
		`INSERT INTO mc_rbac_user_role(user_id,role_id) VALUES (101,10)`,
		`INSERT INTO mc_rbac_menu(id,parent_id,link_url,data_permission) VALUES (900030,0,'/dashboard/channelCode/index#GET',1),(900031,0,'/dashboard/workContact/index@read',1)`,
		`INSERT INTO mc_rbac_role_menu(role_id,menu_id) VALUES (10,900030),(10,900031)`,
		`INSERT INTO mochat_go_saas_subscriptions(id,tenant_id,status) VALUES (1,1,'active'),(2,2,'active')`,
		`INSERT INTO mc_corp(id,tenant_id,name,wx_corpid) VALUES (7,1,'corp-1','wx-corp-1'),(8,2,'corp-2','wx-corp-2')`,
		`INSERT INTO mc_work_employee(id,corp_id,log_user_id,name,status) VALUES (700,7,101,'target',1),(701,8,201,'other',1)`,
		`INSERT INTO mc_work_department(id,corp_id,name,wx_parentid,path) VALUES (70,7,'dept-70',0,'/70/')`,
		`INSERT INTO mc_work_employee_department(employee_id,department_id) VALUES (700,70)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	for tenantID, packageID := range map[int]int{1: 1, 2: 2} {
		if _, err := db.Exec(`INSERT INTO mochat_go_saas_tenant_packages(id,tenant_id,package_code,package_name,status,limits_json) VALUES (?,?,?,'fixture',1,?)`, packageID, tenantID, fmt.Sprintf("fixture-%d", tenantID), contactBatchLimitsJSON()); err != nil {
			t.Fatal(err)
		}
	}
}

func containsEffectivePermission(items []dashboard.EffectivePermission, code string) bool {
	for _, item := range items {
		if item.Code == code {
			return true
		}
	}
	return false
}

func findEffectivePermission(items []dashboard.EffectivePermission, code string) (dashboard.EffectivePermission, bool) {
	for _, item := range items {
		if item.Code == code {
			return item, true
		}
	}
	return dashboard.EffectivePermission{}, false
}
func hasSourceType(item dashboard.EffectivePermission, sourceType string) bool {
	for _, source := range item.Sources {
		if source.Type == sourceType {
			return true
		}
	}
	return false
}
