package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSaaSAdminPaymentRefundsRequirePlatformAdmin(t *testing.T) {
	store := newFakeSaaSAdminRefundStore()
	store.refundReport = SaaSAdminPaymentRefundReport{
		Summary: SaaSAdminPaymentRefundSummary{RefundCount: 1, ProcessingCount: 1, PendingAmountCents: 8800},
		Refunds: []SaaSAdminPaymentRefund{{ID: 1, RefundNo: "REF-1", OrderNo: "PAY-1", TenantID: 12, Status: SaaSPaymentRefundStatusProcessing, AmountCents: 8800, Currency: "CNY"}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/paymentRefunds?tenantId=12&status=processing&provider=gateway&orderNo=PAY-1&keyword=REF&limit=10", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.PaymentRefunds(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastRefundOptions.TenantID != 12 || store.lastRefundOptions.Status != SaaSPaymentRefundStatusProcessing || store.lastRefundOptions.Provider != "gateway" || store.lastRefundOptions.OrderNo != "PAY-1" || store.lastRefundOptions.Keyword != "REF" || store.lastRefundOptions.Limit != 10 {
		t.Fatalf("options=%+v", store.lastRefundOptions)
	}
	payload := decodeBody(t, rec.Body.Bytes())
	if payload["code"].(float64) != http.StatusOK {
		t.Fatalf("payload=%v", payload)
	}
	data := payload["data"].(map[string]any)
	if data["returnedCount"].(float64) != 1 || data["summary"].(map[string]any)["pendingAmountCents"].(float64) != 8800 {
		t.Fatalf("data=%v", data)
	}

	tenantReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/paymentRefunds", nil)
	tenantReq.Header.Set("X-Mochat-Go-User-ID", "2")
	tenantRec := httptest.NewRecorder()
	handler.PaymentRefunds(tenantRec, tenantReq)
	if tenantRec.Code != http.StatusForbidden {
		t.Fatalf("tenant status=%d body=%s", tenantRec.Code, tenantRec.Body.String())
	}
}

func TestSaaSAdminCreateAndCancelPaymentRefund(t *testing.T) {
	store := newFakeSaaSAdminRefundStore()
	store.createRefundResult = SaaSAdminPaymentRefundCreateResult{
		Refund: SaaSAdminPaymentRefund{ID: 7, RefundNo: "REF-EXPLICIT", OrderNo: "PAY-1", Status: SaaSPaymentRefundStatusRequested, Version: 1},
		Order:  SaaSAdminPaymentOrder{ID: 9, OrderNo: "PAY-1", Status: SaaSPaymentOrderStatusPaid, RefundPendingCents: 8800}, OperationID: 70,
	}
	store.cancelRefundResult = SaaSAdminPaymentRefundCancelResult{
		Refund: SaaSAdminPaymentRefund{ID: 7, RefundNo: "REF-EXPLICIT", OrderNo: "PAY-1", Status: SaaSPaymentRefundStatusCanceled, Version: 2},
		Order:  SaaSAdminPaymentOrder{ID: 9, OrderNo: "PAY-1", Status: SaaSPaymentOrderStatusPaid}, PreviousStatus: SaaSPaymentRefundStatusRequested, OperationID: 71,
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	body := `{"refundNo":"REF-EXPLICIT","orderNo":"PAY-1","providerRefundNo":"GW-REF-1","idempotencyKey":"refund-1","amountCents":8800,"currency":"cny","reason":"重复扣款","entitlementAction":"cancel","remark":"全额退款","metadata":{"ticket":"CS-1"}}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/paymentRefund", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.CreatePaymentRefund(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if decodeBody(t, rec.Body.Bytes())["code"].(float64) != http.StatusOK {
		t.Fatalf("body=%s", rec.Body.String())
	}
	create := store.lastRefundCreate
	if create.RefundNo != "REF-EXPLICIT" || create.OrderNo != "PAY-1" || create.ProviderRefundNo != "GW-REF-1" || create.IdempotencyKey != "refund-1" || create.AmountCents != 8800 || create.Currency != "CNY" || create.Reason != "重复扣款" || create.EntitlementAction != SaaSPaymentRefundEntitlementCancel || create.ActorUserID != 1 || create.ActorTenantID != 1 || !strings.Contains(create.MetadataJSON, `"ticket":"CS-1"`) {
		t.Fatalf("create=%+v", create)
	}

	cancelReq := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/paymentRefundCancel", strings.NewReader(`{"refundNo":"REF-EXPLICIT","expectedVersion":1,"reason":"客户撤回"}`))
	cancelReq.Header.Set("Content-Type", "application/json")
	cancelReq.Header.Set("X-Mochat-Go-User-ID", "1")
	cancelRec := httptest.NewRecorder()
	handler.CancelPaymentRefund(cancelRec, cancelReq)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("cancel status=%d body=%s", cancelRec.Code, cancelRec.Body.String())
	}
	if decodeBody(t, cancelRec.Body.Bytes())["code"].(float64) != http.StatusOK {
		t.Fatalf("cancel body=%s", cancelRec.Body.String())
	}
	if store.lastRefundCancel.RefundNo != "REF-EXPLICIT" || store.lastRefundCancel.ExpectedVersion != 1 || store.lastRefundCancel.Reason != "客户撤回" || store.lastRefundCancel.ActorUserID != 1 {
		t.Fatalf("cancel=%+v", store.lastRefundCancel)
	}
}

func TestSaaSAdminPaymentRefundCSV(t *testing.T) {
	store := newFakeSaaSAdminRefundStore()
	store.refundReport = SaaSAdminPaymentRefundReport{Refunds: []SaaSAdminPaymentRefund{{RefundNo: "REF-CSV", OrderNo: "PAY-CSV", TenantID: 12, TenantName: "租户B", Provider: "gateway", Status: SaaSPaymentRefundStatusSucceeded, AmountCents: 8800, Currency: "CNY", Version: 2}}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=paymentRefunds&status=succeeded", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ExportCSV(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "refundNo,orderNo") || !strings.Contains(rec.Body.String(), "REF-CSV,PAY-CSV,12") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), `filename="mochat-saas-payment-refunds-`) {
		t.Fatalf("Content-Disposition=%q", rec.Header().Get("Content-Disposition"))
	}
}

func TestSaaSPaymentWebhookNormalizesRefundEvent(t *testing.T) {
	fixedNow := time.Date(2026, 7, 10, 20, 30, 0, 0, time.Local)
	secret := "payment-webhook-secret-at-least-32-characters"
	store := &fakeSaaSPaymentWebhookStore{result: SaaSPaymentWebhookResult{
		Event:  SaaSAdminPaymentWebhookEvent{ID: 8, Provider: "gateway", EventID: "evt-ref-1", RefundID: 7, RefundNo: "REF-1", Status: SaaSPaymentWebhookEventStatusProcessed},
		Order:  SaaSAdminPaymentOrder{ID: 9, OrderNo: "PAY-1", Status: SaaSPaymentOrderStatusPaid},
		Refund: SaaSAdminPaymentRefund{ID: 7, RefundNo: "REF-1", OrderNo: "PAY-1", Status: SaaSPaymentRefundStatusSucceeded}, BillingEventID: 10,
	}}
	handler := NewSaaSPaymentWebhookHandler(store, secret, 5*time.Minute)
	handler.now = func() time.Time { return fixedNow }
	body := []byte(`{"provider":"gateway","eventId":"evt-ref-1","eventType":"refund.succeeded","orderNo":"PAY-1","providerOrderNo":"GW-1","refundNo":"REF-1","providerRefundNo":"GW-REF-1","amountCents":8800,"currency":"cny","refundedAt":"2026-07-10 20:29:00"}`)
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
	if input.EventType != SaaSPaymentWebhookTypeRefundSucceeded || input.RefundNo != "REF-1" || input.ProviderRefundNo != "GW-REF-1" || input.AmountCents != 8800 || input.Currency != "CNY" || input.RefundedAt != "2026-07-10 20:29:00" {
		t.Fatalf("input=%+v", input)
	}
	data := decodeBody(t, rec.Body.Bytes())["data"].(map[string]any)
	if data["refund"].(map[string]any)["status"] != SaaSPaymentRefundStatusSucceeded || data["billingEventId"].(float64) != 10 {
		t.Fatalf("data=%v", data)
	}
}

type fakeSaaSAdminRefundStore struct {
	*fakeSaaSAdminPaymentStore
	refundReport       SaaSAdminPaymentRefundReport
	createRefundResult SaaSAdminPaymentRefundCreateResult
	cancelRefundResult SaaSAdminPaymentRefundCancelResult
	lastRefundOptions  SaaSAdminPaymentRefundOptions
	lastRefundCreate   SaaSAdminPaymentRefundCreate
	lastRefundCancel   SaaSAdminPaymentRefundCancel
}

func newFakeSaaSAdminRefundStore() *fakeSaaSAdminRefundStore {
	return &fakeSaaSAdminRefundStore{fakeSaaSAdminPaymentStore: newFakeSaaSAdminPaymentStore()}
}

func (s *fakeSaaSAdminRefundStore) SaaSAdminPaymentRefunds(_ context.Context, options SaaSAdminPaymentRefundOptions) (SaaSAdminPaymentRefundReport, error) {
	s.lastRefundOptions = options
	return s.refundReport, nil
}

func (s *fakeSaaSAdminRefundStore) CreateSaaSAdminPaymentRefund(_ context.Context, create SaaSAdminPaymentRefundCreate) (SaaSAdminPaymentRefundCreateResult, error) {
	s.lastRefundCreate = create
	return s.createRefundResult, nil
}

func (s *fakeSaaSAdminRefundStore) CancelSaaSAdminPaymentRefund(_ context.Context, cancel SaaSAdminPaymentRefundCancel) (SaaSAdminPaymentRefundCancelResult, error) {
	s.lastRefundCancel = cancel
	return s.cancelRefundResult, nil
}
