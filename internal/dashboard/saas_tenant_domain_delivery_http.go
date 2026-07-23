package dashboard

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	SaaSTenantDomainDeliveryBridgePath             = "/v1/domain-deliveries"
	SaaSTenantDomainDeliverySignatureHeader        = "X-Mochat-Go-Domain-Signature"
	SaaSTenantDomainDeliveryTimestampHeader        = "X-Mochat-Go-Domain-Timestamp"
	SaaSTenantDomainDeliveryEventIDHeader          = "X-Mochat-Go-Event-ID"
	SaaSTenantDomainDeliveryMaxResponseBytes int64 = 1 << 20
)

type SaaSTenantDomainDeliveryHTTPBridgeClient struct {
	baseURL string
	token   string
	client  *http.Client
}

type saasTenantDomainDeliveryBridgeRequest struct {
	JobNo       string                               `json:"jobNo"`
	Action      string                               `json:"action"`
	CallbackURL string                               `json:"callbackUrl,omitempty"`
	Domain      saasTenantDomainDeliveryBridgeDomain `json:"domain"`
}

type saasTenantDomainDeliveryBridgeDomain struct {
	ID        int64  `json:"id"`
	TenantID  int    `json:"tenantId"`
	Hostname  string `json:"hostname"`
	Status    string `json:"status"`
	IsPrimary bool   `json:"isPrimary"`
}

func NewSaaSTenantDomainDeliveryHTTPBridgeClient(baseURL, token string, timeout time.Duration) (*SaaSTenantDomainDeliveryHTTPBridgeClient, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("tenant domain delivery bridge URL is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, errors.New("tenant domain delivery bridge URL must be an absolute HTTP(S) URL")
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if timeout > 120*time.Second {
		return nil, errors.New("tenant domain delivery bridge timeout must not exceed 120 seconds")
	}
	return &SaaSTenantDomainDeliveryHTTPBridgeClient{baseURL: baseURL, token: strings.TrimSpace(token), client: &http.Client{Timeout: timeout}}, nil
}

func (c *SaaSTenantDomainDeliveryHTTPBridgeClient) Deliver(ctx context.Context, job SaaSTenantDomainDeliveryJob, callbackURL string) (SaaSTenantDomainDeliveryBridgeUpdate, string, error) {
	if c == nil || c.client == nil || c.baseURL == "" {
		return SaaSTenantDomainDeliveryBridgeUpdate{}, "", errors.New("tenant domain delivery bridge client is not configured")
	}
	payload := saasTenantDomainDeliveryBridgeRequest{
		JobNo: job.JobNo, Action: job.Action, CallbackURL: strings.TrimSpace(callbackURL),
		Domain: saasTenantDomainDeliveryBridgeDomain{ID: job.DomainID, TenantID: job.TenantID, Hostname: job.Domain.Hostname, Status: job.Domain.Status, IsPrimary: job.Domain.IsPrimary},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return SaaSTenantDomainDeliveryBridgeUpdate{}, "", err
	}
	digest := sha256.Sum256(body)
	requestHash := hex.EncodeToString(digest[:])
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+SaaSTenantDomainDeliveryBridgePath, bytes.NewReader(body))
	if err != nil {
		return SaaSTenantDomainDeliveryBridgeUpdate{}, requestHash, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set(SaaSTenantDomainDeliveryEventIDHeader, job.JobNo)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return SaaSTenantDomainDeliveryBridgeUpdate{}, requestHash, fmt.Errorf("tenant domain delivery bridge request: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, SaaSTenantDomainDeliveryMaxResponseBytes+1))
	if err != nil {
		return SaaSTenantDomainDeliveryBridgeUpdate{}, requestHash, fmt.Errorf("read tenant domain delivery bridge response: %w", err)
	}
	if int64(len(responseBody)) > SaaSTenantDomainDeliveryMaxResponseBytes {
		return SaaSTenantDomainDeliveryBridgeUpdate{}, requestHash, errors.New("tenant domain delivery bridge response exceeds 1MB")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SaaSTenantDomainDeliveryBridgeUpdate{}, requestHash, fmt.Errorf("tenant domain delivery bridge returned HTTP %d", resp.StatusCode)
	}
	var update SaaSTenantDomainDeliveryBridgeUpdate
	decoder := json.NewDecoder(bytes.NewReader(responseBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&update); err != nil {
		return SaaSTenantDomainDeliveryBridgeUpdate{}, requestHash, fmt.Errorf("decode tenant domain delivery bridge response: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return SaaSTenantDomainDeliveryBridgeUpdate{}, requestHash, errors.New("decode tenant domain delivery bridge response: trailing JSON content")
	}
	update, err = normalizeSaaSTenantDomainDeliveryBridgeUpdate(update)
	if err != nil {
		return SaaSTenantDomainDeliveryBridgeUpdate{}, requestHash, fmt.Errorf("validate tenant domain delivery bridge response: %w", err)
	}
	return update, requestHash, nil
}

type SaaSTenantDomainDeliveryWebhookHandler struct {
	store     SaaSTenantDomainDeliveryStore
	secret    string
	tolerance time.Duration
	now       func() time.Time
}

func NewSaaSTenantDomainDeliveryWebhookHandler(store SaaSTenantDomainDeliveryStore, secret string, tolerance time.Duration) *SaaSTenantDomainDeliveryWebhookHandler {
	if tolerance <= 0 {
		tolerance = 5 * time.Minute
	}
	return &SaaSTenantDomainDeliveryWebhookHandler{store: store, secret: strings.TrimSpace(secret), tolerance: tolerance, now: time.Now}
}

func (h *SaaSTenantDomainDeliveryWebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if h == nil || h.store == nil || len(h.secret) < 32 {
		writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "domain delivery callback is not configured", nil)
		return
	}
	body, err := readTenantDomainDeliveryBody(r, 1<<20)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	timestamp, err := strconv.ParseInt(strings.TrimSpace(r.Header.Get(SaaSTenantDomainDeliveryTimestampHeader)), 10, 64)
	if err != nil || timestamp <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "invalid domain delivery callback timestamp", nil)
		return
	}
	now := h.now()
	if delta := now.Sub(time.Unix(timestamp, 0)); delta > h.tolerance || delta < -h.tolerance {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "domain delivery callback timestamp outside tolerance", nil)
		return
	}
	gotSignature := strings.TrimSpace(r.Header.Get(SaaSTenantDomainDeliverySignatureHeader))
	wantSignature := "v1=" + SaaSTenantDomainDeliveryWebhookSignature(h.secret, timestamp, body)
	if !hmac.Equal([]byte(gotSignature), []byte(wantSignature)) {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "invalid domain delivery callback signature", nil)
		return
	}
	var event SaaSTenantDomainDeliveryCallbackEvent
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&event); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid JSON body", nil)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid trailing JSON content", nil)
		return
	}
	if err := validateSaaSTenantDomainDeliveryCallbackEvent(&event); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if eventHeader := strings.TrimSpace(r.Header.Get(SaaSTenantDomainDeliveryEventIDHeader)); eventHeader == "" || eventHeader != event.EventID {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "domain delivery callback event id mismatch", nil)
		return
	}
	digest := sha256.Sum256(body)
	event.PayloadSHA256 = hex.EncodeToString(digest[:])
	event.SignatureTimestamp = timestamp
	result, err := h.store.ApplySaaSTenantDomainDeliveryCallback(r.Context(), event)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"duplicate": result.Duplicate, "ignored": result.Ignored, "delivery": result.Delivery, "job": result.Job})
}

func SaaSTenantDomainDeliveryWebhookSignature(secret string, timestamp int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func validateSaaSTenantDomainDeliveryCallbackEvent(event *SaaSTenantDomainDeliveryCallbackEvent) error {
	if event == nil {
		return errors.New("callback event is required")
	}
	event.EventID = truncateTenantDomainDeliveryText(event.EventID, 128)
	event.EventType = truncateTenantDomainDeliveryText(event.EventType, 32)
	event.JobNo = truncateTenantDomainDeliveryText(event.JobNo, 64)
	event.Hostname = strings.TrimSpace(event.Hostname)
	if event.EventID == "" || event.JobNo == "" || event.DomainID <= 0 {
		return errors.New("eventId、jobNo 和 domainId 必填")
	}
	if event.EventType == "" {
		event.EventType = "delivery.status"
	}
	if event.EventType != "delivery.status" {
		return errors.New("eventType 无效")
	}
	hostname, err := NormalizeSaaSTenantDomainHostname(event.Hostname)
	if err != nil {
		return errors.New("hostname 无效")
	}
	event.Hostname = hostname
	update, err := normalizeSaaSTenantDomainDeliveryBridgeUpdate(SaaSTenantDomainDeliveryBridgeUpdate{
		Status: event.Status, Provider: event.Provider, ProviderRequestID: event.ProviderRequestID,
		RoutingStatus: event.RoutingStatus, CertificateStatus: event.CertificateStatus,
		CertificateID: event.CertificateID, CertificateNotBefore: event.CertificateNotBefore,
		CertificateExpiresAt: event.CertificateExpiresAt, Error: event.Error, OccurredAt: event.OccurredAt,
	})
	if err != nil {
		return err
	}
	if update.OccurredAt == "" {
		return errors.New("occurredAt 必填")
	}
	event.Status, event.Provider, event.ProviderRequestID = update.Status, update.Provider, update.ProviderRequestID
	event.RoutingStatus, event.CertificateStatus = update.RoutingStatus, update.CertificateStatus
	event.CertificateID, event.CertificateNotBefore, event.CertificateExpiresAt = update.CertificateID, update.CertificateNotBefore, update.CertificateExpiresAt
	event.Error, event.OccurredAt = update.Error, update.OccurredAt
	return nil
}
