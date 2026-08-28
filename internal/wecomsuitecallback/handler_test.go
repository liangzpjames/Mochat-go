package wecomsuitecallback

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/archivefixture"
	"jiyi/mochat-go/internal/wecomarchivedemo"
)

const (
	testSuiteID       = "ww-local-fixture-suite"
	testSuiteSecret   = "suite-secret-local"
	testCallbackToken = "callback-token-local"
	testCallbackAES   = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
)

type callbackStoreStub struct {
	events        map[string]bool
	ticket        string
	authorization archivefixture.SuiteAuthorization
}

func (s *callbackStoreStub) ClaimSuiteCallback(_ context.Context, _, digest, _ string, _ time.Time) (bool, error) {
	if s.events == nil {
		s.events = map[string]bool{}
	}
	if s.events[digest] {
		return false, nil
	}
	s.events[digest] = true
	return true, nil
}
func (s *callbackStoreStub) CompleteSuiteCallback(context.Context, string, string) error { return nil }
func (s *callbackStoreStub) FailSuiteCallback(_ context.Context, _, digest string) error {
	delete(s.events, digest)
	return nil
}
func (s *callbackStoreStub) SaveSuiteTicket(_ context.Context, _, ticket string, _ time.Time) error {
	s.ticket = ticket
	return nil
}
func (s *callbackStoreStub) LoadSuiteTicket(context.Context, string) (string, error) {
	return s.ticket, nil
}
func (s *callbackStoreStub) SaveSuiteAuthorization(_ context.Context, _ string, authorization archivefixture.SuiteAuthorization) error {
	s.authorization = authorization
	return nil
}

type callbackExchangeStub struct {
	calls int
	err   error
}

func (s *callbackExchangeStub) ExchangeAuthorization(_ context.Context, suiteID, suiteSecret, ticket, authCode string) (archivefixture.SuiteAuthorization, error) {
	s.calls++
	if s.err != nil {
		return archivefixture.SuiteAuthorization{}, s.err
	}
	if suiteID != testSuiteID || suiteSecret != testSuiteSecret || ticket != "ticket-local" || authCode != "auth-local" {
		return archivefixture.SuiteAuthorization{}, errors.New("invalid fixture exchange")
	}
	return archivefixture.SuiteAuthorization{TenantID: 21, CorpID: "ww-delegated", PermanentCode: "permanent-local"}, nil
}

func TestHandlerConsumesEncryptedTicketAndAuthorizationOnce(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	provider, err := archivefixture.NewSuiteProvider(testSuiteID, testSuiteSecret, testCallbackToken, testCallbackAES)
	if err != nil {
		t.Fatal(err)
	}
	store := &callbackStoreStub{}
	exchange := &callbackExchangeStub{}
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	handler, err := NewHandler(Config{SuiteID: testSuiteID, SuiteSecret: testSuiteSecret, CallbackToken: testCallbackToken, EncodingAESKey: testCallbackAES, Logger: logger}, store, exchange)
	if err != nil {
		t.Fatal(err)
	}
	handler.WithClock(func() time.Time { return now })

	values, encrypted, err := provider.BuildTicketCallback("ticket-local", "1787832000", "nonce-ticket")
	if err != nil {
		t.Fatal(err)
	}
	response := serveSuiteCallback(t, handler, values, encrypted)
	if response.Code != http.StatusOK || store.ticket != "ticket-local" || bytes.Contains(response.Body.Bytes(), []byte("ticket-local")) {
		t.Fatalf("ticket response=%d %q ticket=%q", response.Code, response.Body.String(), store.ticket)
	}

	values, encrypted, err = provider.BuildAuthorizationCallback("auth-local", "1787832000", "nonce-auth")
	if err != nil {
		t.Fatal(err)
	}
	response = serveSuiteCallback(t, handler, values, encrypted)
	if response.Code != http.StatusOK || exchange.calls != 1 || store.authorization.TenantID != 21 || store.authorization.PermanentCode != "permanent-local" {
		t.Fatalf("auth response=%d calls=%d authorization=%+v", response.Code, exchange.calls, store.authorization)
	}
	replay := serveSuiteCallback(t, handler, values, encrypted)
	if replay.Code != http.StatusOK || exchange.calls != 1 {
		t.Fatalf("replay response=%d calls=%d", replay.Code, exchange.calls)
	}
	text := logs.String()
	for _, required := range []string{
		"wecom_suite_ticket_saved", "wecom_authorization_saved", "wecom_callback_duplicate",
		`"object_type":"wecom_callback_type"`, `"object_id":"suite_ticket"`, `"object_id":"create_auth"`,
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("missing %q: %s", required, text)
		}
	}
	for _, forbidden := range []string{"ticket-local", "auth-local", "permanent-local", "ww-delegated", "nonce-auth", "msg_signature", "business_object"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("callback log leaked %q: %s", forbidden, text)
		}
	}
}

func TestHandlerRejectsExpiredCallbackBeforePersisting(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	provider, _ := archivefixture.NewSuiteProvider(testSuiteID, testSuiteSecret, testCallbackToken, testCallbackAES)
	store := &callbackStoreStub{}
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler, _ := NewHandler(Config{SuiteID: testSuiteID, SuiteSecret: testSuiteSecret, CallbackToken: testCallbackToken, EncodingAESKey: testCallbackAES, Logger: logger}, store, &callbackExchangeStub{})
	handler.WithClock(func() time.Time { return now })
	values, encrypted, _ := provider.BuildTicketCallback("ticket-local", "1787831000", "nonce-old")
	response := serveSuiteCallback(t, handler, values, encrypted)
	if response.Code != http.StatusBadRequest || store.ticket != "" || len(store.events) != 0 {
		t.Fatalf("response=%d ticket=%q events=%v", response.Code, store.ticket, store.events)
	}
	if !strings.Contains(logs.String(), "wecom_callback_rejected") || !strings.Contains(logs.String(), `"error_code":"WECOM_CALLBACK_TIMESTAMP_INVALID"`) {
		t.Fatalf("rejection log = %s", logs.String())
	}
}

func TestHandlerLogsAuthorizationExchangeFailureWithoutCallbackSecrets(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	provider, _ := archivefixture.NewSuiteProvider(testSuiteID, testSuiteSecret, testCallbackToken, testCallbackAES)
	store := &callbackStoreStub{ticket: "ticket-local"}
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler, err := NewHandler(Config{SuiteID: testSuiteID, SuiteSecret: testSuiteSecret, CallbackToken: testCallbackToken, EncodingAESKey: testCallbackAES, Logger: logger}, store, &callbackExchangeStub{err: errors.New("exchange response contains permanent-local")})
	if err != nil {
		t.Fatal(err)
	}
	handler.WithClock(func() time.Time { return now })
	values, encrypted, _ := provider.BuildAuthorizationCallback("auth-local", "1787832000", "nonce-auth")
	response := serveSuiteCallback(t, handler, values, encrypted)
	if response.Code != http.StatusBadGateway {
		t.Fatalf("response=%d", response.Code)
	}
	text := logs.String()
	if !strings.Contains(text, "wecom_authorization_exchange_failed") || !strings.Contains(text, `"level":"ERROR"`) {
		t.Fatalf("exchange log = %s", text)
	}
	for _, forbidden := range []string{"ticket-local", "auth-local", "permanent-local", "nonce-auth", "msg_signature"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("callback log leaked %q: %s", forbidden, text)
		}
	}
}

func TestHandlerVerifiesGETChallengeWithoutPersisting(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	provider, _ := archivefixture.NewSuiteProvider(testSuiteID, testSuiteSecret, testCallbackToken, testCallbackAES)
	store := &callbackStoreStub{}
	handler, _ := NewHandler(Config{SuiteID: testSuiteID, SuiteSecret: testSuiteSecret, CallbackToken: testCallbackToken, EncodingAESKey: testCallbackAES}, store, &callbackExchangeStub{})
	handler.WithClock(func() time.Time { return now })
	values, encrypted, err := provider.BuildTicketCallback("ticket-local", "1787832000", "nonce-challenge")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := wecomarchivedemo.VerifyAndDecryptCallback(testCallbackToken, testCallbackAES, testSuiteID, values, encrypted)
	if err != nil {
		t.Fatal(err)
	}
	values.Set("echostr", encrypted)
	request := httptest.NewRequest(http.MethodGet, "/wecom/suite/callback?"+values.Encode(), nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), plain.Message) || store.ticket != "" || len(store.events) != 0 {
		t.Fatalf("response=%d body=%q ticket=%q events=%v", response.Code, response.Body.String(), store.ticket, store.events)
	}
}

func serveSuiteCallback(t *testing.T, handler http.Handler, values url.Values, encrypted string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := xml.Marshal(struct {
		XMLName xml.Name `xml:"xml"`
		Encrypt string   `xml:"Encrypt"`
	}{Encrypt: encrypted})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/wecom/suite/callback?"+values.Encode(), bytes.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
