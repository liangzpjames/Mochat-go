package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeSaaSPaymentSettlementResolveApprovalStore struct {
	*fakeSaaSPaymentSettlementCloseApprovalStore
	entry         SaaSPaymentSettlementEntry
	batch         SaaSPaymentSettlementBatch
	resolveResult SaaSPaymentSettlementEntryResolveResult
	lastResolve   SaaSPaymentSettlementEntryResolve
	resolveCalls  int
}

func newFakeSaaSPaymentSettlementResolveApprovalStore(user User) *fakeSaaSPaymentSettlementResolveApprovalStore {
	base := newSaaSPaymentSettlementCloseApprovalStore(user)
	batch := base.report.Batches[0]
	batch.OpenIssueCount = 1
	batch.ResolvedIssueCount = 0
	batch.DifferenceAmountCents = 200
	entry := SaaSPaymentSettlementEntry{
		ID: 9301, BatchID: batch.ID, BatchNo: batch.BatchNo, BatchStatus: batch.Status, LineNo: 4,
		Provider: batch.Provider, ProviderTransactionNo: "GW-TXN-9301", TransactionType: SaaSPaymentSettlementTransactionPayment,
		OrderNo: "PAY-9301", ProviderOrderNo: "GW-PAY-9301", AmountCents: 88000, FeeCents: -800,
		NetAmountCents: 87200, Currency: "CNY", OccurredAt: "2026-07-02 00:00:30",
		MatchedPaymentOrderID: 701, MatchedTenantID: 9, MatchedTenantName: "测试租户",
		MatchedInternalNo: "PAY-9301", ExpectedAmountCents: 87800, ExpectedCurrency: "CNY", ExpectedStatus: "paid",
		ReconciliationStatus: SaaSPaymentSettlementReconciliationAmountMismatch, DifferenceAmountCents: 200,
		IssueCodesJSON: `["amount_mismatch"]`, IssueMessage: "结算金额与内部订单不一致",
		HandlingStatus: SaaSPaymentSettlementHandlingOpen, Version: 3, RawJSON: `{"source":"gateway"}`,
		CreatedAt: "2026-07-02 00:01:00", UpdatedAt: "2026-07-02 00:02:00",
	}
	return &fakeSaaSPaymentSettlementResolveApprovalStore{
		fakeSaaSPaymentSettlementCloseApprovalStore: base,
		entry: entry, batch: batch,
		resolveResult: SaaSPaymentSettlementEntryResolveResult{
			Entry:       SaaSPaymentSettlementEntry{ID: entry.ID, BatchID: batch.ID, BatchNo: batch.BatchNo, HandlingStatus: SaaSPaymentSettlementHandlingResolved, Version: 4},
			Batch:       SaaSPaymentSettlementBatch{ID: batch.ID, BatchNo: batch.BatchNo, Status: batch.Status, OpenIssueCount: 0, ResolvedIssueCount: 1, Version: 8},
			OperationID: 9302,
		},
	}
}

func (s *fakeSaaSPaymentSettlementResolveApprovalStore) SaaSAdminPaymentSettlementResolveApprovalSnapshot(context.Context, int64) (SaaSPaymentSettlementEntry, SaaSPaymentSettlementBatch, error) {
	return s.entry, s.batch, nil
}

func (s *fakeSaaSPaymentSettlementResolveApprovalStore) ResolveSaaSAdminPaymentSettlementEntry(_ context.Context, input SaaSPaymentSettlementEntryResolve) (SaaSPaymentSettlementEntryResolveResult, error) {
	s.resolveCalls++
	s.lastResolve = input
	return s.resolveResult, nil
}

func TestSaaSPaymentSettlementResolveDirectRequiresApproval(t *testing.T) {
	store := newFakeSaaSPaymentSettlementResolveApprovalStore(User{ID: 1, Name: "平台超管", TenantID: 1, IsSuperAdmin: 1})
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/paymentSettlementResolve", strings.NewReader(`{
		"entryId":9301,"expectedVersion":3,"handlingStatus":"resolved","reason":"财务已核验"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ResolvePaymentSettlementEntry(rec, req)
	if rec.Code != http.StatusPreconditionRequired || store.resolveCalls != 0 ||
		!strings.Contains(rec.Body.String(), SaaSAdminApprovalActionPaymentSettlementResolve) {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.resolveCalls, rec.Body.String())
	}
}

func TestSaaSPaymentSettlementResolveApprovalRequestFreezesEntryAndBatch(t *testing.T) {
	store := newFakeSaaSPaymentSettlementResolveApprovalStore(User{ID: 7, Name: "财务申请人", TenantID: 1, IsSuperAdmin: 1})
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"payment.settlement.resolve",
		"payload":{"entryId":9301,"expectedVersion":3,"handlingStatus":"ignored","reason":"渠道确认无需入账"},
		"reason":"财务与内控复核差异","idempotencyKey":"settlement-resolve-9301"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusOK || store.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
	input := store.createInput
	if input.ActionType != SaaSAdminApprovalActionPaymentSettlementResolve || input.RiskLevel != SaaSAdminApprovalRiskCritical ||
		input.RequiredPermission != SaaSAdminPermissionFinanceManage || input.RequiredApprovals != 2 ||
		input.TargetType != SaaSAdminOperationTargetSettlementEntry || input.TargetID != "9301" {
		t.Fatalf("create input = %+v", input)
	}
	var plan SaaSPaymentSettlementResolveApprovalPlan
	if err := json.Unmarshal([]byte(input.RequestJSON), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.SchemaVersion != SaaSPaymentSettlementResolveApprovalPlanSchemaVersion ||
		plan.Resolve.EntryID != 9301 || plan.Resolve.ExpectedVersion != 3 || plan.Resolve.HandlingStatus != SaaSPaymentSettlementHandlingIgnored ||
		plan.Entry.ID != 9301 || plan.Entry.BatchID != 91 || plan.Entry.Version != 3 ||
		plan.Entry.RawJSON != `{"source":"gateway"}` || plan.Entry.IssueCodesJSON != `["amount_mismatch"]` ||
		plan.Batch.ID != 91 || plan.Batch.Version != 7 || plan.Batch.OpenIssueCount != 1 || plan.Batch.DifferenceAmountCents != 200 {
		t.Fatalf("frozen plan = %+v", plan)
	}
}

func TestSaaSPaymentSettlementResolveApprovalRequestRejectsInvalidState(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*fakeSaaSPaymentSettlementResolveApprovalStore)
		body   string
		want   string
	}{
		{name: "stale", body: `{"entryId":9301,"expectedVersion":2,"handlingStatus":"resolved","reason":"核验"}`, want: "版本已变化"},
		{name: "matched", mutate: func(s *fakeSaaSPaymentSettlementResolveApprovalStore) {
			s.entry.ReconciliationStatus = SaaSPaymentSettlementReconciliationMatched
			s.entry.HandlingStatus = SaaSPaymentSettlementHandlingNone
		}, body: `{"entryId":9301,"expectedVersion":3,"handlingStatus":"resolved","reason":"核验"}`, want: "不需要处理差异"},
		{name: "closed", mutate: func(s *fakeSaaSPaymentSettlementResolveApprovalStore) {
			s.entry.BatchStatus = SaaSPaymentSettlementBatchStatusClosed
			s.batch.Status = SaaSPaymentSettlementBatchStatusClosed
		}, body: `{"entryId":9301,"expectedVersion":3,"handlingStatus":"resolved","reason":"核验"}`, want: "不是已对账状态"},
		{name: "same status", body: `{"entryId":9301,"expectedVersion":3,"handlingStatus":"open","reason":"核验"}`, want: "状态变化无效"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newFakeSaaSPaymentSettlementResolveApprovalStore(User{ID: 7, Name: "财务申请人", TenantID: 1, IsSuperAdmin: 1})
			if test.mutate != nil {
				test.mutate(store)
			}
			handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
			req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{"actionType":"payment.settlement.resolve","payload":`+test.body+`,"reason":"复核","idempotencyKey":"invalid-`+test.name+`"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Mochat-Go-User-ID", "7")
			rec := httptest.NewRecorder()
			handler.ApprovalRequest(rec, req)
			if rec.Code != http.StatusConflict || store.createCalls != 0 || !strings.Contains(rec.Body.String(), test.want) {
				t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
			}
		})
	}
}

func TestSaaSPaymentSettlementResolveApprovalExecuteUsesFrozenPlan(t *testing.T) {
	store := newFakeSaaSPaymentSettlementResolveApprovalStore(User{ID: 9, Name: "财务执行人", TenantID: 1, IsSuperAdmin: 1})
	plan, err := (&SaaSAdminHandler{store: store}).planSaaSPaymentSettlementResolve(context.Background(), SaaSPaymentSettlementEntryResolve{
		EntryID: 9301, ExpectedVersion: 3, HandlingStatus: SaaSPaymentSettlementHandlingResolved, Reason: "差异已核验",
	})
	if err != nil {
		t.Fatal(err)
	}
	requestJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	store.beginResult = SaaSAdminApproval{
		ID: 93, RequestNo: "APR-SETTLEMENT-RESOLVE-93", ActionType: SaaSAdminApprovalActionPaymentSettlementResolve,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 6,
		RequestJSON: string(requestJSON),
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":93,"expectedVersion":5}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)
	if rec.Code != http.StatusOK || store.resolveCalls != 1 || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d calls=%d finish=%+v body=%s", rec.Code, store.resolveCalls, store.finishInput, rec.Body.String())
	}
	input := store.lastResolve
	if input.ActorUserID != 9 || input.ActorTenantID != 1 || input.ApprovalExecutionID != 93 ||
		input.ApprovalExecutionVersion != 6 || input.ApprovalPlan == nil || input.ApprovalPlan.Entry.ID != 9301 ||
		input.ApprovalPlan.Batch.ID != 91 || input.HandlingStatus != SaaSPaymentSettlementHandlingResolved {
		t.Fatalf("resolve input = %+v", input)
	}
}
