package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSaaSAlertIndexFiltersTenantAndStatus(t *testing.T) {
	store := &fakeSaaSAlertDashboardStore{
		users: map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		page: SaaSAlertListPage{
			Items: []SaaSAlertRecord{{
				ID:              99,
				AlertKey:        "tenant:10:async_executions:quota_exceeded:lifetime",
				TenantID:        10,
				AlertType:       SaaSAlertTypeQuotaExceeded,
				Severity:        SaaSAlertSeverityWarning,
				Status:          SaaSAlertStatusOpen,
				Metric:          SaaSMetricAsyncExecutions,
				PeriodKey:       SaaSAlertPeriodLifetime,
				CurrentValue:    2,
				LimitValue:      1,
				AdditionalValue: 0,
				OccurrenceCount: 3,
				Source:          "worker.queue_item",
				Message:         "套餐额度已达上限：异步执行量 2/1",
				ContextJSON:     `{"task":"employee-apply"}`,
				FirstSeenAt:     "2026-07-04T10:00:00+08:00",
				LastSeenAt:      "2026-07-04T10:01:00+08:00",
			}},
			Total:     1,
			TotalPage: 1,
		},
	}
	handler := NewSaaSAlertHandler(store, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAlert/index?metric=async_executions&page=2&perPage=5", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastOptions.TenantID != 10 || store.lastOptions.Status != SaaSAlertStatusOpen || store.lastOptions.Metric != SaaSMetricAsyncExecutions || store.lastOptions.AlertType != SaaSAlertTypeQuotaExceeded || store.lastOptions.Page != 2 || store.lastOptions.PerPage != 5 {
		t.Fatalf("options = %+v", store.lastOptions)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	page := data["page"].(map[string]any)
	if page["total"].(float64) != 1 || page["totalPage"].(float64) != 1 || page["perPage"].(float64) != 5 {
		t.Fatalf("page = %+v", page)
	}
	list := data["list"].([]any)
	item := list[0].(map[string]any)
	if item["tenantId"].(float64) != 10 || item["metric"] != SaaSMetricAsyncExecutions || item["currentValue"].(float64) != 2 {
		t.Fatalf("item = %+v", item)
	}
	contextPayload := item["context"].(map[string]any)
	if contextPayload["task"] != "employee-apply" {
		t.Fatalf("context = %+v", contextPayload)
	}
}

func TestSaaSAlertIndexSupportsAllStatus(t *testing.T) {
	store := &fakeSaaSAlertDashboardStore{
		users: map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
	}
	handler := NewSaaSAlertHandler(store, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAlert/index?status=all", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastOptions.Status != "" {
		t.Fatalf("status filter = %q, want empty", store.lastOptions.Status)
	}
}

func TestSaaSAlertResolveUsesCurrentTenant(t *testing.T) {
	store := &fakeSaaSAlertDashboardStore{
		users:    map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		resolved: true,
	}
	handler := NewSaaSAlertHandler(store, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodPut, "/dashboard/saasAlert/resolve", strings.NewReader(`{"metric":"async_executions"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Resolve(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.resolvedTenantID != 10 || store.resolvedMetric != SaaSMetricAsyncExecutions || store.resolvedAlertType != SaaSAlertTypeQuotaExceeded {
		t.Fatalf("resolve args tenant=%d metric=%q alertType=%q", store.resolvedTenantID, store.resolvedMetric, store.resolvedAlertType)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if data["resolved"] != true {
		t.Fatalf("data = %+v", data)
	}
}

func TestSaaSAlertRejectsNonSuperAdmin(t *testing.T) {
	store := &fakeSaaSAlertDashboardStore{
		users: map[int]User{2: {ID: 2, TenantID: 10, IsSuperAdmin: 0}},
	}
	handler := NewSaaSAlertHandler(store, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAlert/index", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "2")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.listCalled {
		t.Fatalf("ListSaaSAlerts should not be called")
	}
}

func TestSaaSAlertResolveRejectsMissingMetric(t *testing.T) {
	store := &fakeSaaSAlertDashboardStore{
		users: map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
	}
	handler := NewSaaSAlertHandler(store, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodPut, "/dashboard/saasAlert/resolve", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Resolve(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.resolveCalled {
		t.Fatalf("ResolveSaaSAlert should not be called")
	}
}

func TestSaaSAlertSettingShowsDefault(t *testing.T) {
	store := &fakeSaaSAlertDashboardStore{
		users: map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
	}
	handler := NewSaaSAlertHandler(store, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAlert/setting", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Setting(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.settingTenantID != 10 || store.settingChannel != SaaSAlertNotificationChannelWebhook {
		t.Fatalf("setting lookup tenant=%d channel=%q", store.settingTenantID, store.settingChannel)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	setting := data["setting"].(map[string]any)
	if setting["tenantId"].(float64) != 10 || setting["enabled"] != false || setting["exists"] != false {
		t.Fatalf("setting = %+v", setting)
	}
	if setting["webhookTimeoutSeconds"].(float64) != 5 || setting["webhookSecretConfigured"] != false {
		t.Fatalf("setting defaults = %+v", setting)
	}
	if setting["minimumSeverity"] != SaaSAlertSeverityWarning || setting["quietHoursEnabled"] != false || setting["hourlyLimit"].(float64) != 0 {
		t.Fatalf("policy defaults = %+v", setting)
	}
	if len(setting["alertTypes"].([]any)) != 0 || setting["quietHoursStart"] != "22:00" || setting["quietHoursEnd"] != "08:00" || setting["timezone"] != "Asia/Shanghai" {
		t.Fatalf("policy defaults = %+v", setting)
	}
	if _, leaked := setting["webhookSecret"]; leaked {
		t.Fatalf("webhook secret should not be returned")
	}
}

func TestSaaSAlertSettingSavePreservesExistingSecret(t *testing.T) {
	store := &fakeSaaSAlertDashboardStore{
		users: map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		setting: SaaSAlertSetting{
			ID:            7,
			TenantID:      10,
			Channel:       SaaSAlertNotificationChannelWebhook,
			Enabled:       false,
			WebhookSecret: "old-secret",
		},
		settingExists: true,
	}
	handler := NewSaaSAlertHandler(store, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodPut, "/dashboard/saasAlert/setting", strings.NewReader(`{
		"enabled": true,
		"webhookUrl": "https://alerts.example.com/hook",
		"webhookTimeoutSeconds": 9,
		"webhookRetryAttempts": 2,
		"webhookRetryDelayMs": 125,
		"webhookTitleTemplate": "租户 {{.TenantID}} 告警",
		"webhookBodyTemplate": "当前 {{.CurrentValue}}/{{.LimitValue}}",
		"notificationMaxAttempts": 4,
		"notificationRetryDelaySeconds": 60,
		"minimumSeverity": "critical",
		"alertTypes": ["quota_exceeded", "tenant_renewal_reminder"],
		"quietHoursEnabled": true,
		"quietHoursStart": "21:30",
		"quietHoursEnd": "07:15",
		"timezone": "Asia/Shanghai",
		"hourlyLimit": 12
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Setting(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !store.saveSettingCalled {
		t.Fatalf("SaveSaaSAlertSetting should be called")
	}
	if !store.savedSetting.Enabled || store.savedSetting.WebhookURL != "https://alerts.example.com/hook" || store.savedSetting.WebhookSecret != "old-secret" {
		t.Fatalf("saved setting = %+v", store.savedSetting)
	}
	if store.savedSetting.WebhookTimeoutSeconds != 9 || store.savedSetting.WebhookRetryAttempts != 2 || store.savedSetting.NotificationMaxAttempts != 4 {
		t.Fatalf("saved numeric setting = %+v", store.savedSetting)
	}
	if store.savedSetting.MinimumSeverity != SaaSAlertSeverityCritical || len(store.savedSetting.AllowedAlertTypes) != 2 || !store.savedSetting.QuietHoursEnabled || store.savedSetting.HourlyLimit != 12 {
		t.Fatalf("saved policy setting = %+v", store.savedSetting)
	}
	body := decodeBody(t, rec.Body.Bytes())
	setting := body["data"].(map[string]any)["setting"].(map[string]any)
	if setting["webhookSecretConfigured"] != true {
		t.Fatalf("response setting = %+v", setting)
	}
	if _, leaked := setting["webhookSecret"]; leaked {
		t.Fatalf("webhook secret should not be returned")
	}
}

func TestSaaSAlertSettingRejectsInvalidWebhookURL(t *testing.T) {
	store := &fakeSaaSAlertDashboardStore{
		users: map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
	}
	handler := NewSaaSAlertHandler(store, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAlert/setting", strings.NewReader(`{"enabled":true,"webhookUrl":"not-a-url"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Setting(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.saveSettingCalled {
		t.Fatalf("SaveSaaSAlertSetting should not be called")
	}
}

func TestSaaSAlertSettingRejectsPrivateWebhookDestination(t *testing.T) {
	store := &fakeSaaSAlertDashboardStore{
		users: map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
	}
	handler := NewSaaSAlertHandler(store, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAlert/setting", strings.NewReader(`{"enabled":true,"webhookUrl":"https://10.0.0.8/internal"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Setting(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "出站安全策略") {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.saveSettingCalled {
		t.Fatal("SaveSaaSAlertSetting should not be called")
	}
}

func TestSaaSAlertPageServesStandaloneConsole(t *testing.T) {
	handler := NewSaaSAlertPageHandler()

	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAlert/page", nil)
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
		"MoChat Go SaaS 告警管理",
		"/dashboard/saasAlert/index",
		"/dashboard/saasAlert/resolve",
		"/dashboard/saasAlert/setting",
		"mochat_go_saas_alert_token",
		"async_executions",
		"minimumSeverity",
		"alertTypeQuota",
		"alertTypePaymentFailed",
		"payment_failed_reminder",
		"quietHoursEnabled",
		"settingTimezone",
		"hourlyLimit",
		"selectedAlertTypes",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("page missing %q", want)
		}
	}
	if strings.Contains(body, "http://") || strings.Contains(body, "https://") {
		t.Fatalf("page should not depend on external assets")
	}
}

func TestSaaSAlertPageRejectsPost(t *testing.T) {
	handler := NewSaaSAlertPageHandler()

	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAlert/page", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("Allow = %q", rec.Header().Get("Allow"))
	}
}

type fakeSaaSAlertDashboardStore struct {
	users             map[int]User
	page              SaaSAlertListPage
	lastOptions       SaaSAlertListOptions
	listCalled        bool
	resolved          bool
	resolveCalled     bool
	resolvedTenantID  int
	resolvedMetric    string
	resolvedAlertType string
	setting           SaaSAlertSetting
	settingExists     bool
	settingTenantID   int
	settingChannel    string
	savedSetting      SaaSAlertSetting
	saveSettingCalled bool
}

func (s *fakeSaaSAlertDashboardStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeSaaSAlertDashboardStore) ListSaaSAlerts(_ context.Context, options SaaSAlertListOptions) (SaaSAlertListPage, error) {
	s.listCalled = true
	s.lastOptions = options
	return s.page, nil
}

func (s *fakeSaaSAlertDashboardStore) ResolveSaaSAlert(_ context.Context, tenantID int, metric string, alertType string) (bool, error) {
	s.resolveCalled = true
	s.resolvedTenantID = tenantID
	s.resolvedMetric = metric
	s.resolvedAlertType = alertType
	return s.resolved, nil
}

func (s *fakeSaaSAlertDashboardStore) GetSaaSAlertSetting(_ context.Context, tenantID int, channel string) (SaaSAlertSetting, bool, error) {
	s.settingTenantID = tenantID
	s.settingChannel = channel
	return s.setting, s.settingExists, nil
}

func (s *fakeSaaSAlertDashboardStore) SaveSaaSAlertSetting(_ context.Context, setting SaaSAlertSetting) (SaaSAlertSetting, error) {
	s.saveSettingCalled = true
	s.savedSetting = setting
	s.savedSetting.ID = 7
	return s.savedSetting, nil
}
