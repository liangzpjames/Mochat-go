package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSensitiveWordIndexReturnsPage(t *testing.T) {
	store := &fakeSensitiveWordStore{
		user: User{ID: 1, TenantID: 1, IsSuperAdmin: 1},
		page: SensitiveWordPage{
			Items: []SensitiveWordItem{{
				ID:          11,
				CorpID:      7,
				GroupID:     3,
				GroupName:   "默认分组",
				Name:        "敏感词A",
				Status:      1,
				EmployeeNum: 2,
				ContactNum:  5,
				CreatedAt:   "2026-07-05 08:00:00",
			}},
			Total:     1,
			TotalPage: 1,
			PerPage:   10,
		},
	}
	handler := NewSensitiveWordHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/sensitiveWord/index?keyWords=A&page=1&perPage=10", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	list := data["list"].([]any)
	if len(list) != 1 {
		t.Fatalf("list len = %d", len(list))
	}
	row := list[0].(map[string]any)
	if row["name"] != "敏感词A" || int(row["employeeNum"].(float64)) != 2 || int(row["contactNum"].(float64)) != 5 {
		t.Fatalf("row = %#v", row)
	}
	if store.lastFilter.KeyWords != "A" || store.lastFilter.CorpID != 7 {
		t.Fatalf("filter = %#v", store.lastFilter)
	}
}

func TestSensitiveWordStoreSplitsNames(t *testing.T) {
	store := &fakeSensitiveWordStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}}
	handler := NewSensitiveWordHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/sensitiveWord/store", strings.NewReader(`{"groupId":3,"name":"敏感词A，敏感词B、敏感词A"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.createdCorpID != 7 || store.createdGroupID != 3 {
		t.Fatalf("created corp/group = %d/%d", store.createdCorpID, store.createdGroupID)
	}
	if strings.Join(store.createdNames, ",") != "敏感词A,敏感词B" {
		t.Fatalf("created names = %#v", store.createdNames)
	}
	if store.quotaTenantID != 8 || store.quotaMetric != SaaSMetricSensitiveWords || store.quotaAdditional != 2 {
		t.Fatalf("quota check = tenant:%d metric:%s additional:%d", store.quotaTenantID, store.quotaMetric, store.quotaAdditional)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricSensitiveWords {
		t.Fatalf("refresh = tenant:%d metric:%s", store.refreshTenantID, store.refreshMetric)
	}
}

func TestSensitiveWordStoreRejectsSaaSQuotaExceeded(t *testing.T) {
	store := &fakeSensitiveWordStore{
		user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1},
		quota: SaaSQuotaStatus{
			Metric:   SaaSMetricSensitiveWords,
			TenantID: 8,
			Current:  1,
			Limit:    1,
		},
	}
	handler := NewSensitiveWordHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/sensitiveWord/store", strings.NewReader(`{"groupId":3,"name":"敏感词A，敏感词B"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Store(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "套餐额度已达上限：敏感词词库数 1/1" {
		t.Fatalf("body = %#v", body)
	}
	if store.createCalls != 0 {
		t.Fatalf("create calls = %d", store.createCalls)
	}
	if store.refreshMetric != "" {
		t.Fatalf("unexpected refresh = %s", store.refreshMetric)
	}
}

func TestSensitiveWordDestroyRefreshesSaaSUsage(t *testing.T) {
	store := &fakeSensitiveWordStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}}
	handler := NewSensitiveWordHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodDelete, "/dashboard/sensitiveWord/destroy", strings.NewReader(`{"sensitiveWordId":11}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Destroy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.deletedWordID != 11 {
		t.Fatalf("deleted word id = %d", store.deletedWordID)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricSensitiveWords {
		t.Fatalf("refresh = tenant:%d metric:%s", store.refreshTenantID, store.refreshMetric)
	}
}

func TestSensitiveWordsMonitorShowReturnsMessages(t *testing.T) {
	store := &fakeSensitiveWordStore{
		user: User{ID: 1, TenantID: 1, IsSuperAdmin: 1},
		messages: []SensitiveWordsMonitorMessage{{
			Sender:     "客户A",
			MsgType:    1,
			SendTime:   "2026-07-05 08:01:00",
			IsTrigger:  1,
			MsgContent: map[string]any{"content": "包含敏感词"},
		}},
	}
	handler := NewSensitiveWordHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/sensitiveWordsMonitor/show?sensitiveWordsMonitorId=9", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.MonitorShow(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	if len(data) != 1 || data[0].(map[string]any)["sender"] != "客户A" {
		t.Fatalf("data = %#v", data)
	}
	if store.lastMonitorID != 9 || store.lastMonitorCorpID != 7 {
		t.Fatalf("monitor lookup = %d/%d", store.lastMonitorCorpID, store.lastMonitorID)
	}
}

func TestSensitiveWordPageServesStandaloneConsole(t *testing.T) {
	handler := NewSensitiveWordPageHandler()

	req := httptest.NewRequest(http.MethodGet, "/dashboard/sensitiveWords/page", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") || !strings.Contains(contentType, "charset=utf-8") {
		t.Fatalf("Content-Type = %q", contentType)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"MoChat Go 敏感词管理",
		"/dashboard/sensitiveWordGroup/select",
		"/dashboard/sensitiveWord/index",
		"/dashboard/sensitiveWord/store",
		"/dashboard/sensitiveWordsMonitor/index",
		"/dashboard/sensitiveWordsMonitor/show",
		"mochat_go_sensitive_word_token",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("page missing %q", want)
		}
	}
	if strings.Contains(body, "http://") || strings.Contains(body, "https://") {
		t.Fatalf("page should not depend on external assets")
	}
}

func TestSensitiveWordPageRejectsPost(t *testing.T) {
	handler := NewSensitiveWordPageHandler()

	req := httptest.NewRequest(http.MethodPost, "/dashboard/sensitiveWords/page", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("Allow = %q", rec.Header().Get("Allow"))
	}
}

type fakeSensitiveWordStore struct {
	user              User
	page              SensitiveWordPage
	lastFilter        SensitiveWordFilter
	createdCorpID     int
	createdGroupID    int
	createdNames      []string
	createCalls       int
	deletedWordID     int
	groups            []SensitiveWordGroup
	monitorPage       SensitiveWordsMonitorPage
	messages          []SensitiveWordsMonitorMessage
	lastMonitorCorpID int
	lastMonitorID     int
	quota             SaaSQuotaStatus
	quotaTenantID     int
	quotaMetric       string
	quotaAdditional   int64
	refreshTenantID   int
	refreshMetric     string
}

func (s *fakeSensitiveWordStore) UserByID(context.Context, int) (User, bool, error) {
	return s.user, true, nil
}

func (s *fakeSensitiveWordStore) EmployeeIDByUserCorp(context.Context, int, int) (int, error) {
	return 99, nil
}

func (s *fakeSensitiveWordStore) FirstEmployeeByUser(context.Context, int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeSensitiveWordStore) SensitiveWordPage(_ context.Context, filter SensitiveWordFilter) (SensitiveWordPage, error) {
	s.lastFilter = filter
	return s.page, nil
}

func (s *fakeSensitiveWordStore) CreateSensitiveWords(_ context.Context, corpID int, groupID int, names []string) error {
	s.createCalls++
	s.createdCorpID = corpID
	s.createdGroupID = groupID
	s.createdNames = names
	return nil
}

func (s *fakeSensitiveWordStore) UpdateSensitiveWordStatus(context.Context, int, int, int) (bool, error) {
	return true, nil
}

func (s *fakeSensitiveWordStore) MoveSensitiveWord(context.Context, int, int, int) (bool, error) {
	return true, nil
}

func (s *fakeSensitiveWordStore) DeleteSensitiveWord(_ context.Context, _ int, wordID int) (bool, error) {
	s.deletedWordID = wordID
	return true, nil
}

func (s *fakeSensitiveWordStore) SensitiveWordGroups(context.Context, int) ([]SensitiveWordGroup, error) {
	return s.groups, nil
}

func (s *fakeSensitiveWordStore) CreateSensitiveWordGroups(context.Context, int, []string) error {
	return nil
}

func (s *fakeSensitiveWordStore) UpdateSensitiveWordGroup(context.Context, int, int, string) (bool, error) {
	return true, nil
}

func (s *fakeSensitiveWordStore) SensitiveWordsMonitorPage(context.Context, SensitiveWordsMonitorFilter) (SensitiveWordsMonitorPage, error) {
	return s.monitorPage, nil
}

func (s *fakeSensitiveWordStore) SensitiveWordsMonitorMessages(_ context.Context, corpID int, monitorID int) ([]SensitiveWordsMonitorMessage, bool, error) {
	s.lastMonitorCorpID = corpID
	s.lastMonitorID = monitorID
	return s.messages, true, nil
}

func (s *fakeSensitiveWordStore) SaaSQuotaStatus(_ context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error) {
	s.quotaTenantID = tenantID
	s.quotaMetric = metric
	s.quotaAdditional = additional
	if s.quota.Metric != "" || s.quota.Limit != 0 || s.quota.Current != 0 {
		s.quota.TenantID = tenantID
		s.quota.Metric = metric
		s.quota.Additional = additional
		return s.quota, nil
	}
	return SaaSQuotaStatus{Metric: metric, TenantID: tenantID, Additional: additional}, nil
}

func (s *fakeSensitiveWordStore) RefreshSaaSUsageCounter(_ context.Context, tenantID int, metric string) error {
	s.refreshTenantID = tenantID
	s.refreshMetric = metric
	return nil
}
