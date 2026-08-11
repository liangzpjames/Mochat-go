package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoomCalendarStorePreservesFormListPushJSON(t *testing.T) {
	store := &fakeRoomCalendarStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}, createID: 88}
	handler := NewRoomCalendarHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	body := `{
		"name":"社群促活日历",
		"rooms":[{"wxChatId":"wr001","name":"客户群"}],
		"form":{"name":"周一提醒","date":"2026-08-01","time":"09:30"},
		"list":[{"type":"text","content":"今日互动提醒"}]
	}`
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/roomCalendar/store", strings.NewReader(body))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.created.CorpID != 7 || store.created.CreateUserID != 1 || store.created.Name != "社群促活日历" || len(store.created.Pushes) != 1 {
		t.Fatalf("created = %#v", store.created)
	}
	if !strings.Contains(store.created.RoomsRaw, "wr001") || store.created.Pushes[0].Name != "周一提醒" || store.created.Pushes[0].Day != "2026-08-01 09:30" || !strings.Contains(store.created.Pushes[0].PushContentRaw, "今日互动提醒") {
		t.Fatalf("created json = %#v", store.created)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricRoomCalendars {
		t.Fatalf("refresh = tenant %d metric %s", store.refreshTenantID, store.refreshMetric)
	}
}

func TestRoomCalendarStoreRejectsSaaSQuotaExceeded(t *testing.T) {
	store := &fakeRoomCalendarStore{
		user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1},
		quota: SaaSQuotaStatus{
			Metric:     SaaSMetricRoomCalendars,
			TenantID:   8,
			Current:    1,
			Limit:      1,
			Additional: 1,
		},
	}
	handler := NewRoomCalendarHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/roomCalendar/store", strings.NewReader(`{"name":"额度外群日历"}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Store(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "套餐额度已达上限：群日历数 1/1") {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if store.createCalls != 0 {
		t.Fatalf("create calls = %d", store.createCalls)
	}
	if store.quotaTenantID != 8 || store.quotaMetric != SaaSMetricRoomCalendars || store.quotaAdditional != 1 {
		t.Fatalf("quota check = tenant %d metric %s additional %d", store.quotaTenantID, store.quotaMetric, store.quotaAdditional)
	}
	if store.refreshMetric != "" {
		t.Fatalf("refresh should not run, got %s", store.refreshMetric)
	}
}

func TestRoomCalendarDestroyRefreshesSaaSUsage(t *testing.T) {
	store := &fakeRoomCalendarStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}}
	handler := NewRoomCalendarHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := authenticatedDashboardRequestForTest(http.MethodDelete, "/dashboard/roomCalendar/destroy", strings.NewReader(`{"roomCalendarId":88}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Destroy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.deletedID != 88 {
		t.Fatalf("deleted id = %d", store.deletedID)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricRoomCalendars {
		t.Fatalf("refresh = tenant %d metric %s", store.refreshTenantID, store.refreshMetric)
	}
}

func TestRoomCalendarAddRoomAcceptsScalarRoomID(t *testing.T) {
	store := &fakeRoomCalendarStore{user: User{ID: 1, IsSuperAdmin: 1}}
	handler := NewRoomCalendarHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/roomCalendar/addRoom", strings.NewReader(`{"id":12,"roomId":"wr002"}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.AddRoom(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.addRoomID != 12 || !strings.Contains(store.addRoomsRaw, "wr002") {
		t.Fatalf("add room id=%d raw=%s", store.addRoomID, store.addRoomsRaw)
	}
}

type fakeRoomCalendarStore struct {
	user            User
	page            RoomCalendarPage
	item            RoomCalendarItem
	itemFound       bool
	created         RoomCalendarWrite
	updated         RoomCalendarWrite
	createID        int
	createCalls     int
	quota           SaaSQuotaStatus
	quotaTenantID   int
	quotaMetric     string
	quotaAdditional int64
	refreshTenantID int
	refreshMetric   string
	deletedID       int
	addRoomID       int
	addRoomsRaw     string
	removeID        int
	removeKey       string
}

func (s *fakeRoomCalendarStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	if userID != s.user.ID {
		return User{}, false, nil
	}
	return s.user, true, nil
}

func (s *fakeRoomCalendarStore) EmployeeIDByUserCorp(context.Context, int, int) (int, error) {
	return 99, nil
}

func (s *fakeRoomCalendarStore) FirstEmployeeByUser(context.Context, int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeRoomCalendarStore) RoomCalendarPage(context.Context, RoomCalendarFilter) (RoomCalendarPage, error) {
	return s.page, nil
}

func (s *fakeRoomCalendarStore) RoomCalendarByID(context.Context, int, int) (RoomCalendarItem, bool, error) {
	return s.item, s.itemFound, nil
}

func (s *fakeRoomCalendarStore) CreateRoomCalendar(_ context.Context, values RoomCalendarWrite) (int, error) {
	s.createCalls++
	s.created = values
	if s.createID > 0 {
		return s.createID, nil
	}
	return 1, nil
}

func (s *fakeRoomCalendarStore) UpdateRoomCalendar(_ context.Context, _ int, _ int, values RoomCalendarWrite) (bool, error) {
	s.updated = values
	return true, nil
}

func (s *fakeRoomCalendarStore) DeleteRoomCalendar(_ context.Context, _ int, id int) (bool, error) {
	s.deletedID = id
	return true, nil
}

func (s *fakeRoomCalendarStore) AddRoomCalendarRooms(_ context.Context, _ int, id int, roomsRaw string) (bool, error) {
	s.addRoomID = id
	s.addRoomsRaw = roomsRaw
	return true, nil
}

func (s *fakeRoomCalendarStore) RemoveRoomCalendarRoom(_ context.Context, _ int, id int, roomKey string) (bool, error) {
	s.removeID = id
	s.removeKey = roomKey
	return true, nil
}

func (s *fakeRoomCalendarStore) SaaSQuotaStatus(_ context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error) {
	s.quotaTenantID = tenantID
	s.quotaMetric = metric
	s.quotaAdditional = additional
	if s.quota.Metric != "" {
		return s.quota, nil
	}
	return SaaSQuotaStatus{Metric: metric, TenantID: tenantID, Additional: additional}, nil
}

func (s *fakeRoomCalendarStore) RefreshSaaSUsageCounter(_ context.Context, tenantID int, metric string) error {
	s.refreshTenantID = tenantID
	s.refreshMetric = metric
	return nil
}
