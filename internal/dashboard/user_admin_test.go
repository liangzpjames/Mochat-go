package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/authjwt"
)

func TestUserAdminIndexReturnsCountsRolesAndDepartments(t *testing.T) {
	store := &fakeUserAdminStore{
		users:         map[int]User{1: {ID: 1, TenantID: 8, IsSuperAdmin: 1}},
		tenantUserIDs: []int{1, 2},
		statusCounts:  UserAdminStatusCounts{NotEnabled: 1, Normal: 1},
		page: UserAdminPage{
			Items: []UserAdminItem{{
				ID:           2,
				Name:         "张三",
				Phone:        "13800000000",
				Gender:       1,
				Status:       1,
				TenantID:     8,
				IsSuperAdmin: 0,
				CreatedAt:    "2026-07-03 10:00:00",
			}},
			Total:     1,
			TotalPage: 1,
			PerPage:   10,
		},
		roles: map[int]UserAdminRoleInfo{2: {RoleID: 3, RoleName: "运营"}},
		departments: map[int][]UserAdminDepartment{2: {
			{DepartmentID: 5, DepartmentName: "销售部"},
		}},
	}
	handler := NewUserAdminHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, nil, "secret", nil, authjwt.Parser{}, 0)

	rec := performAuthenticatedDashboardRequest(handler.Index, "GET", "/dashboard/user/index?phone=138&page=1&perPage=10&status=1", nil, map[string]string{"X-Mochat-Go-User-ID": "1"})
	if rec.Code != 200 {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !reflect.DeepEqual(store.statusCountIDs, []int{1, 2}) {
		t.Fatalf("status count ids = %+v", store.statusCountIDs)
	}
	if store.lastFilter.TenantID != 8 || store.lastFilter.Phone != "138" || store.lastFilter.Status == nil || *store.lastFilter.Status != 1 || !reflect.DeepEqual(store.lastFilter.UserIDs, []int{1, 2}) {
		t.Fatalf("filter = %+v", store.lastFilter)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if data["normalNum"].(float64) != 1 || data["notEnabledNum"].(float64) != 1 {
		t.Fatalf("counts = %#v", data)
	}
	item := data["list"].([]any)[0].(map[string]any)
	if item["roleName"] != "运营" || item["statusText"] != "正常" {
		t.Fatalf("item = %#v", item)
	}
	department := item["department"].([]any)[0].(map[string]any)
	if department["departmentName"] != "销售部" {
		t.Fatalf("department = %#v", department)
	}
}

func TestUserAdminStoreHashesPasswordAndSyncsRole(t *testing.T) {
	store := &fakeUserAdminStore{
		users:     map[int]User{1: {ID: 1, TenantID: 8, IsSuperAdmin: 1}},
		createdID: 10,
	}
	handler := NewUserAdminHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, nil, "secret", nil, authjwt.Parser{}, 0)

	body := `{"userName":"李四","phone":"13900000000","gender":2,"status":1,"roleId":4,"password":"abc123","department":"5"}`
	rec := performAuthenticatedDashboardRequest(handler.Store, "POST", "/dashboard/user/store", strings.NewReader(body), map[string]string{
		"Content-Type":        "application/json",
		"X-Mochat-Go-User-ID": "1",
	})
	if rec.Code != 200 {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if store.createdCorpID != 7 || store.created.Name != "李四" || store.created.RoleID != 4 || store.created.Department != "5" {
		t.Fatalf("created = %+v corpID=%d", store.created, store.createdCorpID)
	}
	if store.created.Password == "abc123" || !authjwt.CheckPasswordHash("secret", "abc123", store.created.Password) {
		t.Fatalf("password was not PHP-JWT-compatible hashed: %q", store.created.Password)
	}
}

func TestUserAdminStoreRejectsSaaSUserQuotaExceeded(t *testing.T) {
	base := &fakeUserAdminStore{
		users: map[int]User{1: {ID: 1, TenantID: 8, IsSuperAdmin: 1}},
	}
	store := &fakeUserAdminQuotaStore{
		fakeUserAdminStore: base,
		quota:              SaaSQuotaStatus{Metric: SaaSMetricUsers, TenantID: 8, Current: 10, Limit: 10, Additional: 1},
	}
	handler := NewUserAdminHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, nil, "secret", nil, authjwt.Parser{}, 0)

	body := `{"userName":"李四","phone":"13900000000","gender":2,"status":1,"roleId":4,"password":"abc123","department":"5"}`
	rec := performAuthenticatedDashboardRequest(handler.Store, "POST", "/dashboard/user/store", strings.NewReader(body), map[string]string{
		"Content-Type":        "application/json",
		"X-Mochat-Go-User-ID": "1",
	})
	if rec.Code != 400 {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	decoded := decodeBody(t, rec.Body.Bytes())
	if decoded["msg"] != "套餐额度已达上限：子账号数 10/10" {
		t.Fatalf("body = %#v", decoded)
	}
	if store.created.Phone != "" {
		t.Fatalf("CreateUserAdmin should not run")
	}
}

func TestUserAdminStoreRefreshesSaaSUserUsage(t *testing.T) {
	base := &fakeUserAdminStore{
		users:     map[int]User{1: {ID: 1, TenantID: 8, IsSuperAdmin: 1}},
		createdID: 10,
	}
	store := &fakeUserAdminQuotaStore{
		fakeUserAdminStore: base,
		quota:              SaaSQuotaStatus{Metric: SaaSMetricUsers, TenantID: 8, Current: 9, Limit: 10, Additional: 1},
	}
	handler := NewUserAdminHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, nil, "secret", nil, authjwt.Parser{}, 0)

	body := `{"userName":"李四","phone":"13900000000","gender":2,"status":1,"roleId":4,"password":"abc123","department":"5"}`
	rec := performAuthenticatedDashboardRequest(handler.Store, "POST", "/dashboard/user/store", strings.NewReader(body), map[string]string{
		"Content-Type":        "application/json",
		"X-Mochat-Go-User-ID": "1",
	})
	if rec.Code != 200 {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if store.quotaMetric != SaaSMetricUsers || store.refreshMetric != SaaSMetricUsers {
		t.Fatalf("quota metric=%q refresh=%q", store.quotaMetric, store.refreshMetric)
	}
}

func TestUserAdminStatusUpdateRejectsRepeatedStatus(t *testing.T) {
	store := &fakeUserAdminStore{
		users:      map[int]User{1: {ID: 1, TenantID: 8, IsSuperAdmin: 1}},
		itemsByIDs: []UserAdminItem{{ID: 2, Name: "张三", Status: 1}},
	}
	handler := NewUserAdminHandler(store, nil, HeaderUserIDResolver{}, nil, "secret", nil, authjwt.Parser{}, 0)

	rec := performAuthenticatedDashboardRequest(handler.StatusUpdate, "PUT", "/dashboard/user/statusUpdate", strings.NewReader(`{"userId":"2","status":1}`), map[string]string{
		"Content-Type":        "application/json",
		"X-Mochat-Go-User-ID": "1",
	})
	if rec.Code != 400 {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if store.statusUpdated != 0 {
		t.Fatalf("status update should not run")
	}
	body := decodeBody(t, rec.Body.Bytes())
	if !strings.Contains(body["msg"].(string), "张三当前状态：正常") {
		t.Fatalf("msg = %v", body["msg"])
	}
}

func TestUserAdminPasswordUpdateChecksOldPassword(t *testing.T) {
	oldHash, err := authjwt.GeneratePasswordHash("secret", "old123")
	if err != nil {
		t.Fatal(err)
	}
	store := &fakeUserAdminStore{
		users:      map[int]User{1: {ID: 1, TenantID: 8, IsSuperAdmin: 1}},
		adminItems: map[int]UserAdminItem{1: {ID: 1, Name: "管理员", Status: 1}},
		authUsers:  map[int]AuthUser{1: {ID: 1, Status: 1, Password: oldHash}},
	}
	handler := NewUserAdminHandler(store, nil, HeaderUserIDResolver{}, nil, "secret", nil, authjwt.Parser{}, 0)

	rec := performAuthenticatedDashboardRequest(handler.PasswordUpdate, "PUT", "/dashboard/user/passwordUpdate", strings.NewReader(`{"oldPassword":"old123","newPassword":"new456","againNewPassword":"new456"}`), map[string]string{
		"Content-Type":        "application/json",
		"X-Mochat-Go-User-ID": "1",
	})
	if rec.Code != 200 {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if store.passwordUpdatedID != 1 || !authjwt.CheckPasswordHash("secret", "new456", store.passwordHash) {
		t.Fatalf("password update = id %d hash %q", store.passwordUpdatedID, store.passwordHash)
	}
}

func TestUserAdminRejectsOrdinaryUserBeforeManagementQueries(t *testing.T) {
	store := &fakeUserAdminStore{users: map[int]User{2: {ID: 2, TenantID: 8}}}
	handler := NewUserAdminHandler(store, nil, HeaderUserIDResolver{}, nil, "secret", nil, authjwt.Parser{}, 0)
	req := authenticatedDashboardRequestForTestAs(http.MethodGet, "/dashboard/user/index", nil, 2, 8, 7, 99)
	req.Header.Set("X-Mochat-Go-User-ID", "2")
	response := httptest.NewRecorder()
	handler.Index(response, req)
	if response.Code != http.StatusForbidden || machineCode(t, response) != DashboardPermissionDeniedCode {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if len(store.statusCountIDs) != 0 || store.lastFilter.TenantID != 0 {
		t.Fatalf("management queries ran: statusIDs=%v filter=%+v", store.statusCountIDs, store.lastFilter)
	}
}

type fakeUserAdminStore struct {
	users              map[int]User
	adminItems         map[int]UserAdminItem
	authUsers          map[int]AuthUser
	tenantUserIDs      []int
	corpLogUserIDs     []int
	employeeLogUserIDs []int
	statusCounts       UserAdminStatusCounts
	statusCountIDs     []int
	page               UserAdminPage
	lastFilter         UserAdminFilter
	roles              map[int]UserAdminRoleInfo
	departments        map[int][]UserAdminDepartment
	idsByPhone         []int
	created            UserAdminWrite
	createdCorpID      int
	createdID          int
	updatedID          int
	updated            UserAdminWrite
	updatedCorpID      int
	itemsByIDs         []UserAdminItem
	statusUpdatedIDs   []int
	statusUpdated      int
	passwordUpdatedID  int
	passwordHash       string
}

func (s *fakeUserAdminStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeUserAdminStore) UserAuthByID(_ context.Context, userID int) (AuthUser, bool, error) {
	user, ok := s.authUsers[userID]
	return user, ok, nil
}

func (s *fakeUserAdminStore) EmployeeIDByUserCorp(_ context.Context, _ int, _ int) (int, error) {
	return 99, nil
}

func (s *fakeUserAdminStore) FirstEmployeeByUser(_ context.Context, _ int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeUserAdminStore) UserAdminIDsByTenant(_ context.Context, _ int) ([]int, error) {
	return s.tenantUserIDs, nil
}

func (s *fakeUserAdminStore) UserAdminLogUserIDsByCorp(_ context.Context, _ int) ([]int, error) {
	return s.corpLogUserIDs, nil
}

func (s *fakeUserAdminStore) UserAdminLogUserIDsByEmployees(_ context.Context, _ []int) ([]int, error) {
	return s.employeeLogUserIDs, nil
}

func (s *fakeUserAdminStore) UserAdminStatusCounts(_ context.Context, userIDs []int) (UserAdminStatusCounts, error) {
	s.statusCountIDs = append([]int{}, userIDs...)
	return s.statusCounts, nil
}

func (s *fakeUserAdminStore) UserAdminPage(_ context.Context, filter UserAdminFilter) (UserAdminPage, error) {
	s.lastFilter = filter
	return s.page, nil
}

func (s *fakeUserAdminStore) UserAdminByID(_ context.Context, userID int) (UserAdminItem, bool, error) {
	item, ok := s.adminItems[userID]
	return item, ok, nil
}

func (s *fakeUserAdminStore) UserAdminRolesByUserIDs(_ context.Context, _ []int) (map[int]UserAdminRoleInfo, error) {
	if s.roles == nil {
		return map[int]UserAdminRoleInfo{}, nil
	}
	return s.roles, nil
}

func (s *fakeUserAdminStore) UserAdminDepartmentsByUserIDs(_ context.Context, _ int, _ []int) (map[int][]UserAdminDepartment, error) {
	if s.departments == nil {
		return map[int][]UserAdminDepartment{}, nil
	}
	return s.departments, nil
}

func (s *fakeUserAdminStore) UserAdminIDsByPhone(_ context.Context, _ string) ([]int, error) {
	return s.idsByPhone, nil
}

func (s *fakeUserAdminStore) CreateUserAdmin(_ context.Context, values UserAdminWrite, corpID int) (int, error) {
	s.created = values
	s.createdCorpID = corpID
	return s.createdID, nil
}

func (s *fakeUserAdminStore) UpdateUserAdmin(_ context.Context, userID int, values UserAdminWrite, corpID int) (bool, error) {
	s.updatedID = userID
	s.updated = values
	s.updatedCorpID = corpID
	return true, nil
}

func (s *fakeUserAdminStore) UserAdminItemsByIDs(_ context.Context, _ []int) ([]UserAdminItem, error) {
	return s.itemsByIDs, nil
}

func (s *fakeUserAdminStore) UpdateUserAdminStatuses(_ context.Context, userIDs []int, status int) error {
	s.statusUpdatedIDs = append([]int{}, userIDs...)
	s.statusUpdated = status
	return nil
}

func (s *fakeUserAdminStore) UpdateUserAdminPassword(_ context.Context, userID int, passwordHash string) (bool, error) {
	s.passwordUpdatedID = userID
	s.passwordHash = passwordHash
	return true, nil
}

type fakeUserAdminQuotaStore struct {
	*fakeUserAdminStore
	quota         SaaSQuotaStatus
	quotaMetric   string
	refreshMetric string
}

func (s *fakeUserAdminQuotaStore) SaaSQuotaStatus(_ context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error) {
	s.quotaMetric = metric
	status := s.quota
	status.TenantID = tenantID
	status.Metric = metric
	status.Additional = additional
	return status, nil
}

func (s *fakeUserAdminQuotaStore) RefreshSaaSUsageCounter(_ context.Context, _ int, metric string) error {
	s.refreshMetric = metric
	return nil
}
