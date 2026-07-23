package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoomRemindStoreParsesFlagsAndRooms(t *testing.T) {
	store := &fakeRoomRemindStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}, createID: 66}
	handler := NewRoomRemindHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	body := `{
		"name":"群违规提醒",
		"rooms":[{"wxChatId":"wr001","name":"客户群"}],
		"isQrcode":1,
		"isLink":1,
		"keyword":"报价",
		"status":1
	}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/roomRemind/store", strings.NewReader(body))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.created.CorpID != 7 || store.created.CreateUserID != 1 || store.created.Name != "群违规提醒" {
		t.Fatalf("created = %#v", store.created)
	}
	if !strings.Contains(store.created.RoomsRaw, "wr001") || store.created.IsQrcode != 1 || store.created.IsLink != 1 || store.created.IsKeyword != 1 || store.created.Keyword != "报价" {
		t.Fatalf("created values = %#v", store.created)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricRoomReminds {
		t.Fatalf("refresh = tenant %d metric %s", store.refreshTenantID, store.refreshMetric)
	}
}

func TestRoomRemindStoreRejectsSaaSQuotaExceeded(t *testing.T) {
	store := &fakeRoomRemindStore{
		user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1},
		quota: SaaSQuotaStatus{
			Metric:     SaaSMetricRoomReminds,
			TenantID:   8,
			Current:    1,
			Limit:      1,
			Additional: 1,
		},
	}
	handler := NewRoomRemindHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/roomRemind/store", strings.NewReader(`{"name":"额度外客户群提醒"}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Store(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "套餐额度已达上限：客户群提醒数 1/1") {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if store.createCalls != 0 {
		t.Fatalf("create calls = %d", store.createCalls)
	}
	if store.quotaTenantID != 8 || store.quotaMetric != SaaSMetricRoomReminds || store.quotaAdditional != 1 {
		t.Fatalf("quota check = tenant %d metric %s additional %d", store.quotaTenantID, store.quotaMetric, store.quotaAdditional)
	}
	if store.refreshMetric != "" {
		t.Fatalf("refresh should not run, got %s", store.refreshMetric)
	}
}

func TestRoomRemindDestroyRefreshesSaaSUsage(t *testing.T) {
	store := &fakeRoomRemindStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}}
	handler := NewRoomRemindHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodDelete, "/dashboard/roomRemind/destroy", strings.NewReader(`{"roomRemindId":88}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Destroy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.deletedID != 88 {
		t.Fatalf("deleted id = %d", store.deletedID)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricRoomReminds {
		t.Fatalf("refresh = tenant %d metric %s", store.refreshTenantID, store.refreshMetric)
	}
}

func TestRoomRemindStatusUsesGetQuery(t *testing.T) {
	store := &fakeRoomRemindStore{user: User{ID: 1, IsSuperAdmin: 1}}
	handler := NewRoomRemindHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/roomRemind/status?id=12&status=0", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Status(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.statusID != 12 || store.statusValue != 0 {
		t.Fatalf("status id=%d value=%d", store.statusID, store.statusValue)
	}
}

type fakeRoomRemindStore struct {
	user            User
	page            RoomRemindPage
	item            RoomRemindItem
	itemFound       bool
	created         RoomRemindWrite
	updated         RoomRemindWrite
	createID        int
	createCalls     int
	quota           SaaSQuotaStatus
	quotaTenantID   int
	quotaMetric     string
	quotaAdditional int64
	refreshTenantID int
	refreshMetric   string
	deletedID       int
	statusID        int
	statusValue     int
}

func (s *fakeRoomRemindStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	if userID != s.user.ID {
		return User{}, false, nil
	}
	return s.user, true, nil
}

func (s *fakeRoomRemindStore) EmployeeIDByUserCorp(context.Context, int, int) (int, error) {
	return 99, nil
}

func (s *fakeRoomRemindStore) FirstEmployeeByUser(context.Context, int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeRoomRemindStore) RoomRemindPage(context.Context, RoomRemindFilter) (RoomRemindPage, error) {
	return s.page, nil
}

func (s *fakeRoomRemindStore) RoomRemindByID(context.Context, int, int) (RoomRemindItem, bool, error) {
	return s.item, s.itemFound, nil
}

func (s *fakeRoomRemindStore) CreateRoomRemind(_ context.Context, values RoomRemindWrite) (int, error) {
	s.createCalls++
	s.created = values
	if s.createID > 0 {
		return s.createID, nil
	}
	return 1, nil
}

func (s *fakeRoomRemindStore) UpdateRoomRemind(_ context.Context, _ int, _ int, values RoomRemindWrite) (bool, error) {
	s.updated = values
	return true, nil
}

func (s *fakeRoomRemindStore) UpdateRoomRemindStatus(_ context.Context, _ int, id int, status int) (bool, error) {
	s.statusID = id
	s.statusValue = status
	return true, nil
}

func (s *fakeRoomRemindStore) DeleteRoomRemind(_ context.Context, _ int, id int) (bool, error) {
	s.deletedID = id
	return true, nil
}

func (s *fakeRoomRemindStore) SaaSQuotaStatus(_ context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error) {
	s.quotaTenantID = tenantID
	s.quotaMetric = metric
	s.quotaAdditional = additional
	if s.quota.Metric != "" {
		return s.quota, nil
	}
	return SaaSQuotaStatus{Metric: metric, TenantID: tenantID, Additional: additional}, nil
}

func (s *fakeRoomRemindStore) RefreshSaaSUsageCounter(_ context.Context, tenantID int, metric string) error {
	s.refreshTenantID = tenantID
	s.refreshMetric = metric
	return nil
}
