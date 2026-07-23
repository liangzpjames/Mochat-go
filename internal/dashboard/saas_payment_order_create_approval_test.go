package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeSaaSPaymentOrderCreateApprovalStore struct {
	*fakeSaaSAdminApprovalStore
	createResult SaaSAdminPaymentOrderCreateResult
	lastCreate   SaaSAdminPaymentOrderCreate
	createCalls  int
}

func newSaaSPaymentOrderCreateApprovalStore(user User) *fakeSaaSPaymentOrderCreateApprovalStore {
	base := &fakeSaaSAdminStore{
		users: map[int]User{user.ID: user},
		overview: SaaSAdminOverview{Tenants: []SaaSAdminTenantOverview{{
			TenantID: 12, TenantName: "收款客户", TenantStatus: 1,
		}}},
		packages: []SaaSAdminPackage{{
			Code: "growth", Name: "增长版", Status: 1, Version: 4,
			Limits: SaaSAdminPackageLimits{MaxUsers: 50, MaxContacts: 5000, StorageMB: 2048},
		}},
	}
	return &fakeSaaSPaymentOrderCreateApprovalStore{
		fakeSaaSAdminApprovalStore: &fakeSaaSAdminApprovalStore{
			fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{fakeSaaSAdminStore: base},
		},
		createResult: SaaSAdminPaymentOrderCreateResult{
			Order: SaaSAdminPaymentOrder{
				ID: 90, OrderNo: "APR-PAY-ORDER", TenantID: 12, TenantName: "收款客户",
				Status: SaaSPaymentOrderStatusPending, PackageCode: "growth", PackageName: "增长版",
				PackageVersion: 4, PackageLimits: SaaSAdminPackageLimits{MaxUsers: 50, MaxContacts: 5000, StorageMB: 2048},
				AmountCents: 128000, Currency: "CNY", Version: 1,
			},
			OperationID: 9001,
		},
	}
}

func (s *fakeSaaSPaymentOrderCreateApprovalStore) SaaSAdminPaymentOrders(context.Context, SaaSAdminPaymentOrderOptions) (SaaSAdminPaymentOrderReport, error) {
	return SaaSAdminPaymentOrderReport{}, nil
}

func (s *fakeSaaSPaymentOrderCreateApprovalStore) SaaSAdminPaymentWebhookEvents(context.Context, SaaSAdminPaymentWebhookEventOptions) ([]SaaSAdminPaymentWebhookEvent, error) {
	return nil, nil
}

func (s *fakeSaaSPaymentOrderCreateApprovalStore) CreateSaaSAdminPaymentOrder(_ context.Context, input SaaSAdminPaymentOrderCreate) (SaaSAdminPaymentOrderCreateResult, error) {
	s.createCalls++
	s.lastCreate = input
	return s.createResult, nil
}

func (s *fakeSaaSPaymentOrderCreateApprovalStore) CancelSaaSAdminPaymentOrder(context.Context, SaaSAdminPaymentOrderCancel) (SaaSAdminPaymentOrderCancelResult, error) {
	return SaaSAdminPaymentOrderCancelResult{}, nil
}

func (s *fakeSaaSPaymentOrderCreateApprovalStore) ProcessSaaSPaymentDunning(context.Context, SaaSPaymentDunningOptions) (SaaSPaymentDunningResult, error) {
	return SaaSPaymentDunningResult{}, nil
}

const paymentOrderCreateApprovalPayload = `{
	"tenantId":12,"provider":"gateway","packageCode":"growth","billingCycle":"yearly",
	"serviceExpiresAt":"2037-12-31 00:00:00","amountCents":128000,"currency":"CNY",
	"checkoutExpiresAt":"2037-01-01 00:30:00","maxDunningAttempts":3,
	"idempotencyKey":"payment-order-unit","remark":"年度合同收款"
}`

func TestSaaSPaymentOrderCreateDirectRequiresApproval(t *testing.T) {
	store := newSaaSPaymentOrderCreateApprovalStore(User{ID: 1, Name: "平台超管", TenantID: 1, IsSuperAdmin: 1})
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/paymentOrder", strings.NewReader(paymentOrderCreateApprovalPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.CreatePaymentOrder(rec, req)

	if rec.Code != http.StatusPreconditionRequired || store.createCalls != 0 ||
		!strings.Contains(rec.Body.String(), SaaSAdminApprovalActionPaymentOrderCreate) {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
}

func TestSaaSPaymentOrderCreateApprovalFreezesTenantAndPackage(t *testing.T) {
	store := newSaaSPaymentOrderCreateApprovalStore(User{ID: 7, Name: "财务申请人", TenantID: 1, IsSuperAdmin: 1})
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"payment.order.create","payload":`+paymentOrderCreateApprovalPayload+`,
		"reason":"合同、租户与套餐权益已复核","idempotencyKey":"approval-payment-order-unit"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)

	if rec.Code != http.StatusOK || store.fakeSaaSAdminApprovalStore.createCalls != 1 {
		t.Fatalf("status=%d approvalCalls=%d body=%s", rec.Code, store.fakeSaaSAdminApprovalStore.createCalls, rec.Body.String())
	}
	input := store.createInput
	if input.ActionType != SaaSAdminApprovalActionPaymentOrderCreate ||
		input.RequiredPermission != SaaSAdminPermissionFinanceManage || input.RequiredApprovals != 2 ||
		input.TargetType != SaaSAdminOperationTargetPaymentOrder || !strings.HasPrefix(input.TargetID, "APR-PAY-") {
		t.Fatalf("approval input = %+v", input)
	}
	var plan SaaSAdminPaymentOrderCreateApprovalPlan
	if err := json.Unmarshal([]byte(input.RequestJSON), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Create.OrderNo != input.TargetID || plan.Create.IdempotencyKey != "payment-order-unit" ||
		plan.Create.ExpectedTenantStatus != 1 || plan.Create.ExpectedPackageVersion != 4 ||
		plan.Tenant.TenantID != 12 || plan.Tenant.TenantName != "收款客户" ||
		plan.Package.Code != "growth" || plan.Package.Version != 4 ||
		plan.Package.Limits.MaxUsers != 50 || plan.Package.Limits.MaxContacts != 5000 || plan.Package.Limits.StorageMB != 2048 {
		t.Fatalf("frozen plan = %+v", plan)
	}
}

func TestSaaSPaymentOrderCreateApprovalExecuteUsesFrozenPlan(t *testing.T) {
	store := newSaaSPaymentOrderCreateApprovalStore(User{ID: 9, Name: "财务执行人", TenantID: 1, IsSuperAdmin: 1})
	plan, err := (&SaaSAdminHandler{store: store}).planSaaSAdminPaymentOrderCreate(context.Background(), SaaSAdminPaymentOrderCreate{
		OrderNo: "APR-PAY-ORDER", TenantID: 12, Provider: "gateway", IdempotencyKey: "payment-order-unit",
		PackageCode: "growth", BillingCycle: "yearly", ServiceExpiresAt: "2037-12-31 00:00:00",
		AmountCents: 128000, Currency: "CNY", CheckoutExpiresAt: "2037-01-01 00:30:00", MaxDunningAttempts: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	requestJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	store.beginResult = SaaSAdminApproval{
		ID: 90, RequestNo: "APR-PAYMENT-90", ActionType: SaaSAdminApprovalActionPaymentOrderCreate,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 6,
		RequestJSON: string(requestJSON),
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":90,"expectedVersion":5}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)

	if rec.Code != http.StatusOK || store.createCalls != 1 || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d createCalls=%d finish=%+v body=%s", rec.Code, store.createCalls, store.finishInput, rec.Body.String())
	}
	input := store.lastCreate
	if input.OrderNo != "APR-PAY-ORDER" || input.ExpectedTenantStatus != 1 || input.ExpectedPackageVersion != 4 ||
		input.ActorUserID != 9 || input.ActorTenantID != 1 ||
		input.ApprovalExecutionID != 90 || input.ApprovalExecutionVersion != 6 {
		t.Fatalf("payment create input = %+v", input)
	}
	data := decodeSaaSAdminResponse(t, rec)
	result := data["result"].(map[string]any)
	order := result["order"].(map[string]any)
	if result["operationId"].(float64) != 9001 || order["packageVersion"].(float64) != 4 {
		t.Fatalf("result = %+v", result)
	}
}
