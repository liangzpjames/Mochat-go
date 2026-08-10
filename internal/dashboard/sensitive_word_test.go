package dashboard

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
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
				Version:     "word-v1",
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
	if row["name"] != "敏感词A" || row["version"] != "word-v1" || int(row["employeeNum"].(float64)) != 2 || int(row["contactNum"].(float64)) != 5 {
		t.Fatalf("row = %#v", row)
	}
	if store.lastFilter.KeyWords != "A" || store.lastFilter.CorpID != 7 {
		t.Fatalf("filter = %#v", store.lastFilter)
	}
}

func TestSensitiveWordStoreSplitsNames(t *testing.T) {
	store := &fakeSensitiveWordStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}}
	handler := NewSensitiveWordHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/sensitiveWord/store", strings.NewReader(`{"groupId":3,"name":"敏感词A，敏感词B、敏感词A","version":"0","idempotencyKey":"word-create-1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastMutation.CorpID != 7 || store.lastMutation.GroupID != 3 {
		t.Fatalf("created corp/group = %d/%d", store.lastMutation.CorpID, store.lastMutation.GroupID)
	}
	if strings.Join(store.lastMutation.Names, ",") != "敏感词A,敏感词B" {
		t.Fatalf("created names = %#v", store.lastMutation.Names)
	}
	if store.lastMutation.Action != SensitiveWordMutationCreateWords || store.lastMutation.Version != "0" || store.lastMutation.IdempotencyKey != "word-create-1" || store.lastMutation.ActorUserID != 1 || store.lastMutation.TenantID != 8 {
		t.Fatalf("mutation = %#v", store.lastMutation)
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
	req := httptest.NewRequest(http.MethodPost, "/dashboard/sensitiveWord/store", strings.NewReader(`{"groupId":3,"name":"敏感词A，敏感词B","version":"0","idempotencyKey":"quota-1"}`))
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

func TestSensitiveWordStoreReplaysIdempotentCreateBeforeQuotaCheck(t *testing.T) {
	store := &fakeSensitiveWordStore{
		user:         User{ID: 1, TenantID: 8, IsSuperAdmin: 1},
		replayFound:  true,
		replayResult: SensitiveWordMutationResult{Version: "0", Idempotent: true},
		quota:        SaaSQuotaStatus{Metric: SaaSMetricSensitiveWords, TenantID: 8, Current: 20, Limit: 20},
	}
	handler := NewSensitiveWordHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/sensitiveWord/store", strings.NewReader(`{"groupId":3,"name":"敏感词A","version":"0","idempotencyKey":"replay-at-limit"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Store(rec, req)

	if rec.Code != http.StatusOK || store.quotaMetric != "" || store.mutationCalls != 0 {
		t.Fatalf("status=%d quota=%q mutations=%d body=%s", rec.Code, store.quotaMetric, store.mutationCalls, rec.Body.String())
	}
}

func TestSensitiveWordDestroyRefreshesSaaSUsage(t *testing.T) {
	store := &fakeSensitiveWordStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}}
	handler := NewSensitiveWordHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodDelete, "/dashboard/sensitiveWord/destroy", strings.NewReader(`{"sensitiveWordId":11,"version":"word-v1","idempotencyKey":"word-delete-1","confirmed":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Destroy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastMutation.WordID != 11 || store.lastMutation.Action != SensitiveWordMutationDeleteWord {
		t.Fatalf("mutation = %#v", store.lastMutation)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricSensitiveWords {
		t.Fatalf("refresh = tenant:%d metric:%s", store.refreshTenantID, store.refreshMetric)
	}
}

func TestSensitiveWordMutationsRequireVersionAndIdempotencyKey(t *testing.T) {
	store := &fakeSensitiveWordStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}}
	handler := NewSensitiveWordHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	tests := []struct {
		name   string
		method string
		body   string
		serve  func(http.ResponseWriter, *http.Request)
	}{
		{name: "create word missing version", method: http.MethodPost, body: `{"groupId":3,"name":"敏感词" ,"idempotencyKey":"k1"}`, serve: handler.Store},
		{name: "toggle missing idempotency", method: http.MethodPut, body: `{"sensitiveWordId":11,"status":2,"version":"v1"}`, serve: handler.StatusUpdate},
		{name: "delete missing confirmation", method: http.MethodDelete, body: `{"sensitiveWordId":11,"version":"v1","idempotencyKey":"k3"}`, serve: handler.Destroy},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(test.method, "/dashboard/sensitive-word", strings.NewReader(test.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Mochat-Go-User-ID", "1")
			rec := httptest.NewRecorder()
			test.serve(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
	if store.mutationCalls != 0 {
		t.Fatalf("mutation calls = %d", store.mutationCalls)
	}
}

func TestSensitiveWordMutationMapsVersionConflictAndCarriesAuditScope(t *testing.T) {
	store := &fakeSensitiveWordStore{
		user:        User{ID: 1, TenantID: 8, IsSuperAdmin: 1},
		mutationErr: NewSensitiveWordConflict("敏感词版本已变化，请刷新后重试"),
	}
	handler := NewSensitiveWordHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodPut, "/dashboard/sensitiveWord/statusUpdate", strings.NewReader(`{"sensitiveWordId":11,"status":2,"version":"word-v1","idempotencyKey":"toggle-1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.StatusUpdate(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastMutation.ActorUserID != 1 || store.lastMutation.TenantID != 8 || store.lastMutation.CorpID != 7 || store.lastMutation.Action != SensitiveWordMutationSetStatus {
		t.Fatalf("mutation audit scope = %#v", store.lastMutation)
	}
}

func TestSensitiveWordsMonitorIndexAppliesRecordFilters(t *testing.T) {
	store := &fakeSensitiveWordStore{user: User{ID: 1, TenantID: 1, IsSuperAdmin: 1}}
	handler := NewSensitiveWordHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/sensitiveWordsMonitor/index?employeeId=3,5&workRoomId=9&intelligentGroupId=4&triggerStart=2026-07-01%2000:00:00&triggerEnd=2026-07-02%2000:00:00&page=2&perPage=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.MonitorIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	filter := store.lastMonitorFilter
	if strings.Trim(strings.Join([]string{filter.TriggerStart, filter.TriggerEnd}, ","), " ") != "2026-07-01 00:00:00,2026-07-02 00:00:00" || filter.WorkRoomID != 9 || filter.IntelligentGroupID != 4 || filter.Page != 2 || filter.PerPage != 20 || len(filter.EmployeeIDs) != 2 {
		t.Fatalf("filter = %#v", filter)
	}
}

func TestSensitiveWordIndexRejectsForbiddenRBAC(t *testing.T) {
	store := &fakeSensitiveWordStore{user: User{ID: 1, TenantID: 1}}
	handler := NewSensitiveWordHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, &recordingAuthorizer{err: ErrPermissionDenied})
	req := httptest.NewRequest(http.MethodGet, "/dashboard/sensitiveWord/index", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Index(rec, req)

	if rec.Code != http.StatusForbidden || store.pageCalls != 0 {
		t.Fatalf("status=%d pageCalls=%d body=%s", rec.Code, store.pageCalls, rec.Body.String())
	}
}

func TestSensitiveWordsMonitorShowReportsArchiveConnectionFailure(t *testing.T) {
	store := &fakeSensitiveWordStore{user: User{ID: 1, TenantID: 1, IsSuperAdmin: 1}, messageErr: errors.New("archive connection unavailable")}
	handler := NewSensitiveWordHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/sensitiveWordsMonitor/show?id=9", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.MonitorShow(rec, req)

	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "扫描结果暂时无法连接") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
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

func TestSensitiveWordsMonitorShowRestrictsMessagesToAuthorizedEmployees(t *testing.T) {
	store := &fakeSensitiveWordStore{
		user:     User{ID: 1, TenantID: 1},
		messages: []SensitiveWordsMonitorMessage{{Sender: "employee-81"}},
	}
	authorizer := &recordingAuthorizer{
		accessSet: true,
		access: AccessContext{
			User:            store.user,
			CorpID:          7,
			WorkEmployeeID:  81,
			DataPermission:  DataPermissionDepartment,
			DeptEmployeeIDs: []int{81, 82},
			PermissionKey:   "/dashboard/sensitiveWordsMonitor/show#get",
		},
	}
	handler := NewSensitiveWordHandler(store, staticAdminCache("7-81"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, authorizer)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/sensitiveWordsMonitor/show?id=9", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.MonitorShow(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	filter := store.lastMonitorMessageFilter
	if filter.CorpID != 7 || filter.MonitorID != 9 || !filter.RestrictEmployeeIDs || strings.Join(intSliceStrings(filter.AllowedEmployeeIDs), ",") != "81,82" {
		t.Fatalf("message filter = %#v", filter)
	}
}

func intSliceStrings(values []int) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, strconv.Itoa(value))
	}
	return result
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
	user                     User
	page                     SensitiveWordPage
	lastFilter               SensitiveWordFilter
	createdCorpID            int
	createdGroupID           int
	createdNames             []string
	createCalls              int
	pageCalls                int
	mutationCalls            int
	lastMutation             SensitiveWordMutation
	mutationErr              error
	replayFound              bool
	replayResult             SensitiveWordMutationResult
	groups                   []SensitiveWordGroup
	monitorPage              SensitiveWordsMonitorPage
	lastMonitorFilter        SensitiveWordsMonitorFilter
	messages                 []SensitiveWordsMonitorMessage
	messageErr               error
	lastMonitorCorpID        int
	lastMonitorID            int
	lastMonitorMessageFilter SensitiveWordsMonitorMessageFilter
	quota                    SaaSQuotaStatus
	quotaTenantID            int
	quotaMetric              string
	quotaAdditional          int64
	refreshTenantID          int
	refreshMetric            string
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
	s.pageCalls++
	s.lastFilter = filter
	return s.page, nil
}

func (s *fakeSensitiveWordStore) MutateSensitiveWords(_ context.Context, mutation SensitiveWordMutation) (SensitiveWordMutationResult, error) {
	s.mutationCalls++
	s.createCalls++
	s.lastMutation = mutation
	return SensitiveWordMutationResult{Version: "word-v2"}, s.mutationErr
}

func (s *fakeSensitiveWordStore) SensitiveWordMutationReplay(context.Context, SensitiveWordMutation) (SensitiveWordMutationResult, bool, error) {
	return s.replayResult, s.replayFound, nil
}

func (s *fakeSensitiveWordStore) SensitiveWordGroups(context.Context, int) ([]SensitiveWordGroup, error) {
	return s.groups, nil
}

func (s *fakeSensitiveWordStore) SensitiveWordsMonitorPage(_ context.Context, filter SensitiveWordsMonitorFilter) (SensitiveWordsMonitorPage, error) {
	s.lastMonitorFilter = filter
	return s.monitorPage, nil
}

func (s *fakeSensitiveWordStore) SensitiveWordsMonitorMessages(_ context.Context, filter SensitiveWordsMonitorMessageFilter) ([]SensitiveWordsMonitorMessage, bool, error) {
	s.lastMonitorCorpID = filter.CorpID
	s.lastMonitorID = filter.MonitorID
	s.lastMonitorMessageFilter = filter
	return s.messages, true, s.messageErr
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
