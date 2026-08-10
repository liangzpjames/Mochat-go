package store

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakeDashboardAccessRows struct {
	rows  [][]any
	index int
	err   error
}

func (rows *fakeDashboardAccessRows) Next() bool {
	return rows.index < len(rows.rows)
}

func (rows *fakeDashboardAccessRows) Scan(dest ...any) error {
	if rows.index >= len(rows.rows) {
		return errors.New("scan without row")
	}
	values := rows.rows[rows.index]
	rows.index++
	if len(dest) != len(values) {
		return errors.New("unexpected scan destination count")
	}
	for index := range dest {
		target := reflect.ValueOf(dest[index])
		value := reflect.ValueOf(values[index])
		if !target.IsValid() || target.Kind() != reflect.Pointer || !value.IsValid() || !value.Type().AssignableTo(target.Elem().Type()) {
			return errors.New("unexpected scan destination type")
		}
		target.Elem().Set(value)
	}
	return nil
}

func (rows *fakeDashboardAccessRows) Err() error   { return rows.err }
func (rows *fakeDashboardAccessRows) Close() error { return nil }

func TestMySQLStoreDashboardAccessIdentityUsesJWTUserID(t *testing.T) {
	var query string
	var args []any
	store := &MySQLStore{dashboardAccessQueryRow: func(_ context.Context, gotQuery string, gotArgs ...any) dashboardTenantAccessRow {
		query, args = gotQuery, gotArgs
		return fakeDashboardTenantAccessRow{values: []any{7, 9, "普通用户", 1, 0}}
	}}
	identity, found, err := store.DashboardAccessIdentity(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if !found || identity.UserID != 7 || identity.TenantID != 9 || identity.IsSuperAdmin {
		t.Fatalf("identity=%+v found=%v", identity, found)
	}
	if !strings.Contains(query, "WHERE id = ?") || !reflect.DeepEqual(args, []any{7}) {
		t.Fatalf("query=%q args=%v", query, args)
	}
}

func TestMySQLStoreDashboardPermissionCatalogAggregatesScopeRequirement(t *testing.T) {
	var query string
	store := &MySQLStore{dashboardAccessQuery: func(_ context.Context, gotQuery string, _ ...any) (dashboardAccessRows, error) {
		query = gotQuery
		return &fakeDashboardAccessRows{rows: [][]any{
			{int64(1), "dashboard.alpha", "/alpha", "Alpha", "group-a", 1, 0, 1},
			{int64(2), "dashboard.staff", "/company-setting/staff", "Staff", "company-settings", 50, 1, 0},
		}}, nil
	}}
	catalog, err := store.DashboardPermissionCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) != 2 || !catalog[0].ScopeRequired || !catalog[1].SuperadminOnly {
		t.Fatalf("catalog=%+v", catalog)
	}
	if !strings.Contains(query, "MAX(COALESCE(resource.scope_required, 0))") || !strings.Contains(query, "permission.status = 1") {
		t.Fatalf("query=%q", query)
	}
}

func TestMySQLStoreDashboardPermissionResourcesUseRegisteredMethod(t *testing.T) {
	var query string
	var args []any
	store := &MySQLStore{dashboardAccessQuery: func(_ context.Context, gotQuery string, gotArgs ...any) (dashboardAccessRows, error) {
		query, args = gotQuery, gotArgs
		return &fakeDashboardAccessRows{rows: [][]any{
			{"dashboard.contacts", "GET", "/dashboard/workContact/{id}", 1},
			{"dashboard.contacts", "GET", "/dashboard/workContact/index", 0},
		}}, nil
	}}
	resources, err := store.DashboardPermissionResources(context.Background(), " get ")
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 2 || !resources[0].ScopeRequired || resources[1].PathPattern != "/dashboard/workContact/index" {
		t.Fatalf("resources=%+v", resources)
	}
	if !reflect.DeepEqual(args, []any{"GET"}) {
		t.Fatalf("args=%v", args)
	}
	for _, contract := range []string{"resource.http_method = ?", "resource.status = 1", "resource.deleted_at IS NULL", "permission.status = 1", "permission.deleted_at IS NULL"} {
		if !strings.Contains(query, contract) {
			t.Fatalf("query missing %q: %s", contract, query)
		}
	}
}

func TestMySQLStoreDashboardPermissionGrantsAreTenantScoped(t *testing.T) {
	var query string
	var args []any
	store := &MySQLStore{dashboardAccessQuery: func(_ context.Context, gotQuery string, gotArgs ...any) (dashboardAccessRows, error) {
		query, args = gotQuery, gotArgs
		return &fakeDashboardAccessRows{rows: [][]any{
			{9, int64(1), "direct", 7, "直接授权", 1, 0, "self"},
			{9, int64(1), "role", 11, "销售", 1, 0, "department"},
			{9, int64(2), "role", 12, "运营", 1, 0, "tenant"},
		}}, nil
	}}
	grants, err := store.DashboardPermissionGrants(context.Background(), 9, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 3 || grants[0].TenantID != 9 || grants[1].SourceID != 11 || grants[2].Scope != "tenant" {
		t.Fatalf("grants=%+v", grants)
	}
	if !reflect.DeepEqual(args, []any{9, 7, 9, 7}) {
		t.Fatalf("args=%v", args)
	}
	for _, contract := range []string{
		"direct_permission.tenant_id = ?", "direct_permission.user_id = ?", "direct_permission.effect = 'allow'",
		"user_role.tenant_id = ?", "user_role.user_id = ?", "role.status = 1", "role.deleted_at IS NULL",
		"permission.superadmin_only = 0",
	} {
		if !strings.Contains(query, contract) {
			t.Fatalf("query missing %q: %s", contract, query)
		}
	}
}

func TestMySQLStoreDashboardEmployeeScopeIsTenantAndCorpScoped(t *testing.T) {
	var rowQuery string
	var rowArgs []any
	var rowsQuery string
	var rowsArgs []any
	store := &MySQLStore{
		dashboardAccessQueryRow: func(_ context.Context, query string, args ...any) dashboardTenantAccessRow {
			rowQuery, rowArgs = query, args
			return fakeDashboardTenantAccessRow{values: []any{31}}
		},
		dashboardAccessQuery: func(_ context.Context, query string, args ...any) (dashboardAccessRows, error) {
			rowsQuery, rowsArgs = query, args
			return &fakeDashboardAccessRows{rows: [][]any{{2, 31}, {2, 32}, {3, 33}}}, nil
		},
	}
	scope, found, err := store.DashboardEmployeeScope(context.Background(), 9, 7, 21)
	if err != nil {
		t.Fatal(err)
	}
	if !found || scope.EmployeeID != 31 || !reflect.DeepEqual(scope.DepartmentIDs, []int{2, 3}) || !reflect.DeepEqual(scope.DepartmentEmployeeIDs, []int{31, 32, 33}) {
		t.Fatalf("scope=%+v found=%v", scope, found)
	}
	for _, evidence := range []struct {
		query string
		args  []any
	}{
		{query: rowQuery, args: rowArgs},
		{query: rowsQuery, args: rowsArgs},
	} {
		if !strings.Contains(evidence.query, "corp.tenant_id = ?") || !strings.Contains(evidence.query, "corp.id = ?") || !strings.Contains(evidence.query, "employee.log_user_id = ?") {
			t.Fatalf("query=%q", evidence.query)
		}
		if !reflect.DeepEqual(evidence.args, []any{9, 21, 7}) {
			t.Fatalf("args=%v", evidence.args)
		}
	}
}
