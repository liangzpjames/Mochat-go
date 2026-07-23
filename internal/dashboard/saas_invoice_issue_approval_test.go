package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeSaaSInvoiceIssueApprovalStore struct {
	*fakeSaaSAdminApprovalStore
	invoiceReport          SaaSInvoiceDocumentReport
	transitionResult       SaaSInvoiceDocumentTransitionResult
	lastInvoiceOptions     SaaSInvoiceDocumentOptions
	lastTransition         SaaSInvoiceDocumentTransition
	invoiceTransitionCalls int
}

func newSaaSInvoiceIssueApprovalStore(user User) *fakeSaaSInvoiceIssueApprovalStore {
	base := &fakeSaaSAdminStore{users: map[int]User{user.ID: user}}
	return &fakeSaaSInvoiceIssueApprovalStore{
		fakeSaaSAdminApprovalStore: &fakeSaaSAdminApprovalStore{
			fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{fakeSaaSAdminStore: base},
		},
		invoiceReport: SaaSInvoiceDocumentReport{Documents: []SaaSInvoiceDocument{{
			ID: 31, DocumentNo: "INV-ISSUE-31", TenantID: 12, TenantName: "开票客户",
			PaymentOrderID: 41, OrderNo: "PAY-ISSUE-41", OrderStatus: SaaSPaymentOrderStatusPaid, OrderVersion: 8,
			Kind: SaaSInvoiceKindInvoice, Status: SaaSInvoiceStatusProcessing, AmountCents: 88000, Currency: "CNY",
			InvoiceType: SaaSInvoiceTypeSpecial, InvoiceTitle: "开票客户有限公司", TaxIdentifier: "91310000TEST000012",
			Provider: "manual", Version: 3, OrderAmountCents: 100000, OrderRefundPending: 0,
			OrderRefundedCents: 0, OrderInvoicePending: 88000, OrderInvoicedCents: 0,
			OrderCreditPending: 0, OrderCreditedCents: 0,
		}}},
		transitionResult: SaaSInvoiceDocumentTransitionResult{
			Document: SaaSInvoiceDocument{
				ID: 31, DocumentNo: "INV-ISSUE-31", TenantID: 12, OrderNo: "PAY-ISSUE-41",
				Kind: SaaSInvoiceKindInvoice, Status: SaaSInvoiceStatusIssued, Version: 4,
			},
			Order: SaaSAdminPaymentOrder{
				ID: 41, OrderNo: "PAY-ISSUE-41", TenantID: 12, Status: SaaSPaymentOrderStatusPaid,
				AmountCents: 100000, InvoicedAmountCents: 88000, Version: 9,
			},
			PreviousStatus: SaaSInvoiceStatusProcessing,
			OperationID:    8902,
		},
	}
}

func (s *fakeSaaSInvoiceIssueApprovalStore) SaaSBillingProfile(context.Context, int) (SaaSBillingProfile, bool, error) {
	return SaaSBillingProfile{}, false, nil
}

func (s *fakeSaaSInvoiceIssueApprovalStore) SaveSaaSBillingProfile(context.Context, SaaSBillingProfileUpdate) (SaaSBillingProfileUpdateResult, error) {
	return SaaSBillingProfileUpdateResult{}, nil
}

func (s *fakeSaaSInvoiceIssueApprovalStore) SaaSInvoiceDocuments(_ context.Context, options SaaSInvoiceDocumentOptions) (SaaSInvoiceDocumentReport, error) {
	s.lastInvoiceOptions = options
	report := s.invoiceReport
	report.Options = options
	return report, nil
}

func (s *fakeSaaSInvoiceIssueApprovalStore) CreateSaaSInvoiceDocument(context.Context, SaaSInvoiceDocumentCreate) (SaaSInvoiceDocumentCreateResult, error) {
	return SaaSInvoiceDocumentCreateResult{}, nil
}

func (s *fakeSaaSInvoiceIssueApprovalStore) TransitionSaaSInvoiceDocument(_ context.Context, transition SaaSInvoiceDocumentTransition) (SaaSInvoiceDocumentTransitionResult, error) {
	s.invoiceTransitionCalls++
	s.lastTransition = transition
	return s.transitionResult, nil
}

func TestSaaSInvoiceIssueDirectRequiresApprovalButProcessingDoesNot(t *testing.T) {
	store := newSaaSInvoiceIssueApprovalStore(User{ID: 1, Name: "平台超管", TenantID: 1, IsSuperAdmin: 1})
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)

	issueReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/invoiceTransition", strings.NewReader(`{
		"documentNo":"INV-ISSUE-31","expectedVersion":3,"status":"issued",
		"provider":"manual","providerDocumentNo":"FP-ISSUE-31"
	}`))
	issueReq.Header.Set("Content-Type", "application/json")
	issueReq.Header.Set("X-Mochat-Go-User-ID", "1")
	issueRec := httptest.NewRecorder()
	handler.TransitionInvoiceDocument(issueRec, issueReq)
	if issueRec.Code != http.StatusPreconditionRequired || store.invoiceTransitionCalls != 0 ||
		!strings.Contains(issueRec.Body.String(), SaaSAdminApprovalActionInvoiceIssue) {
		t.Fatalf("issue status=%d calls=%d body=%s", issueRec.Code, store.invoiceTransitionCalls, issueRec.Body.String())
	}

	processingReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/invoiceTransition", strings.NewReader(`{
		"documentNo":"INV-ISSUE-31","expectedVersion":3,"status":"processing","provider":"manual"
	}`))
	processingReq.Header.Set("Content-Type", "application/json")
	processingReq.Header.Set("X-Mochat-Go-User-ID", "1")
	processingRec := httptest.NewRecorder()
	handler.TransitionInvoiceDocument(processingRec, processingReq)
	if processingRec.Code != http.StatusOK || store.invoiceTransitionCalls != 1 ||
		store.lastTransition.Status != SaaSInvoiceStatusProcessing {
		t.Fatalf("processing status=%d calls=%d transition=%+v body=%s", processingRec.Code, store.invoiceTransitionCalls, store.lastTransition, processingRec.Body.String())
	}
}

func TestSaaSInvoiceIssueApprovalRequestFreezesDocumentAndOrder(t *testing.T) {
	store := newSaaSInvoiceIssueApprovalStore(User{ID: 7, Name: "财务申请人", TenantID: 1, IsSuperAdmin: 1})
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"billing.invoice.issue",
		"payload":{"documentNo":"INV-ISSUE-31","expectedVersion":3,"status":"issued","provider":"manual","providerDocumentNo":"FP-ISSUE-31","documentUrl":"https://invoice.example.test/FP-ISSUE-31.pdf","remark":"正式开具"},
		"reason":"票面和回款已复核","idempotencyKey":"approval-unit-invoice-issue"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)

	if rec.Code != http.StatusOK || store.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
	input := store.createInput
	if input.ActionType != SaaSAdminApprovalActionInvoiceIssue ||
		input.RequiredPermission != SaaSAdminPermissionFinanceManage ||
		input.RequiredApprovals != 2 ||
		input.TargetType != SaaSAdminOperationTargetInvoiceDocument ||
		input.TargetID != "INV-ISSUE-31" ||
		input.TargetName != "开票客户 / 蓝票 / INV-ISSUE-31" {
		t.Fatalf("create input = %+v", input)
	}
	if store.lastInvoiceOptions.DocumentNo != "INV-ISSUE-31" || store.lastInvoiceOptions.Limit != 2 {
		t.Fatalf("invoice options = %+v", store.lastInvoiceOptions)
	}
	var plan SaaSInvoiceIssueApprovalPlan
	if err := json.Unmarshal([]byte(input.RequestJSON), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Document.ID != 31 || plan.Document.Version != 3 ||
		plan.Document.Status != SaaSInvoiceStatusProcessing ||
		plan.Document.AmountCents != 88000 ||
		plan.Order.ID != 41 || plan.Order.Version != 8 ||
		plan.Order.InvoicePendingCents != 88000 ||
		plan.Transition.Status != SaaSInvoiceStatusIssued ||
		plan.Transition.ExpectedVersion != 3 ||
		plan.Transition.ProviderDocumentNo != "FP-ISSUE-31" {
		t.Fatalf("frozen plan = %+v", plan)
	}
}

func TestSaaSInvoiceIssueApprovalRequestRejectsStaleVersion(t *testing.T) {
	store := newSaaSInvoiceIssueApprovalStore(User{ID: 7, Name: "财务申请人", TenantID: 1, IsSuperAdmin: 1})
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"billing.invoice.issue",
		"payload":{"documentNo":"INV-ISSUE-31","expectedVersion":2,"status":"issued","provider":"manual","providerDocumentNo":"FP-ISSUE-31"},
		"reason":"票面和回款已复核","idempotencyKey":"approval-unit-invoice-issue-stale"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)

	if rec.Code != http.StatusConflict || store.createCalls != 0 ||
		!strings.Contains(rec.Body.String(), "发票单据版本已变化") {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
}

func TestSaaSInvoiceIssueApprovalExecuteUsesFrozenPlan(t *testing.T) {
	store := newSaaSInvoiceIssueApprovalStore(User{ID: 9, Name: "财务执行人", TenantID: 1, IsSuperAdmin: 1})
	plan, err := (&SaaSAdminHandler{store: store}).planSaaSInvoiceIssue(context.Background(), SaaSInvoiceDocumentTransition{
		DocumentNo: "INV-ISSUE-31", ExpectedVersion: 3, Status: SaaSInvoiceStatusIssued,
		Provider: "manual", ProviderDocumentNo: "FP-ISSUE-31", Remark: "正式开具",
	})
	if err != nil {
		t.Fatal(err)
	}
	requestJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	store.beginResult = SaaSAdminApproval{
		ID: 89, RequestNo: "APR-INVOICE-89", ActionType: SaaSAdminApprovalActionInvoiceIssue,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 6,
		RequestJSON: string(requestJSON),
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":89,"expectedVersion":5}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)

	if rec.Code != http.StatusOK || store.invoiceTransitionCalls != 1 ||
		store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d calls=%d finish=%+v body=%s", rec.Code, store.invoiceTransitionCalls, store.finishInput, rec.Body.String())
	}
	input := store.lastTransition
	if input.ActorUserID != 9 || input.ActorTenantID != 1 ||
		input.ApprovalExecutionID != 89 || input.ApprovalExecutionVersion != 6 ||
		input.ApprovalPlan == nil || input.ApprovalPlan.Document.ID != 31 ||
		input.ApprovalPlan.Order.Version != 8 ||
		input.Status != SaaSInvoiceStatusIssued {
		t.Fatalf("transition input = %+v", input)
	}
	data := decodeSaaSAdminResponse(t, rec)
	result := data["result"].(map[string]any)
	if result["operationId"].(float64) != 8902 ||
		result["previousStatus"] != SaaSInvoiceStatusProcessing {
		t.Fatalf("data = %+v", data)
	}
}
