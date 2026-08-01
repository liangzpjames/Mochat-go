package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestCorpDataIndexReturnsSummary(t *testing.T) {
	store := &fakeCorpDataStore{
		users:           map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		corpIDsByTenant: []int{5},
		summary: CorpDataSummary{
			WeChatContactNum:          10,
			WeChatRoomNum:             2,
			RoomMemberNum:             30,
			CorpMemberNum:             4,
			AddContactNum:             5,
			LastAddContactNum:         3,
			AddFriendsNum:             20,
			LastAddFriendsNum:         18,
			MonthAddRoomNum:           6,
			LastMonthAddRoomNum:       4,
			MonthAddRoomMemberNum:     9,
			LastMonthAddRoomMemberNum: 7,
			MonthLossContactNum:       1,
			LastMonthLossContactNum:   2,
			UpdateTime:                "2026-07-02 12:00:00",
		},
		points: []CorpDataPoint{
			{ID: 1, AddContactNum: 2, AddIntoRoomNum: 3, LossContactNum: 1, QuitRoomNum: 0, Date: "2026-07-01 00:00:00"},
		},
	}
	handler := NewCorpDataHandler(store, staticAdminCache("5-9"), HeaderUserIDResolver{})
	handler.now = func() time.Time { return time.Date(2026, 7, 2, 12, 0, 0, 0, time.Local) }

	req := httptest.NewRequest(
		http.MethodGet,
		"/dashboard/corpData/index?corpId=5&startDate=2026-07-01&endDate=2026-07-02&employeeIds=&departmentIds=&period=day&page=1&pageSize=20",
		nil,
	)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastSummaryScope.CorpID != 5 || store.lastSummaryScope.TenantID != 10 {
		t.Fatalf("scope = %#v", store.lastSummaryScope)
	}
	if got := store.lastSummaryTime.Format("2006-01-02"); got != "2026-07-02" {
		t.Fatalf("summary date = %s", got)
	}
	if got := store.lastLineFrom.Format("2006-01-02"); got != "2026-07-01" {
		t.Fatalf("line from = %s", got)
	}
	if got := store.lastLineTo.Format("2006-01-02"); got != "2026-07-02" {
		t.Fatalf("line to = %s", got)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if int(data["weChatContactNum"].(float64)) != 10 || int(data["addFriendsNum"].(float64)) != 20 || data["updateTime"] != "2026-07-02 12:00:00" {
		t.Fatalf("data = %#v", data)
	}
	cards := data["cards"].([]any)
	if len(cards) != 4 {
		t.Fatalf("cards = %#v", cards)
	}
	firstCard := cards[0].(map[string]any)
	if firstCard["key"] != "contacts" || int(firstCard["value"].(float64)) != 10 {
		t.Fatalf("first card = %#v", firstCard)
	}
	trend := data["trend"].([]any)
	if len(trend) != 1 {
		t.Fatalf("trend = %#v", trend)
	}
	point := trend[0].(map[string]any)
	if point["date"] != "2026-07-01" || int(point["addContactNum"].(float64)) != 2 {
		t.Fatalf("trend point = %#v", point)
	}
	if data["updatedAt"] != "2026-07-02 12:00:00" {
		t.Fatalf("updatedAt = %#v", data["updatedAt"])
	}
}

func TestCorpDataLineChatReturnsPoints(t *testing.T) {
	store := &fakeCorpDataStore{
		users:           map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		corpIDsByTenant: []int{5},
		points: []CorpDataPoint{
			{ID: 1, AddContactNum: 2, AddIntoRoomNum: 3, LossContactNum: 1, QuitRoomNum: 0, Date: "2026-07-02 00:00:00"},
		},
	}
	handler := NewCorpDataHandler(store, staticAdminCache("5-9"), HeaderUserIDResolver{})

	req := httptest.NewRequest(
		http.MethodGet,
		"/dashboard/corpData/lineChat?corpId=5&startDate=2026-07-01&endDate=2026-07-02&employeeIds=&departmentIds=&period=day&page=1&pageSize=20",
		nil,
	)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.LineChat(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastLineScope.CorpID != 5 || store.lastLineScope.TenantID != 10 {
		t.Fatalf("scope = %#v", store.lastLineScope)
	}
	if got := store.lastLineFrom.Format("2006-01-02"); got != "2026-07-01" {
		t.Fatalf("from = %s", got)
	}
	if got := store.lastLineTo.Format("2006-01-02"); got != "2026-07-02" {
		t.Fatalf("to = %s", got)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	point := data[0].(map[string]any)
	if int(point["addContactNum"].(float64)) != 2 || point["date"] != "2026-07-02 00:00:00" {
		t.Fatalf("point = %#v", point)
	}
}

func TestCorpDataIndexRequiresSelectedCorp(t *testing.T) {
	store := &fakeCorpDataStore{
		users: map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
	}
	handler := NewCorpDataHandler(store, staticAdminCache(""), HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/corpData/index?startDate=2026-07-01&endDate=2026-07-02&employeeIds=&departmentIds=&period=day&page=1&pageSize=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestCorpDataIndexRequiresCompleteOverviewQuery(t *testing.T) {
	store := &fakeCorpDataStore{
		users:           map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		corpIDsByTenant: []int{5},
	}
	handler := NewCorpDataHandler(store, staticAdminCache("5-9"), HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/corpData/index?corpId=5&startDate=2026-07-01&endDate=2026-07-02", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastSummaryScope.CorpID != 0 || store.lastLineScope.CorpID != 0 {
		t.Fatalf("store queried for summary=%#v line=%#v", store.lastSummaryScope, store.lastLineScope)
	}
}

func TestCorpDataIndexIgnoresCrossTenantCachedCorp(t *testing.T) {
	store := &fakeCorpDataStore{
		users:           map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		corpIDsByTenant: []int{5},
		firstCorpID:     5,
		firstEmployeeID: 9,
		summary: CorpDataSummary{
			WeChatContactNum: 10,
			AddFriendsNum:    20,
		},
	}
	handler := NewCorpDataHandler(store, staticAdminCache("99-0"), HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/corpData/index?startDate=2026-07-01&endDate=2026-07-02&employeeIds=&departmentIds=&period=day&page=1&pageSize=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastSummaryScope.CorpID != 5 || store.lastSummaryScope.TenantID != 10 {
		t.Fatalf("scope = %#v", store.lastSummaryScope)
	}
}

func TestCorpDataIndexRejectsRequestedCorpOutsideSelectedScope(t *testing.T) {
	store := &fakeCorpDataStore{
		users:           map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		corpIDsByTenant: []int{5},
	}
	handler := NewCorpDataHandler(store, staticAdminCache("5-9"), HeaderUserIDResolver{})

	req := httptest.NewRequest(
		http.MethodGet,
		"/dashboard/corpData/index?corpId=99&startDate=2026-07-01&endDate=2026-07-02&employeeIds=&departmentIds=&period=day&page=1&pageSize=20",
		nil,
	)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastSummaryScope.CorpID != 0 || store.lastLineScope.CorpID != 0 {
		t.Fatalf("store queried for summary=%#v line=%#v", store.lastSummaryScope, store.lastLineScope)
	}
}

func TestCorpDataIndexRejectsInvalidDateRange(t *testing.T) {
	store := &fakeCorpDataStore{
		users:           map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		corpIDsByTenant: []int{5},
	}
	handler := NewCorpDataHandler(store, staticAdminCache("5-9"), HeaderUserIDResolver{})

	req := httptest.NewRequest(
		http.MethodGet,
		"/dashboard/corpData/index?corpId=5&startDate=2026-07-03&endDate=2026-07-02&employeeIds=&departmentIds=&period=day&page=1&pageSize=20",
		nil,
	)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastSummaryScope.CorpID != 0 || store.lastLineScope.CorpID != 0 {
		t.Fatalf("store queried for summary=%#v line=%#v", store.lastSummaryScope, store.lastLineScope)
	}
}

func TestCorpDataIndexReturnsEmptyArraysForEmptyCorpData(t *testing.T) {
	store := &fakeCorpDataStore{
		users:           map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		corpIDsByTenant: []int{5},
	}
	handler := NewCorpDataHandler(store, staticAdminCache("5-9"), HeaderUserIDResolver{})

	req := httptest.NewRequest(
		http.MethodGet,
		"/dashboard/corpData/index?corpId=5&startDate=2026-07-01&endDate=2026-07-02&employeeIds=&departmentIds=&period=day&page=1&pageSize=20",
		nil,
	)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if cards := data["cards"].([]any); len(cards) != 0 {
		t.Fatalf("cards = %#v", cards)
	}
	if trend := data["trend"].([]any); len(trend) != 0 {
		t.Fatalf("trend = %#v", trend)
	}
}

func TestCorpDataIndexGroupsWeeklyTrendAndReturnsPaginationMetadata(t *testing.T) {
	store := &fakeCorpDataStore{
		users:           map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		corpIDsByTenant: []int{5},
		points: []CorpDataPoint{
			{AddContactNum: 2, AddIntoRoomNum: 1, LossContactNum: 1, Date: "2026-07-06 00:00:00"},
			{AddContactNum: 3, AddIntoRoomNum: 2, LossContactNum: 1, Date: "2026-07-08 00:00:00"},
		},
	}
	handler := NewCorpDataHandler(store, staticAdminCache("5-9"), HeaderUserIDResolver{})
	req := httptest.NewRequest(http.MethodGet, "/dashboard/corpData/index?corpId=5&startDate=2026-07-01&endDate=2026-07-31&employeeIds=&departmentIds=&period=week&page=1&pageSize=1", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	data := decodeBody(t, rec.Body.Bytes())["data"].(map[string]any)
	if int(data["total"].(float64)) != 1 || int(data["page"].(float64)) != 1 || int(data["pageSize"].(float64)) != 1 {
		t.Fatalf("pagination = %#v", data)
	}
	trend := data["trend"].([]any)
	if len(trend) != 1 {
		t.Fatalf("trend = %#v", trend)
	}
	point := trend[0].(map[string]any)
	if point["date"] != "2026-07-06" || int(point["addContactNum"].(float64)) != 5 || int(point["addIntoRoomNum"].(float64)) != 3 {
		t.Fatalf("weekly point = %#v", point)
	}
}

func TestCorpDataIndexPassesProductionRBACIntersectionAndDepartmentScope(t *testing.T) {
	store := &fakeCorpDataStore{
		users:         map[int]User{1: {ID: 1, TenantID: 10}},
		corpIDsByUser: []int{5},
	}
	authorizer := NewRBACResolver(&fakeRBACStore{
		user:           User{ID: 1, TenantID: 10},
		roles:          []Role{{ID: 8, DataPermission: `[{"corpId":5,"permissionType":1}]`}},
		menu:           Menu{ID: 20, DataPermission: DataPermissionDepartment},
		menuOK:         true,
		roleMenu:       []RoleMenu{{RoleID: 8, MenuID: 20}},
		deptEmployeeID: []int{3, 4, 5},
	})
	handler := NewCorpDataHandler(store, staticAdminCache("5-9"), HeaderUserIDResolver{}, authorizer)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/corpData/index?corpId=5&startDate=2026-07-01&endDate=2026-07-02&employeeIds=4&employeeIds=7&departmentIds=12&period=day&page=1&pageSize=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	want := CorpDataScope{TenantID: 10, CorpID: 5, EmployeeIDs: []int{4}, DepartmentIDs: []int{12}, EmployeeScopeRestricted: true}
	if !reflect.DeepEqual(store.lastSummaryScope, want) {
		t.Fatalf("summary scope = %#v, want %#v", store.lastSummaryScope, want)
	}
	if !reflect.DeepEqual(store.lastLineScope, want) {
		t.Fatalf("trend scope = %#v, want %#v", store.lastLineScope, want)
	}
}

func TestCorpDataIndexPreservesEmptyProductionRBACScope(t *testing.T) {
	store := &fakeCorpDataStore{
		users:         map[int]User{1: {ID: 1, TenantID: 10}},
		corpIDsByUser: []int{5},
	}
	authorizer := NewRBACResolver(&fakeRBACStore{
		user:     User{ID: 1, TenantID: 10},
		roles:    []Role{{ID: 8, DataPermission: `[{"corpId":5,"permissionType":2}]`}},
		menu:     Menu{ID: 20, DataPermission: DataPermissionDepartment},
		menuOK:   true,
		roleMenu: []RoleMenu{{RoleID: 8, MenuID: 20}},
	})
	handler := NewCorpDataHandler(store, staticAdminCache("5-0"), HeaderUserIDResolver{}, authorizer)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/corpData/index?corpId=5&startDate=2026-07-01&endDate=2026-07-02&employeeIds=&departmentIds=&period=day&page=1&pageSize=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !store.lastSummaryScope.EmployeeScopeRestricted || len(store.lastSummaryScope.EmployeeIDs) != 0 {
		t.Fatalf("summary scope = %#v", store.lastSummaryScope)
	}
	if !store.lastLineScope.EmployeeScopeRestricted || len(store.lastLineScope.EmployeeIDs) != 0 {
		t.Fatalf("trend scope = %#v", store.lastLineScope)
	}
}

func TestCorpDataIndexUsesConfiguredTimezoneAtUTCDateBoundary(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	store := &fakeCorpDataStore{
		users:           map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		corpIDsByTenant: []int{5},
	}
	handler := NewCorpDataHandler(store, staticAdminCache("5-9"), HeaderUserIDResolver{}).WithLocation(location)
	handler.now = func() time.Time { return time.Date(2026, 8, 1, 16, 30, 0, 0, time.UTC) }
	req := httptest.NewRequest(http.MethodGet, "/dashboard/corpData/index?corpId=5&startDate=2026-08-02&endDate=2026-08-02&employeeIds=&departmentIds=&period=day&page=1&pageSize=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if got := store.lastLineFrom.Location().String(); got != "Asia/Shanghai" {
		t.Fatalf("from location = %s", got)
	}
	if got := store.lastLineFrom.Unix(); got != time.Date(2026, 8, 2, 0, 0, 0, 0, location).Unix() {
		t.Fatalf("from unix = %d", got)
	}
}

func TestCorpDataIndexRejectsPageThatWouldOverflowOffset(t *testing.T) {
	store := &fakeCorpDataStore{
		users:           map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		corpIDsByTenant: []int{5},
	}
	handler := NewCorpDataHandler(store, staticAdminCache("5-9"), HeaderUserIDResolver{})
	req := httptest.NewRequest(http.MethodGet, "/dashboard/corpData/index?corpId=5&startDate=2026-07-01&endDate=2026-07-02&employeeIds=&departmentIds=&period=day&page=9223372036854775807&pageSize=100", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Index(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastSummaryScope.CorpID != 0 || store.lastLineScope.CorpID != 0 {
		t.Fatalf("store queried for summary=%#v line=%#v", store.lastSummaryScope, store.lastLineScope)
	}
}

func TestCorpDataAggregatePeriodGroupsCalendarMonths(t *testing.T) {
	points := []CorpDataPoint{
		{Date: "2026-07-31 23:59:59", AddContactNum: 2},
		{Date: "2026-08-01 00:00:00", AddContactNum: 3},
		{Date: "2026-08-31 23:59:59", AddContactNum: 5},
	}

	got := corpDataAggregatePeriod(points, "month")

	want := []CorpDataPoint{{Date: "2026-07", AddContactNum: 2}, {Date: "2026-08", AddContactNum: 8}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("monthly points = %#v, want %#v", got, want)
	}
}

type fakeCorpDataStore struct {
	users            map[int]User
	corpIDsByTenant  []int
	corpIDsByUser    []int
	firstCorpID      int
	firstEmployeeID  int
	summary          CorpDataSummary
	points           []CorpDataPoint
	lastSummaryScope CorpDataScope
	lastSummaryTime  time.Time
	lastLineScope    CorpDataScope
	lastLineFrom     time.Time
	lastLineTo       time.Time
}

func (s *fakeCorpDataStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeCorpDataStore) EmployeeIDByUserCorp(_ context.Context, userID int, corpID int) (int, error) {
	if s.firstCorpID == corpID {
		return s.firstEmployeeID, nil
	}
	return 0, nil
}

func (s *fakeCorpDataStore) FirstEmployeeByUser(_ context.Context, userID int) (int, int, bool, error) {
	if s.firstCorpID == 0 {
		return 0, 0, false, nil
	}
	return s.firstCorpID, s.firstEmployeeID, true, nil
}

func (s *fakeCorpDataStore) CorpIDsByTenant(_ context.Context, tenantID int) ([]int, error) {
	return append([]int{}, s.corpIDsByTenant...), nil
}

func (s *fakeCorpDataStore) CorpIDsByUser(_ context.Context, userID int) ([]int, error) {
	return append([]int{}, s.corpIDsByUser...), nil
}

func (s *fakeCorpDataStore) CorpDataSummary(_ context.Context, scope CorpDataScope, now time.Time) (CorpDataSummary, error) {
	s.lastSummaryScope = scope
	s.lastSummaryTime = now
	return s.summary, nil
}

func (s *fakeCorpDataStore) CorpDataLineChat(_ context.Context, scope CorpDataScope, from time.Time, to time.Time) ([]CorpDataPoint, error) {
	s.lastLineScope = scope
	s.lastLineFrom = from
	s.lastLineTo = to
	return s.points, nil
}
