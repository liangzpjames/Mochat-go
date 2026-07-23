package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPersistentSaaSAlertNotifierMarksDelivered(t *testing.T) {
	store := &fakeSaaSAlertNotificationStore{}
	inner := &fakeNotificationNotifier{}
	notifier := NewPersistentSaaSAlertNotifier(store, inner, SaaSAlertNotificationChannelWebhook, 3, time.Minute)

	if err := notifier.NotifySaaSQuotaAlert(context.Background(), sampleNotificationAlert()); err != nil {
		t.Fatal(err)
	}
	if store.enqueued.ID != 1 || store.enqueued.MaxAttempts != 3 {
		t.Fatalf("enqueued = %+v", store.enqueued)
	}
	if store.deliveredID != 1 {
		t.Fatalf("deliveredID = %d", store.deliveredID)
	}
	if inner.calls != 1 {
		t.Fatalf("inner calls = %d", inner.calls)
	}
}

func TestPersistentSaaSAlertNotifierMarksFailed(t *testing.T) {
	store := &fakeSaaSAlertNotificationStore{}
	inner := &fakeNotificationNotifier{err: errors.New("webhook down")}
	notifier := NewPersistentSaaSAlertNotifier(store, inner, SaaSAlertNotificationChannelWebhook, 3, 2*time.Minute)

	err := notifier.NotifySaaSQuotaAlert(context.Background(), sampleNotificationAlert())
	if err == nil {
		t.Fatal("expected notifier error")
	}
	if store.failedID != 1 || store.failedDelay != 2*time.Minute || store.failedError != "webhook down" {
		t.Fatalf("failed = id:%d delay:%s err:%q", store.failedID, store.failedDelay, store.failedError)
	}
}

func TestPersistentSaaSAlertNotifierUsesTenantNotificationSetting(t *testing.T) {
	store := &fakeSaaSAlertNotificationStoreWithSetting{
		setting: SaaSAlertSetting{
			TenantID:                      7,
			Channel:                       SaaSAlertNotificationChannelWebhook,
			Enabled:                       true,
			WebhookURL:                    "http://alerts.example.com/hook",
			NotificationMaxAttempts:       7,
			NotificationRetryDelaySeconds: 12,
		},
		found: true,
	}
	inner := &fakeNotificationNotifier{err: errors.New("webhook down")}
	notifier := NewPersistentSaaSAlertNotifier(store, inner, SaaSAlertNotificationChannelWebhook, 3, 2*time.Minute)

	err := notifier.NotifySaaSQuotaAlert(context.Background(), sampleNotificationAlert())
	if err == nil {
		t.Fatal("expected notifier error")
	}
	if store.enqueued.MaxAttempts != 7 {
		t.Fatalf("max attempts = %d", store.enqueued.MaxAttempts)
	}
	if store.failedDelay != 12*time.Second {
		t.Fatalf("failedDelay = %s", store.failedDelay)
	}
}

func TestDispatchDueSaaSAlertNotifications(t *testing.T) {
	store := &fakeSaaSAlertNotificationStore{
		due: []SaaSAlertNotification{{
			ID:       10,
			Alert:    sampleNotificationAlert(),
			Attempts: 1,
		}},
	}
	inner := &fakeNotificationNotifier{}

	result, err := DispatchDueSaaSAlertNotifications(context.Background(), store, inner, 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanned != 1 || result.Delivered != 1 || result.Failed != 0 || result.Dead != 0 {
		t.Fatalf("result = %+v", result)
	}
	if store.deliveredID != 10 {
		t.Fatalf("deliveredID = %d", store.deliveredID)
	}
}

func TestDispatchDueSaaSAlertNotificationsUsesTenantRetryDelay(t *testing.T) {
	store := &fakeSaaSAlertNotificationStoreWithSetting{
		fakeSaaSAlertNotificationStore: fakeSaaSAlertNotificationStore{
			due: []SaaSAlertNotification{{
				ID:       10,
				TenantID: 7,
				Channel:  SaaSAlertNotificationChannelWebhook,
				Alert:    sampleNotificationAlert(),
			}},
		},
		setting: SaaSAlertSetting{
			TenantID:                      7,
			Channel:                       SaaSAlertNotificationChannelWebhook,
			Enabled:                       true,
			WebhookURL:                    "http://alerts.example.com/hook",
			NotificationRetryDelaySeconds: 17,
		},
		found: true,
	}
	inner := &fakeNotificationNotifier{err: errors.New("temporary failure")}

	result, err := DispatchDueSaaSAlertNotifications(context.Background(), store, inner, 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if result.Failed != 1 || store.failedDelay != 17*time.Second {
		t.Fatalf("result=%+v failedDelay=%s", result, store.failedDelay)
	}
}

func TestPersistentSaaSAlertNotifierSuppressesUnsubscribedAlert(t *testing.T) {
	store := &fakeSaaSAlertNotificationStoreWithSetting{
		setting: SaaSAlertSetting{
			TenantID:          7,
			Channel:           SaaSAlertNotificationChannelWebhook,
			Enabled:           true,
			AllowedAlertTypes: []string{SaaSAlertTypeTenantRenewal},
		},
		found: true,
	}
	inner := &fakeNotificationNotifier{}
	notifier := NewPersistentSaaSAlertNotifier(store, inner, SaaSAlertNotificationChannelWebhook, 3, time.Minute)

	if err := notifier.NotifySaaSQuotaAlert(context.Background(), sampleNotificationAlert()); err != nil {
		t.Fatal(err)
	}
	if store.enqueued.ID != 1 || store.suppressedID != 1 || !strings.Contains(store.suppressedReason, "not subscribed") {
		t.Fatalf("store = %+v", store.fakeSaaSAlertNotificationStore)
	}
	if inner.calls != 0 {
		t.Fatalf("notifier calls = %d", inner.calls)
	}
}

func TestDispatchDueSaaSAlertNotificationsSuppressesBySeverity(t *testing.T) {
	store := &fakeSaaSAlertNotificationStoreWithSetting{
		fakeSaaSAlertNotificationStore: fakeSaaSAlertNotificationStore{due: []SaaSAlertNotification{{
			ID:       10,
			TenantID: 7,
			Channel:  SaaSAlertNotificationChannelWebhook,
			Alert:    sampleNotificationAlert(),
		}}},
		setting: SaaSAlertSetting{
			TenantID:        7,
			Channel:         SaaSAlertNotificationChannelWebhook,
			Enabled:         true,
			MinimumSeverity: SaaSAlertSeverityCritical,
		},
		found: true,
	}
	inner := &fakeNotificationNotifier{}

	result, err := dispatchDueSaaSAlertNotificationsAt(context.Background(), store, inner, 10, time.Minute, func() time.Time {
		return time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Suppressed != 1 || store.suppressedID != 10 || !strings.Contains(store.suppressedReason, "below minimum") || inner.calls != 0 {
		t.Fatalf("result=%+v store=%+v calls=%d", result, store.fakeSaaSAlertNotificationStore, inner.calls)
	}
}

func TestDispatchDueSaaSAlertNotificationsDefersDuringQuietHours(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 10, 23, 30, 0, 0, location)
	store := &fakeSaaSAlertNotificationStoreWithSetting{
		fakeSaaSAlertNotificationStore: fakeSaaSAlertNotificationStore{due: []SaaSAlertNotification{{
			ID:       10,
			TenantID: 7,
			Channel:  SaaSAlertNotificationChannelWebhook,
			Alert:    sampleNotificationAlert(),
		}}},
		setting: SaaSAlertSetting{
			TenantID:          7,
			Channel:           SaaSAlertNotificationChannelWebhook,
			Enabled:           true,
			QuietHoursEnabled: true,
			QuietHoursStart:   "22:00",
			QuietHoursEnd:     "08:00",
			Timezone:          "Asia/Shanghai",
		},
		found: true,
	}
	inner := &fakeNotificationNotifier{}

	result, err := dispatchDueSaaSAlertNotificationsAt(context.Background(), store, inner, 10, time.Minute, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	wantRetry := time.Date(2026, 7, 11, 8, 0, 0, 0, location)
	if result.Deferred != 1 || store.deferredID != 10 || !store.deferredUntil.Equal(wantRetry) || !strings.Contains(store.deferredReason, "quiet hours") || inner.calls != 0 {
		t.Fatalf("result=%+v store=%+v calls=%d", result, store.fakeSaaSAlertNotificationStore, inner.calls)
	}
}

func TestDispatchDueSaaSAlertNotificationsDefersAtHourlyLimit(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	nextAvailable := now.Add(30 * time.Minute)
	store := &fakeSaaSAlertNotificationStoreWithSetting{
		fakeSaaSAlertNotificationStore: fakeSaaSAlertNotificationStore{
			due: []SaaSAlertNotification{{
				ID:       10,
				TenantID: 7,
				Channel:  SaaSAlertNotificationChannelWebhook,
				Alert:    sampleNotificationAlert(),
			}},
			deliveryWindow: SaaSAlertNotificationDeliveryWindow{DeliveredCount: 2, NextAvailableAt: nextAvailable},
		},
		setting: SaaSAlertSetting{
			TenantID:    7,
			Channel:     SaaSAlertNotificationChannelWebhook,
			Enabled:     true,
			HourlyLimit: 2,
		},
		found: true,
	}
	inner := &fakeNotificationNotifier{}

	result, err := dispatchDueSaaSAlertNotificationsAt(context.Background(), store, inner, 10, time.Minute, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if result.Deferred != 1 || store.deferredID != 10 || !store.deferredUntil.Equal(nextAvailable) || !strings.Contains(store.deferredReason, "hourly limit 2") || inner.calls != 0 {
		t.Fatalf("result=%+v store=%+v calls=%d", result, store.fakeSaaSAlertNotificationStore, inner.calls)
	}
	if store.deliveryWindowTenantID != 7 || store.deliveryWindowChannel != SaaSAlertNotificationChannelWebhook || store.deliveryWindowDuration != time.Hour {
		t.Fatalf("delivery window lookup = tenant:%d channel:%s duration:%s", store.deliveryWindowTenantID, store.deliveryWindowChannel, store.deliveryWindowDuration)
	}
}

func TestDispatchDueSaaSAlertNotificationsPolicyTestBypassesControls(t *testing.T) {
	now := time.Date(2026, 7, 10, 23, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	alert := sampleNotificationAlert()
	alert.Status.Metric = SaaSEventMetricNotificationPolicy
	alert.AlertType = SaaSAlertTypeNotificationPolicyTest
	store := &fakeSaaSAlertNotificationStoreWithSetting{
		fakeSaaSAlertNotificationStore: fakeSaaSAlertNotificationStore{
			due:            []SaaSAlertNotification{{ID: 10, TenantID: 7, Channel: SaaSAlertNotificationChannelWebhook, Alert: alert}},
			deliveryWindow: SaaSAlertNotificationDeliveryWindow{DeliveredCount: 99, NextAvailableAt: now.Add(time.Hour)},
		},
		setting: SaaSAlertSetting{
			TenantID:          7,
			Channel:           SaaSAlertNotificationChannelWebhook,
			Enabled:           true,
			MinimumSeverity:   SaaSAlertSeverityCritical,
			AllowedAlertTypes: []string{SaaSAlertTypeTenantRenewal},
			QuietHoursEnabled: true,
			QuietHoursStart:   "22:00",
			QuietHoursEnd:     "08:00",
			Timezone:          "Asia/Shanghai",
			HourlyLimit:       1,
		},
		found: true,
	}
	inner := &fakeNotificationNotifier{}

	result, err := dispatchDueSaaSAlertNotificationsAt(context.Background(), store, inner, 10, time.Minute, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if result.Delivered != 1 || result.Deferred != 0 || result.Suppressed != 0 || store.deliveredID != 10 || inner.calls != 1 {
		t.Fatalf("result=%+v store=%+v calls=%d", result, store.fakeSaaSAlertNotificationStore, inner.calls)
	}
}

func TestSaaSAlertNotificationDispatchCronRunsOnce(t *testing.T) {
	store := &fakeSaaSAlertNotificationStore{
		due: []SaaSAlertNotification{{
			ID:    10,
			Alert: sampleNotificationAlert(),
		}},
	}
	inner := &fakeNotificationNotifier{}
	cron := NewSaaSAlertNotificationDispatchCron(store, inner, 25, 3*time.Second, nil)

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.listLimit != 25 {
		t.Fatalf("listLimit = %d", store.listLimit)
	}
	if store.deliveredID != 10 || inner.calls != 1 {
		t.Fatalf("deliveredID=%d notifier calls=%d", store.deliveredID, inner.calls)
	}
}

func TestSaaSAlertNotificationDispatchCronRequiresDependencies(t *testing.T) {
	err := NewSaaSAlertNotificationDispatchCron(nil, nil, 0, 0, nil).RunOnce(context.Background())
	if err == nil || err.Error() != "SaaS alert notification dispatch cron dependencies are not configured" {
		t.Fatalf("error = %v", err)
	}
}

func TestStoreBackedSaaSAlertNotifierUsesTenantSetting(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	fallback := &fakeNotificationNotifier{err: errors.New("fallback should not be used")}
	store := &fakeSaaSAlertSettingReader{
		found: true,
		setting: SaaSAlertSetting{
			TenantID:                      7,
			Channel:                       SaaSAlertNotificationChannelWebhook,
			Enabled:                       true,
			WebhookURL:                    server.URL,
			WebhookTimeoutSeconds:         2,
			WebhookRetryAttempts:          1,
			WebhookRetryDelayMS:           0,
			WebhookTitleTemplate:          "租户 {{.TenantID}} {{.Metric}}",
			WebhookBodyTemplate:           "当前 {{.CurrentValue}}/{{.LimitValue}}",
			NotificationMaxAttempts:       3,
			NotificationRetryDelaySeconds: 300,
		},
	}
	notifier := NewStoreBackedSaaSAlertNotifier(store, fallback, SaaSAlertNotificationChannelWebhook, testSaaSAlertWebhookGuard(t))

	if err := notifier.NotifySaaSQuotaAlert(context.Background(), sampleNotificationAlert()); err != nil {
		t.Fatal(err)
	}
	if fallback.calls != 0 {
		t.Fatalf("fallback calls = %d", fallback.calls)
	}
	if store.tenantID != 7 || store.channel != SaaSAlertNotificationChannelWebhook {
		t.Fatalf("lookup tenant=%d channel=%q", store.tenantID, store.channel)
	}
	if payload["tenantId"].(float64) != 7 || payload["title"] != "租户 7 async_executions" || payload["body"] != "当前 12/10" {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestStoreBackedSaaSAlertNotifierDisabledSuppressesFallback(t *testing.T) {
	fallback := &fakeNotificationNotifier{}
	store := &fakeSaaSAlertSettingReader{
		found: true,
		setting: SaaSAlertSetting{
			TenantID: 7,
			Channel:  SaaSAlertNotificationChannelWebhook,
			Enabled:  false,
		},
	}
	notifier := NewStoreBackedSaaSAlertNotifier(store, fallback, SaaSAlertNotificationChannelWebhook)

	if err := notifier.NotifySaaSQuotaAlert(context.Background(), sampleNotificationAlert()); err != nil {
		t.Fatal(err)
	}
	if fallback.calls != 0 {
		t.Fatalf("fallback calls = %d", fallback.calls)
	}
}

func TestStoreBackedSaaSAlertNotifierReportsMissingConfig(t *testing.T) {
	store := &fakeSaaSAlertSettingReader{}
	notifier := NewStoreBackedSaaSAlertNotifier(store, nil, SaaSAlertNotificationChannelWebhook)

	err := notifier.NotifySaaSQuotaAlert(context.Background(), sampleNotificationAlert())
	if err == nil || !strings.Contains(err.Error(), "SaaS alert webhook is not configured for tenant 7") {
		t.Fatalf("error = %v", err)
	}
}

func sampleNotificationAlert() SaaSQuotaAlert {
	return SaaSQuotaAlert{
		Status:    SaaSQuotaStatus{Metric: SaaSMetricAsyncExecutions, TenantID: 7, Current: 12, Limit: 10},
		AlertType: SaaSAlertTypeQuotaExceeded,
		Severity:  SaaSAlertSeverityWarning,
		PeriodKey: SaaSAlertPeriodLifetime,
		Source:    "worker.queue_item",
		Message:   "套餐额度已达上限：异步执行量 12/10",
		Context:   map[string]any{"nonBlocking": true},
	}
}

type fakeNotificationNotifier struct {
	calls int
	err   error
}

func (n *fakeNotificationNotifier) NotifySaaSQuotaAlert(context.Context, SaaSQuotaAlert) error {
	n.calls++
	return n.err
}

type fakeSaaSAlertNotificationStore struct {
	enqueued               SaaSAlertNotification
	due                    []SaaSAlertNotification
	listLimit              int
	deliveredID            int64
	failedID               int64
	failedDelay            time.Duration
	failedError            string
	failedStatus           string
	deferredID             int64
	deferredUntil          time.Time
	deferredReason         string
	suppressedID           int64
	suppressedReason       string
	deliveryWindow         SaaSAlertNotificationDeliveryWindow
	deliveryWindowTenantID int
	deliveryWindowChannel  string
	deliveryWindowDuration time.Duration
}

func (s *fakeSaaSAlertNotificationStore) EnqueueSaaSAlertNotification(_ context.Context, alert SaaSQuotaAlert, channel string, maxAttempts int) (SaaSAlertNotification, error) {
	s.enqueued = SaaSAlertNotification{
		ID:              1,
		NotificationKey: SaaSAlertNotificationKey(alert, channel),
		Alert:           alert,
		Channel:         channel,
		MaxAttempts:     maxAttempts,
	}
	return s.enqueued, nil
}

func (s *fakeSaaSAlertNotificationStore) ListDueSaaSAlertNotifications(_ context.Context, limit int) ([]SaaSAlertNotification, error) {
	s.listLimit = limit
	return s.due, nil
}

func (s *fakeSaaSAlertNotificationStore) MarkSaaSAlertNotificationDelivered(_ context.Context, id int64) error {
	s.deliveredID = id
	return nil
}

func (s *fakeSaaSAlertNotificationStore) MarkSaaSAlertNotificationFailed(_ context.Context, id int64, retryDelay time.Duration, lastError string) (string, error) {
	s.failedID = id
	s.failedDelay = retryDelay
	s.failedError = lastError
	if s.failedStatus == "" {
		s.failedStatus = SaaSAlertNotificationStatusFailed
	}
	return s.failedStatus, nil
}

func (s *fakeSaaSAlertNotificationStore) DeferSaaSAlertNotification(_ context.Context, id int64, nextRetryAt time.Time, reason string) error {
	s.deferredID = id
	s.deferredUntil = nextRetryAt
	s.deferredReason = reason
	return nil
}

func (s *fakeSaaSAlertNotificationStore) SuppressSaaSAlertNotification(_ context.Context, id int64, reason string) error {
	s.suppressedID = id
	s.suppressedReason = reason
	return nil
}

func (s *fakeSaaSAlertNotificationStore) SaaSAlertNotificationDeliveryWindow(_ context.Context, tenantID int, channel string, window time.Duration) (SaaSAlertNotificationDeliveryWindow, error) {
	s.deliveryWindowTenantID = tenantID
	s.deliveryWindowChannel = channel
	s.deliveryWindowDuration = window
	return s.deliveryWindow, nil
}

type fakeSaaSAlertSettingReader struct {
	setting  SaaSAlertSetting
	found    bool
	tenantID int
	channel  string
	err      error
}

func (s *fakeSaaSAlertSettingReader) GetSaaSAlertSetting(_ context.Context, tenantID int, channel string) (SaaSAlertSetting, bool, error) {
	s.tenantID = tenantID
	s.channel = channel
	return s.setting, s.found, s.err
}

type fakeSaaSAlertNotificationStoreWithSetting struct {
	fakeSaaSAlertNotificationStore
	setting SaaSAlertSetting
	found   bool
}

func (s *fakeSaaSAlertNotificationStoreWithSetting) GetSaaSAlertSetting(_ context.Context, tenantID int, channel string) (SaaSAlertSetting, bool, error) {
	setting := s.setting
	if setting.TenantID == 0 {
		setting.TenantID = tenantID
	}
	if setting.Channel == "" {
		setting.Channel = channel
	}
	return setting, s.found, nil
}
