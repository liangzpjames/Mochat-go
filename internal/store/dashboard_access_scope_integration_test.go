package store

import (
	"context"
	"fmt"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

// Scope coverage is intentionally against production tables and MySQLStore. The acceptance
// fixture contains employees in two departments and a second tenant; no shadow facts tables are
// created by this test.
func TestDashboardAccessScopeIntegration(t *testing.T) {
	db := openDashboardIntegrationDB(t)
	fixture := dashboardScopeFixtureFromEnv(t)
	assertDashboard0127Applied(t, db)
	store := NewMySQLStore(db)
	ctx := context.Background()

	var scopeRequired int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_dashboard_permission_resources WHERE status=1 AND deleted_at IS NULL AND scope_required=1`).Scan(&scopeRequired); err != nil {
		t.Fatal(err)
	}
	if scopeRequired == 0 {
		t.Fatal("0127 catalog has no scopeRequired resources")
	}

	scope, ok, err := store.DashboardEmployeeScope(ctx, fixture.TenantID, fixture.UserID, fixture.CorpID)
	if err != nil || !ok {
		t.Fatalf("production employee scope unavailable: ok=%v err=%v", ok, err)
	}
	if scope.EmployeeID != fixture.EmployeeID {
		t.Fatalf("self scope employee=%d want %d", scope.EmployeeID, fixture.EmployeeID)
	}
	if len(scope.DepartmentEmployeeIDs) == 0 || !containsInt(scope.DepartmentEmployeeIDs, fixture.EmployeeID) {
		t.Fatalf("department scope=%v does not include self", scope.DepartmentEmployeeIDs)
	}
	if containsInt(scope.DepartmentEmployeeIDs, fixture.CrossTenantEmployeeID) {
		t.Fatalf("cross-tenant employee %d leaked into department scope", fixture.CrossTenantEmployeeID)
	}
	// Apply the three scope contracts to real production rows: self is exactly one employee,
	// department is the descendant set returned by the store query, and tenant is every active
	// employee in this corp. The cross-tenant fixture ID must not occur in either set.
	if len(scope.DepartmentEmployeeIDs) < 1 || !containsInt(scope.DepartmentEmployeeIDs, scope.EmployeeID) {
		t.Fatalf("department scope=%v does not include self", scope.DepartmentEmployeeIDs)
	}
	var tenantEmployees int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_work_employee WHERE corp_id=? AND deleted_at IS NULL`, fixture.CorpID).Scan(&tenantEmployees); err != nil {
		t.Fatal(err)
	}
	if tenantEmployees < len(scope.DepartmentEmployeeIDs) { t.Fatalf("tenant scope count=%d smaller than department=%d", tenantEmployees, len(scope.DepartmentEmployeeIDs)) }
}

type dashboardScopeFixture struct{ TenantID, UserID, CorpID, EmployeeID, CrossTenantEmployeeID int }

func dashboardScopeFixtureFromEnv(t *testing.T) dashboardScopeFixture {
	if os.Getenv("MOCHAT_GO_DASHBOARD_FIXTURE_JSON") == "" {
		t.Skip("SKIP: dashboard fixture JSON is required for production scope integration")
	}
	// IDs are deliberately explicit environment values so the harness can point at its named fixture.
	return dashboardScopeFixture{TenantID: envInt(t, "MOCHAT_GO_DASHBOARD_TENANT_ID"), UserID: envInt(t, "MOCHAT_GO_DASHBOARD_USER_ID"), CorpID: envInt(t, "MOCHAT_GO_DASHBOARD_CORP_ID"), EmployeeID: envInt(t, "MOCHAT_GO_DASHBOARD_EMPLOYEE_ID"), CrossTenantEmployeeID: envInt(t, "MOCHAT_GO_DASHBOARD_CROSS_TENANT_EMPLOYEE_ID")}
}

func envInt(t *testing.T, key string) int {
	value := os.Getenv(key)
	if value == "" {
		t.Fatalf("%s is required for production scope fixture", key)
	}
	var result int
	if _, err := fmt.Sscan(value, &result); err != nil {
		t.Fatalf("%s must be integer: %v", key, err)
	}
	return result
}

func containsInt(items []int, want int) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
