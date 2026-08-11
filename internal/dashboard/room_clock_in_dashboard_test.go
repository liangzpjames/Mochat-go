package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoomClockInShowContactKeepsZeroStatusFilters(t *testing.T) {
	store := &fakeRoomClockInStore{
		user: User{ID: 1, IsSuperAdmin: 1},
		contactPage: RoomClockInContactPage{
			Page:    1,
			PerPage: 15,
			Total:   1,
			Items: []RoomClockInContactItem{{
				ID:        9,
				ClockInID: 12,
				Nickname:  "未完成打卡客户",
				Status:    0,
				WriteOff:  0,
			}},
		},
	}
	handler := NewRoomClockInHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "")
	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/roomClockIn/showContact?clockInId=12&status=0&writeOff=0&page=1&perPage=15", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ShowContact(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastContactFilter.CorpID != 7 || store.lastContactFilter.ClockInID != 12 || store.lastContactFilter.Status != 0 || store.lastContactFilter.WriteOff != 0 {
		t.Fatalf("filter = %#v", store.lastContactFilter)
	}
	var envelope struct {
		Data struct {
			List []map[string]any `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.List) != 1 || envelope.Data.List[0]["nickname"] != "未完成打卡客户" {
		t.Fatalf("list = %#v", envelope.Data.List)
	}
}

func TestRoomClockInStorePreservesNestedJSON(t *testing.T) {
	store := &fakeRoomClockInStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}, createID: 88}
	handler := NewRoomClockInHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "https://op.example")
	body := `{
		"clockIn":{"official_account_id":3,"active_name":"群打卡活动","description":"连续打卡说明","type":1,"end_time":"2026-08-01 12:00:00","tasks":[{"count":7,"prize":"优惠券"}],"employee_qrcode":"/upload/qr.png","contact_tags":[11,12],"corp_card_status":1,"corp_card":{"name":"极义"}}
	}`
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/roomClockIn/store", strings.NewReader(body))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.created.CorpID != 7 || store.created.CreateUserID != 1 || store.created.ActiveName != "群打卡活动" || store.created.Type != 1 || store.created.CorpCardStatus != 1 {
		t.Fatalf("created = %#v", store.created)
	}
	if !strings.Contains(store.created.TasksRaw, "优惠券") || !strings.Contains(store.created.ContactTagsRaw, "11") || !strings.Contains(store.created.CorpCardRaw, "极义") {
		t.Fatalf("json raw = %#v", store.created)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricRoomClockIns {
		t.Fatalf("refresh = tenant %d metric %s", store.refreshTenantID, store.refreshMetric)
	}
}

func TestRoomClockInStoreRejectsSaaSQuotaExceeded(t *testing.T) {
	store := &fakeRoomClockInStore{
		user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1},
		quota: SaaSQuotaStatus{
			Metric:     SaaSMetricRoomClockIns,
			TenantID:   8,
			Current:    1,
			Limit:      1,
			Additional: 1,
		},
	}
	handler := NewRoomClockInHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "")
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/roomClockIn/store", strings.NewReader(`{"clockIn":{"active_name":"额度外群打卡"}}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Store(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "套餐额度已达上限：群打卡数 1/1") {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if store.createCalls != 0 {
		t.Fatalf("create calls = %d", store.createCalls)
	}
	if store.quotaTenantID != 8 || store.quotaMetric != SaaSMetricRoomClockIns || store.quotaAdditional != 1 {
		t.Fatalf("quota check = tenant %d metric %s additional %d", store.quotaTenantID, store.quotaMetric, store.quotaAdditional)
	}
	if store.refreshMetric != "" {
		t.Fatalf("refresh should not run, got %s", store.refreshMetric)
	}
}

func TestRoomClockInDestroyRefreshesSaaSUsage(t *testing.T) {
	store := &fakeRoomClockInStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}, deleteOK: true}
	handler := NewRoomClockInHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "")
	req := authenticatedDashboardRequestForTest(http.MethodDelete, "/dashboard/roomClockIn/destroy", strings.NewReader(`{"clockInId":88}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Destroy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.deletedID != 88 {
		t.Fatalf("deleted id = %d", store.deletedID)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricRoomClockIns {
		t.Fatalf("refresh = tenant %d metric %s", store.refreshTenantID, store.refreshMetric)
	}
}

type fakeRoomClockInStore struct {
	user              User
	page              RoomClockInPage
	item              RoomClockInItem
	itemFound         bool
	contactPage       RoomClockInContactPage
	dayPage           RoomClockInDayPage
	lastFilter        RoomClockInFilter
	lastContactFilter RoomClockInContactFilter
	created           RoomClockInWrite
	updated           RoomClockInWrite
	createID          int
	createCalls       int
	quota             SaaSQuotaStatus
	quotaTenantID     int
	quotaMetric       string
	quotaAdditional   int64
	refreshTenantID   int
	refreshMetric     string
	deleteOK          bool
	deletedID         int
}

func (s *fakeRoomClockInStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	if userID != s.user.ID {
		return User{}, false, nil
	}
	return s.user, true, nil
}

func (s *fakeRoomClockInStore) EmployeeIDByUserCorp(context.Context, int, int) (int, error) {
	return 99, nil
}

func (s *fakeRoomClockInStore) FirstEmployeeByUser(context.Context, int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeRoomClockInStore) RoomClockInPage(_ context.Context, filter RoomClockInFilter) (RoomClockInPage, error) {
	s.lastFilter = filter
	return s.page, nil
}

func (s *fakeRoomClockInStore) RoomClockInByID(context.Context, int, int) (RoomClockInItem, bool, error) {
	return s.item, s.itemFound, nil
}

func (s *fakeRoomClockInStore) CreateRoomClockIn(_ context.Context, values RoomClockInWrite) (int, error) {
	s.createCalls++
	s.created = values
	if s.createID > 0 {
		return s.createID, nil
	}
	return 1, nil
}

func (s *fakeRoomClockInStore) UpdateRoomClockIn(_ context.Context, _ int, _ int, values RoomClockInWrite) (bool, error) {
	s.updated = values
	return true, nil
}

func (s *fakeRoomClockInStore) DeleteRoomClockIn(_ context.Context, _ int, id int) (bool, error) {
	s.deletedID = id
	if s.deleteOK {
		return true, nil
	}
	return true, nil
}

func (s *fakeRoomClockInStore) RoomClockInContactPage(_ context.Context, filter RoomClockInContactFilter) (RoomClockInContactPage, error) {
	s.lastContactFilter = filter
	return s.contactPage, nil
}

func (s *fakeRoomClockInStore) RoomClockInDayPage(context.Context, int, int, int, string, int, int) (RoomClockInDayPage, error) {
	return s.dayPage, nil
}

func (s *fakeRoomClockInStore) BatchTagRoomClockInContacts(context.Context, int, int, []int, []int) (int, error) {
	return 1, nil
}

func (s *fakeRoomClockInStore) SaaSQuotaStatus(_ context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error) {
	s.quotaTenantID = tenantID
	s.quotaMetric = metric
	s.quotaAdditional = additional
	if s.quota.Metric != "" {
		return s.quota, nil
	}
	return SaaSQuotaStatus{Metric: metric, TenantID: tenantID, Additional: additional}, nil
}

func (s *fakeRoomClockInStore) RefreshSaaSUsageCounter(_ context.Context, tenantID int, metric string) error {
	s.refreshTenantID = tenantID
	s.refreshMetric = metric
	return nil
}
