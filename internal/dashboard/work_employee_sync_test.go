package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWorkEmployeeSyncPullsDepartmentsUsersAndStoresWithoutRBAC(t *testing.T) {
	store := &fakeWorkReadStore{
		users:         map[int]User{1: {ID: 1, TenantID: 3}},
		corpIDsByUser: []int{7, 8},
		employeeSyncCredentials: []WorkEmployeeSyncCredential{
			{CorpID: 7, TenantID: 3, WXCorpID: "ww-one", EmployeeSecret: "employee-secret", ContactSecret: "contact-secret"},
			{CorpID: 8, TenantID: 3, WXCorpID: "ww-two", EmployeeSecret: "employee-secret-2", ContactSecret: "contact-secret-2"},
		},
	}
	client := &fakeWorkEmployeeSyncClient{
		departments: map[int][]WorkEmployeeSyncDepartment{
			7: {
				{WXDepartmentID: 1, Name: "总部", WXParentID: 0, Order: 100},
				{WXDepartmentID: 2, Name: "销售", WXParentID: 1, Order: 90},
			},
			8: {
				{WXDepartmentID: 1, Name: "分部", WXParentID: 0, Order: 80},
			},
		},
		users: map[int]map[int][]WorkEmployeeSyncEmployee{
			7: {
				1: {{
					WXUserID:             "zhangsan",
					Name:                 "张三",
					Mobile:               "13800138000",
					Status:               1,
					WXMainDepartmentID:   2,
					DepartmentIDs:        []int{1, 2},
					IsLeaderInDepartment: []int{0, 1},
					DepartmentOrders:     []int{10, 20},
				}},
				2: {{
					WXUserID:             "zhangsan",
					Name:                 "张三",
					Mobile:               "13800138000",
					Status:               1,
					WXMainDepartmentID:   2,
					DepartmentIDs:        []int{2},
					IsLeaderInDepartment: []int{1},
					DepartmentOrders:     []int{20},
				}},
			},
			8: {
				1: {{WXUserID: "lisi", Name: "李四", Mobile: "13900139000", Status: 1, DepartmentIDs: []int{1}}},
			},
		},
		followUsers: map[int][]string{7: {"zhangsan"}, 8: {"lisi"}},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-88"), HeaderUserIDResolver{}, "", authorizer).
		WithWorkEmployeeSyncClient(client, "secret")

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/workEmployee/synEmployee", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkEmployeeSync(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "" {
		t.Fatalf("sync route should not call RBAC authorizer, got %#v", authorizer)
	}
	if store.lastCorpIDsByUserID != 1 {
		t.Fatalf("CorpIDsByUser user id = %d", store.lastCorpIDsByUserID)
	}
	if got := store.lastEmployeeSyncCredentialCorpIDs; len(got) != 2 || got[0] != 7 || got[1] != 8 {
		t.Fatalf("credential corp ids = %#v", got)
	}
	if len(client.departmentCorpIDs) != 2 || client.departmentCorpIDs[0] != 7 || client.departmentCorpIDs[1] != 8 {
		t.Fatalf("department corp calls = %#v", client.departmentCorpIDs)
	}
	if len(store.lastEmployeeSyncDepartments) != 1 || store.lastEmployeeSyncDepartments[0].Name != "分部" {
		t.Fatalf("last departments = %#v", store.lastEmployeeSyncDepartments)
	}
	if len(store.lastEmployeeSyncEmployees) != 1 || store.lastEmployeeSyncEmployees[0].WXUserID != "lisi" {
		t.Fatalf("last employees = %#v", store.lastEmployeeSyncEmployees)
	}
	if len(store.lastEmployeeSyncFollowUsers) != 1 || store.lastEmployeeSyncFollowUsers[0] != "lisi" {
		t.Fatalf("follow users = %#v", store.lastEmployeeSyncFollowUsers)
	}
	if store.lastEmployeeSyncPasswordHash == "" || !strings.HasPrefix(store.lastEmployeeSyncPasswordHash, "$2y$") {
		t.Fatalf("password hash was not generated")
	}
}

func TestWorkEmployeeSyncRequiresCorpCredential(t *testing.T) {
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
		employeeSyncCredentials: []WorkEmployeeSyncCredential{
			{CorpID: 7, TenantID: 3, WXCorpID: "ww-one"},
		},
	}
	handler := NewWorkReadHandler(store, staticAdminCache("7-88"), HeaderUserIDResolver{}, "").
		WithWorkEmployeeSyncClient(&fakeWorkEmployeeSyncClient{}, "secret")

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/workEmployee/synEmployee", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkEmployeeSync(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "企业授权信息错误" {
		t.Fatalf("msg = %#v", body["msg"])
	}
}

func TestWorkEmployeeSyncAllowsEmptyCorpScope(t *testing.T) {
	store := &fakeWorkReadStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}, corpIDsByUser: []int{}, corpIDsByTenant: []int{}}
	handler := NewWorkReadHandler(store, staticAdminCache(""), HeaderUserIDResolver{}, "").
		WithWorkEmployeeSyncClient(&fakeWorkEmployeeSyncClient{}, "secret")

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/workEmployee/synEmployee", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkEmployeeSync(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(store.lastEmployeeSyncCredentialCorpIDs) != 0 {
		t.Fatalf("credentials should not be requested, got %#v", store.lastEmployeeSyncCredentialCorpIDs)
	}
}

type fakeWorkEmployeeSyncClient struct {
	departments       map[int][]WorkEmployeeSyncDepartment
	users             map[int]map[int][]WorkEmployeeSyncEmployee
	followUsers       map[int][]string
	departmentCorpIDs []int
	userCalls         []int
}

func (c *fakeWorkEmployeeSyncClient) Departments(_ context.Context, credential WorkEmployeeSyncCredential) ([]WorkEmployeeSyncDepartment, error) {
	c.departmentCorpIDs = append(c.departmentCorpIDs, credential.CorpID)
	return c.departments[credential.CorpID], nil
}

func (c *fakeWorkEmployeeSyncClient) DepartmentUsers(_ context.Context, credential WorkEmployeeSyncCredential, wxDepartmentID int) ([]WorkEmployeeSyncEmployee, error) {
	c.userCalls = append(c.userCalls, wxDepartmentID)
	if c.users[credential.CorpID] == nil {
		return []WorkEmployeeSyncEmployee{}, nil
	}
	return c.users[credential.CorpID][wxDepartmentID], nil
}

func (c *fakeWorkEmployeeSyncClient) FollowUsers(_ context.Context, credential WorkEmployeeSyncCredential) ([]string, error) {
	return c.followUsers[credential.CorpID], nil
}

var _ WorkEmployeeSyncClient = (*fakeWorkEmployeeSyncClient)(nil)
