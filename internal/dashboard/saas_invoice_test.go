package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSaaSInvoiceValidationAndTransitions(t *testing.T) {
	valid := SaaSBillingProfileUpdate{
		InvoiceType: SaaSInvoiceTypeNormal, InvoiceTitle: "测试企业有限公司",
		TaxIdentifier: "91310000TEST123456", Email: "billing@example.com",
	}
	if err := validateSaaSBillingProfileUpdate(valid); err != nil {
		t.Fatal(err)
	}
	special := valid
	special.InvoiceType = SaaSInvoiceTypeSpecial
	if err := validateSaaSBillingProfileUpdate(special); err == nil || !strings.Contains(err.Error(), "注册地址") {
		t.Fatalf("special err=%v", err)
	}
	special.RegisteredAddress = "上海市测试路 1 号"
	special.BankName = "测试银行"
	special.BankAccount = "6222000000000000"
	if err := validateSaaSBillingProfileUpdate(special); err != nil {
		t.Fatal(err)
	}
	if !SaaSInvoiceTransitionAllowed(SaaSInvoiceStatusRequested, SaaSInvoiceStatusIssued) ||
		!SaaSInvoiceTransitionAllowed(SaaSInvoiceStatusProcessing, SaaSInvoiceStatusFailed) ||
		SaaSInvoiceTransitionAllowed(SaaSInvoiceStatusIssued, SaaSInvoiceStatusCanceled) {
		t.Fatal("invoice transition graph invalid")
	}
}

func TestSaaSAdminInvoiceDocumentsRequiresPlatformAdmin(t *testing.T) {
	store := newFakeSaaSInvoiceStore()
	store.invoiceReport = SaaSInvoiceDocumentReport{Documents: []SaaSInvoiceDocument{{DocumentNo: "INV-1", TenantID: 12}}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	tenantReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/invoiceDocuments", nil)
	tenantReq.Header.Set("X-Mochat-Go-User-ID", "2")
	tenantRec := httptest.NewRecorder()
	handler.InvoiceDocuments(tenantRec, tenantReq)
	if tenantRec.Code != http.StatusForbidden {
		t.Fatalf("tenant status=%d body=%s", tenantRec.Code, tenantRec.Body.String())
	}

	platformReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/invoiceDocuments?tenantId=12&kind=credit_note&status=issued&orderNo=PAY-1&keyword=INV&limit=10", nil)
	platformReq.Header.Set("X-Mochat-Go-User-ID", "1")
	platformRec := httptest.NewRecorder()
	handler.InvoiceDocuments(platformRec, platformReq)
	if platformRec.Code != http.StatusOK {
		t.Fatalf("platform status=%d body=%s", platformRec.Code, platformRec.Body.String())
	}
	if decodeBody(t, platformRec.Body.Bytes())["code"].(float64) != http.StatusOK {
		t.Fatalf("platform body=%s", platformRec.Body.String())
	}
	if store.lastInvoiceOptions.TenantID != 12 || store.lastInvoiceOptions.Kind != SaaSInvoiceKindCreditNote || store.lastInvoiceOptions.Status != SaaSInvoiceStatusIssued || store.lastInvoiceOptions.OrderNo != "PAY-1" || store.lastInvoiceOptions.Limit != 10 {
		t.Fatalf("options=%+v", store.lastInvoiceOptions)
	}
}

func TestSaaSBillingTenantScopeOverridesRequestTenant(t *testing.T) {
	store := newFakeSaaSInvoiceStore()
	store.profile = SaaSBillingProfile{TenantID: 2, InvoiceType: SaaSInvoiceTypeNormal, InvoiceTitle: "租户二", TaxIdentifier: "91310000TEST000002", Email: "two@example.com", Version: 3, Status: 1}
	store.profileExists = true
	handler := NewSaaSBillingHandler(store, HeaderUserIDResolver{})

	profileReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasBilling/invoiceProfile", strings.NewReader(`{"tenantId":999,"invoiceType":"normal","invoiceTitle":"租户二新抬头","taxIdentifier":"91310000TEST000002","email":"two@example.com","expectedVersion":3}`))
	profileReq.Header.Set("Content-Type", "application/json")
	profileReq.Header.Set("X-Mochat-Go-User-ID", "2")
	profileRec := httptest.NewRecorder()
	handler.InvoiceProfile(profileRec, profileReq)
	if profileRec.Code != http.StatusOK {
		t.Fatalf("profile status=%d body=%s", profileRec.Code, profileRec.Body.String())
	}
	if store.lastProfileUpdate.TenantID != 2 || store.lastProfileUpdate.ActorTenantID != 2 || store.lastProfileUpdate.ActorUserID != 2 {
		t.Fatalf("profile update=%+v", store.lastProfileUpdate)
	}

	invoiceReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasBilling/invoice", strings.NewReader(`{"tenantId":999,"orderNo":"PAY-2","amountCents":8800,"currency":"CNY","idempotencyKey":"tenant-2-invoice","originalDocumentNo":"INV-OTHER"}`))
	invoiceReq.Header.Set("Content-Type", "application/json")
	invoiceReq.Header.Set("X-Mochat-Go-User-ID", "2")
	invoiceRec := httptest.NewRecorder()
	handler.CreateInvoice(invoiceRec, invoiceReq)
	if invoiceRec.Code != http.StatusOK {
		t.Fatalf("invoice status=%d body=%s", invoiceRec.Code, invoiceRec.Body.String())
	}
	if store.lastInvoiceCreate.TenantID != 2 || store.lastInvoiceCreate.Kind != SaaSInvoiceKindInvoice || store.lastInvoiceCreate.OriginalDocumentNo != "" || store.lastInvoiceCreate.ActorTenantID != 2 {
		t.Fatalf("invoice create=%+v", store.lastInvoiceCreate)
	}
}

func TestSaaSBillingSummaryUsesJWTenantForEveryLedger(t *testing.T) {
	store := newFakeSaaSInvoiceStore()
	handler := NewSaaSBillingHandler(store, HeaderUserIDResolver{})
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasBilling/summary?tenantId=999", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "2")
	rec := httptest.NewRecorder()
	handler.Summary(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastPaymentOptions.TenantID != 2 || store.lastRefundOptions.TenantID != 2 || store.lastInvoiceOptions.TenantID != 2 || store.lastProfileTenantID != 2 {
		t.Fatalf("payment=%+v refund=%+v invoice=%+v profileTenant=%d", store.lastPaymentOptions, store.lastRefundOptions, store.lastInvoiceOptions, store.lastProfileTenantID)
	}
}

func TestSaaSBillingPageContainsOperationalControls(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasBilling/page", nil)
	rec := httptest.NewRecorder()
	ServeSaaSBillingPage(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	for _, token := range []string{
		`id="billingSummary"`, `id="saveProfile"`, `id="requestInvoice"`, `id="orders"`, `id="invoices"`, `id="refunds"`,
		`fetch(path`, `/dashboard/saasBilling/summary`, `/dashboard/saasBilling/invoiceProfile`, `/dashboard/saasBilling/invoiceCancel`,
		`succeeded:['已退款','ok']`, `await loadSummary()`,
	} {
		if !strings.Contains(rec.Body.String(), token) {
			t.Fatalf("missing %q", token)
		}
	}
}

type fakeSaaSInvoiceStore struct {
	*fakeSaaSAdminRefundStore
	profile             SaaSBillingProfile
	profileExists       bool
	invoiceReport       SaaSInvoiceDocumentReport
	profileResult       SaaSBillingProfileUpdateResult
	invoiceCreateResult SaaSInvoiceDocumentCreateResult
	transitionResult    SaaSInvoiceDocumentTransitionResult
	lastProfileTenantID int
	lastProfileUpdate   SaaSBillingProfileUpdate
	lastInvoiceOptions  SaaSInvoiceDocumentOptions
	lastInvoiceCreate   SaaSInvoiceDocumentCreate
	lastTransition      SaaSInvoiceDocumentTransition
}

func newFakeSaaSInvoiceStore() *fakeSaaSInvoiceStore {
	return &fakeSaaSInvoiceStore{fakeSaaSAdminRefundStore: newFakeSaaSAdminRefundStore()}
}

func (s *fakeSaaSInvoiceStore) SaaSBillingProfile(_ context.Context, tenantID int) (SaaSBillingProfile, bool, error) {
	s.lastProfileTenantID = tenantID
	return s.profile, s.profileExists, nil
}

func (s *fakeSaaSInvoiceStore) SaveSaaSBillingProfile(_ context.Context, update SaaSBillingProfileUpdate) (SaaSBillingProfileUpdateResult, error) {
	s.lastProfileUpdate = update
	if s.profileResult.Profile.TenantID == 0 {
		s.profileResult.Profile = SaaSBillingProfile{TenantID: update.TenantID, InvoiceType: update.InvoiceType, InvoiceTitle: update.InvoiceTitle, TaxIdentifier: update.TaxIdentifier, Email: update.Email, Version: update.ExpectedVersion + 1, Status: 1}
	}
	return s.profileResult, nil
}

func (s *fakeSaaSInvoiceStore) SaaSInvoiceDocuments(_ context.Context, options SaaSInvoiceDocumentOptions) (SaaSInvoiceDocumentReport, error) {
	s.lastInvoiceOptions = options
	return s.invoiceReport, nil
}

func (s *fakeSaaSInvoiceStore) CreateSaaSInvoiceDocument(_ context.Context, create SaaSInvoiceDocumentCreate) (SaaSInvoiceDocumentCreateResult, error) {
	s.lastInvoiceCreate = create
	if s.invoiceCreateResult.Document.DocumentNo == "" {
		s.invoiceCreateResult.Document = SaaSInvoiceDocument{DocumentNo: create.DocumentNo, TenantID: create.TenantID, Kind: create.Kind, OrderNo: create.OrderNo, AmountCents: create.AmountCents, Currency: create.Currency, Status: SaaSInvoiceStatusRequested}
	}
	return s.invoiceCreateResult, nil
}

func (s *fakeSaaSInvoiceStore) TransitionSaaSInvoiceDocument(_ context.Context, transition SaaSInvoiceDocumentTransition) (SaaSInvoiceDocumentTransitionResult, error) {
	s.lastTransition = transition
	return s.transitionResult, nil
}
