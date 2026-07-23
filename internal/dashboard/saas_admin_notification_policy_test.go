package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeSaaSAlertCredentialProtectionStore struct {
	*fakeSaaSAdminStore
	statuses       []SaaSAlertCredentialProtectionStatus
	statusCalls    int
	rotationResult SaaSAlertCredentialRotationResult
	rotationCalls  int
	rotationTenant int
	rotationLimit  int
}

func (s *fakeSaaSAlertCredentialProtectionStore) SaaSAlertCredentialProtection(_ context.Context) (SaaSAlertCredentialProtectionStatus, error) {
	index := s.statusCalls
	s.statusCalls++
	if len(s.statuses) == 0 {
		return SaaSAlertCredentialProtectionStatus{}, nil
	}
	if index >= len(s.statuses) {
		index = len(s.statuses) - 1
	}
	return s.statuses[index], nil
}

func (s *fakeSaaSAlertCredentialProtectionStore) RotateSaaSAlertCredentials(_ context.Context, tenantID int, limit int) (SaaSAlertCredentialRotationResult, error) {
	s.rotationCalls++
	s.rotationTenant = tenantID
	s.rotationLimit = limit
	return s.rotationResult, nil
}

func TestSaaSAdminNotificationPoliciesAllowsPlatformAdmin(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}},
		notificationPolicyReport: SaaSAdminNotificationPolicyReport{
			Summary: SaaSAdminNotificationPolicySummary{
				TenantCount:       3,
				ConfiguredCount:   2,
				EnabledCount:      1,
				DisabledCount:     1,
				UnconfiguredCount: 1,
				MatchedCount:      1,
			},
			Policies: []SaaSAdminNotificationPolicy{{
				TenantID:     10,
				TenantName:   "租户A",
				TenantStatus: 1,
				PackageCode:  "growth",
				PackageName:  "成长版",
				Configured:   false,
				Setting:      DefaultSaaSAlertSetting(10, SaaSAlertNotificationChannelWebhook),
			}},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/notificationPolicies?state=unconfigured&keyword=A&limit=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.NotificationPolicies(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.notificationPolicyCalls != 1 {
		t.Fatalf("notification policy calls = %d", store.notificationPolicyCalls)
	}
	if options := store.lastNotificationPolicyOptions; options.State != SaaSAdminNotificationPolicyStateUnconfigured || options.Keyword != "A" || options.Channel != SaaSAlertNotificationChannelWebhook || options.Limit != 20 {
		t.Fatalf("options = %+v", options)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["returnedCount"].(float64) != 1 {
		t.Fatalf("data = %+v", data)
	}
	security := data["webhookSecurity"].(map[string]any)
	if security["requireHttps"] != true || security["privateNetworksBlocked"] != true || security["dnsPinningEnabled"] != true || security["allowedCidrCount"].(float64) != 0 {
		t.Fatalf("webhook security = %+v", security)
	}
	summary := data["summary"].(map[string]any)
	if summary["tenantCount"].(float64) != 3 || summary["unconfiguredCount"].(float64) != 1 || summary["matchedCount"].(float64) != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	if strings.Contains(rec.Body.String(), "webhookSecret\"") {
		t.Fatalf("response leaked webhook secret: %s", rec.Body.String())
	}
}

func TestSaaSAdminNotificationPoliciesExposeCredentialProtection(t *testing.T) {
	store := &fakeSaaSAlertCredentialProtectionStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}},
		statuses: []SaaSAlertCredentialProtectionStatus{{
			EncryptionConfigured: true, RequireEncryption: true, DedicatedConfigured: true,
			ActiveKeyID: "alert-q3", KeyCount: 2, ConfiguredCredentialCount: 3,
			EncryptedCredentialCount: 2, LegacyPlaintextCount: 1, RotationRequiredCount: 2,
			UnavailableKeyCount: 1, UnavailableKeyIDs: []string{"retired"}, Healthy: false,
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/notificationPolicies", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.NotificationPolicies(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	protection := decodeSaaSAdminResponse(t, rec)["credentialProtection"].(map[string]any)
	if protection["supported"] != true || protection["activeKeyId"] != "alert-q3" ||
		protection["legacyPlaintextCount"].(float64) != 1 || protection["rotationRequiredCount"].(float64) != 2 ||
		protection["healthy"] != false || protection["rotationAvailable"] != true {
		t.Fatalf("credential protection=%+v", protection)
	}
	if strings.Contains(rec.Body.String(), "111111") || strings.Contains(rec.Body.String(), "webhookSecret\"") {
		t.Fatalf("credential response leaked secret material: %s", rec.Body.String())
	}
}

func TestSaaSAdminNotificationCredentialRotationAuditsCountsOnly(t *testing.T) {
	base := &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}}
	store := &fakeSaaSAlertCredentialProtectionStore{
		fakeSaaSAdminStore: base,
		statuses: []SaaSAlertCredentialProtectionStatus{
			{EncryptionConfigured: true, ActiveKeyID: "alert-q3", KeyCount: 2, ConfiguredCredentialCount: 3, LegacyPlaintextCount: 1, RotationRequiredCount: 2},
			{EncryptionConfigured: true, ActiveKeyID: "alert-q3", KeyCount: 2, ConfiguredCredentialCount: 3, EncryptedCredentialCount: 3, ActiveKeyCredentialCount: 3, Healthy: true},
		},
		rotationResult: SaaSAlertCredentialRotationResult{TenantID: 10, Limit: 25, ScannedCount: 2, RotatedCount: 2, LegacyCount: 1, ReencryptedCount: 1, ActiveKeyID: "alert-q3"},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/notificationCredentialRotation", strings.NewReader(`{"tenantId":10,"limit":25,"remark":"季度轮换"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.NotificationCredentialRotation(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.rotationCalls != 1 || store.rotationTenant != 10 || store.rotationLimit != 25 || store.statusCalls != 2 {
		t.Fatalf("rotation calls=%d tenant=%d limit=%d status_calls=%d", store.rotationCalls, store.rotationTenant, store.rotationLimit, store.statusCalls)
	}
	logItem := base.lastRecordedOperationLog
	if logItem.Action != SaaSAdminOperationActionNotificationCredentialRotate || logItem.TargetType != SaaSAdminOperationTargetNotificationCredential ||
		logItem.TargetID != "10" || logItem.TenantID != 10 || logItem.ActorUserID != 1 {
		t.Fatalf("operation log=%+v", logItem)
	}
	if strings.Contains(logItem.BeforeJSON+logItem.AfterJSON+rec.Body.String(), "https://") || strings.Contains(logItem.BeforeJSON+logItem.AfterJSON+rec.Body.String(), "secret") {
		t.Fatalf("rotation leaked credential material: before=%s after=%s body=%s", logItem.BeforeJSON, logItem.AfterJSON, rec.Body.String())
	}
	data := decodeSaaSAdminResponse(t, rec)
	rotation := data["rotation"].(map[string]any)
	if rotation["rotatedCount"].(float64) != 2 || data["operationId"].(float64) <= 0 {
		t.Fatalf("rotation response=%+v", data)
	}
}

func TestSaaSAdminNotificationCredentialRotationValidatesAndRequiresPlatformAdmin(t *testing.T) {
	t.Run("invalid limit", func(t *testing.T) {
		store := &fakeSaaSAlertCredentialProtectionStore{fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}}}
		handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
		req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/notificationCredentialRotation", strings.NewReader(`{"limit":1001}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Mochat-Go-User-ID", "1")
		rec := httptest.NewRecorder()
		handler.NotificationCredentialRotation(rec, req)
		if rec.Code != http.StatusBadRequest || store.rotationCalls != 0 {
			t.Fatalf("status=%d body=%s calls=%d", rec.Code, rec.Body.String(), store.rotationCalls)
		}
	})

	t.Run("tenant admin", func(t *testing.T) {
		store := &fakeSaaSAlertCredentialProtectionStore{fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, TenantID: 10, IsSuperAdmin: 1}}}}
		handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
		req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/notificationCredentialRotation", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Mochat-Go-User-ID", "7")
		rec := httptest.NewRecorder()
		handler.NotificationCredentialRotation(rec, req)
		if rec.Code != http.StatusForbidden || store.rotationCalls != 0 {
			t.Fatalf("status=%d body=%s calls=%d", rec.Code, rec.Body.String(), store.rotationCalls)
		}
	})
}

func TestSaaSAdminNotificationPolicyRejectsUnsafeWebhookDestination(t *testing.T) {
	existing := DefaultSaaSAlertSetting(10, SaaSAlertNotificationChannelWebhook)
	store := &fakeSaaSAdminStore{
		users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}},
		notificationPolicyReport: SaaSAdminNotificationPolicyReport{Policies: []SaaSAdminNotificationPolicy{{
			TenantID: 10, TenantName: "租户A", Configured: true, Setting: existing,
		}}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/notificationPolicy", strings.NewReader(`{"tenantId":10,"enabled":true,"webhookUrl":"https://127.0.0.1/internal","remark":"SSRF test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.NotificationPolicy(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "出站安全策略") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.alertSettingSaveCalls != 0 || store.recordOperationLogCalls != 0 {
		t.Fatalf("save calls=%d audit calls=%d", store.alertSettingSaveCalls, store.recordOperationLogCalls)
	}
}

func TestSaaSAdminNotificationPolicyTestRejectsLegacyUnsafeDestination(t *testing.T) {
	setting := NormalizeSaaSAlertSetting(SaaSAlertSetting{
		TenantID: 10, Channel: SaaSAlertNotificationChannelWebhook, Enabled: true,
		WebhookURL: "https://169.254.169.254/latest/meta-data",
	})
	store := &fakeSaaSAdminStore{
		users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}},
		notificationPolicyReport: SaaSAdminNotificationPolicyReport{Policies: []SaaSAdminNotificationPolicy{{
			TenantID: 10, TenantName: "租户A", Configured: true, Setting: setting,
		}}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/notificationPolicyTest", strings.NewReader(`{"tenantId":10}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.NotificationPolicyTest(rec, req)

	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "metadata endpoint") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.notificationEnqueueCalls != 0 || store.recordOperationLogCalls != 0 {
		t.Fatalf("enqueue=%d audit=%d", store.notificationEnqueueCalls, store.recordOperationLogCalls)
	}
}

func TestSaaSAdminNotificationPolicySavePreservesSecretAndAudits(t *testing.T) {
	existing := NormalizeSaaSAlertSetting(SaaSAlertSetting{
		ID:                            22,
		TenantID:                      10,
		Channel:                       SaaSAlertNotificationChannelWebhook,
		Enabled:                       false,
		WebhookURL:                    "https://old.example.com/hook",
		WebhookSecret:                 "existing-secret",
		NotificationMaxAttempts:       3,
		NotificationRetryDelaySeconds: 300,
	})
	store := &fakeSaaSAdminStore{
		users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}},
		notificationPolicyReport: SaaSAdminNotificationPolicyReport{Policies: []SaaSAdminNotificationPolicy{{
			TenantID:    10,
			TenantName:  "租户A",
			PackageCode: "growth",
			Configured:  true,
			Setting:     existing,
		}}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	body := `{"tenantId":10,"enabled":true,"webhookUrl":"https://new.example.com/hook","webhookTimeoutSeconds":8,"webhookRetryAttempts":2,"webhookRetryDelayMs":100,"notificationMaxAttempts":5,"notificationRetryDelaySeconds":120,"minimumSeverity":"critical","alertTypes":["quota_exceeded","admin_task_sla_reminder"],"quietHoursEnabled":true,"quietHoursStart":"22:30","quietHoursEnd":"07:30","timezone":"Asia/Shanghai","hourlyLimit":20,"remark":"平台代管通知"}`
	req := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/notificationPolicy", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.NotificationPolicy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.alertSettingSaveCalls != 1 {
		t.Fatalf("save calls = %d", store.alertSettingSaveCalls)
	}
	saved := store.lastSavedAlertSetting
	if !saved.Enabled || saved.WebhookURL != "https://new.example.com/hook" || saved.WebhookSecret != "existing-secret" {
		t.Fatalf("saved = %+v", saved)
	}
	if saved.NotificationMaxAttempts != 5 || saved.NotificationRetryDelaySeconds != 120 || saved.WebhookRetryAttempts != 2 {
		t.Fatalf("saved retries = %+v", saved)
	}
	if saved.MinimumSeverity != SaaSAlertSeverityCritical || len(saved.AllowedAlertTypes) != 2 || !saved.QuietHoursEnabled || saved.QuietHoursStart != "22:30" || saved.QuietHoursEnd != "07:30" || saved.HourlyLimit != 20 {
		t.Fatalf("saved policy controls = %+v", saved)
	}
	logItem := store.lastRecordedOperationLog
	if logItem.Action != SaaSAdminOperationActionNotificationPolicyUpdate || logItem.TargetType != SaaSAdminOperationTargetNotificationPolicy || logItem.TargetID != "10:webhook" || logItem.ActorUserID != 1 || logItem.TenantID != 10 {
		t.Fatalf("operation log = %+v", logItem)
	}
	if strings.Contains(logItem.BeforeJSON, "existing-secret") || strings.Contains(logItem.AfterJSON, "existing-secret") || strings.Contains(rec.Body.String(), "existing-secret") {
		t.Fatalf("secret leaked: before=%s after=%s body=%s", logItem.BeforeJSON, logItem.AfterJSON, rec.Body.String())
	}
	data := decodeSaaSAdminResponse(t, rec)
	policy := data["policy"].(map[string]any)
	setting := policy["setting"].(map[string]any)
	if setting["webhookSecretConfigured"] != true {
		t.Fatalf("setting = %+v", setting)
	}
	if setting["minimumSeverity"] != SaaSAlertSeverityCritical || setting["quietHoursEnabled"] != true || setting["hourlyLimit"].(float64) != 20 {
		t.Fatalf("setting policy = %+v", setting)
	}
	if _, leaked := setting["webhookSecret"]; leaked {
		t.Fatalf("setting leaked secret = %+v", setting)
	}
}

func TestSaaSAdminNotificationPolicyTestEnqueuesAndAudits(t *testing.T) {
	setting := NormalizeSaaSAlertSetting(SaaSAlertSetting{
		ID:                      22,
		TenantID:                10,
		Channel:                 SaaSAlertNotificationChannelWebhook,
		Enabled:                 true,
		WebhookURL:              "https://alerts.example.com/hook",
		NotificationMaxAttempts: 7,
	})
	store := &fakeSaaSAdminStore{
		users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}},
		notificationPolicyReport: SaaSAdminNotificationPolicyReport{Policies: []SaaSAdminNotificationPolicy{{
			TenantID:   10,
			TenantName: "租户A",
			Configured: true,
			Setting:    setting,
		}}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/notificationPolicyTest", strings.NewReader(`{"tenantId":10,"remark":"上线前测试"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.NotificationPolicyTest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.notificationEnqueueCalls != 1 || store.lastEnqueuedMaxAttempts != 7 || store.lastEnqueuedChannel != SaaSAlertNotificationChannelWebhook {
		t.Fatalf("enqueue calls=%d max=%d channel=%s", store.notificationEnqueueCalls, store.lastEnqueuedMaxAttempts, store.lastEnqueuedChannel)
	}
	alert := store.lastEnqueuedAlert
	if alert.Status.TenantID != 10 || alert.Status.Metric != SaaSEventMetricNotificationPolicy || alert.AlertType != SaaSAlertTypeNotificationPolicyTest || !strings.HasPrefix(alert.PeriodKey, "notification_policy_test_") {
		t.Fatalf("alert = %+v", alert)
	}
	if logItem := store.lastRecordedOperationLog; logItem.Action != SaaSAdminOperationActionNotificationPolicyTest || logItem.TargetType != SaaSAdminOperationTargetAlertNotification || logItem.TenantID != 10 {
		t.Fatalf("operation log = %+v", logItem)
	}
	data := decodeSaaSAdminResponse(t, rec)
	notification := data["notification"].(map[string]any)
	if notification["status"] != SaaSAlertNotificationStatusPending {
		t.Fatalf("notification = %+v", notification)
	}
}

func TestSaaSAdminNotificationPolicyRequiresPlatformAdminAndEnabledPolicy(t *testing.T) {
	t.Run("tenant admin", func(t *testing.T) {
		store := &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, TenantID: 10, IsSuperAdmin: 1}}}
		handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
		for _, test := range []struct {
			method string
			path   string
			body   string
			call   func(http.ResponseWriter, *http.Request)
		}{
			{http.MethodGet, "/dashboard/saasAdmin/notificationPolicies", "", handler.NotificationPolicies},
			{http.MethodPut, "/dashboard/saasAdmin/notificationPolicy", `{"tenantId":10}`, handler.NotificationPolicy},
			{http.MethodPost, "/dashboard/saasAdmin/notificationPolicyTest", `{"tenantId":10}`, handler.NotificationPolicyTest},
		} {
			req := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Mochat-Go-User-ID", "7")
			rec := httptest.NewRecorder()
			test.call(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s %s status=%d body=%s", test.method, test.path, rec.Code, rec.Body.String())
			}
		}
	})

	t.Run("disabled policy", func(t *testing.T) {
		store := &fakeSaaSAdminStore{
			users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}},
			notificationPolicyReport: SaaSAdminNotificationPolicyReport{Policies: []SaaSAdminNotificationPolicy{{
				TenantID:   10,
				TenantName: "租户A",
				Configured: true,
				Setting: NormalizeSaaSAlertSetting(SaaSAlertSetting{
					TenantID:   10,
					Channel:    SaaSAlertNotificationChannelWebhook,
					Enabled:    false,
					WebhookURL: "https://alerts.example.com/hook",
				}),
			}}},
		}
		handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
		req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/notificationPolicyTest", strings.NewReader(`{"tenantId":10}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Mochat-Go-User-ID", "1")
		rec := httptest.NewRecorder()

		handler.NotificationPolicyTest(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
		}
		if store.notificationEnqueueCalls != 0 || store.recordOperationLogCalls != 0 {
			t.Fatalf("enqueue=%d audit=%d", store.notificationEnqueueCalls, store.recordOperationLogCalls)
		}
	})
}

func TestSaaSAdminNotificationsExposeSuppressedPolicyResult(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}},
		notifications: []SaaSAlertNotification{{
			ID:              91,
			NotificationKey: "10:users:quota_exceeded:lifetime:webhook",
			TenantID:        10,
			Channel:         SaaSAlertNotificationChannelWebhook,
			Status:          SaaSAlertNotificationStatusSuppressed,
			LastError:       "policy suppressed: alert type is not subscribed",
			Alert:           SaaSQuotaAlert{Status: SaaSQuotaStatus{TenantID: 10, Metric: SaaSMetricUsers}, AlertType: SaaSAlertTypeQuotaExceeded},
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/notifications?tenantId=10&status=suppressed&limit=10", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Notifications(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeSaaSAdminResponse(t, rec)
	summary := data["summary"].(map[string]any)
	if summary["suppressedCount"].(float64) != 1 || summary["notificationCount"].(float64) != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	items := data["notifications"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["status"] != SaaSAlertNotificationStatusSuppressed {
		t.Fatalf("notifications = %+v", items)
	}
}
