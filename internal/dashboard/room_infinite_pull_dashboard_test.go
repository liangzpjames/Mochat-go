package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoomInfinitePullStorePreservesQwCodeJSON(t *testing.T) {
	store := &fakeRoomInfinitePullStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}, createID: 77}
	handler := NewRoomInfinitePullHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	body := `{
		"name":"无限拉群A",
		"titleStatus":1,
		"title":"客户群",
		"describeStatus":1,
		"describe":"扫码入群",
		"qwCode":[{"qrcode":"https://example.com/qrcode-a.png","upper_limit":200,"status":1}]
	}`
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/roomInfinitePull/store", strings.NewReader(body))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.created.CorpID != 7 || store.created.CreateUserID != 1 || store.created.Name != "无限拉群A" {
		t.Fatalf("created = %#v", store.created)
	}
	if store.created.Title != "客户群" || store.created.Describe != "扫码入群" || !strings.Contains(store.created.QwCodeRaw, "qrcode-a.png") {
		t.Fatalf("created values = %#v", store.created)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricRoomInfinitePulls {
		t.Fatalf("refresh = tenant %d metric %q", store.refreshTenantID, store.refreshMetric)
	}
}

func TestRoomInfinitePullStoreRejectsSaaSQuotaExceeded(t *testing.T) {
	store := &fakeRoomInfinitePullStore{
		user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1},
		quota: SaaSQuotaStatus{
			Metric:     SaaSMetricRoomInfinitePulls,
			TenantID:   8,
			Current:    1,
			Limit:      1,
			Additional: 1,
		},
	}
	handler := NewRoomInfinitePullHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/roomInfinitePull/store", strings.NewReader(`{"name":"额度外无限拉群"}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Store(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Msg != "套餐额度已达上限：无限拉群数 1/1" {
		t.Fatalf("envelope = %#v", envelope)
	}
	if store.quotaTenantID != 8 || store.quotaMetric != SaaSMetricRoomInfinitePulls || store.quotaAdditional != 1 {
		t.Fatalf("quota = tenant %d metric %q additional %d", store.quotaTenantID, store.quotaMetric, store.quotaAdditional)
	}
	if store.createCalls != 0 || store.refreshMetric != "" {
		t.Fatalf("side effects = createCalls %d refresh %q", store.createCalls, store.refreshMetric)
	}
}

func TestRoomInfinitePullDestroyRefreshesSaaSUsage(t *testing.T) {
	store := &fakeRoomInfinitePullStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}, deleteOK: true}
	handler := NewRoomInfinitePullHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := authenticatedDashboardRequestForTest(http.MethodDelete, "/dashboard/roomInfinitePull/destroy", strings.NewReader(`{"roomInfinitePullId":88}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Destroy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.deletedID != 88 {
		t.Fatalf("deleted id = %d", store.deletedID)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricRoomInfinitePulls {
		t.Fatalf("refresh = tenant %d metric %q", store.refreshTenantID, store.refreshMetric)
	}
}

func TestRoomInfinitePullInfoBuildsLinkFromRequestHost(t *testing.T) {
	store := &fakeRoomInfinitePullStore{
		user:      User{ID: 1, IsSuperAdmin: 1},
		itemFound: true,
		item:      RoomInfinitePullItem{ID: 12, Name: "无限拉群A", QwCodeRaw: "[]"},
	}
	handler := NewRoomInfinitePullHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/roomInfinitePull/info?id=12", nil)
	req.Host = "mochat.example.com"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Info(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "https://mochat.example.com/roomInfinitePull?id=12") {
		t.Fatalf("body missing generated link: %s", rec.Body.String())
	}
}

type fakeRoomInfinitePullStore struct {
	user            User
	page            RoomInfinitePullPage
	item            RoomInfinitePullItem
	itemFound       bool
	created         RoomInfinitePullWrite
	updated         RoomInfinitePullWrite
	createID        int
	createCalls     int
	quota           SaaSQuotaStatus
	quotaTenantID   int
	quotaMetric     string
	quotaAdditional int64
	refreshTenantID int
	refreshMetric   string
	deleteOK        bool
	deletedID       int
}

func (s *fakeRoomInfinitePullStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	if userID != s.user.ID {
		return User{}, false, nil
	}
	return s.user, true, nil
}

func (s *fakeRoomInfinitePullStore) EmployeeIDByUserCorp(context.Context, int, int) (int, error) {
	return 99, nil
}

func (s *fakeRoomInfinitePullStore) FirstEmployeeByUser(context.Context, int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeRoomInfinitePullStore) RoomInfinitePullPage(context.Context, RoomInfinitePullFilter) (RoomInfinitePullPage, error) {
	return s.page, nil
}

func (s *fakeRoomInfinitePullStore) RoomInfinitePullByID(context.Context, int, int) (RoomInfinitePullItem, bool, error) {
	return s.item, s.itemFound, nil
}

func (s *fakeRoomInfinitePullStore) CreateRoomInfinitePull(_ context.Context, values RoomInfinitePullWrite) (int, error) {
	s.createCalls++
	s.created = values
	if s.createID > 0 {
		return s.createID, nil
	}
	return 1, nil
}

func (s *fakeRoomInfinitePullStore) UpdateRoomInfinitePull(_ context.Context, _ int, _ int, values RoomInfinitePullWrite) (bool, error) {
	s.updated = values
	return true, nil
}

func (s *fakeRoomInfinitePullStore) DeleteRoomInfinitePull(_ context.Context, _ int, id int) (bool, error) {
	s.deletedID = id
	if s.deleteOK {
		return true, nil
	}
	return true, nil
}

func (s *fakeRoomInfinitePullStore) SaaSQuotaStatus(_ context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error) {
	s.quotaTenantID = tenantID
	s.quotaMetric = metric
	s.quotaAdditional = additional
	if s.quota.Metric != "" {
		return s.quota, nil
	}
	return SaaSQuotaStatus{Metric: metric, TenantID: tenantID, Additional: additional}, nil
}

func (s *fakeRoomInfinitePullStore) RefreshSaaSUsageCounter(_ context.Context, tenantID int, metric string) error {
	s.refreshTenantID = tenantID
	s.refreshMetric = metric
	return nil
}
