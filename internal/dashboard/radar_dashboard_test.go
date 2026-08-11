package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRadarIndexReturnsLaravelPage(t *testing.T) {
	store := &fakeRadarStore{
		user: User{ID: 1, IsSuperAdmin: 1},
		page: RadarPage{
			Page:    2,
			PerPage: 1,
			Total:   3,
			Items: []RadarItem{{
				ID:             31,
				Type:           1,
				Title:          "官网链接",
				Link:           "https://example.com",
				ClickNum:       12,
				ClickPersonNum: 5,
				ChannelNum:     2,
				CorpID:         7,
				CreatedAt:      "2026-07-05 10:00:00",
			}},
		},
	}
	handler := NewRadarHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "http://operation.example.com")
	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/radar/index?page=2&perPage=1&type=1&title=官网", nil)
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
	if envelope.Code != 200 || envelope.Data.CurrentPage != 2 || envelope.Data.Total != 3 || len(envelope.Data.Data) != 1 {
		t.Fatalf("envelope = %#v", envelope)
	}
	item := envelope.Data.Data[0]
	if item["radarId"].(float64) != 31 || item["title"] != "官网链接" || item["clickNum"].(float64) != 12 {
		t.Fatalf("item = %#v", item)
	}
	if store.lastFilter.CorpID != 7 || store.lastFilter.Type != 1 || store.lastFilter.Title != "官网" || store.lastFilter.Page != 2 {
		t.Fatalf("filter = %#v", store.lastFilter)
	}
}

func TestRadarStorePreservesJSONFields(t *testing.T) {
	store := &fakeRadarStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}, createID: 88}
	handler := NewRadarHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "http://operation.example.com")
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/radar/store", strings.NewReader(`{"type":3,"title":"文章雷达","link":"https://example.com/a","articleType":2,"article":{"content":"正文"},"employeeCard":1,"actionNotice":1,"dynamicNotice":1,"contactTags":[{"id":9,"name":"意向"}],"tagStatus":1,"contactGrade":[{"score":5}]}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.created.CorpID != 7 || store.created.CreateUserID != 1 || store.created.Type != 3 || store.created.Title != "文章雷达" {
		t.Fatalf("created = %#v", store.created)
	}
	if !strings.Contains(store.created.ArticleRaw, "正文") || !strings.Contains(store.created.ContactTagsRaw, "意向") || !strings.Contains(store.created.ContactGradeRaw, "score") {
		t.Fatalf("json raw article=%s tags=%s grade=%s", store.created.ArticleRaw, store.created.ContactTagsRaw, store.created.ContactGradeRaw)
	}
	if store.created.EmployeeCard != 1 || store.created.ActionNotice != 1 || store.created.DynamicNotice != 1 || store.created.TagStatus != 1 {
		t.Fatalf("created flags = %#v", store.created)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricRadars {
		t.Fatalf("refresh = tenant %d metric %q", store.refreshTenantID, store.refreshMetric)
	}
}

func TestRadarStoreRejectsSaaSQuotaExceeded(t *testing.T) {
	store := &fakeRadarStore{
		user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1},
		quota: SaaSQuotaStatus{
			Metric:     SaaSMetricRadars,
			TenantID:   8,
			Current:    1,
			Limit:      1,
			Additional: 1,
		},
	}
	handler := NewRadarHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "http://operation.example.com")
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/radar/store", strings.NewReader(`{"type":1,"title":"额度外雷达","link":"https://example.com/extra"}`))
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
	if envelope.Msg != "套餐额度已达上限：互动雷达数 1/1" {
		t.Fatalf("envelope = %#v", envelope)
	}
	if store.quotaTenantID != 8 || store.quotaMetric != SaaSMetricRadars || store.quotaAdditional != 1 {
		t.Fatalf("quota = tenant %d metric %q additional %d", store.quotaTenantID, store.quotaMetric, store.quotaAdditional)
	}
	if store.createCalls != 0 || store.refreshMetric != "" {
		t.Fatalf("side effects = createCalls %d refresh %q", store.createCalls, store.refreshMetric)
	}
}

func TestRadarDestroyRefreshesSaaSUsage(t *testing.T) {
	store := &fakeRadarStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}, deleteOK: true}
	handler := NewRadarHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "http://operation.example.com")
	req := authenticatedDashboardRequestForTest(http.MethodDelete, "/dashboard/radar/destroy", strings.NewReader(`{"radarId":88}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Destroy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.deletedID != 88 {
		t.Fatalf("deleted id = %d", store.deletedID)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricRadars {
		t.Fatalf("refresh = tenant %d metric %q", store.refreshTenantID, store.refreshMetric)
	}
}

func TestRadarStoreChannelLinkBuildsOperationURL(t *testing.T) {
	store := &fakeRadarStore{
		user: User{ID: 1, IsSuperAdmin: 1},
		channelLink: RadarChannelLinkItem{
			ID:          77,
			RadarID:     31,
			RadarTitle:  "官网链接",
			ChannelID:   9,
			ChannelName: "朋友圈",
			EmployeeID:  99,
			CorpID:      7,
		},
	}
	handler := NewRadarHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "http://operation.example.com")
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/radar/storeChannelLink", strings.NewReader(`{"radar_id":31,"channel_id":9,"employeeId":99,"type":1}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.StoreChannelLink(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.channelLinkWrite.CorpID != 7 || store.channelLinkWrite.RadarID != 31 || store.channelLinkWrite.ChannelID != 9 || store.channelLinkWrite.EmployeeID != 99 {
		t.Fatalf("channel link write = %#v", store.channelLinkWrite)
	}
	if !strings.HasPrefix(store.updatedLink, "http://operation.example.com/auth/radar?id=31&target=") || !strings.Contains(store.updatedLink, "employee_id%3D99") || !strings.Contains(store.updatedLink, "target_id%3D77") {
		t.Fatalf("updated link = %s", store.updatedLink)
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data["link"] != store.updatedLink {
		t.Fatalf("response link = %#v want %s", envelope.Data["link"], store.updatedLink)
	}
}

type fakeRadarStore struct {
	user             User
	page             RadarPage
	item             RadarItem
	overview         RadarOverview
	channelLink      RadarChannelLinkItem
	created          RadarWrite
	updated          RadarWrite
	channelLinkWrite RadarChannelLinkWrite
	updatedLink      string
	createID         int
	createCalls      int
	lastFilter       RadarFilter
	quota            SaaSQuotaStatus
	quotaTenantID    int
	quotaMetric      string
	quotaAdditional  int64
	refreshTenantID  int
	refreshMetric    string
	deleteOK         bool
	deletedID        int
}

func (s *fakeRadarStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	if userID != s.user.ID {
		return User{}, false, nil
	}
	return s.user, true, nil
}

func (s *fakeRadarStore) EmployeeIDByUserCorp(context.Context, int, int) (int, error) {
	return 99, nil
}

func (s *fakeRadarStore) FirstEmployeeByUser(context.Context, int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeRadarStore) RadarPage(_ context.Context, filter RadarFilter) (RadarPage, error) {
	s.lastFilter = filter
	return s.page, nil
}

func (s *fakeRadarStore) RadarByID(context.Context, int, int) (RadarItem, bool, error) {
	if s.item.ID <= 0 {
		return RadarItem{}, false, nil
	}
	return s.item, true, nil
}

func (s *fakeRadarStore) CreateRadar(_ context.Context, values RadarWrite) (int, error) {
	s.createCalls++
	s.created = values
	if s.createID > 0 {
		return s.createID, nil
	}
	return 1, nil
}

func (s *fakeRadarStore) UpdateRadar(_ context.Context, _ int, _ int, values RadarWrite) (bool, error) {
	s.updated = values
	return true, nil
}

func (s *fakeRadarStore) DeleteRadar(_ context.Context, _ int, id int) (bool, error) {
	s.deletedID = id
	if s.deleteOK {
		return true, nil
	}
	return true, nil
}

func (s *fakeRadarStore) CreateRadarChannel(context.Context, int, int, string) (int, error) {
	return 1, nil
}

func (s *fakeRadarStore) RadarChannelPage(context.Context, int, string, int, int) (RadarChannelPage, error) {
	return RadarChannelPage{}, nil
}

func (s *fakeRadarStore) UpsertRadarChannelLink(_ context.Context, values RadarChannelLinkWrite) (RadarChannelLinkItem, error) {
	s.channelLinkWrite = values
	return s.channelLink, nil
}

func (s *fakeRadarStore) UpdateRadarChannelLinkURL(_ context.Context, _ int, _ int, link string) (bool, error) {
	s.updatedLink = link
	s.channelLink.Link = link
	return true, nil
}

func (s *fakeRadarStore) RadarChannelLinkPage(context.Context, int, int, int, int) (RadarChannelLinkPage, error) {
	return RadarChannelLinkPage{}, nil
}

func (s *fakeRadarStore) RadarOverview(context.Context, int, int, int) (RadarOverview, error) {
	return s.overview, nil
}

func (s *fakeRadarStore) RadarRecordPage(context.Context, int, int, int, int, int) (RadarRecordPage, error) {
	return RadarRecordPage{}, nil
}

func (s *fakeRadarStore) RadarChannelStats(context.Context, int, int, int, int) (RadarChannelLinkPage, error) {
	return RadarChannelLinkPage{}, nil
}

func (s *fakeRadarStore) SaaSQuotaStatus(_ context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error) {
	s.quotaTenantID = tenantID
	s.quotaMetric = metric
	s.quotaAdditional = additional
	if s.quota.Metric != "" {
		return s.quota, nil
	}
	return SaaSQuotaStatus{Metric: metric, TenantID: tenantID, Additional: additional}, nil
}

func (s *fakeRadarStore) RefreshSaaSUsageCounter(_ context.Context, tenantID int, metric string) error {
	s.refreshTenantID = tenantID
	s.refreshMetric = metric
	return nil
}
