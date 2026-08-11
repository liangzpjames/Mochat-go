package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestShopCodeIndexReturnsLaravelPage(t *testing.T) {
	store := &fakeShopCodeStore{
		user: User{ID: 1, IsSuperAdmin: 1},
		page: ShopCodePage{
			Page:    2,
			PerPage: 1,
			Total:   2,
			Items: []ShopCodeItem{{
				ID:            11,
				Name:          "杭州门店",
				Type:          1,
				EmployeeRaw:   `[{"id":31,"name":"店主"}]`,
				SearchKeyword: "西湖",
				Address:       "杭州市西湖区",
				City:          "杭州",
				Status:        1,
				CorpID:        7,
				CreatedAt:     "2026-07-05 10:00:00",
			}},
		},
	}
	handler := NewShopCodeHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "http://operation.example.com")
	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/shopCode/index?page=2&perPage=1&name=杭州&type=1", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			CurrentPage int              `json:"current_page"`
			Total       int              `json:"total"`
			Data        []map[string]any `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != 200 || envelope.Data.CurrentPage != 2 || envelope.Data.Total != 2 || len(envelope.Data.Data) != 1 {
		t.Fatalf("envelope = %#v", envelope)
	}
	item := envelope.Data.Data[0]
	if item["shopCodeId"].(float64) != 11 || item["name"] != "杭州门店" || item["city"] != "杭州" {
		t.Fatalf("item = %#v", item)
	}
	if store.lastFilter.CorpID != 7 || store.lastFilter.Type != 1 || store.lastFilter.Name != "杭州" || store.lastFilter.Page != 2 {
		t.Fatalf("filter = %#v", store.lastFilter)
	}
}

func TestShopCodeStorePreservesJSONFields(t *testing.T) {
	store := &fakeShopCodeStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}, createID: 66}
	handler := NewShopCodeHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "http://operation.example.com")
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/shopCode/store", strings.NewReader(`{"name":"杭州门店","type":1,"employee":[{"id":31,"name":"店主"}],"qrcode":{"url":"/qrcode/a.png"},"qwCode":[{"id":88}],"searchKeyword":"西湖","address":"杭州市西湖区","city":"杭州","lat":"30.1","lng":"120.1"}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.created.CorpID != 7 || store.created.CreateUserID != 1 || store.created.Name != "杭州门店" || store.created.Type != 1 {
		t.Fatalf("created = %#v", store.created)
	}
	if !strings.Contains(store.created.EmployeeRaw, "店主") || !strings.Contains(store.created.EmployeeQRCodeRaw, "qrcode") || store.created.QWCodeRaw != `[{"id":88}]` {
		t.Fatalf("json raw employee=%s qrcode=%s qw=%s", store.created.EmployeeRaw, store.created.EmployeeQRCodeRaw, store.created.QWCodeRaw)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricShopCodes {
		t.Fatalf("refresh = tenant %d metric %q", store.refreshTenantID, store.refreshMetric)
	}
}

func TestShopCodeStoreRejectsSaaSQuotaExceeded(t *testing.T) {
	store := &fakeShopCodeStore{
		user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1},
		quota: SaaSQuotaStatus{
			Metric:     SaaSMetricShopCodes,
			TenantID:   8,
			Current:    1,
			Limit:      1,
			Additional: 1,
		},
	}
	handler := NewShopCodeHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "http://operation.example.com")
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/shopCode/store", strings.NewReader(`{"name":"额度外门店","type":1,"employee":[{"id":31,"name":"店主"}]}`))
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
	if envelope.Msg != "套餐额度已达上限：门店活码数 1/1" {
		t.Fatalf("envelope = %#v", envelope)
	}
	if store.quotaTenantID != 8 || store.quotaMetric != SaaSMetricShopCodes || store.quotaAdditional != 1 {
		t.Fatalf("quota = tenant %d metric %q additional %d", store.quotaTenantID, store.quotaMetric, store.quotaAdditional)
	}
	if store.createCalls != 0 || store.refreshMetric != "" {
		t.Fatalf("side effects = createCalls %d refresh %q", store.createCalls, store.refreshMetric)
	}
}

func TestShopCodeDestroyRefreshesSaaSUsage(t *testing.T) {
	store := &fakeShopCodeStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}, deleteOK: true}
	handler := NewShopCodeHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "http://operation.example.com")
	req := authenticatedDashboardRequestForTest(http.MethodDelete, "/dashboard/shopCode/destroy", strings.NewReader(`{"shopCodeId":66}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Destroy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.deletedID != 66 {
		t.Fatalf("deleted id = %d", store.deletedID)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricShopCodes {
		t.Fatalf("refresh = tenant %d metric %q", store.refreshTenantID, store.refreshMetric)
	}
}

func TestShopCodePageSetUpsertsSettings(t *testing.T) {
	store := &fakeShopCodeStore{user: User{ID: 1, IsSuperAdmin: 1}, pageSettingID: 90}
	handler := NewShopCodeHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "http://operation.example.com")
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/shopCode/pageSet", strings.NewReader(`{"type":2,"title":"门店群","showType":2,"default":{"guide":"扫码入群"},"poster":"/poster.png","autoPass":1}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.PageSet(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.pageSetting.Type != 2 || store.pageSetting.Title != "门店群" || store.pageSetting.ShowType != 2 || store.pageSetting.AutoPass != 1 {
		t.Fatalf("page setting = %#v", store.pageSetting)
	}
	if !strings.Contains(store.pageSetting.DefaultRaw, "扫码入群") {
		t.Fatalf("default raw = %s", store.pageSetting.DefaultRaw)
	}
}

func TestShopCodeShareBuildsOperationURL(t *testing.T) {
	store := &fakeShopCodeStore{
		user: User{ID: 1, IsSuperAdmin: 1},
		item: ShopCodeItem{ID: 11, Name: "杭州门店", Type: 3, CorpID: 7},
	}
	handler := NewShopCodeHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "http://operation.example.com")
	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/shopCode/share?id=11", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Share(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	link, _ := envelope.Data["shareUrl"].(string)
	if !strings.HasPrefix(link, "http://operation.example.com/auth/shopCode?id=11&target=") || !strings.Contains(link, "type%3D3") {
		t.Fatalf("share url = %s", link)
	}
}

type fakeShopCodeStore struct {
	user            User
	page            ShopCodePage
	item            ShopCodeItem
	created         ShopCodeWrite
	updated         ShopCodeWrite
	pageSetting     ShopCodePageSettingWrite
	createID        int
	createCalls     int
	pageSettingID   int
	lastFilter      ShopCodeFilter
	quota           SaaSQuotaStatus
	quotaTenantID   int
	quotaMetric     string
	quotaAdditional int64
	refreshTenantID int
	refreshMetric   string
	deleteOK        bool
	deletedID       int
}

func (s *fakeShopCodeStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	if userID != s.user.ID {
		return User{}, false, nil
	}
	return s.user, true, nil
}

func (s *fakeShopCodeStore) EmployeeIDByUserCorp(context.Context, int, int) (int, error) {
	return 99, nil
}

func (s *fakeShopCodeStore) FirstEmployeeByUser(context.Context, int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeShopCodeStore) ShopCodePage(_ context.Context, filter ShopCodeFilter) (ShopCodePage, error) {
	s.lastFilter = filter
	return s.page, nil
}

func (s *fakeShopCodeStore) ShopCodeByID(context.Context, int, int) (ShopCodeItem, bool, error) {
	return s.item, s.item.ID > 0, nil
}

func (s *fakeShopCodeStore) CreateShopCode(_ context.Context, values ShopCodeWrite) (int, error) {
	s.createCalls++
	s.created = values
	if s.createID > 0 {
		return s.createID, nil
	}
	return 1, nil
}

func (s *fakeShopCodeStore) UpdateShopCode(_ context.Context, _ int, _ int, values ShopCodeWrite) (bool, error) {
	s.updated = values
	return true, nil
}

func (s *fakeShopCodeStore) UpdateShopCodeStatus(context.Context, int, int, int) (bool, error) {
	return true, nil
}

func (s *fakeShopCodeStore) DeleteShopCode(_ context.Context, _ int, id int) (bool, error) {
	s.deletedID = id
	if s.deleteOK {
		return true, nil
	}
	return true, nil
}

func (s *fakeShopCodeStore) ShopCodePageSetting(context.Context, int, int) (ShopCodePageSetting, bool, error) {
	return ShopCodePageSetting{}, false, nil
}

func (s *fakeShopCodeStore) UpsertShopCodePageSetting(_ context.Context, values ShopCodePageSettingWrite) (int, error) {
	s.pageSetting = values
	if s.pageSettingID > 0 {
		return s.pageSettingID, nil
	}
	return 1, nil
}

func (s *fakeShopCodeStore) ShopCodeAddressSuggestions(context.Context, int, string, string) ([]ShopCodeAddressSuggestion, error) {
	return nil, nil
}

func (s *fakeShopCodeStore) ShopCodeCities(context.Context, int, string) ([]ShopCodeCity, error) {
	return nil, nil
}

func (s *fakeShopCodeStore) ShopCodeOverview(context.Context, int, int) (ShopCodeOverview, error) {
	return ShopCodeOverview{}, nil
}

func (s *fakeShopCodeStore) ShopCodeRecordPage(context.Context, ShopCodeRecordFilter) (ShopCodeRecordPage, error) {
	return ShopCodeRecordPage{}, nil
}

func (s *fakeShopCodeStore) ShopCodeShopStatPage(context.Context, ShopCodeFilter) (ShopCodeShopStatPage, error) {
	return ShopCodeShopStatPage{}, nil
}

func (s *fakeShopCodeStore) SaaSQuotaStatus(_ context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error) {
	s.quotaTenantID = tenantID
	s.quotaMetric = metric
	s.quotaAdditional = additional
	if s.quota.Metric != "" {
		return s.quota, nil
	}
	return SaaSQuotaStatus{Metric: metric, TenantID: tenantID, Additional: additional}, nil
}

func (s *fakeShopCodeStore) RefreshSaaSUsageCounter(_ context.Context, tenantID int, metric string) error {
	s.refreshTenantID = tenantID
	s.refreshMetric = metric
	return nil
}
