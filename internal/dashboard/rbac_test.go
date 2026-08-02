package dashboard

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestPermissionKeyFromRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/dashboard/workContact/index?foo=bar", nil)
	got := PermissionKeyFromRequest(req)
	if got != "/dashboard/workContact/index#post" {
		t.Fatalf("permission key = %q", got)
	}
}

func TestRBACResolverAllowsSuperAdminWithoutRoleLookup(t *testing.T) {
	store := &fakeRBACStore{
		user: User{ID: 1, TenantID: 9, IsSuperAdmin: 1},
	}
	resolver := NewRBACResolver(store)

	access, err := resolver.Resolve(context.Background(), 1, "/dashboard/workContact/index#get", 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if access.DataPermission != DataPermissionAll {
		t.Fatalf("data permission = %d", access.DataPermission)
	}
	if store.rolesCalled {
		t.Fatal("super admin should not load roles")
	}
}

func TestRBACResolverDeniesMissingMenu(t *testing.T) {
	store := &fakeRBACStore{
		user:  User{ID: 1, TenantID: 9},
		roles: []Role{{ID: 8}},
	}
	resolver := NewRBACResolver(store)

	_, err := resolver.Resolve(context.Background(), 1, "/dashboard/not-exists#get", 2, 3)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("err = %v", err)
	}
}

func TestRBACResolverLooksUpSeedMenuLinkWithoutHTTPMethodSuffix(t *testing.T) {
	store := &fakeRBACStore{
		user:                User{ID: 1, TenantID: 9},
		roles:               []Role{{ID: 8}},
		menu:                Menu{ID: 219, LinkURL: "/dashboard/corpData/index", DataPermission: DataPermissionSelf},
		menuOK:              true,
		roleMenu:            []RoleMenu{{RoleID: 8, MenuID: 219}},
		expectedMenuLinkURL: "/dashboard/corpData/index",
	}
	resolver := NewRBACResolver(store)

	access, err := resolver.Resolve(context.Background(), 1, "/dashboard/corpData/index#get", 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if store.menuLinkURL != "/dashboard/corpData/index" {
		t.Fatalf("menu lookup key = %q", store.menuLinkURL)
	}
	if access.PermissionKey != "/dashboard/corpData/index#get" {
		t.Fatalf("access permission key = %q", access.PermissionKey)
	}
}

func TestRBACResolverDeniesRoleWithoutMenu(t *testing.T) {
	store := &fakeRBACStore{
		user:     User{ID: 1, TenantID: 9},
		roles:    []Role{{ID: 8}},
		menu:     Menu{ID: 20, DataPermission: DataPermissionDepartment},
		menuOK:   true,
		roleMenu: []RoleMenu{{RoleID: 8, MenuID: 19}},
	}
	resolver := NewRBACResolver(store)

	_, err := resolver.Resolve(context.Background(), 1, "/dashboard/workContact/index#get", 2, 3)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("err = %v", err)
	}
}

func TestRBACResolverMapsMenuDataPermissionDisabledToAll(t *testing.T) {
	store := &fakeRBACStore{
		user:     User{ID: 1, TenantID: 9},
		roles:    []Role{{ID: 8, DataPermission: `[{"corpId":2,"permissionType":1}]`}},
		menu:     Menu{ID: 20, DataPermission: DataPermissionSelf},
		menuOK:   true,
		roleMenu: []RoleMenu{{RoleID: 8, MenuID: 20}},
	}
	resolver := NewRBACResolver(store)

	access, err := resolver.Resolve(context.Background(), 1, "/dashboard/workContact/index#get", 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if access.DataPermission != DataPermissionAll {
		t.Fatalf("data permission = %d", access.DataPermission)
	}
	if len(access.DeptEmployeeIDs) != 0 {
		t.Fatalf("dept ids = %+v", access.DeptEmployeeIDs)
	}
}

func TestRBACResolverUsesRoleCorpOverrideForSelf(t *testing.T) {
	store := &fakeRBACStore{
		user:     User{ID: 1, TenantID: 9},
		roles:    []Role{{ID: 8, DataPermission: `[{"corpId":2,"permissionType":2}]`}},
		menu:     Menu{ID: 20, DataPermission: DataPermissionDepartment},
		menuOK:   true,
		roleMenu: []RoleMenu{{RoleID: 8, MenuID: 20}},
	}
	resolver := NewRBACResolver(store)

	access, err := resolver.Resolve(context.Background(), 1, "/dashboard/workContact/index#get", 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if access.DataPermission != DataPermissionSelf {
		t.Fatalf("data permission = %d", access.DataPermission)
	}
	if !reflect.DeepEqual(access.DeptEmployeeIDs, []int{3}) {
		t.Fatalf("dept ids = %+v", access.DeptEmployeeIDs)
	}
}

func TestRBACResolverUsesDepartmentEmployeeIDs(t *testing.T) {
	store := &fakeRBACStore{
		user:           User{ID: 1, TenantID: 9},
		roles:          []Role{{ID: 8, DataPermission: `[{"corp_id":2,"permission_type":1}]`}},
		menu:           Menu{ID: 20, DataPermission: DataPermissionDepartment},
		menuOK:         true,
		roleMenu:       []RoleMenu{{RoleID: 8, MenuID: 20}},
		deptEmployeeID: []int{3, 4, 5},
	}
	resolver := NewRBACResolver(store)

	access, err := resolver.Resolve(context.Background(), 1, "/dashboard/workContact/index#get", 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if access.DataPermission != DataPermissionDepartment {
		t.Fatalf("data permission = %d", access.DataPermission)
	}
	if !reflect.DeepEqual(access.DeptEmployeeIDs, []int{3, 4, 5}) {
		t.Fatalf("dept ids = %+v", access.DeptEmployeeIDs)
	}
}

func TestRBACResolverFallsBackToSelfWhenDepartmentIsEmpty(t *testing.T) {
	store := &fakeRBACStore{
		user:     User{ID: 1, TenantID: 9},
		roles:    []Role{{ID: 8}},
		menu:     Menu{ID: 20, DataPermission: DataPermissionDepartment},
		menuOK:   true,
		roleMenu: []RoleMenu{{RoleID: 8, MenuID: 20}},
	}
	resolver := NewRBACResolver(store)

	access, err := resolver.Resolve(context.Background(), 1, "/dashboard/workContact/index#get", 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(access.DeptEmployeeIDs, []int{3}) {
		t.Fatalf("dept ids = %+v", access.DeptEmployeeIDs)
	}
}

type fakeRBACStore struct {
	user                User
	roles               []Role
	rolesCalled         bool
	menu                Menu
	menuOK              bool
	roleMenu            []RoleMenu
	deptEmployeeID      []int
	menuLinkURL         string
	expectedMenuLinkURL string
}

func (s *fakeRBACStore) UserByID(context.Context, int) (User, bool, error) {
	return s.user, s.user.ID != 0, nil
}

func (s *fakeRBACStore) RolesByUserTenant(context.Context, int, int) ([]Role, error) {
	s.rolesCalled = true
	return s.roles, nil
}

func (s *fakeRBACStore) MenuByLinkURL(_ context.Context, linkURL string) (Menu, bool, error) {
	s.menuLinkURL = linkURL
	if s.expectedMenuLinkURL != "" && linkURL != s.expectedMenuLinkURL {
		return Menu{}, false, nil
	}
	return s.menu, s.menuOK, nil
}

func (s *fakeRBACStore) RoleMenusByRoleIDs(context.Context, []int) ([]RoleMenu, error) {
	return s.roleMenu, nil
}

func (s *fakeRBACStore) DepartmentEmployeeIDs(context.Context, int) ([]int, error) {
	return s.deptEmployeeID, nil
}
