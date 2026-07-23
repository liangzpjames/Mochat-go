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
	}
	handler := NewCorpDataHandler(store, staticAdminCache("5-9"), HeaderUserIDResolver{})
	handler.now = func() time.Time { return time.Date(2026, 7, 2, 12, 0, 0, 0, time.Local) }

	req := httptest.NewRequest(http.MethodGet, "/dashboard/corpData/index", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastSummaryCorpID != 5 {
		t.Fatalf("corpID = %d", store.lastSummaryCorpID)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if int(data["weChatContactNum"].(float64)) != 10 || int(data["addFriendsNum"].(float64)) != 20 || data["updateTime"] != "2026-07-02 12:00:00" {
		t.Fatalf("data = %#v", data)
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

	req := httptest.NewRequest(http.MethodGet, "/dashboard/corpData/lineChat", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.LineChat(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastLineCorpID != 5 {
		t.Fatalf("corpID = %d", store.lastLineCorpID)
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

	req := httptest.NewRequest(http.MethodGet, "/dashboard/corpData/index", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
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

	req := httptest.NewRequest(http.MethodGet, "/dashboard/corpData/index", nil)
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

type fakeCorpDataStore struct {
	users             map[int]User
	corpIDsByTenant   []int
	corpIDsByUser     []int
	firstCorpID       int
	firstEmployeeID   int
	summary           CorpDataSummary
	points            []CorpDataPoint
	lastSummaryCorpID int
	lastLineCorpID    int
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
	return s.summary, nil
}

func (s *fakeCorpDataStore) CorpDataLineChat(_ context.Context, corpID int, now time.Time) ([]CorpDataPoint, error) {
	s.lastLineCorpID = corpID
	return s.points, nil
}
