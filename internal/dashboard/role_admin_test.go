package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoleAdminIndexReturnsPageAndAuthorizes(t *testing.T) {
	store := &fakeRoleAdminStore{
		users: map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		rolePage: RoleListPage{
			Items: []RoleListItem{{
				ID:          8,
				Name:        "运营角色",
				Remarks:     "负责客户运营",
				UpdatedAt:   "2026-07-02 12:00:00",
				Status:      1,
				EmployeeNum: 3,
			}},
			Total:     1,
			TotalPage: 1,
		},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewRoleAdminHandler(store, staticAdminCache("5-9"), HeaderUserIDResolver{}, authorizer)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/role/index?name=运营&page=2&perPage=5", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/role/index#get" {
		t.Fatalf("permission key = %q", authorizer.permissionKey)
	}
	if store.lastRoleFilter.TenantID != 10 || store.lastRoleFilter.CorpID != 5 || store.lastRoleFilter.Name != "运营" || store.lastRoleFilter.Page != 2 || store.lastRoleFilter.PerPage != 5 {
		t.Fatalf("filter = %+v", store.lastRoleFilter)
	}

	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	list := data["list"].([]any)
	row := list[0].(map[string]any)
	if int(row["roleId"].(float64)) != 8 || int(row["employeeNum"].(float64)) != 3 {
		t.Fatalf("row = %#v", row)
	}
}

func TestRoleAdminShowReturnsCorpDataPermission(t *testing.T) {
	store := &fakeRoleAdminStore{
		users: map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		details: map[int]RoleDetail{
			8: {ID: 8, Name: "运营角色", Remarks: "负责客户运营", DataPermission: `[{"corpId":5,"permissionType":2},{"corpId":6,"permissionType":1}]`},
		},
	}
	handler := NewRoleAdminHandler(store, staticAdminCache("5-9"), HeaderUserIDResolver{}, &recordingAuthorizer{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/role/show?roleId=8", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Show(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if int(data["roleId"].(float64)) != 8 || int(data["dataPermission"].(float64)) != 2 {
		t.Fatalf("data = %#v", data)
	}
}

func TestRoleAdminPermissionShowReturnsHalfCheckedTree(t *testing.T) {
	store := &fakeRoleAdminStore{
		users:      map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		checkedIDs: []int{2},
		permissionMenus: []RolePermissionMenu{
			{ID: 1, ParentID: 0, Name: "系统", Level: 1, IsPageMenu: 1},
			{ID: 2, ParentID: 1, Name: "角色", Level: 2, IsPageMenu: 1},
			{ID: 3, ParentID: 1, Name: "菜单", Level: 2, IsPageMenu: 1},
		},
	}
	handler := NewRoleAdminHandler(store, staticAdminCache("5-9"), HeaderUserIDResolver{}, &recordingAuthorizer{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/role/permissionShow?roleId=8", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.PermissionShow(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	root := data[0].(map[string]any)
	if int(root["checked"].(float64)) != 3 {
		t.Fatalf("root = %#v", root)
	}
	children := root["children"].([]any)
	if int(children[0].(map[string]any)["checked"].(float64)) != 2 || int(children[1].(map[string]any)["checked"].(float64)) != 1 {
		t.Fatalf("children = %#v", children)
	}
}

func TestRoleAdminShowEmployeeReturnsDepartment(t *testing.T) {
	store := &fakeRoleAdminStore{
		users: map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		employeePage: RoleEmployeePage{
			Items: []RoleEmployee{{ID: 3, Name: "张三", Mobile: "13800000000", Email: "a@example.com", Department: "销售部,客服部"}},
			Total: 1, TotalPage: 1,
		},
	}
	handler := NewRoleAdminHandler(store, staticAdminCache("5-9"), HeaderUserIDResolver{}, &recordingAuthorizer{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/role/showEmployee?roleId=8&page=1&perPage=10", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ShowEmployee(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastEmployeeFilter.RoleID != 8 || store.lastEmployeeFilter.CorpID != 5 {
		t.Fatalf("filter = %+v", store.lastEmployeeFilter)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	row := data["list"].([]any)[0].(map[string]any)
	if row["employeeName"] != "张三" || row["department"] != "销售部,客服部" {
		t.Fatalf("row = %#v", row)
	}
}

func TestRoleAdminIndexRejectsPermissionDenied(t *testing.T) {
	store := &fakeRoleAdminStore{
		users:    map[int]User{2: {ID: 2, TenantID: 10, IsSuperAdmin: 0}},
		rolePage: RoleListPage{Items: []RoleListItem{{ID: 8}}, Total: 1, TotalPage: 1},
	}
	handler := NewRoleAdminHandler(store, staticAdminCache("5-9"), HeaderUserIDResolver{}, &recordingAuthorizer{err: ErrPermissionDenied})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/role/index", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "2")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.roleListCalls != 0 {
		t.Fatalf("role list should not be queried after permission denied")
	}
}

func TestRoleAdminStoreCreatesRoleWithCorpPermission(t *testing.T) {
	store := &fakeRoleAdminStore{
		users: map[int]User{1: {ID: 1, Name: "管理员", TenantID: 10, IsSuperAdmin: 1}},
	}
	handler := NewRoleAdminHandler(store, staticAdminCache("5-9"), HeaderUserIDResolver{}, &recordingAuthorizer{})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/role/store", strings.NewReader(`{"name":"运营","remarks":"客户运营","dataPermission":2,"roleId":8}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.createdRole.Name != "运营" || store.createdRole.TenantID != 10 || store.createdRole.OperateName != "管理员" || store.copyFromRoleID != 8 {
		t.Fatalf("created role = %+v copy=%d", store.createdRole, store.copyFromRoleID)
	}
	if !strings.Contains(store.createdRole.DataPermission, `"corpId":5`) || !strings.Contains(store.createdRole.DataPermission, `"permissionType":2`) {
		t.Fatalf("data permission = %s", store.createdRole.DataPermission)
	}
}

func TestRoleAdminPermissionStoreExpandsAndReplacesMenus(t *testing.T) {
	store := &fakeRoleAdminStore{
		users:          map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		expandedOutput: []int{1, 2, 8},
	}
	handler := NewRoleAdminHandler(store, staticAdminCache("5-9"), HeaderUserIDResolver{}, &recordingAuthorizer{})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/role/permissionStore", strings.NewReader(`{"roleId":8,"menuIds":[2,8]}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.PermissionStore(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.expandedInput[0] != 2 || store.expandedInput[1] != 8 {
		t.Fatalf("expanded input = %+v", store.expandedInput)
	}
	if store.replacedRoleID != 8 || len(store.replacedMenuIDs) != 3 || store.replacedMenuIDs[0] != 1 {
		t.Fatalf("replace role=%d menus=%+v", store.replacedRoleID, store.replacedMenuIDs)
	}
}

func TestRoleAdminStatusUpdateRejectsSystemPresetRole(t *testing.T) {
	store := &fakeRoleAdminStore{
		users: map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		details: map[int]RoleDetail{
			1: {ID: 1, Name: "超级管理员", Remarks: "系统预置全权限角色"},
		},
	}
	handler := NewRoleAdminHandler(store, staticAdminCache("5-9"), HeaderUserIDResolver{}, &recordingAuthorizer{})

	req := httptest.NewRequest(http.MethodPut, "/dashboard/role/statusUpdate", strings.NewReader(`{"roleId":1,"status":2}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.StatusUpdate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.statusRoleID != 0 {
		t.Fatalf("status update must not run for preset role")
	}
}

func TestRoleAdminDestroyRejectsSystemPresetRole(t *testing.T) {
	store := &fakeRoleAdminStore{
		users: map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		details: map[int]RoleDetail{
			1: {ID: 1, Name: "超级管理员", Remarks: "bootstrap full-access role"},
		},
	}
	handler := NewRoleAdminHandler(store, staticAdminCache("5-9"), HeaderUserIDResolver{}, &recordingAuthorizer{})

	req := httptest.NewRequest(http.MethodDelete, "/dashboard/role/destroy", strings.NewReader(`{"roleId":1}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Destroy(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.deletedRoleID != 0 {
		t.Fatalf("delete must not run for preset role")
	}
}

type fakeRoleAdminStore struct {
	users              map[int]User
	rolePage           RoleListPage
	details            map[int]RoleDetail
	permissionMenus    []RolePermissionMenu
	checkedIDs         []int
	employeePage       RoleEmployeePage
	lastRoleFilter     RoleListFilter
	lastEmployeeFilter RoleEmployeeFilter
	roleListCalls      int
	nameExists         bool
	createdRole        RoleCreateValues
	copyFromRoleID     int
	updatedRoleID      int
	updatedRole        RoleUpdateValues
	statusRoleID       int
	statusValue        int
	deletedRoleID      int
	employeeCount      int
	expandedInput      []int
	expandedOutput     []int
	replacedRoleID     int
	replacedMenuIDs    []int
}

func (s *fakeRoleAdminStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeRoleAdminStore) EmployeeIDByUserCorp(_ context.Context, userID int, corpID int) (int, error) {
	return 0, nil
}

func (s *fakeRoleAdminStore) FirstEmployeeByUser(_ context.Context, userID int) (int, int, bool, error) {
	return 0, 0, false, nil
}

func (s *fakeRoleAdminStore) RoleList(_ context.Context, filter RoleListFilter) (RoleListPage, error) {
	s.roleListCalls++
	s.lastRoleFilter = filter
	return s.rolePage, nil
}

func (s *fakeRoleAdminStore) RoleDetailByIDTenant(_ context.Context, roleID int, tenantID int) (RoleDetail, bool, error) {
	role, ok := s.details[roleID]
	return role, ok, nil
}

func (s *fakeRoleAdminStore) RolePermissionMenus(_ context.Context, roleID int) ([]RolePermissionMenu, []int, error) {
	return s.permissionMenus, s.checkedIDs, nil
}

func (s *fakeRoleAdminStore) RoleEmployees(_ context.Context, filter RoleEmployeeFilter) (RoleEmployeePage, error) {
	s.lastEmployeeFilter = filter
	return s.employeePage, nil
}

func (s *fakeRoleAdminStore) RoleNameExists(_ context.Context, name string, tenantID int, excludeRoleID int) (bool, error) {
	return s.nameExists, nil
}

func (s *fakeRoleAdminStore) CreateRole(_ context.Context, values RoleCreateValues, copyFromRoleID int) (int, error) {
	s.createdRole = values
	s.copyFromRoleID = copyFromRoleID
	return 101, nil
}

func (s *fakeRoleAdminStore) UpdateRole(_ context.Context, roleID int, tenantID int, values RoleUpdateValues) (bool, error) {
	s.updatedRoleID = roleID
	s.updatedRole = values
	return true, nil
}

func (s *fakeRoleAdminStore) UpdateRoleStatus(_ context.Context, roleID int, tenantID int, status int) (bool, error) {
	s.statusRoleID = roleID
	s.statusValue = status
	return true, nil
}

func (s *fakeRoleAdminStore) DeleteRole(_ context.Context, roleID int, tenantID int) (bool, error) {
	s.deletedRoleID = roleID
	return true, nil
}

func (s *fakeRoleAdminStore) RoleEmployeeCount(_ context.Context, roleID int, corpID int) (int, error) {
	return s.employeeCount, nil
}

func (s *fakeRoleAdminStore) ExpandedMenuIDs(_ context.Context, menuIDs []int) ([]int, error) {
	s.expandedInput = append([]int{}, menuIDs...)
	if s.expandedOutput != nil {
		return s.expandedOutput, nil
	}
	return menuIDs, nil
}

func (s *fakeRoleAdminStore) ReplaceRoleMenus(_ context.Context, roleID int, menuIDs []int) error {
	s.replacedRoleID = roleID
	s.replacedMenuIDs = append([]int{}, menuIDs...)
	return nil
}
