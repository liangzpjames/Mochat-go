package dashboard

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeTenantDomainDeliveryStore struct {
	claimed          []SaaSTenantDomainDeliveryJob
	completionResult SaaSTenantDomainDeliveryJob
	completions      []SaaSTenantDomainDeliveryCompletion
	callbackEvents   []SaaSTenantDomainDeliveryCallbackEvent
	callbackResult   SaaSTenantDomainDeliveryCallbackResult
}

func (s *fakeTenantDomainDeliveryStore) SaaSAdminTenantDomainDeliveryJobs(context.Context, SaaSTenantDomainDeliveryJobOptions) ([]SaaSTenantDomainDeliveryJob, error) {
	return nil, nil
}

func (s *fakeTenantDomainDeliveryStore) RequestSaaSTenantDomainDelivery(context.Context, SaaSTenantDomainDeliveryRequest) (SaaSTenantDomainDeliveryRequestResult, error) {
	return SaaSTenantDomainDeliveryRequestResult{}, nil
}

func (s *fakeTenantDomainDeliveryStore) RetrySaaSTenantDomainDelivery(context.Context, SaaSTenantDomainDeliveryRetry) (SaaSTenantDomainDeliveryRequestResult, error) {
	return SaaSTenantDomainDeliveryRequestResult{}, nil
}

func (s *fakeTenantDomainDeliveryStore) ClaimSaaSTenantDomainDeliveryJobs(context.Context, SaaSTenantDomainDeliveryClaimOptions) ([]SaaSTenantDomainDeliveryJob, error) {
	items := append([]SaaSTenantDomainDeliveryJob(nil), s.claimed...)
	s.claimed = nil
	return items, nil
}

func (s *fakeTenantDomainDeliveryStore) CompleteSaaSTenantDomainDeliveryJob(_ context.Context, input SaaSTenantDomainDeliveryCompletion) (SaaSTenantDomainDeliveryJob, error) {
	s.completions = append(s.completions, input)
	return s.completionResult, nil
}

func (s *fakeTenantDomainDeliveryStore) ApplySaaSTenantDomainDeliveryCallback(_ context.Context, event SaaSTenantDomainDeliveryCallbackEvent) (SaaSTenantDomainDeliveryCallbackResult, error) {
	s.callbackEvents = append(s.callbackEvents, event)
	return s.callbackResult, nil
}

type fakeTenantDomainDeliveryBridge struct {
	update      SaaSTenantDomainDeliveryBridgeUpdate
	requestHash string
	err         error
	callbackURL string
	jobs        []SaaSTenantDomainDeliveryJob
}

func (b *fakeTenantDomainDeliveryBridge) Deliver(_ context.Context, job SaaSTenantDomainDeliveryJob, callbackURL string) (SaaSTenantDomainDeliveryBridgeUpdate, string, error) {
	b.jobs = append(b.jobs, job)
	b.callbackURL = callbackURL
	return b.update, b.requestHash, b.err
}

func TestSaaSTenantDomainDeliveryHTTPBridgeClientContract(t *testing.T) {
	const token = "bridge-token-1234567890"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != SaaSTenantDomainDeliveryBridgePath {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get(SaaSTenantDomainDeliveryEventIDHeader); got != "DDJ-1" {
			t.Errorf("event id header = %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		text := string(body)
		for _, want := range []string{`"jobNo":"DDJ-1"`, `"action":"provision"`, `"callbackUrl":"https://api.example/webhooks/saas/domain-delivery"`, `"hostname":"tenant.example.com"`} {
			if !strings.Contains(text, want) {
				t.Errorf("request body missing %s: %s", want, text)
			}
		}
		for _, forbidden := range []string{"privateKey", "certificatePem", token} {
			if strings.Contains(text, forbidden) {
				t.Errorf("request body contains forbidden value %q", forbidden)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"ready","provider":"ingress","requestId":"req-1","routingStatus":"ready","certificateStatus":"active","certificateId":"cert-ref-1","certificateNotBefore":"2026-07-01T00:00:00Z","certificateExpiresAt":"2026-10-01T00:00:00Z"}`)
	}))
	defer server.Close()

	client, err := NewSaaSTenantDomainDeliveryHTTPBridgeClient(server.URL, token, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	job := SaaSTenantDomainDeliveryJob{
		JobNo: "DDJ-1", DomainID: 9, TenantID: 7, Action: SaaSTenantDomainDeliveryActionProvision,
		Domain: SaaSTenantDomain{ID: 9, TenantID: 7, Hostname: "tenant.example.com", Status: SaaSTenantDomainStatusActive, IsPrimary: true},
	}
	update, requestHash, err := client.Deliver(context.Background(), job, "https://api.example/webhooks/saas/domain-delivery")
	if err != nil {
		t.Fatal(err)
	}
	if update.Status != SaaSTenantDomainDeliveryBridgeStatusReady || update.CertificateID != "cert-ref-1" || len(requestHash) != 64 {
		t.Fatalf("update=%+v hash=%q", update, requestHash)
	}
}

func TestSaaSTenantDomainDeliveryHTTPBridgeDoesNotLeakErrorBody(t *testing.T) {
	const secretResponse = "upstream-private-key-material"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, secretResponse)
	}))
	defer server.Close()
	client, err := NewSaaSTenantDomainDeliveryHTTPBridgeClient(server.URL, "bridge-token-1234567890", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = client.Deliver(context.Background(), SaaSTenantDomainDeliveryJob{JobNo: "DDJ-2", DomainID: 1, TenantID: 1, Action: SaaSTenantDomainDeliveryActionRefresh, Domain: SaaSTenantDomain{Hostname: "tenant.example.com"}}, "")
	if err == nil || strings.Contains(err.Error(), secretResponse) || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("err = %v", err)
	}
}

func TestSaaSTenantDomainDeliveryWebhookValidatesSignatureAndEventID(t *testing.T) {
	const secret = "0123456789abcdef0123456789abcdef"
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	body := []byte(`{"eventId":"evt-1","eventType":"delivery.status","jobNo":"DDJ-1","domainId":9,"hostname":"tenant.example.com","status":"ready","provider":"ingress","requestId":"req-1","routingStatus":"ready","certificateStatus":"active","certificateId":"cert-ref-1","certificateNotBefore":"2026-07-01T00:00:00Z","certificateExpiresAt":"2026-10-01T00:00:00Z","occurredAt":"2026-07-11T11:59:00Z"}`)
	store := &fakeTenantDomainDeliveryStore{callbackResult: SaaSTenantDomainDeliveryCallbackResult{Delivery: SaaSTenantDomainDelivery{DeliveryStatus: SaaSTenantDomainDeliveryStatusReady}}}
	handler := NewSaaSTenantDomainDeliveryWebhookHandler(store, secret, 5*time.Minute)
	handler.now = func() time.Time { return now }

	request := func(eventID, signature string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/webhooks/saas/domain-delivery", strings.NewReader(string(body)))
		req.Header.Set(SaaSTenantDomainDeliveryTimestampHeader, "1783771200")
		req.Header.Set(SaaSTenantDomainDeliveryEventIDHeader, eventID)
		req.Header.Set(SaaSTenantDomainDeliverySignatureHeader, signature)
		return req
	}
	timestamp := now.Unix()
	signature := "v1=" + SaaSTenantDomainDeliveryWebhookSignature(secret, timestamp, body)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request("evt-1", signature))
	if recorder.Code != http.StatusOK || len(store.callbackEvents) != 1 {
		t.Fatalf("status=%d body=%s events=%d", recorder.Code, recorder.Body.String(), len(store.callbackEvents))
	}
	if event := store.callbackEvents[0]; event.EventID != "evt-1" || len(event.PayloadSHA256) != 64 || event.SignatureTimestamp != timestamp {
		t.Fatalf("event = %+v", event)
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request("evt-other", signature))
	if recorder.Code != http.StatusUnauthorized || len(store.callbackEvents) != 1 {
		t.Fatalf("mismatched event id status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request("evt-1", "v1=bad"))
	if recorder.Code != http.StatusUnauthorized || len(store.callbackEvents) != 1 {
		t.Fatalf("bad signature status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSaaSTenantDomainDeliveryProcessorCarriesLeaseAttempt(t *testing.T) {
	job := SaaSTenantDomainDeliveryJob{ID: 17, JobNo: "DDJ-17", Attempts: 2, MaxAttempts: 5, Status: SaaSTenantDomainDeliveryJobStatusProcessing}
	store := &fakeTenantDomainDeliveryStore{
		claimed:          []SaaSTenantDomainDeliveryJob{job},
		completionResult: SaaSTenantDomainDeliveryJob{ID: 17, JobNo: "DDJ-17", Status: SaaSTenantDomainDeliveryJobStatusWaiting},
	}
	bridge := &fakeTenantDomainDeliveryBridge{
		update:      SaaSTenantDomainDeliveryBridgeUpdate{Status: SaaSTenantDomainDeliveryBridgeStatusAccepted, RoutingStatus: SaaSTenantDomainRoutingStatusProvisioning, CertificateStatus: SaaSTenantDomainCertificateStatusProvisioning},
		requestHash: strings.Repeat("a", 64),
	}
	processor := NewSaaSTenantDomainDeliveryProcessor(store, bridge, "https://api.example/webhooks/saas/domain-delivery", 10, time.Minute, time.Second, time.Minute, log.New(io.Discard, "", 0))
	result, err := processor.Process(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Claimed != 1 || result.Waiting != 1 || len(store.completions) != 1 {
		t.Fatalf("result=%+v completions=%+v", result, store.completions)
	}
	if store.completions[0].ExpectedAttempts != 2 || bridge.callbackURL == "" || len(bridge.jobs) != 1 {
		t.Fatalf("completion=%+v callback=%q jobs=%d", store.completions[0], bridge.callbackURL, len(bridge.jobs))
	}
}

func TestSaaSTenantDomainDeliveryProcessorPersistsBridgeFailureAndSchedulesRetry(t *testing.T) {
	job := SaaSTenantDomainDeliveryJob{ID: 18, JobNo: "DDJ-18", Attempts: 1, MaxAttempts: 5, Status: SaaSTenantDomainDeliveryJobStatusProcessing}
	store := &fakeTenantDomainDeliveryStore{
		claimed:          []SaaSTenantDomainDeliveryJob{job},
		completionResult: SaaSTenantDomainDeliveryJob{ID: 18, JobNo: "DDJ-18", Status: SaaSTenantDomainDeliveryJobStatusPending},
	}
	bridgeErr := errors.New("bridge unavailable")
	processor := NewSaaSTenantDomainDeliveryProcessor(store, &fakeTenantDomainDeliveryBridge{err: bridgeErr}, "", 10, time.Minute, time.Second, time.Minute, log.New(io.Discard, "", 0))
	result, err := processor.Process(context.Background())
	if err != nil || result.Retried != 1 || result.Failed != 0 || len(store.completions) != 1 || store.completions[0].CallError != bridgeErr.Error() {
		t.Fatalf("result=%+v err=%v completions=%+v", result, err, store.completions)
	}
}
