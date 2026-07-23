package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"jiyi/mochat-go/internal/outboundhttp"
)

func TestSaaSAlertWebhookNotifierPostsPayload(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("content-type = %q", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("X-Mochat-Go-Event") != "saas.quota_alert" {
			t.Fatalf("event header = %q", r.Header.Get("X-Mochat-Go-Event"))
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	notifier, err := NewSaaSAlertWebhookNotifierWithTemplates(server.URL, time.Second, "", defaultSaaSAlertWebhookTitleTemplate, defaultSaaSAlertWebhookBodyTemplate, testSaaSAlertWebhookGuard(t))
	if err != nil {
		t.Fatal(err)
	}
	alert := SaaSQuotaAlert{
		Status: SaaSQuotaStatus{
			Metric:     SaaSMetricAsyncExecutions,
			TenantID:   7,
			Current:    12,
			Limit:      10,
			Additional: 0,
		},
		AlertType: SaaSAlertTypeQuotaExceeded,
		Severity:  SaaSAlertSeverityWarning,
		PeriodKey: SaaSAlertPeriodLifetime,
		Source:    "worker.queue_item",
		Message:   "套餐额度已达上限：异步执行量 12/10",
		Context:   map[string]any{"nonBlocking": true},
	}

	if err := notifier.NotifySaaSQuotaAlert(context.Background(), alert); err != nil {
		t.Fatal(err)
	}
	if payload["event"] != "saas.quota_alert" || payload["tenantId"].(float64) != 7 || payload["metric"] != SaaSMetricAsyncExecutions || payload["currentValue"].(float64) != 12 || payload["limitValue"].(float64) != 10 {
		t.Fatalf("payload = %+v", payload)
	}
	if payload["title"] != "SaaS额度告警：租户 7 async_executions" || payload["body"] != "套餐额度已达上限：异步执行量 12/10（当前 12 / 上限 10，来源 worker.queue_item）" {
		t.Fatalf("payload templates = title %q body %q", payload["title"], payload["body"])
	}
	contextPayload := payload["context"].(map[string]any)
	if contextPayload["nonBlocking"] != true {
		t.Fatalf("context = %+v", contextPayload)
	}
}

func TestSaaSAlertWebhookNotifierRendersCustomTemplates(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	notifier, err := NewSaaSAlertWebhookNotifierWithTemplates(
		server.URL,
		time.Second,
		"",
		"租户 {{.TenantID}} {{.Metric}} 告警",
		"当前 {{.CurrentValue}}/{{.LimitValue}} 来源 {{.Source}} {{.Context.nonBlocking}}",
		testSaaSAlertWebhookGuard(t),
	)
	if err != nil {
		t.Fatal(err)
	}
	err = notifier.NotifySaaSQuotaAlert(context.Background(), SaaSQuotaAlert{
		Status:    SaaSQuotaStatus{Metric: SaaSMetricAsyncExecutions, TenantID: 7, Current: 12, Limit: 10},
		AlertType: SaaSAlertTypeQuotaExceeded,
		Severity:  SaaSAlertSeverityWarning,
		PeriodKey: SaaSAlertPeriodLifetime,
		Source:    "worker.queue_item",
		Message:   "套餐额度已达上限：异步执行量 12/10",
		Context:   map[string]any{"nonBlocking": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if payload["title"] != "租户 7 async_executions 告警" || payload["body"] != "当前 12/10 来源 worker.queue_item true" {
		t.Fatalf("payload templates = title %q body %q", payload["title"], payload["body"])
	}
}

func TestSaaSAlertWebhookNotifierSignsPayload(t *testing.T) {
	const secret = "alert-secret"

	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		timestamp := r.Header.Get("X-Mochat-Go-Timestamp")
		if timestamp == "" {
			t.Fatalf("missing timestamp header")
		}
		wantSignature := "v1=" + saasAlertWebhookSignature(secret, timestamp, body)
		if r.Header.Get("X-Mochat-Go-Signature") != wantSignature {
			t.Fatalf("signature = %q, want %q", r.Header.Get("X-Mochat-Go-Signature"), wantSignature)
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatal(err)
		}
		if payload["occurredAt"] != timestamp {
			t.Fatalf("occurredAt = %q, timestamp = %q", payload["occurredAt"], timestamp)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	notifier, err := NewSaaSAlertWebhookNotifierWithTemplates(server.URL, time.Second, secret, defaultSaaSAlertWebhookTitleTemplate, defaultSaaSAlertWebhookBodyTemplate, testSaaSAlertWebhookGuard(t))
	if err != nil {
		t.Fatal(err)
	}
	err = notifier.NotifySaaSQuotaAlert(context.Background(), SaaSQuotaAlert{
		Status:    SaaSQuotaStatus{Metric: SaaSMetricAsyncExecutions, TenantID: 7, Current: 12, Limit: 10},
		AlertType: SaaSAlertTypeQuotaExceeded,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSaaSAlertWebhookNotifierReportsNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	notifier, buildErr := NewSaaSAlertWebhookNotifierWithTemplates(server.URL, time.Second, "", defaultSaaSAlertWebhookTitleTemplate, defaultSaaSAlertWebhookBodyTemplate, testSaaSAlertWebhookGuard(t))
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	err := notifier.NotifySaaSQuotaAlert(context.Background(), SaaSQuotaAlert{
		Status: SaaSQuotaStatus{Metric: SaaSMetricAsyncExecutions, TenantID: 7, Current: 12, Limit: 10},
	})
	if err == nil {
		t.Fatal("expected webhook error")
	}
}

func testSaaSAlertWebhookGuard(t *testing.T) *outboundhttp.Guard {
	t.Helper()
	guard, err := outboundhttp.NewGuard(outboundhttp.Config{
		RequireHTTPS: false,
		AllowedCIDRs: []string{"127.0.0.0/8"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return guard
}

func TestRetryingSaaSAlertNotifierSucceedsAfterRetry(t *testing.T) {
	inner := &fakeRetryingSaaSAlertNotifier{errors: []error{errors.New("temporary failure"), nil}}
	notifier := NewRetryingSaaSAlertNotifier(inner, 2, 0)

	err := notifier.NotifySaaSQuotaAlert(context.Background(), SaaSQuotaAlert{
		Status: SaaSQuotaStatus{Metric: SaaSMetricAsyncExecutions, TenantID: 7, Current: 12, Limit: 10},
	})
	if err != nil {
		t.Fatal(err)
	}
	if inner.calls != 2 {
		t.Fatalf("calls = %d, want 2", inner.calls)
	}
}

func TestRetryingSaaSAlertNotifierStopsAfterMaxAttempts(t *testing.T) {
	inner := &fakeRetryingSaaSAlertNotifier{errors: []error{errors.New("temporary failure"), errors.New("still failing")}}
	notifier := NewRetryingSaaSAlertNotifier(inner, 2, 0)

	err := notifier.NotifySaaSQuotaAlert(context.Background(), SaaSQuotaAlert{
		Status: SaaSQuotaStatus{Metric: SaaSMetricAsyncExecutions, TenantID: 7, Current: 12, Limit: 10},
	})
	if err == nil {
		t.Fatal("expected retry error")
	}
	if inner.calls != 2 {
		t.Fatalf("calls = %d, want 2", inner.calls)
	}
}

func TestRetryingSaaSAlertNotifierStopsWhenContextCanceledDuringDelay(t *testing.T) {
	inner := &fakeRetryingSaaSAlertNotifier{errors: []error{errors.New("temporary failure"), nil}}
	notifier := NewRetryingSaaSAlertNotifier(inner, 2, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	notifier.sleep = func(ctx context.Context, delay time.Duration) error {
		if delay != time.Second {
			t.Fatalf("delay = %s, want 1s", delay)
		}
		cancel()
		return ctx.Err()
	}

	err := notifier.NotifySaaSQuotaAlert(ctx, SaaSQuotaAlert{
		Status: SaaSQuotaStatus{Metric: SaaSMetricAsyncExecutions, TenantID: 7, Current: 12, Limit: 10},
	})
	if err == nil {
		t.Fatal("expected canceled retry error")
	}
	if inner.calls != 1 {
		t.Fatalf("calls = %d, want 1", inner.calls)
	}
}

type fakeRetryingSaaSAlertNotifier struct {
	calls  int
	errors []error
}

func (n *fakeRetryingSaaSAlertNotifier) NotifySaaSQuotaAlert(context.Context, SaaSQuotaAlert) error {
	n.calls++
	if n.calls <= len(n.errors) {
		return n.errors[n.calls-1]
	}
	return nil
}
