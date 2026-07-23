package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

type fakeSaaSPaymentSettlementCloseApprovalStore struct {
	*fakeSaaSAdminApprovalStore
	report           SaaSPaymentSettlementBatchReport
	transitionResult SaaSPaymentSettlementTransitionResult
	lastOptions      SaaSPaymentSettlementBatchOptions
	lastTransition   SaaSPaymentSettlementTransition
	transitionCalls  int
}

func newSaaSPaymentSettlementCloseApprovalStore(user User) *fakeSaaSPaymentSettlementCloseApprovalStore {
	base := &fakeSaaSAdminStore{users: map[int]User{user.ID: user}}
	return &fakeSaaSPaymentSettlementCloseApprovalStore{
		fakeSaaSAdminApprovalStore: &fakeSaaSAdminApprovalStore{
			fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{fakeSaaSAdminStore: base},
		},
		report: SaaSPaymentSettlementBatchReport{Batches: []SaaSPaymentSettlementBatch{{
			ID: 91, BatchNo: "SET-CLOSE-91", Provider: "gateway", ProviderSettlementNo: "GW-SET-CLOSE-91",
			PeriodStart: "2026-07-01 00:00:00", PeriodEnd: "2026-07-02 00:00:00", Currency: "CNY",
			Status: SaaSPaymentSettlementBatchStatusReconciled, SourceSHA256: strings.Repeat("a", 64),
			EntryCount: 4, PaymentCount: 3, RefundCount: 1, MatchedCount: 3, IssueCount: 1,
			OpenIssueCount: 0, ResolvedIssueCount: 1, IgnoredIssueCount: 0,
			TotalAmountCents: 88000, TotalFeeCents: -800, TotalNetCents: 87200,
			DifferenceAmountCents: 200, ImportedByUserID: 3, ImportedByTenantID: 1,
			ImportedAt: "2026-07-02 00:01:00", ReconciledAt: "2026-07-02 00:02:00",
			Version: 7, Remark: "渠道日结", CreatedAt: "2026-07-02 00:01:00", UpdatedAt: "2026-07-02 00:02:00",
		}}},
		transitionResult: SaaSPaymentSettlementTransitionResult{
			Batch:          SaaSPaymentSettlementBatch{ID: 91, BatchNo: "SET-CLOSE-91", Status: SaaSPaymentSettlementBatchStatusClosed, Version: 8},
			PreviousStatus: SaaSPaymentSettlementBatchStatusReconciled, OperationID: 9102,
		},
	}
}

func (s *fakeSaaSPaymentSettlementCloseApprovalStore) SaaSAdminPaymentSettlementBatches(_ context.Context, options SaaSPaymentSettlementBatchOptions) (SaaSPaymentSettlementBatchReport, error) {
	s.lastOptions = options
	report := s.report
	report.Options = options
	return report, nil
}

func (s *fakeSaaSPaymentSettlementCloseApprovalStore) SaaSAdminPaymentSettlementEntries(context.Context, SaaSPaymentSettlementEntryOptions) (SaaSPaymentSettlementEntryReport, error) {
	return SaaSPaymentSettlementEntryReport{}, nil
}

func (s *fakeSaaSPaymentSettlementCloseApprovalStore) ImportSaaSAdminPaymentSettlement(context.Context, SaaSPaymentSettlementImport) (SaaSPaymentSettlementImportResult, error) {
	return SaaSPaymentSettlementImportResult{}, nil
}

func (s *fakeSaaSPaymentSettlementCloseApprovalStore) ReconcileSaaSAdminPaymentSettlement(context.Context, SaaSPaymentSettlementReconcile) (SaaSPaymentSettlementReconcileResult, error) {
	return SaaSPaymentSettlementReconcileResult{}, nil
}

func (s *fakeSaaSPaymentSettlementCloseApprovalStore) ResolveSaaSAdminPaymentSettlementEntry(context.Context, SaaSPaymentSettlementEntryResolve) (SaaSPaymentSettlementEntryResolveResult, error) {
	return SaaSPaymentSettlementEntryResolveResult{}, nil
}

func (s *fakeSaaSPaymentSettlementCloseApprovalStore) TransitionSaaSAdminPaymentSettlement(_ context.Context, input SaaSPaymentSettlementTransition) (SaaSPaymentSettlementTransitionResult, error) {
	s.transitionCalls++
	s.lastTransition = input
	return s.transitionResult, nil
}

func TestSaaSPaymentSettlementCloseAndReopenDirectRequireApproval(t *testing.T) {
	store := newSaaSPaymentSettlementCloseApprovalStore(User{ID: 1, Name: "平台超管", TenantID: 1, IsSuperAdmin: 1})
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)

	closeReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/paymentSettlementTransition", strings.NewReader(`{
		"batchNo":"SET-CLOSE-91","expectedVersion":7,"action":"close","reason":"完成对账"
	}`))
	closeReq.Header.Set("Content-Type", "application/json")
	closeReq.Header.Set("X-Mochat-Go-User-ID", "1")
	closeRec := httptest.NewRecorder()
	handler.TransitionPaymentSettlement(closeRec, closeReq)
	if closeRec.Code != http.StatusPreconditionRequired || store.transitionCalls != 0 ||
		!strings.Contains(closeRec.Body.String(), SaaSAdminApprovalActionPaymentSettlementClose) {
		t.Fatalf("close status=%d calls=%d body=%s", closeRec.Code, store.transitionCalls, closeRec.Body.String())
	}

	reopenReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/paymentSettlementTransition", strings.NewReader(`{
		"batchNo":"SET-CLOSE-91","expectedVersion":8,"action":"reopen","reason":"渠道补发结算明细"
	}`))
	reopenReq.Header.Set("Content-Type", "application/json")
	reopenReq.Header.Set("X-Mochat-Go-User-ID", "1")
	reopenRec := httptest.NewRecorder()
	handler.TransitionPaymentSettlement(reopenRec, reopenReq)
	if reopenRec.Code != http.StatusPreconditionRequired || store.transitionCalls != 0 ||
		!strings.Contains(reopenRec.Body.String(), SaaSAdminApprovalActionPaymentSettlementReopen) {
		t.Fatalf("reopen status=%d calls=%d input=%+v body=%s", reopenRec.Code, store.transitionCalls, store.lastTransition, reopenRec.Body.String())
	}
}

func TestSaaSPaymentSettlementCloseApprovalRequestFreezesBatchLedger(t *testing.T) {
	store := newSaaSPaymentSettlementCloseApprovalStore(User{ID: 7, Name: "财务申请人", TenantID: 1, IsSuperAdmin: 1})
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"payment.settlement.close",
		"payload":{"batchNo":"SET-CLOSE-91","expectedVersion":7,"action":"close","reason":"完成对账"},
		"reason":"渠道结算金额与差异已复核","idempotencyKey":"approval-unit-settlement-close"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)

	if rec.Code != http.StatusOK || store.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
	input := store.createInput
	if input.ActionType != SaaSAdminApprovalActionPaymentSettlementClose ||
		input.RequiredPermission != SaaSAdminPermissionFinanceManage || input.RequiredApprovals != 2 ||
		input.TargetType != SaaSAdminOperationTargetSettlementBatch || input.TargetID != "SET-CLOSE-91" ||
		input.TargetName != "gateway / GW-SET-CLOSE-91 / SET-CLOSE-91" {
		t.Fatalf("create input = %+v", input)
	}
	if store.lastOptions.BatchNo != "SET-CLOSE-91" || store.lastOptions.Status != SaaSPaymentSettlementBatchStatusAll || store.lastOptions.Limit != 2 {
		t.Fatalf("batch options = %+v", store.lastOptions)
	}
	var plan SaaSPaymentSettlementCloseApprovalPlan
	if err := json.Unmarshal([]byte(input.RequestJSON), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.SchemaVersion != SaaSPaymentSettlementApprovalPlanSchemaVersion ||
		plan.Batch.ID != 91 || plan.Batch.Version != 7 || plan.Batch.Status != SaaSPaymentSettlementBatchStatusReconciled ||
		plan.Batch.OpenIssueCount != 0 || plan.Batch.TotalAmountCents != 88000 || plan.Batch.TotalNetCents != 87200 ||
		plan.Batch.DifferenceAmountCents != 200 || plan.Transition.Action != SaaSPaymentSettlementTransitionClose ||
		plan.Transition.ExpectedVersion != 7 || plan.Batch.ImportedByUserID != 3 ||
		plan.Batch.ReconciledAt != "2026-07-02 00:02:00" || plan.Batch.UpdatedAt != "2026-07-02 00:02:00" {
		t.Fatalf("frozen plan = %+v", plan)
	}
}

func TestSaaSPaymentSettlementReopenApprovalRequestFreezesClosedLedger(t *testing.T) {
	store := newSaaSPaymentSettlementCloseApprovalStore(User{ID: 7, Name: "财务申请人", TenantID: 1, IsSuperAdmin: 1})
	batch := &store.report.Batches[0]
	batch.Status = SaaSPaymentSettlementBatchStatusClosed
	batch.Version = 8
	batch.ClosedByUserID = 9
	batch.ClosedAt = "2026-07-02 00:03:00"
	batch.CloseReason = "渠道金额与差异已复核"
	batch.UpdatedAt = "2026-07-02 00:03:00"
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"payment.settlement.reopen",
		"payload":{"batchNo":"SET-CLOSE-91","expectedVersion":8,"action":"reopen","reason":"渠道补发结算明细"},
		"reason":"需要重新核对渠道账单","idempotencyKey":"approval-unit-settlement-reopen"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)

	if rec.Code != http.StatusOK || store.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
	input := store.createInput
	if input.ActionType != SaaSAdminApprovalActionPaymentSettlementReopen ||
		input.RequiredPermission != SaaSAdminPermissionFinanceManage || input.RequiredApprovals != 2 ||
		input.TargetType != SaaSAdminOperationTargetSettlementBatch || input.TargetID != "SET-CLOSE-91" {
		t.Fatalf("create input = %+v", input)
	}
	var plan SaaSPaymentSettlementReopenApprovalPlan
	if err := json.Unmarshal([]byte(input.RequestJSON), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.SchemaVersion != SaaSPaymentSettlementApprovalPlanSchemaVersion ||
		plan.Batch.Status != SaaSPaymentSettlementBatchStatusClosed || plan.Batch.Version != 8 ||
		plan.Batch.ClosedByUserID != 9 || plan.Batch.ClosedAt != "2026-07-02 00:03:00" ||
		plan.Batch.CloseReason != "渠道金额与差异已复核" || plan.Batch.TotalNetCents != 87200 ||
		plan.Transition.Action != SaaSPaymentSettlementTransitionReopen || plan.Transition.ExpectedVersion != 8 {
		t.Fatalf("frozen reopen plan = %+v", plan)
	}
}

func TestSaaSPaymentSettlementReopenApprovalRequestRejectsNonClosedOrIncompleteAudit(t *testing.T) {
	for name, mutate := range map[string]func(*SaaSPaymentSettlementBatch){
		"not closed": func(batch *SaaSPaymentSettlementBatch) {
			batch.Status = SaaSPaymentSettlementBatchStatusReconciled
		},
		"missing close audit": func(batch *SaaSPaymentSettlementBatch) {
			batch.Status = SaaSPaymentSettlementBatchStatusClosed
			batch.ClosedByUserID = 9
			batch.ClosedAt = "2026-07-02 00:03:00"
			batch.CloseReason = ""
		},
	} {
		t.Run(name, func(t *testing.T) {
			store := newSaaSPaymentSettlementCloseApprovalStore(User{ID: 7, Name: "财务申请人", TenantID: 1, IsSuperAdmin: 1})
			batch := &store.report.Batches[0]
			batch.Version = 8
			mutate(batch)
			handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
			req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
				"actionType":"payment.settlement.reopen",
				"payload":{"batchNo":"SET-CLOSE-91","expectedVersion":8,"action":"reopen","reason":"重新核对"},
				"reason":"重新核对","idempotencyKey":"approval-unit-settlement-reopen-invalid"
			}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Mochat-Go-User-ID", "7")
			rec := httptest.NewRecorder()
			handler.ApprovalRequest(rec, req)
			if rec.Code != http.StatusConflict || store.createCalls != 0 {
				t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
			}
		})
	}
}

func TestSaaSPaymentSettlementCloseApprovalRequestRejectsStaleOrOpenBatch(t *testing.T) {
	for name, testCase := range map[string]struct {
		mutate   func(*fakeSaaSPaymentSettlementCloseApprovalStore)
		expected string
	}{
		"stale version": {mutate: func(_ *fakeSaaSPaymentSettlementCloseApprovalStore) {}, expected: "结算批次版本已变化"},
		"open issue": {mutate: func(store *fakeSaaSPaymentSettlementCloseApprovalStore) {
			store.report.Batches[0].OpenIssueCount = 1
		}, expected: "仍有未处理差异"},
	} {
		t.Run(name, func(t *testing.T) {
			store := newSaaSPaymentSettlementCloseApprovalStore(User{ID: 7, Name: "财务申请人", TenantID: 1, IsSuperAdmin: 1})
			testCase.mutate(store)
			version := 7
			if name == "stale version" {
				version = 6
			}
			handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
			body := `{"actionType":"payment.settlement.close","payload":{"batchNo":"SET-CLOSE-91","expectedVersion":` +
				strconv.Itoa(version) + `,"action":"close","reason":"完成对账"},"reason":"复核关账","idempotencyKey":"approval-unit-settlement-invalid"}`
			req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Mochat-Go-User-ID", "7")
			rec := httptest.NewRecorder()
			handler.ApprovalRequest(rec, req)
			if rec.Code != http.StatusConflict || store.createCalls != 0 || !strings.Contains(rec.Body.String(), testCase.expected) {
				t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
			}
		})
	}
}

func TestSaaSPaymentSettlementCloseApprovalExecuteUsesFrozenPlan(t *testing.T) {
	store := newSaaSPaymentSettlementCloseApprovalStore(User{ID: 9, Name: "财务执行人", TenantID: 1, IsSuperAdmin: 1})
	plan, err := (&SaaSAdminHandler{store: store}).planSaaSPaymentSettlementClose(context.Background(), SaaSPaymentSettlementTransition{
		BatchNo: "SET-CLOSE-91", ExpectedVersion: 7, Action: SaaSPaymentSettlementTransitionClose, Reason: "完成对账",
	})
	if err != nil {
		t.Fatal(err)
	}
	requestJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	store.beginResult = SaaSAdminApproval{
		ID: 91, RequestNo: "APR-SETTLEMENT-91", ActionType: SaaSAdminApprovalActionPaymentSettlementClose,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 6,
		RequestJSON: string(requestJSON),
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":91,"expectedVersion":5}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)

	if rec.Code != http.StatusOK || store.transitionCalls != 1 || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d calls=%d finish=%+v body=%s", rec.Code, store.transitionCalls, store.finishInput, rec.Body.String())
	}
	input := store.lastTransition
	if input.ActorUserID != 9 || input.ActorTenantID != 1 || input.ApprovalExecutionID != 91 ||
		input.ApprovalExecutionVersion != 6 || input.ApprovalPlan == nil || input.ApprovalPlan.Batch.ID != 91 ||
		input.ApprovalPlan.Batch.TotalNetCents != 87200 || input.Action != SaaSPaymentSettlementTransitionClose {
		t.Fatalf("transition input = %+v", input)
	}
}

func TestSaaSPaymentSettlementReopenApprovalExecuteUsesFrozenPlan(t *testing.T) {
	store := newSaaSPaymentSettlementCloseApprovalStore(User{ID: 11, Name: "财务执行人", TenantID: 1, IsSuperAdmin: 1})
	batch := &store.report.Batches[0]
	batch.Status = SaaSPaymentSettlementBatchStatusClosed
	batch.Version = 8
	batch.ClosedByUserID = 9
	batch.ClosedAt = "2026-07-02 00:03:00"
	batch.CloseReason = "渠道金额与差异已复核"
	batch.UpdatedAt = "2026-07-02 00:03:00"
	store.transitionResult = SaaSPaymentSettlementTransitionResult{
		Batch:          SaaSPaymentSettlementBatch{ID: 91, BatchNo: "SET-CLOSE-91", Status: SaaSPaymentSettlementBatchStatusReconciled, Version: 9},
		PreviousStatus: SaaSPaymentSettlementBatchStatusClosed, OperationID: 9202,
	}
	plan, err := (&SaaSAdminHandler{store: store}).planSaaSPaymentSettlementReopen(context.Background(), SaaSPaymentSettlementTransition{
		BatchNo: "SET-CLOSE-91", ExpectedVersion: 8, Action: SaaSPaymentSettlementTransitionReopen, Reason: "重新核对",
	})
	if err != nil {
		t.Fatal(err)
	}
	requestJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	store.beginResult = SaaSAdminApproval{
		ID: 92, RequestNo: "APR-SETTLEMENT-REOPEN-92", ActionType: SaaSAdminApprovalActionPaymentSettlementReopen,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 11, Version: 6,
		RequestJSON: string(requestJSON),
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":92,"expectedVersion":5}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "11")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)

	if rec.Code != http.StatusOK || store.transitionCalls != 1 || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d calls=%d finish=%+v body=%s", rec.Code, store.transitionCalls, store.finishInput, rec.Body.String())
	}
	input := store.lastTransition
	if input.ActorUserID != 11 || input.ApprovalExecutionID != 92 || input.ApprovalExecutionVersion != 6 ||
		input.ApprovalPlan == nil || input.ApprovalPlan.Batch.ClosedByUserID != 9 ||
		input.ApprovalPlan.Batch.TotalNetCents != 87200 || input.Action != SaaSPaymentSettlementTransitionReopen {
		t.Fatalf("transition input = %+v", input)
	}
}
