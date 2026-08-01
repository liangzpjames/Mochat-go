package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	if store.lastSummaryCorpID != 5 {
		t.Fatalf("corpID = %d", store.lastSummaryCorpID)
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
	if store.lastLineCorpID != 5 {
		t.Fatalf("corpID = %d", store.lastLineCorpID)
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
	if store.lastSummaryCorpID != 0 || store.lastLineCorpID != 0 {
		t.Fatalf("store queried for summary=%d line=%d", store.lastSummaryCorpID, store.lastLineCorpID)
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
	if store.lastSummaryCorpID != 5 {
		t.Fatalf("corpID = %d", store.lastSummaryCorpID)
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
	if store.lastSummaryCorpID != 0 || store.lastLineCorpID != 0 {
		t.Fatalf("store queried for summary=%d line=%d", store.lastSummaryCorpID, store.lastLineCorpID)
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
	if store.lastSummaryCorpID != 0 || store.lastLineCorpID != 0 {
		t.Fatalf("store queried for summary=%d line=%d", store.lastSummaryCorpID, store.lastLineCorpID)
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

type fakeCorpDataStore struct {
	users             map[int]User
	corpIDsByTenant   []int
	corpIDsByUser     []int
	firstCorpID       int
	firstEmployeeID   int
	summary           CorpDataSummary
	points            []CorpDataPoint
	lastSummaryCorpID int
	lastSummaryTime   time.Time
	lastLineCorpID    int
	lastLineFrom      time.Time
	lastLineTo        time.Time
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

func (s *fakeCorpDataStore) CorpDataSummary(_ context.Context, corpID int, now time.Time) (CorpDataSummary, error) {
	s.lastSummaryCorpID = corpID
	s.lastSummaryTime = now
	return s.summary, nil
}

func (s *fakeCorpDataStore) CorpDataLineChat(_ context.Context, corpID int, from time.Time, to time.Time) ([]CorpDataPoint, error) {
	s.lastLineCorpID = corpID
	s.lastLineFrom = from
	s.lastLineTo = to
	return s.points, nil
}
