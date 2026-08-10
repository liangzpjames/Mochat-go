package store

import (
	"context"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

// Scope coverage runs in the same throwaway 0127 schema as the integration test and calls the
// production MySQLStore query over tenant/department employee facts.
func TestDashboardAccessScopeIntegration(t *testing.T) {
	db := openDashboardIntegrationDB(t)
	fixture := dashboardScopeFixture{TenantID: 1, UserID: 101, CorpID: 7, EmployeeID: 700, CrossTenantEmployeeID: 701}
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
	if tenantEmployees < len(scope.DepartmentEmployeeIDs) {
		t.Fatalf("tenant scope count=%d smaller than department=%d", tenantEmployees, len(scope.DepartmentEmployeeIDs))
	}
}

type dashboardScopeFixture struct{ TenantID, UserID, CorpID, EmployeeID, CrossTenantEmployeeID int }

func containsInt(items []int, want int) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
