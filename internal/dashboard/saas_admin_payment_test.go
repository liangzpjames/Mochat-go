package dashboard

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSaaSAdminPaymentOrdersRequiresPlatformAdminAndReturnsReport(t *testing.T) {
	store := newFakeSaaSAdminPaymentStore()
	store.paymentReport = SaaSAdminPaymentOrderReport{
		Summary: SaaSAdminPaymentOrderSummary{OrderCount: 1, FailedCount: 1, OutstandingCents: 12800},
		Orders:  []SaaSAdminPaymentOrder{{ID: 1, OrderNo: "PAY-1", TenantID: 12, TenantName: "租户B", Provider: "gateway", Status: SaaSPaymentOrderStatusFailed, AmountCents: 12800, Currency: "CNY", Version: 2}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/paymentOrders?tenantId=12&status=failed&provider=gateway&packageCode=growth&keyword=PAY&limit=10", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.PaymentOrders(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastPaymentOptions.TenantID != 12 || store.lastPaymentOptions.Status != SaaSPaymentOrderStatusFailed || store.lastPaymentOptions.Provider != "gateway" || store.lastPaymentOptions.PackageCode != "growth" || store.lastPaymentOptions.Keyword != "PAY" || store.lastPaymentOptions.Limit != 10 {
		t.Fatalf("options=%+v", store.lastPaymentOptions)
	}
	payload := decodeBody(t, rec.Body.Bytes())
	if payload["code"].(float64) != http.StatusOK {
		t.Fatalf("payload=%v", payload)
	}
	data := payload["data"].(map[string]any)
	if data["returnedCount"].(float64) != 1 || data["summary"].(map[string]any)["outstandingCents"].(float64) != 12800 {
		t.Fatalf("data=%v", data)
	}

	tenantReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/paymentOrders", nil)
	tenantReq.Header.Set("X-Mochat-Go-User-ID", "2")
	tenantRec := httptest.NewRecorder()
	handler.PaymentOrders(tenantRec, tenantReq)
	if tenantRec.Code != http.StatusForbidden {
		t.Fatalf("tenant status=%d body=%s", tenantRec.Code, tenantRec.Body.String())
	}
}

func TestSaaSAdminCreateAndCancelPaymentOrder(t *testing.T) {
	store := newFakeSaaSAdminPaymentStore()
	store.createResult = SaaSAdminPaymentOrderCreateResult{Order: SaaSAdminPaymentOrder{ID: 9, OrderNo: "PAY-EXPLICIT", TenantID: 12, Status: SaaSPaymentOrderStatusPending, Version: 1}, OperationID: 90}
	store.cancelResult = SaaSAdminPaymentOrderCancelResult{Order: SaaSAdminPaymentOrder{ID: 9, OrderNo: "PAY-EXPLICIT", TenantID: 12, Status: SaaSPaymentOrderStatusCanceled, Version: 2}, PreviousStatus: SaaSPaymentOrderStatusPending, OperationID: 91}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	body := `{"orderNo":"PAY-EXPLICIT","tenantId":12,"provider":"gateway","providerOrderNo":"GW-1","idempotencyKey":"create-1","packageCode":"growth","billingCycle":"yearly","serviceExpiresAt":"2027-08-01 00:00:00","amountCents":12800,"currency":"cny","checkoutUrl":"https://pay.example.com/PAY-EXPLICIT","checkoutExpiresAt":"2026-07-10 19:00:00","maxDunningAttempts":4,"remark":"待支付","metadata":{"source":"test"}}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/paymentOrder", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.CreatePaymentOrder(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if decodeBody(t, rec.Body.Bytes())["code"].(float64) != http.StatusOK {
		t.Fatalf("body=%s", rec.Body.String())
	}
	create := store.lastPaymentCreate
	if create.OrderNo != "PAY-EXPLICIT" || create.TenantID != 12 || create.Provider != "gateway" || create.ProviderOrderNo != "GW-1" || create.IdempotencyKey != "create-1" || create.PackageCode != "growth" || create.BillingCycle != "yearly" || create.AmountCents != 12800 || create.Currency != "CNY" || create.MaxDunningAttempts != 4 || create.ActorUserID != 1 || create.ActorTenantID != 1 || !strings.Contains(create.MetadataJSON, `"source":"test"`) {
		t.Fatalf("create=%+v", create)
	}

	cancelReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/paymentOrderCancel", strings.NewReader(`{"orderNo":"PAY-EXPLICIT","expectedVersion":1,"reason":"订单过期"}`))
	cancelReq.Header.Set("Content-Type", "application/json")
	cancelReq.Header.Set("X-Mochat-Go-User-ID", "1")
	cancelRec := httptest.NewRecorder()
	handler.CancelPaymentOrder(cancelRec, cancelReq)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("cancel status=%d body=%s", cancelRec.Code, cancelRec.Body.String())
	}
	if decodeBody(t, cancelRec.Body.Bytes())["code"].(float64) != http.StatusOK {
		t.Fatalf("cancel body=%s", cancelRec.Body.String())
	}
	if store.lastPaymentCancel.OrderNo != "PAY-EXPLICIT" || store.lastPaymentCancel.ExpectedVersion != 1 || store.lastPaymentCancel.ActorUserID != 1 || store.lastPaymentCancel.Reason != "订单过期" {
		t.Fatalf("cancel=%+v", store.lastPaymentCancel)
	}
}

func TestSaaSPaymentTimestampsStayWithinMySQL57Range(t *testing.T) {
	if _, err := normalizeSaaSPaymentOptionalTimestamp("2039-01-01 00:00:00", "serviceExpiresAt"); err == nil || !strings.Contains(err.Error(), "MySQL 5.7 TIMESTAMP") {
		t.Fatalf("future timestamp error=%v", err)
	}
	if _, err := normalizeSaaSPaymentOptionalTimestamp("1969-12-31 23:59:59", "occurredAt"); err == nil || !strings.Contains(err.Error(), "MySQL 5.7 TIMESTAMP") {
		t.Fatalf("past timestamp error=%v", err)
	}
	value, err := normalizeSaaSPaymentOptionalTimestamp("2038-01-01 00:00:00", "serviceExpiresAt")
	if err != nil || value != "2038-01-01 00:00:00" {
		t.Fatalf("valid timestamp value=%q error=%v", value, err)
	}
}

func TestSaaSAdminPaymentOrdersCSV(t *testing.T) {
	store := newFakeSaaSAdminPaymentStore()
	store.paymentReport = SaaSAdminPaymentOrderReport{Orders: []SaaSAdminPaymentOrder{{OrderNo: "PAY-CSV", TenantID: 12, TenantName: "租户B", Provider: "gateway", Status: SaaSPaymentOrderStatusPaid, PackageCode: "growth", AmountCents: 8800, Currency: "CNY", BillingEventID: 99, Version: 2}}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=paymentOrders&status=paid", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ExportCSV(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "orderNo,tenantId") || !strings.Contains(rec.Body.String(), "PAY-CSV,12") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), `filename="mochat-saas-payment-orders-`) {
		t.Fatalf("Content-Disposition=%q", rec.Header().Get("Content-Disposition"))
	}
}

func TestSaaSPaymentWebhookValidatesSignatureAndForwardsNormalizedEvent(t *testing.T) {
	fixedNow := time.Date(2026, 7, 10, 18, 30, 0, 0, time.Local)
	secret := "payment-webhook-secret-at-least-32-characters"
	store := &fakeSaaSPaymentWebhookStore{result: SaaSPaymentWebhookResult{
		Event: SaaSAdminPaymentWebhookEvent{ID: 7, Provider: "gateway", EventID: "evt-1", Status: SaaSPaymentWebhookEventStatusProcessed},
		Order: SaaSAdminPaymentOrder{ID: 8, OrderNo: "PAY-1", Status: SaaSPaymentOrderStatusPaid}, BillingEventID: 9,
	}}
	handler := NewSaaSPaymentWebhookHandler(store, secret, 5*time.Minute)
	handler.now = func() time.Time { return fixedNow }
	body := []byte(`{"provider":"gateway","eventId":"evt-1","eventType":"payment.succeeded","orderNo":"PAY-1","providerOrderNo":"GW-1","amountCents":8800,"currency":"cny","paidAt":"2026-07-10 18:29:00","metadata":{"channel":"test"}}`)
	timestamp := fixedNow.Unix()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/saas/payment", strings.NewReader(string(body)))
	req.Header.Set(SaaSPaymentWebhookTimestampHeader, strconvFormatInt(timestamp))
	req.Header.Set(SaaSPaymentWebhookSignatureHeader, "v1="+SaaSPaymentWebhookSignature(secret, timestamp, body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	input := store.lastInput
	if input.Provider != "gateway" || input.EventID != "evt-1" || input.EventType != SaaSPaymentWebhookTypeSucceeded || input.OrderNo != "PAY-1" || input.AmountCents != 8800 || input.Currency != "CNY" || input.PayloadSHA256 == "" || input.SignatureTimestamp != timestamp || !strings.Contains(input.MetadataJSON, `"channel":"test"`) {
		t.Fatalf("input=%+v", input)
	}
	data := decodeBody(t, rec.Body.Bytes())["data"].(map[string]any)
	if data["billingEventId"].(float64) != 9 || data["order"].(map[string]any)["status"] != SaaSPaymentOrderStatusPaid {
		t.Fatalf("data=%v", data)
	}

	badReq := httptest.NewRequest(http.MethodPost, "/webhooks/saas/payment", strings.NewReader(string(body)))
	badReq.Header.Set(SaaSPaymentWebhookTimestampHeader, strconvFormatInt(timestamp))
	badReq.Header.Set(SaaSPaymentWebhookSignatureHeader, "v1=bad")
	badRec := httptest.NewRecorder()
	handler.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusUnauthorized || store.calls != 1 {
		t.Fatalf("bad status=%d calls=%d", badRec.Code, store.calls)
	}
}

func TestSaaSPaymentWebhookRejectsExpiredTimestamp(t *testing.T) {
	secret := "payment-webhook-secret-at-least-32-characters"
	store := &fakeSaaSPaymentWebhookStore{}
	handler := NewSaaSPaymentWebhookHandler(store, secret, time.Minute)
	handler.now = func() time.Time { return time.Unix(2000, 0) }
	body := []byte(`{"provider":"gateway","eventId":"evt-1","eventType":"payment.failed","orderNo":"PAY-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/saas/payment", strings.NewReader(string(body)))
	req.Header.Set(SaaSPaymentWebhookTimestampHeader, "1000")
	req.Header.Set(SaaSPaymentWebhookSignatureHeader, "v1="+SaaSPaymentWebhookSignature(secret, 1000, body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || store.calls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.calls, rec.Body.String())
	}
}

func TestSaaSPaymentDunningHandlerAndCron(t *testing.T) {
	store := newFakeSaaSAdminPaymentStore()
	store.dunningResult = SaaSPaymentDunningResult{MatchedCount: 2, EnqueuedCount: 2, OrderNos: []string{"PAY-1", "PAY-2"}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/paymentDunning", strings.NewReader(`{"limit":25,"dryRun":true,"retryDelaySeconds":3600,"notificationMaxAttempts":4}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.PaymentDunning(rec, req)
	if rec.Code != http.StatusOK || !store.lastDunningOptions.DryRun || store.lastDunningOptions.Limit != 25 || store.lastDunningOptions.ActorTenantID != 1 {
		t.Fatalf("status=%d options=%+v body=%s", rec.Code, store.lastDunningOptions, rec.Body.String())
	}

	cron := NewSaaSPaymentDunningCron(store, 40, 7200, 5, 1, log.New(io.Discard, "", 0))
	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.lastDunningOptions.DryRun || store.lastDunningOptions.Limit != 40 || store.lastDunningOptions.RetryDelaySeconds != 7200 || store.lastDunningOptions.NotificationMaxAttempts != 5 || store.lastDunningOptions.ActorTenantID != 1 {
		t.Fatalf("cron options=%+v", store.lastDunningOptions)
	}
}

type fakeSaaSAdminPaymentStore struct {
	*fakeSaaSAdminStore
	paymentReport           SaaSAdminPaymentOrderReport
	paymentEvents           []SaaSAdminPaymentWebhookEvent
	createResult            SaaSAdminPaymentOrderCreateResult
	cancelResult            SaaSAdminPaymentOrderCancelResult
	dunningResult           SaaSPaymentDunningResult
	lastPaymentOptions      SaaSAdminPaymentOrderOptions
	lastPaymentEventOptions SaaSAdminPaymentWebhookEventOptions
	lastPaymentCreate       SaaSAdminPaymentOrderCreate
	lastPaymentCancel       SaaSAdminPaymentOrderCancel
	lastDunningOptions      SaaSPaymentDunningOptions
}

func newFakeSaaSAdminPaymentStore() *fakeSaaSAdminPaymentStore {
	return &fakeSaaSAdminPaymentStore{fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{
		1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
		2: {ID: 2, TenantID: 2, IsSuperAdmin: 1},
	}}}
}

func (s *fakeSaaSAdminPaymentStore) SaaSAdminPaymentOrders(_ context.Context, options SaaSAdminPaymentOrderOptions) (SaaSAdminPaymentOrderReport, error) {
	s.lastPaymentOptions = options
	return s.paymentReport, nil
}

func (s *fakeSaaSAdminPaymentStore) SaaSAdminPaymentWebhookEvents(_ context.Context, options SaaSAdminPaymentWebhookEventOptions) ([]SaaSAdminPaymentWebhookEvent, error) {
	s.lastPaymentEventOptions = options
	return s.paymentEvents, nil
}

func (s *fakeSaaSAdminPaymentStore) CreateSaaSAdminPaymentOrder(_ context.Context, create SaaSAdminPaymentOrderCreate) (SaaSAdminPaymentOrderCreateResult, error) {
	s.lastPaymentCreate = create
	return s.createResult, nil
}

func (s *fakeSaaSAdminPaymentStore) CancelSaaSAdminPaymentOrder(_ context.Context, cancel SaaSAdminPaymentOrderCancel) (SaaSAdminPaymentOrderCancelResult, error) {
	s.lastPaymentCancel = cancel
	return s.cancelResult, nil
}

func (s *fakeSaaSAdminPaymentStore) ProcessSaaSPaymentDunning(_ context.Context, options SaaSPaymentDunningOptions) (SaaSPaymentDunningResult, error) {
	s.lastDunningOptions = options
	return s.dunningResult, nil
}

type fakeSaaSPaymentWebhookStore struct {
	result    SaaSPaymentWebhookResult
	lastInput SaaSPaymentWebhookInput
	calls     int
}

func (s *fakeSaaSPaymentWebhookStore) ProcessSaaSPaymentWebhook(_ context.Context, input SaaSPaymentWebhookInput) (SaaSPaymentWebhookResult, error) {
	s.calls++
	s.lastInput = input
	return s.result, nil
}

func strconvFormatInt(value int64) string {
	return strconv.FormatInt(value, 10)
}
