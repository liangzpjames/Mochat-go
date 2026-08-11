package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoomQualityShowContactKeepsZeroStatusFilter(t *testing.T) {
	store := &fakeRoomQualityStore{
		user: User{ID: 1, IsSuperAdmin: 1},
		contactPage: RoomQualityContactPage{
			Page:    1,
			PerPage: 15,
			Total:   1,
			Items: []RoomQualityContactItem{{
				ID:        9,
				QualityID: 12,
				Nickname:  "触发记录",
				Status:    0,
			}},
		},
	}
	handler := NewRoomQualityHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/roomQuality/showContact?roomQualityId=12&status=0&page=1&perPage=15", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ShowContact(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastContactFilter.CorpID != 7 || store.lastContactFilter.QualityID != 12 || store.lastContactFilter.Status != 0 {
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
	if len(envelope.Data.List) != 1 || envelope.Data.List[0]["nickname"] != "触发记录" {
		t.Fatalf("list = %#v", envelope.Data.List)
	}
}

func TestRoomQualityStorePreservesRuleJSON(t *testing.T) {
	store := &fakeRoomQualityStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}, createID: 88}
	handler := NewRoomQualityHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	body := `{
		"quality":{"name":"群质检规则","description":"敏感动作提醒","rule":[{"num":3,"time_type":1,"showEmployee":[11,12]}],"rooms":[101,102],"status":0}
	}`
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/roomQuality/store", strings.NewReader(body))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.created.CorpID != 7 || store.created.CreateUserID != 1 || store.created.Name != "群质检规则" || store.created.Status != 0 {
		t.Fatalf("created = %#v", store.created)
	}
	if !strings.Contains(store.created.RuleRaw, "showEmployee") || !strings.Contains(store.created.RuleRaw, "11") || !strings.Contains(store.created.RoomsRaw, "101") {
		t.Fatalf("json raw = %#v", store.created)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricRoomQualities {
		t.Fatalf("refresh = tenant %d metric %s", store.refreshTenantID, store.refreshMetric)
	}
}

func TestRoomQualityStoreRejectsSaaSQuotaExceeded(t *testing.T) {
	store := &fakeRoomQualityStore{
		user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1},
		quota: SaaSQuotaStatus{
			Metric:     SaaSMetricRoomQualities,
			TenantID:   8,
			Current:    1,
			Limit:      1,
			Additional: 1,
		},
	}
	handler := NewRoomQualityHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/roomQuality/store", strings.NewReader(`{"quality":{"name":"额度外群质检"}}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Store(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "套餐额度已达上限：群质检规则数 1/1") {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if store.createCalls != 0 {
		t.Fatalf("create calls = %d", store.createCalls)
	}
	if store.quotaTenantID != 8 || store.quotaMetric != SaaSMetricRoomQualities || store.quotaAdditional != 1 {
		t.Fatalf("quota check = tenant %d metric %s additional %d", store.quotaTenantID, store.quotaMetric, store.quotaAdditional)
	}
	if store.refreshMetric != "" {
		t.Fatalf("refresh should not run, got %s", store.refreshMetric)
	}
}

func TestRoomQualityDestroyRefreshesSaaSUsage(t *testing.T) {
	store := &fakeRoomQualityStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}}
	handler := NewRoomQualityHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := authenticatedDashboardRequestForTest(http.MethodDelete, "/dashboard/roomQuality/destroy", strings.NewReader(`{"roomQualityId":88}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Destroy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.deletedID != 88 {
		t.Fatalf("deleted id = %d", store.deletedID)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricRoomQualities {
		t.Fatalf("refresh = tenant %d metric %s", store.refreshTenantID, store.refreshMetric)
	}
}

type fakeRoomQualityStore struct {
	user              User
	page              RoomQualityPage
	item              RoomQualityItem
	itemFound         bool
	contactPage       RoomQualityContactPage
	contactItem       RoomQualityContactItem
	contactFound      bool
	lastFilter        RoomQualityFilter
	lastContactFilter RoomQualityContactFilter
	created           RoomQualityWrite
	updated           RoomQualityWrite
	statusID          int
	statusValue       int
	createID          int
	createCalls       int
	quota             SaaSQuotaStatus
	quotaTenantID     int
	quotaMetric       string
	quotaAdditional   int64
	refreshTenantID   int
	refreshMetric     string
	deletedID         int
}

func (s *fakeRoomQualityStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	if userID != s.user.ID {
		return User{}, false, nil
	}
	return s.user, true, nil
}

func (s *fakeRoomQualityStore) EmployeeIDByUserCorp(context.Context, int, int) (int, error) {
	return 99, nil
}

func (s *fakeRoomQualityStore) FirstEmployeeByUser(context.Context, int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeRoomQualityStore) RoomQualityPage(_ context.Context, filter RoomQualityFilter) (RoomQualityPage, error) {
	s.lastFilter = filter
	return s.page, nil
}

func (s *fakeRoomQualityStore) RoomQualityByID(context.Context, int, int) (RoomQualityItem, bool, error) {
	return s.item, s.itemFound, nil
}

func (s *fakeRoomQualityStore) CreateRoomQuality(_ context.Context, values RoomQualityWrite) (int, error) {
	s.createCalls++
	s.created = values
	if s.createID > 0 {
		return s.createID, nil
	}
	return 1, nil
}

func (s *fakeRoomQualityStore) UpdateRoomQuality(_ context.Context, _ int, _ int, values RoomQualityWrite) (bool, error) {
	s.updated = values
	return true, nil
}

func (s *fakeRoomQualityStore) UpdateRoomQualityStatus(_ context.Context, _ int, id int, status int) (bool, error) {
	s.statusID = id
	s.statusValue = status
	return true, nil
}

func (s *fakeRoomQualityStore) DeleteRoomQuality(_ context.Context, _ int, id int) (bool, error) {
	s.deletedID = id
	return true, nil
}

func (s *fakeRoomQualityStore) RoomQualityContactPage(_ context.Context, filter RoomQualityContactFilter) (RoomQualityContactPage, error) {
	s.lastContactFilter = filter
	return s.contactPage, nil
}

func (s *fakeRoomQualityStore) RoomQualityContactByID(context.Context, int, int) (RoomQualityContactItem, bool, error) {
	return s.contactItem, s.contactFound, nil
}

func (s *fakeRoomQualityStore) SaaSQuotaStatus(_ context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error) {
	s.quotaTenantID = tenantID
	s.quotaMetric = metric
	s.quotaAdditional = additional
	if s.quota.Metric != "" {
		return s.quota, nil
	}
	return SaaSQuotaStatus{Metric: metric, TenantID: tenantID, Additional: additional}, nil
}

func (s *fakeRoomQualityStore) RefreshSaaSUsageCounter(_ context.Context, tenantID int, metric string) error {
	s.refreshTenantID = tenantID
	s.refreshMetric = metric
	return nil
}
