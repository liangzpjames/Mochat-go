package dashboard

import (
	"context"
	"reflect"
	"testing"
)

type fakeDashboardAccessStore struct {
	identity      DashboardAccessIdentity
	identityFound bool
	catalog       []DashboardPermissionDefinition
	grants        []DashboardPermissionGrantFact
	employeeScope DashboardEmployeeScope
	employeeFound bool
	grantsCalls   int
	lastTenantID  int
	lastUserID    int
}

func (store *fakeDashboardAccessStore) DashboardAccessIdentity(context.Context, int) (DashboardAccessIdentity, bool, error) {
	return store.identity, store.identityFound, nil
}

func (store *fakeDashboardAccessStore) DashboardPermissionCatalog(context.Context) ([]DashboardPermissionDefinition, error) {
	return append([]DashboardPermissionDefinition(nil), store.catalog...), nil
}

func (store *fakeDashboardAccessStore) DashboardPermissionGrants(_ context.Context, tenantID, userID int) ([]DashboardPermissionGrantFact, error) {
	store.grantsCalls++
	store.lastTenantID = tenantID
	store.lastUserID = userID
	return append([]DashboardPermissionGrantFact(nil), store.grants...), nil
}

func (store *fakeDashboardAccessStore) DashboardEmployeeScope(_ context.Context, tenantID, userID, _ int) (DashboardEmployeeScope, bool, error) {
	if tenantID != store.identity.TenantID || userID != store.identity.UserID {
		return DashboardEmployeeScope{}, false, nil
	}
	return store.employeeScope, store.employeeFound, nil
}

func TestDashboardAccessResolvesDirectAndAllEnabledRoles(t *testing.T) {
	catalog := []DashboardPermissionDefinition{
		{ID: 1, Code: "dashboard.alpha", Path: "/alpha", Name: "Alpha", ScopeRequired: true},
		{ID: 2, Code: "dashboard.beta", Path: "/beta", Name: "Beta", ScopeRequired: true},
		{ID: 3, Code: "dashboard.gamma", Path: "/gamma", Name: "Gamma"},
		{ID: 4, Code: "dashboard.staff", Path: "/company-setting/staff", Name: "Staff", SuperadminOnly: true},
		{ID: 5, Code: "dashboard.missing_scope", Path: "/missing-scope", Name: "Missing scope", ScopeRequired: true},
	}
	store := &fakeDashboardAccessStore{
		identityFound: true,
		identity:      DashboardAccessIdentity{UserID: 7, TenantID: 9, UserName: "普通用户", CorpName: "极义科技", Status: 1},
		catalog:       catalog,
		employeeFound: true,
		employeeScope: DashboardEmployeeScope{EmployeeID: 31, DepartmentIDs: []int{2}, DepartmentEmployeeIDs: []int{31, 32}},
		grants: []DashboardPermissionGrantFact{
			{TenantID: 9, PermissionID: 1, SourceType: PermissionSourceDirect, SourceID: 7, SourceName: "直接授权", Scope: string(DataScopeSelf)},
			{TenantID: 9, PermissionID: 1, SourceType: PermissionSourceRole, SourceID: 11, SourceName: "销售", SourceStatus: 1, Scope: string(DataScopeDepartment)},
			{TenantID: 9, PermissionID: 1, SourceType: PermissionSourceRole, SourceID: 11, SourceName: "销售", SourceStatus: 1, Scope: string(DataScopeDepartment)},
			{TenantID: 9, PermissionID: 1, SourceType: PermissionSourceRole, SourceID: 12, SourceName: "运营", SourceStatus: 1, Scope: string(DataScopeTenant)},
			{TenantID: 9, PermissionID: 2, SourceType: PermissionSourceDirect, SourceID: 7, SourceName: "直接授权", Scope: ""},
			{TenantID: 9, PermissionID: 3, SourceType: PermissionSourceRole, SourceID: 13, SourceName: "停用角色", SourceStatus: 2, Scope: string(DataScopeTenant)},
			{TenantID: 9, PermissionID: 3, SourceType: PermissionSourceRole, SourceID: 14, SourceName: "已删除角色", SourceStatus: 1, SourceDeleted: true, Scope: string(DataScopeTenant)},
			{TenantID: 10, PermissionID: 3, SourceType: PermissionSourceRole, SourceID: 15, SourceName: "跨租户角色", SourceStatus: 1, Scope: string(DataScopeTenant)},
			{TenantID: 9, PermissionID: 4, SourceType: PermissionSourceDirect, SourceID: 7, SourceName: "非法管理页直授", Scope: string(DataScopeTenant)},
			{TenantID: 9, PermissionID: 4, SourceType: PermissionSourceRole, SourceID: 16, SourceName: "非法管理页角色", SourceStatus: 1, Scope: string(DataScopeTenant)},
			{TenantID: 9, PermissionID: 5, SourceType: PermissionSourceRole, SourceID: 17, SourceName: "缺范围角色", SourceStatus: 1, Scope: ""},
		},
	}
	profile, err := NewDashboardAccessService(store).Resolve(context.Background(), 7, 21)
	if err != nil {
		t.Fatal(err)
	}
	if profile.UserID != 7 || profile.UserName != "普通用户" || profile.TenantID != 9 || profile.CorpID != 21 || profile.CorpName != "极义科技" || profile.WorkEmployeeID != 31 || profile.IsSuperAdmin {
		t.Fatalf("profile = %+v", profile)
	}
	if !reflect.DeepEqual(profile.DepartmentIDs, []int{2}) || !reflect.DeepEqual(profile.DepartmentEmployeeIDs, []int{31, 32}) {
		t.Fatalf("employee scope = departments=%v employees=%v", profile.DepartmentIDs, profile.DepartmentEmployeeIDs)
	}
	if store.grantsCalls != 1 || store.lastTenantID != 9 || store.lastUserID != 7 {
		t.Fatalf("grant query calls=%d tenant=%d user=%d", store.grantsCalls, store.lastTenantID, store.lastUserID)
	}
	if got := []string{profile.EffectivePermissions[0].Code, profile.EffectivePermissions[1].Code}; !reflect.DeepEqual(got, []string{"dashboard.alpha", "dashboard.beta"}) {
		t.Fatalf("effective codes = %v, permissions=%+v", got, profile.EffectivePermissions)
	}
	alpha := profile.EffectivePermissions[0]
	if alpha.Scope != DataScopeTenant || len(alpha.Sources) != 3 || alpha.Sources[0].Type != PermissionSourceDirect || alpha.Sources[0].ID != 7 || alpha.Sources[1].ID != 11 || alpha.Sources[2].ID != 12 {
		t.Fatalf("alpha = %+v", alpha)
	}
	beta := profile.EffectivePermissions[1]
	if beta.Scope != DataScopeSelf || len(beta.Sources) != 1 || beta.Sources[0].Type != PermissionSourceDirect {
		t.Fatalf("beta = %+v", beta)
	}
	if !profile.AllowsPage("/alpha") || !profile.AllowsPage("/beta") || profile.AllowsPage("/gamma") || profile.AllowsPage("/company-setting/staff") || profile.AllowsPage("/missing-scope") {
		t.Fatalf("allowed routes = %v", profile.AllowedRoutes)
	}
	if profile.ScopeFor("dashboard.alpha") != DataScopeTenant || profile.ScopeFor("dashboard.beta") != DataScopeSelf || profile.ScopeFor("dashboard.gamma") != "" {
		t.Fatalf("scopes = alpha:%q beta:%q gamma:%q", profile.ScopeFor("dashboard.alpha"), profile.ScopeFor("dashboard.beta"), profile.ScopeFor("dashboard.gamma"))
	}
}

func TestDashboardAccessScopeUnionOrder(t *testing.T) {
	if got := MergeDataScopes(DataScopeSelf, DataScopeDepartment); got != DataScopeDepartment {
		t.Fatalf("self + department = %q", got)
	}
	if got := MergeDataScopes(DataScopeDepartment, DataScopeTenant); got != DataScopeTenant {
		t.Fatalf("department + tenant = %q", got)
	}
	if got := MergeDataScopes(DataScopeTenant, DataScopeSelf); got != DataScopeTenant {
		t.Fatalf("tenant + self = %q", got)
	}
}

func TestDashboardAccessSuperadminImplicitlyReceivesEntireCatalog(t *testing.T) {
	store := &fakeDashboardAccessStore{
		identityFound: true,
		identity:      DashboardAccessIdentity{UserID: 1, TenantID: 9, UserName: "超管", Status: 1, IsSuperAdmin: true},
		catalog: []DashboardPermissionDefinition{
			{ID: 1, Code: "dashboard.alpha", Path: "/alpha", Name: "Alpha"},
			{ID: 2, Code: "dashboard.staff", Path: "/company-setting/staff", Name: "Staff", SuperadminOnly: true},
			{ID: 3, Code: "dashboard.role", Path: "/setting/role", Name: "Role", SuperadminOnly: true},
			{ID: 4, Code: "dashboard.additional", Path: "/setting/additional", Name: "Additional", SuperadminOnly: true},
			{ID: 5, Code: "dashboard.authorization", Path: "/setting/authorization", Name: "Authorization", SuperadminOnly: true},
		},
	}
	profile, err := NewDashboardAccessService(store).Resolve(context.Background(), 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !profile.IsSuperAdmin || len(profile.EffectivePermissions) != 5 || len(profile.AllowedRoutes) != 5 || store.grantsCalls != 0 {
		t.Fatalf("profile=%+v grantsCalls=%d", profile, store.grantsCalls)
	}
	for _, permission := range profile.EffectivePermissions {
		if permission.Scope != DataScopeTenant || len(permission.Sources) != 1 || permission.Sources[0].Type != PermissionSourceSuperadmin {
			t.Fatalf("permission=%+v", permission)
		}
	}
}

func TestDashboardAccessRejectsMissingOrDisabledIdentity(t *testing.T) {
	for _, test := range []struct {
		name  string
		store *fakeDashboardAccessStore
	}{
		{name: "missing", store: &fakeDashboardAccessStore{}},
		{name: "disabled", store: &fakeDashboardAccessStore{identityFound: true, identity: DashboardAccessIdentity{UserID: 7, TenantID: 9, Status: 2}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewDashboardAccessService(test.store).Resolve(context.Background(), 7, 0); err != ErrUnauthorized {
				t.Fatalf("err=%v, want ErrUnauthorized", err)
			}
		})
	}
}
