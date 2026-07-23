package dashboard

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"text/template"
	"time"

	"jiyi/mochat-go/internal/outboundhttp"
)

const (
	defaultSaaSAlertWebhookTitleTemplate = "SaaS额度告警：租户 {{.TenantID}} {{.Metric}}"
	defaultSaaSAlertWebhookBodyTemplate  = "{{.Message}}（当前 {{.CurrentValue}} / 上限 {{.LimitValue}}，来源 {{.Source}}）"
)

type SaaSAlertNotifier interface {
	NotifySaaSQuotaAlert(ctx context.Context, alert SaaSQuotaAlert) error
}

type saasAlertNotifierContextKey struct{}

func WithSaaSAlertNotifier(ctx context.Context, notifier SaaSAlertNotifier) context.Context {
	if notifier == nil {
		return ctx
	}
	return context.WithValue(ctx, saasAlertNotifierContextKey{}, notifier)
}

func saasAlertNotifierFromContext(ctx context.Context) (SaaSAlertNotifier, bool) {
	notifier, ok := ctx.Value(saasAlertNotifierContextKey{}).(SaaSAlertNotifier)
	return notifier, ok && notifier != nil
}

type RetryingSaaSAlertNotifier struct {
	inner    SaaSAlertNotifier
	attempts int
	delay    time.Duration
	sleep    func(context.Context, time.Duration) error
}

func NewRetryingSaaSAlertNotifier(inner SaaSAlertNotifier, attempts int, delay time.Duration) *RetryingSaaSAlertNotifier {
	if inner == nil {
		return nil
	}
	if attempts < 1 {
		attempts = 1
	}
	if delay < 0 {
		delay = 0
	}
	return &RetryingSaaSAlertNotifier{
		inner:    inner,
		attempts: attempts,
		delay:    delay,
		sleep:    sleepContext,
	}
}

func (n *RetryingSaaSAlertNotifier) NotifySaaSQuotaAlert(ctx context.Context, alert SaaSQuotaAlert) error {
	if n == nil || n.inner == nil {
		return nil
	}
	attempts := n.attempts
	if attempts < 1 {
		attempts = 1
	}
	delay := n.delay
	if delay < 0 {
		delay = 0
	}
	sleep := n.sleep
	if sleep == nil {
		sleep = sleepContext
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return fmt.Errorf("SaaS alert webhook retry stopped after %d/%d attempt(s): %w", attempt-1, attempts, err)
			}
			return err
		}
		if err := n.inner.NotifySaaSQuotaAlert(ctx, alert); err != nil {
			lastErr = err
		} else {
			return nil
		}
		if attempt == attempts {
			break
		}
		if delay > 0 {
			if err := sleep(ctx, delay); err != nil {
				return fmt.Errorf("SaaS alert webhook retry stopped after %d/%d attempt(s): %w", attempt, attempts, err)
			}
		}
	}
	return fmt.Errorf("SaaS alert webhook failed after %d attempt(s): %w", attempts, lastErr)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type SaaSAlertWebhookNotifier struct {
	endpoint      string
	secret        string
	timeout       time.Duration
	titleTemplate *template.Template
	bodyTemplate  *template.Template
	client        httpDoer
	guard         *outboundhttp.Guard
	initErr       error
}

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

func NewSaaSAlertWebhookNotifier(endpoint string, timeout time.Duration, secret ...string) *SaaSAlertWebhookNotifier {
	signingSecret := ""
	if len(secret) > 0 {
		signingSecret = strings.TrimSpace(secret[0])
	}
	notifier, err := NewSaaSAlertWebhookNotifierWithTemplates(endpoint, timeout, signingSecret, defaultSaaSAlertWebhookTitleTemplate, defaultSaaSAlertWebhookBodyTemplate)
	if err != nil {
		guard := defaultSaaSAlertWebhookGuard
		return &SaaSAlertWebhookNotifier{
			endpoint: strings.TrimSpace(endpoint), secret: signingSecret, timeout: timeout,
			guard: guard, client: guard.NewClient(), initErr: err,
		}
	}
	return notifier
}

func NewSaaSAlertWebhookNotifierWithTemplates(endpoint string, timeout time.Duration, secret string, titleTemplate string, bodyTemplate string, guards ...*outboundhttp.Guard) (*SaaSAlertWebhookNotifier, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, nil
	}
	guard := defaultSaaSAlertWebhookGuard
	if len(guards) > 0 && guards[0] != nil {
		guard = guards[0]
	}
	if err := guard.ValidateURL(endpoint); err != nil {
		return nil, fmt.Errorf("SaaS alert webhook URL rejected: %w", err)
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	compiledTitle, err := parseSaaSAlertWebhookTemplate("title", titleTemplate, defaultSaaSAlertWebhookTitleTemplate)
	if err != nil {
		return nil, err
	}
	compiledBody, err := parseSaaSAlertWebhookTemplate("body", bodyTemplate, defaultSaaSAlertWebhookBodyTemplate)
	if err != nil {
		return nil, err
	}
	return &SaaSAlertWebhookNotifier{
		endpoint:      endpoint,
		secret:        strings.TrimSpace(secret),
		timeout:       timeout,
		titleTemplate: compiledTitle,
		bodyTemplate:  compiledBody,
		client:        guard.NewClient(),
		guard:         guard,
	}, nil
}

func (n *SaaSAlertWebhookNotifier) NotifySaaSQuotaAlert(ctx context.Context, alert SaaSQuotaAlert) error {
	if n == nil || strings.TrimSpace(n.endpoint) == "" {
		return nil
	}
	if n.initErr != nil {
		return n.initErr
	}
	if err := normalizeSaaSAlertWebhookGuard(n.guard).ValidateURL(n.endpoint); err != nil {
		return fmt.Errorf("SaaS alert webhook URL rejected: %w", err)
	}
	occurredAt := time.Now().UTC().Format(time.RFC3339)
	templateData := saasAlertWebhookTemplateData{
		Event:           "saas.quota_alert",
		AlertType:       alert.AlertType,
		Severity:        alert.Severity,
		PeriodKey:       alert.PeriodKey,
		Source:          alert.Source,
		Message:         alert.Message,
		Context:         alert.Context,
		TenantID:        alert.Status.TenantID,
		Metric:          alert.Status.Metric,
		CurrentValue:    alert.Status.Current,
		LimitValue:      alert.Status.Limit,
		AdditionalValue: alert.Status.Additional,
		OccurredAt:      occurredAt,
	}
	title, err := renderSaaSAlertWebhookTemplate(n.titleTemplate, templateData)
	if err != nil {
		return err
	}
	bodyText, err := renderSaaSAlertWebhookTemplate(n.bodyTemplate, templateData)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"event":           "saas.quota_alert",
		"title":           title,
		"body":            bodyText,
		"alertType":       alert.AlertType,
		"severity":        alert.Severity,
		"periodKey":       alert.PeriodKey,
		"source":          alert.Source,
		"message":         alert.Message,
		"context":         alert.Context,
		"tenantId":        alert.Status.TenantID,
		"metric":          alert.Status.Metric,
		"currentValue":    alert.Status.Current,
		"limitValue":      alert.Status.Limit,
		"additionalValue": alert.Status.Additional,
		"occurredAt":      occurredAt,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if n.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, n.timeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "mochat-go-saas-alert/1.0")
	req.Header.Set("X-Mochat-Go-Event", "saas.quota_alert")
	if n.secret != "" {
		req.Header.Set("X-Mochat-Go-Timestamp", occurredAt)
		req.Header.Set("X-Mochat-Go-Signature", "v1="+saasAlertWebhookSignature(n.secret, occurredAt, body))
	}
	client := n.client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("SaaS alert webhook returned status %d", resp.StatusCode)
	}
	return nil
}

type saasAlertWebhookTemplateData struct {
	Event           string
	AlertType       string
	Severity        string
	PeriodKey       string
	Source          string
	Message         string
	TenantID        int
	Metric          string
	CurrentValue    int64
	LimitValue      int64
	AdditionalValue int64
	OccurredAt      string
	Context         map[string]any
}

func parseSaaSAlertWebhookTemplate(name string, raw string, fallback string) (*template.Template, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = fallback
	}
	return template.New(name).Option("missingkey=error").Parse(raw)
}

func renderSaaSAlertWebhookTemplate(tmpl *template.Template, data saasAlertWebhookTemplateData) (string, error) {
	if tmpl == nil {
		return "", nil
	}
	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data); err != nil {
		return "", err
	}
	return strings.TrimSpace(rendered.String()), nil
}

func saasAlertWebhookSignature(secret string, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
