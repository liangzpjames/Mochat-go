package dashboard

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	SaaSAdminExportKindPaymentSettlementBatches = "paymentSettlementBatches"
	SaaSAdminExportKindPaymentSettlementEntries = "paymentSettlementEntries"

	SaaSPaymentSettlementBatchStatusAll        = "all"
	SaaSPaymentSettlementBatchStatusReconciled = "reconciled"
	SaaSPaymentSettlementBatchStatusClosed     = "closed"

	SaaSPaymentSettlementTransactionAll     = "all"
	SaaSPaymentSettlementTransactionPayment = "payment"
	SaaSPaymentSettlementTransactionRefund  = "refund"

	SaaSPaymentSettlementReconciliationAll                = "all"
	SaaSPaymentSettlementReconciliationMatched            = "matched"
	SaaSPaymentSettlementReconciliationMissingInternal    = "missing_internal"
	SaaSPaymentSettlementReconciliationIdentifierConflict = "identifier_conflict"
	SaaSPaymentSettlementReconciliationStatusMismatch     = "status_mismatch"
	SaaSPaymentSettlementReconciliationCurrencyMismatch   = "currency_mismatch"
	SaaSPaymentSettlementReconciliationAmountMismatch     = "amount_mismatch"

	SaaSPaymentSettlementHandlingAll      = "all"
	SaaSPaymentSettlementHandlingNone     = "none"
	SaaSPaymentSettlementHandlingOpen     = "open"
	SaaSPaymentSettlementHandlingResolved = "resolved"
	SaaSPaymentSettlementHandlingIgnored  = "ignored"

	SaaSPaymentSettlementTransitionClose  = "close"
	SaaSPaymentSettlementTransitionReopen = "reopen"

	SaaSAdminOperationActionSettlementImport     = "payment.settlement.import"
	SaaSAdminOperationActionSettlementReconcile  = "payment.settlement.reconcile"
	SaaSAdminOperationActionSettlementResolve    = "payment.settlement.resolve"
	SaaSAdminOperationActionSettlementTransition = "payment.settlement.transition"
	SaaSAdminOperationTargetSettlementBatch      = "payment_settlement_batch"
	SaaSAdminOperationTargetSettlementEntry      = "payment_settlement_entry"

	saasPaymentSettlementImportMaxEntries = 5000
	saasPaymentSettlementImportMaxBytes   = 8 << 20
)

type SaaSPaymentSettlementBatchOptions struct {
	BatchNo  string
	Provider string
	Status   string
	Currency string
	Keyword  string
	Limit    int
}

type SaaSPaymentSettlementBatch struct {
	ID                    int64
	BatchNo               string
	Provider              string
	ProviderSettlementNo  string
	PeriodStart           string
	PeriodEnd             string
	Currency              string
	Status                string
	SourceSHA256          string
	EntryCount            int
	PaymentCount          int
	RefundCount           int
	MatchedCount          int
	IssueCount            int
	OpenIssueCount        int
	ResolvedIssueCount    int
	IgnoredIssueCount     int
	TotalAmountCents      int64
	TotalFeeCents         int64
	TotalNetCents         int64
	DifferenceAmountCents int64
	ImportedByUserID      int
	ImportedByTenantID    int
	ImportedAt            string
	ReconciledAt          string
	ClosedByUserID        int
	ClosedAt              string
	CloseReason           string
	Version               int
	Remark                string
	CreatedAt             string
	UpdatedAt             string
}

type SaaSPaymentSettlementBatchSummary struct {
	BatchCount            int
	ReconciledCount       int
	ClosedCount           int
	EntryCount            int
	PaymentCount          int
	RefundCount           int
	MatchedCount          int
	IssueCount            int
	OpenIssueCount        int
	ResolvedIssueCount    int
	IgnoredIssueCount     int
	TotalAmountCents      int64
	TotalFeeCents         int64
	TotalNetCents         int64
	DifferenceAmountCents int64
	ProviderCount         int
	CurrencyCount         int
}

type SaaSPaymentSettlementBatchReport struct {
	Options SaaSPaymentSettlementBatchOptions
	Summary SaaSPaymentSettlementBatchSummary
	Batches []SaaSPaymentSettlementBatch
}

type SaaSPaymentSettlementEntryOptions struct {
	BatchNo              string
	Provider             string
	TransactionType      string
	ReconciliationStatus string
	HandlingStatus       string
	Keyword              string
	Limit                int
}

type SaaSPaymentSettlementEntry struct {
	ID                    int64
	BatchID               int64
	BatchNo               string
	BatchStatus           string
	LineNo                int
	Provider              string
	ProviderTransactionNo string
	TransactionType       string
	OrderNo               string
	ProviderOrderNo       string
	RefundNo              string
	ProviderRefundNo      string
	AmountCents           int64
	FeeCents              int64
	NetAmountCents        int64
	Currency              string
	OccurredAt            string
	MatchedPaymentOrderID int64
	MatchedRefundID       int64
	MatchedTenantID       int
	MatchedTenantName     string
	MatchedInternalNo     string
	ExpectedAmountCents   int64
	ExpectedCurrency      string
	ExpectedStatus        string
	ReconciliationStatus  string
	DifferenceAmountCents int64
	IssueCodesJSON        string
	IssueMessage          string
	HandlingStatus        string
	HandledByUserID       int
	HandledByTenantID     int
	HandledAt             string
	HandlingReason        string
	Version               int
	RawJSON               string
	CreatedAt             string
	UpdatedAt             string
}

type SaaSPaymentSettlementEntrySummary struct {
	EntryCount              int
	PaymentCount            int
	RefundCount             int
	MatchedCount            int
	MissingInternalCount    int
	IdentifierConflictCount int
	StatusMismatchCount     int
	CurrencyMismatchCount   int
	AmountMismatchCount     int
	OpenIssueCount          int
	ResolvedIssueCount      int
	IgnoredIssueCount       int
	TotalAmountCents        int64
	TotalFeeCents           int64
	TotalNetCents           int64
	ExpectedAmountCents     int64
	DifferenceAmountCents   int64
	TenantCount             int
}

type SaaSPaymentSettlementEntryReport struct {
	Options SaaSPaymentSettlementEntryOptions
	Summary SaaSPaymentSettlementEntrySummary
	Entries []SaaSPaymentSettlementEntry
}

type SaaSPaymentSettlementImportEntry struct {
	LineNo                int
	ProviderTransactionNo string
	TransactionType       string
	OrderNo               string
	ProviderOrderNo       string
	RefundNo              string
	ProviderRefundNo      string
	AmountCents           int64
	FeeCents              int64
	NetAmountCents        int64
	Currency              string
	OccurredAt            string
	RawJSON               string
}

type SaaSPaymentSettlementImport struct {
	BatchNo              string
	Provider             string
	ProviderSettlementNo string
	PeriodStart          string
	PeriodEnd            string
	Currency             string
	SourceSHA256         string
	Remark               string
	ActorUserID          int
	ActorTenantID        int
	Entries              []SaaSPaymentSettlementImportEntry
}

type SaaSPaymentSettlementImportResult struct {
	Batch       SaaSPaymentSettlementBatch
	Entries     SaaSPaymentSettlementEntryReport
	OperationID int64
	Idempotent  bool
}

type SaaSPaymentSettlementReconcile struct {
	BatchNo         string
	ExpectedVersion int
	DryRun          bool
	ActorUserID     int
	ActorTenantID   int
}

type SaaSPaymentSettlementReconcileResult struct {
	Batch       SaaSPaymentSettlementBatch
	Entries     SaaSPaymentSettlementEntryReport
	OperationID int64
	DryRun      bool
}

type SaaSPaymentSettlementEntryResolve struct {
	EntryID                  int64                                     `json:"entryId"`
	ExpectedVersion          int                                       `json:"expectedVersion"`
	HandlingStatus           string                                    `json:"handlingStatus"`
	Reason                   string                                    `json:"reason"`
	ApprovalPlan             *SaaSPaymentSettlementResolveApprovalPlan `json:"-"`
	ActorUserID              int                                       `json:"-"`
	ActorTenantID            int                                       `json:"-"`
	ApprovalExecutionID      int64                                     `json:"-"`
	ApprovalExecutionVersion int                                       `json:"-"`
}

type SaaSPaymentSettlementEntryResolveResult struct {
	Entry       SaaSPaymentSettlementEntry
	Batch       SaaSPaymentSettlementBatch
	OperationID int64
}

type SaaSPaymentSettlementTransition struct {
	BatchNo                  string                                       `json:"batchNo"`
	ExpectedVersion          int                                          `json:"expectedVersion"`
	Action                   string                                       `json:"action"`
	Reason                   string                                       `json:"reason"`
	ApprovalPlan             *SaaSPaymentSettlementTransitionApprovalPlan `json:"-"`
	ActorUserID              int                                          `json:"-"`
	ActorTenantID            int                                          `json:"-"`
	ApprovalExecutionID      int64                                        `json:"-"`
	ApprovalExecutionVersion int                                          `json:"-"`
}

const SaaSPaymentSettlementApprovalPlanSchemaVersion = 2

const SaaSPaymentSettlementResolveApprovalPlanSchemaVersion = 1

type SaaSPaymentSettlementBatchApprovalSnapshot struct {
	ID                    int64  `json:"id"`
	BatchNo               string `json:"batchNo"`
	Provider              string `json:"provider"`
	ProviderSettlementNo  string `json:"providerSettlementNo"`
	PeriodStart           string `json:"periodStart"`
	PeriodEnd             string `json:"periodEnd"`
	Currency              string `json:"currency"`
	Status                string `json:"status"`
	SourceSHA256          string `json:"sourceSha256"`
	EntryCount            int    `json:"entryCount"`
	PaymentCount          int    `json:"paymentCount"`
	RefundCount           int    `json:"refundCount"`
	MatchedCount          int    `json:"matchedCount"`
	IssueCount            int    `json:"issueCount"`
	OpenIssueCount        int    `json:"openIssueCount"`
	ResolvedIssueCount    int    `json:"resolvedIssueCount"`
	IgnoredIssueCount     int    `json:"ignoredIssueCount"`
	TotalAmountCents      int64  `json:"totalAmountCents"`
	TotalFeeCents         int64  `json:"totalFeeCents"`
	TotalNetCents         int64  `json:"totalNetCents"`
	DifferenceAmountCents int64  `json:"differenceAmountCents"`
	ImportedByUserID      int    `json:"importedByUserId"`
	ImportedByTenantID    int    `json:"importedByTenantId"`
	ImportedAt            string `json:"importedAt"`
	ReconciledAt          string `json:"reconciledAt"`
	ClosedByUserID        int    `json:"closedByUserId"`
	ClosedAt              string `json:"closedAt"`
	CloseReason           string `json:"closeReason"`
	Version               int    `json:"version"`
	Remark                string `json:"remark"`
	CreatedAt             string `json:"createdAt"`
	UpdatedAt             string `json:"updatedAt"`
}

type SaaSPaymentSettlementTransitionApprovalPlan struct {
	SchemaVersion int                                        `json:"schemaVersion"`
	Transition    SaaSPaymentSettlementTransition            `json:"transition"`
	Batch         SaaSPaymentSettlementBatchApprovalSnapshot `json:"batch"`
}

type SaaSPaymentSettlementEntryApprovalSnapshot struct {
	ID                    int64  `json:"id"`
	BatchID               int64  `json:"batchId"`
	BatchNo               string `json:"batchNo"`
	BatchStatus           string `json:"batchStatus"`
	LineNo                int    `json:"lineNo"`
	Provider              string `json:"provider"`
	ProviderTransactionNo string `json:"providerTransactionNo"`
	TransactionType       string `json:"transactionType"`
	OrderNo               string `json:"orderNo"`
	ProviderOrderNo       string `json:"providerOrderNo"`
	RefundNo              string `json:"refundNo"`
	ProviderRefundNo      string `json:"providerRefundNo"`
	AmountCents           int64  `json:"amountCents"`
	FeeCents              int64  `json:"feeCents"`
	NetAmountCents        int64  `json:"netAmountCents"`
	Currency              string `json:"currency"`
	OccurredAt            string `json:"occurredAt"`
	MatchedPaymentOrderID int64  `json:"matchedPaymentOrderId"`
	MatchedRefundID       int64  `json:"matchedRefundId"`
	MatchedTenantID       int    `json:"matchedTenantId"`
	MatchedTenantName     string `json:"matchedTenantName"`
	MatchedInternalNo     string `json:"matchedInternalNo"`
	ExpectedAmountCents   int64  `json:"expectedAmountCents"`
	ExpectedCurrency      string `json:"expectedCurrency"`
	ExpectedStatus        string `json:"expectedStatus"`
	ReconciliationStatus  string `json:"reconciliationStatus"`
	DifferenceAmountCents int64  `json:"differenceAmountCents"`
	IssueCodesJSON        string `json:"issueCodesJson"`
	IssueMessage          string `json:"issueMessage"`
	HandlingStatus        string `json:"handlingStatus"`
	HandledByUserID       int    `json:"handledByUserId"`
	HandledByTenantID     int    `json:"handledByTenantId"`
	HandledAt             string `json:"handledAt"`
	HandlingReason        string `json:"handlingReason"`
	Version               int    `json:"version"`
	RawJSON               string `json:"rawJson"`
	CreatedAt             string `json:"createdAt"`
	UpdatedAt             string `json:"updatedAt"`
}

type SaaSPaymentSettlementResolveApprovalPlan struct {
	SchemaVersion int                                        `json:"schemaVersion"`
	Resolve       SaaSPaymentSettlementEntryResolve          `json:"resolve"`
	Entry         SaaSPaymentSettlementEntryApprovalSnapshot `json:"entry"`
	Batch         SaaSPaymentSettlementBatchApprovalSnapshot `json:"batch"`
}

type SaaSPaymentSettlementCloseBatchSnapshot = SaaSPaymentSettlementBatchApprovalSnapshot
type SaaSPaymentSettlementCloseApprovalPlan = SaaSPaymentSettlementTransitionApprovalPlan
type SaaSPaymentSettlementReopenApprovalPlan = SaaSPaymentSettlementTransitionApprovalPlan

type SaaSPaymentSettlementTransitionResult struct {
	Batch          SaaSPaymentSettlementBatch
	PreviousStatus string
	OperationID    int64
}

type SaaSAdminPaymentSettlementStore interface {
	SaaSAdminPaymentSettlementBatches(context.Context, SaaSPaymentSettlementBatchOptions) (SaaSPaymentSettlementBatchReport, error)
	SaaSAdminPaymentSettlementEntries(context.Context, SaaSPaymentSettlementEntryOptions) (SaaSPaymentSettlementEntryReport, error)
	ImportSaaSAdminPaymentSettlement(context.Context, SaaSPaymentSettlementImport) (SaaSPaymentSettlementImportResult, error)
	ReconcileSaaSAdminPaymentSettlement(context.Context, SaaSPaymentSettlementReconcile) (SaaSPaymentSettlementReconcileResult, error)
	ResolveSaaSAdminPaymentSettlementEntry(context.Context, SaaSPaymentSettlementEntryResolve) (SaaSPaymentSettlementEntryResolveResult, error)
	TransitionSaaSAdminPaymentSettlement(context.Context, SaaSPaymentSettlementTransition) (SaaSPaymentSettlementTransitionResult, error)
}

type SaaSAdminPaymentSettlementResolveApprovalStore interface {
	SaaSAdminPaymentSettlementStore
	SaaSAdminPaymentSettlementResolveApprovalSnapshot(context.Context, int64) (SaaSPaymentSettlementEntry, SaaSPaymentSettlementBatch, error)
}

func (h *SaaSAdminHandler) PaymentSettlementBatches(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.paymentSettlementStore(w)
	if !ok {
		return
	}
	options, err := parseSaaSPaymentSettlementBatchOptions(r, saasAdminListMaxLimit)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	report, err := store.SaaSAdminPaymentSettlementBatches(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", saasPaymentSettlementBatchReportPayload(report))
}

func (h *SaaSAdminHandler) PaymentSettlementEntries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.paymentSettlementStore(w)
	if !ok {
		return
	}
	options, err := parseSaaSPaymentSettlementEntryOptions(r, saasAdminListMaxLimit)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	report, err := store.SaaSAdminPaymentSettlementEntries(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", saasPaymentSettlementEntryReportPayload(report))
}

func (h *SaaSAdminHandler) ImportPaymentSettlement(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.paymentSettlementStore(w)
	if !ok {
		return
	}
	input, err := parseSaaSPaymentSettlementImport(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if input.BatchNo == "" {
		input.BatchNo, err = newSaaSPaymentSettlementBatchNo()
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
	}
	input.ActorUserID = user.ID
	input.ActorTenantID = user.TenantID
	result, err := store.ImportSaaSAdminPaymentSettlement(r.Context(), input)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", saasPaymentSettlementImportResultPayload(result))
}

func (h *SaaSAdminHandler) ReconcilePaymentSettlement(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.paymentSettlementStore(w)
	if !ok {
		return
	}
	input, err := parseSaaSPaymentSettlementReconcile(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	input.ActorUserID = user.ID
	input.ActorTenantID = user.TenantID
	result, err := store.ReconcileSaaSAdminPaymentSettlement(r.Context(), input)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", saasPaymentSettlementReconcileResultPayload(result))
}

func (h *SaaSAdminHandler) ResolvePaymentSettlementEntry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.paymentSettlementStore(w)
	if !ok {
		return
	}
	input, err := parseSaaSPaymentSettlementEntryResolve(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionPaymentSettlementResolve, 0) {
		return
	}
	input.ActorUserID = user.ID
	input.ActorTenantID = user.TenantID
	result, err := store.ResolveSaaSAdminPaymentSettlementEntry(r.Context(), input)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{
		"entry": saasPaymentSettlementEntryPayload(result.Entry), "batch": saasPaymentSettlementBatchPayload(result.Batch), "operationId": result.OperationID,
	})
}

func (h *SaaSAdminHandler) planSaaSPaymentSettlementResolve(ctx context.Context, resolve SaaSPaymentSettlementEntryResolve) (SaaSPaymentSettlementResolveApprovalPlan, error) {
	store, ok := h.store.(SaaSAdminPaymentSettlementResolveApprovalStore)
	if !ok || store == nil {
		return SaaSPaymentSettlementResolveApprovalPlan{}, errors.New("payment settlement resolve approval store is not configured")
	}
	entry, batch, err := store.SaaSAdminPaymentSettlementResolveApprovalSnapshot(ctx, resolve.EntryID)
	if err != nil {
		return SaaSPaymentSettlementResolveApprovalPlan{}, err
	}
	if entry.ID <= 0 || entry.BatchID <= 0 || entry.BatchNo == "" || entry.Provider == "" ||
		entry.ProviderTransactionNo == "" || entry.Version <= 0 || batch.ID <= 0 || batch.Version <= 0 {
		return SaaSPaymentSettlementResolveApprovalPlan{}, &SaaSAdminOperationError{Status: http.StatusConflict, Message: "结算差异审批快照不完整，不能申请审批"}
	}
	if entry.BatchID != batch.ID || entry.BatchNo != batch.BatchNo || entry.BatchStatus != batch.Status {
		return SaaSPaymentSettlementResolveApprovalPlan{}, &SaaSAdminOperationError{Status: http.StatusConflict, Message: "结算差异与所属批次不一致，请先修复数据"}
	}
	if resolve.ExpectedVersion != entry.Version {
		return SaaSPaymentSettlementResolveApprovalPlan{}, &SaaSAdminOperationError{
			Status: http.StatusConflict, Message: fmt.Sprintf("结算差异版本已变化，请刷新后重试: current=%d", entry.Version),
		}
	}
	if batch.Status != SaaSPaymentSettlementBatchStatusReconciled {
		return SaaSPaymentSettlementResolveApprovalPlan{}, &SaaSAdminOperationError{Status: http.StatusConflict, Message: "结算批次不是已对账状态，不能处理差异"}
	}
	if entry.ReconciliationStatus == SaaSPaymentSettlementReconciliationMatched || entry.HandlingStatus == SaaSPaymentSettlementHandlingNone {
		return SaaSPaymentSettlementResolveApprovalPlan{}, &SaaSAdminOperationError{Status: http.StatusConflict, Message: "已匹配结算明细不需要处理差异"}
	}
	if !saasPaymentSettlementResolveTransitionValid(entry.HandlingStatus, resolve.HandlingStatus) {
		return SaaSPaymentSettlementResolveApprovalPlan{}, &SaaSAdminOperationError{Status: http.StatusConflict, Message: "结算差异处理状态变化无效，请刷新后重试"}
	}
	resolve.ExpectedVersion = entry.Version
	return SaaSPaymentSettlementResolveApprovalPlan{
		SchemaVersion: SaaSPaymentSettlementResolveApprovalPlanSchemaVersion,
		Resolve:       resolve,
		Entry:         saasPaymentSettlementEntryApprovalSnapshot(entry),
		Batch:         saasPaymentSettlementApprovalSnapshot(batch),
	}, nil
}

func saasPaymentSettlementResolveTransitionValid(current, target string) bool {
	if current == SaaSPaymentSettlementHandlingOpen {
		return target == SaaSPaymentSettlementHandlingResolved || target == SaaSPaymentSettlementHandlingIgnored
	}
	if current == SaaSPaymentSettlementHandlingResolved || current == SaaSPaymentSettlementHandlingIgnored {
		return target == SaaSPaymentSettlementHandlingOpen
	}
	return false
}

func (h *SaaSAdminHandler) TransitionPaymentSettlement(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.paymentSettlementStore(w)
	if !ok {
		return
	}
	input, err := parseSaaSPaymentSettlementTransition(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	approvalAction := SaaSAdminApprovalActionPaymentSettlementClose
	if input.Action == SaaSPaymentSettlementTransitionReopen {
		approvalAction = SaaSAdminApprovalActionPaymentSettlementReopen
	}
	if h.rejectDirectHighRiskAction(r.Context(), w, approvalAction, 0) {
		return
	}
	input.ActorUserID = user.ID
	input.ActorTenantID = user.TenantID
	result, err := store.TransitionSaaSAdminPaymentSettlement(r.Context(), input)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{
		"batch": saasPaymentSettlementBatchPayload(result.Batch), "previousStatus": result.PreviousStatus, "operationId": result.OperationID,
	})
}

func (h *SaaSAdminHandler) planSaaSPaymentSettlementClose(ctx context.Context, transition SaaSPaymentSettlementTransition) (SaaSPaymentSettlementCloseApprovalPlan, error) {
	if transition.Action != SaaSPaymentSettlementTransitionClose {
		return SaaSPaymentSettlementCloseApprovalPlan{}, NewSaaSAdminBadRequest("结算关账审批只支持 close 动作")
	}
	current, err := h.saaSPaymentSettlementBatchForApproval(ctx, transition)
	if err != nil {
		return SaaSPaymentSettlementCloseApprovalPlan{}, err
	}
	if current.Status != SaaSPaymentSettlementBatchStatusReconciled {
		return SaaSPaymentSettlementCloseApprovalPlan{}, &SaaSAdminOperationError{Status: http.StatusConflict, Message: "结算批次不是已对账状态，不能申请关账"}
	}
	if current.OpenIssueCount > 0 {
		return SaaSPaymentSettlementCloseApprovalPlan{}, &SaaSAdminOperationError{Status: http.StatusConflict, Message: "结算批次仍有未处理差异，不能申请关账"}
	}
	transition.ExpectedVersion = current.Version
	return SaaSPaymentSettlementCloseApprovalPlan{
		SchemaVersion: SaaSPaymentSettlementApprovalPlanSchemaVersion,
		Transition:    transition,
		Batch:         saasPaymentSettlementApprovalSnapshot(current),
	}, nil
}

func (h *SaaSAdminHandler) planSaaSPaymentSettlementReopen(ctx context.Context, transition SaaSPaymentSettlementTransition) (SaaSPaymentSettlementReopenApprovalPlan, error) {
	if transition.Action != SaaSPaymentSettlementTransitionReopen {
		return SaaSPaymentSettlementReopenApprovalPlan{}, NewSaaSAdminBadRequest("结算重开审批只支持 reopen 动作")
	}
	current, err := h.saaSPaymentSettlementBatchForApproval(ctx, transition)
	if err != nil {
		return SaaSPaymentSettlementReopenApprovalPlan{}, err
	}
	if current.Status != SaaSPaymentSettlementBatchStatusClosed {
		return SaaSPaymentSettlementReopenApprovalPlan{}, &SaaSAdminOperationError{Status: http.StatusConflict, Message: "结算批次不是已关闭状态，不能申请重开"}
	}
	if current.ClosedByUserID <= 0 || current.ClosedAt == "" || current.CloseReason == "" {
		return SaaSPaymentSettlementReopenApprovalPlan{}, &SaaSAdminOperationError{Status: http.StatusConflict, Message: "结算批次关账审计不完整，不能申请重开"}
	}
	transition.ExpectedVersion = current.Version
	return SaaSPaymentSettlementReopenApprovalPlan{
		SchemaVersion: SaaSPaymentSettlementApprovalPlanSchemaVersion,
		Transition:    transition,
		Batch:         saasPaymentSettlementApprovalSnapshot(current),
	}, nil
}

func (h *SaaSAdminHandler) saaSPaymentSettlementBatchForApproval(ctx context.Context, transition SaaSPaymentSettlementTransition) (SaaSPaymentSettlementBatch, error) {
	store, ok := h.store.(SaaSAdminPaymentSettlementStore)
	if !ok || store == nil {
		return SaaSPaymentSettlementBatch{}, errors.New("payment settlement store is not configured")
	}
	report, err := store.SaaSAdminPaymentSettlementBatches(ctx, SaaSPaymentSettlementBatchOptions{
		BatchNo: transition.BatchNo,
		Status:  SaaSPaymentSettlementBatchStatusAll,
		Limit:   2,
	})
	if err != nil {
		return SaaSPaymentSettlementBatch{}, err
	}
	if len(report.Batches) == 0 {
		return SaaSPaymentSettlementBatch{}, NewSaaSAdminNotFound("payment settlement batch not found")
	}
	if len(report.Batches) != 1 || report.Batches[0].BatchNo != transition.BatchNo {
		return SaaSPaymentSettlementBatch{}, &SaaSAdminOperationError{Status: http.StatusConflict, Message: "结算批次编号不唯一，请先修复数据"}
	}
	current := report.Batches[0]
	if transition.ExpectedVersion != current.Version {
		return SaaSPaymentSettlementBatch{}, &SaaSAdminOperationError{
			Status: http.StatusConflict, Message: fmt.Sprintf("结算批次版本已变化，请刷新后重试: current=%d", current.Version),
		}
	}
	if current.ID <= 0 || current.Version <= 0 ||
		current.BatchNo == "" || current.Provider == "" || current.ProviderSettlementNo == "" || current.Currency == "" {
		return SaaSPaymentSettlementBatch{}, &SaaSAdminOperationError{Status: http.StatusConflict, Message: "结算批次快照不完整，不能申请审批"}
	}
	return current, nil
}

func saasPaymentSettlementApprovalSnapshot(current SaaSPaymentSettlementBatch) SaaSPaymentSettlementBatchApprovalSnapshot {
	return SaaSPaymentSettlementBatchApprovalSnapshot{
		ID: current.ID, BatchNo: current.BatchNo, Provider: current.Provider, ProviderSettlementNo: current.ProviderSettlementNo,
		PeriodStart: current.PeriodStart, PeriodEnd: current.PeriodEnd, Currency: current.Currency, Status: current.Status,
		SourceSHA256: current.SourceSHA256, EntryCount: current.EntryCount, PaymentCount: current.PaymentCount,
		RefundCount: current.RefundCount, MatchedCount: current.MatchedCount, IssueCount: current.IssueCount,
		OpenIssueCount: current.OpenIssueCount, ResolvedIssueCount: current.ResolvedIssueCount,
		IgnoredIssueCount: current.IgnoredIssueCount, TotalAmountCents: current.TotalAmountCents,
		TotalFeeCents: current.TotalFeeCents, TotalNetCents: current.TotalNetCents,
		DifferenceAmountCents: current.DifferenceAmountCents, ImportedByUserID: current.ImportedByUserID,
		ImportedByTenantID: current.ImportedByTenantID, ImportedAt: current.ImportedAt, ReconciledAt: current.ReconciledAt,
		ClosedByUserID: current.ClosedByUserID, ClosedAt: current.ClosedAt, CloseReason: current.CloseReason,
		Version: current.Version, Remark: current.Remark, CreatedAt: current.CreatedAt, UpdatedAt: current.UpdatedAt,
	}
}

func saasPaymentSettlementEntryApprovalSnapshot(current SaaSPaymentSettlementEntry) SaaSPaymentSettlementEntryApprovalSnapshot {
	return SaaSPaymentSettlementEntryApprovalSnapshot{
		ID: current.ID, BatchID: current.BatchID, BatchNo: current.BatchNo, BatchStatus: current.BatchStatus,
		LineNo: current.LineNo, Provider: current.Provider, ProviderTransactionNo: current.ProviderTransactionNo,
		TransactionType: current.TransactionType, OrderNo: current.OrderNo, ProviderOrderNo: current.ProviderOrderNo,
		RefundNo: current.RefundNo, ProviderRefundNo: current.ProviderRefundNo, AmountCents: current.AmountCents,
		FeeCents: current.FeeCents, NetAmountCents: current.NetAmountCents, Currency: current.Currency,
		OccurredAt: current.OccurredAt, MatchedPaymentOrderID: current.MatchedPaymentOrderID,
		MatchedRefundID: current.MatchedRefundID, MatchedTenantID: current.MatchedTenantID,
		MatchedTenantName: current.MatchedTenantName, MatchedInternalNo: current.MatchedInternalNo,
		ExpectedAmountCents: current.ExpectedAmountCents, ExpectedCurrency: current.ExpectedCurrency,
		ExpectedStatus: current.ExpectedStatus, ReconciliationStatus: current.ReconciliationStatus,
		DifferenceAmountCents: current.DifferenceAmountCents, IssueCodesJSON: current.IssueCodesJSON,
		IssueMessage: current.IssueMessage, HandlingStatus: current.HandlingStatus,
		HandledByUserID: current.HandledByUserID, HandledByTenantID: current.HandledByTenantID,
		HandledAt: current.HandledAt, HandlingReason: current.HandlingReason, Version: current.Version,
		RawJSON: current.RawJSON, CreatedAt: current.CreatedAt, UpdatedAt: current.UpdatedAt,
	}
}

func (h *SaaSAdminHandler) paymentSettlementStore(w http.ResponseWriter) (SaaSAdminPaymentSettlementStore, bool) {
	store, ok := h.store.(SaaSAdminPaymentSettlementStore)
	if !ok || store == nil {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "payment settlement store is not configured", nil)
		return nil, false
	}
	return store, true
}

func parseSaaSPaymentSettlementBatchOptions(r *http.Request, maxLimit int) (SaaSPaymentSettlementBatchOptions, error) {
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status == "" {
		status = SaaSPaymentSettlementBatchStatusAll
	}
	if status != SaaSPaymentSettlementBatchStatusAll && status != SaaSPaymentSettlementBatchStatusReconciled && status != SaaSPaymentSettlementBatchStatusClosed {
		return SaaSPaymentSettlementBatchOptions{}, errors.New("status 必须是 all、reconciled 或 closed")
	}
	currency := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("currency")))
	if currency != "" {
		var err error
		currency, err = normalizeSaaSPaymentCurrency(currency)
		if err != nil {
			return SaaSPaymentSettlementBatchOptions{}, err
		}
	}
	limit := positiveQueryInt(r, "limit", 100)
	if limit > maxLimit {
		limit = maxLimit
	}
	return SaaSPaymentSettlementBatchOptions{
		BatchNo:  strings.TrimSpace(r.URL.Query().Get("batchNo")),
		Provider: strings.ToLower(strings.TrimSpace(r.URL.Query().Get("provider"))), Status: status,
		Currency: currency, Keyword: strings.TrimSpace(r.URL.Query().Get("keyword")), Limit: limit,
	}, nil
}

func parseSaaSPaymentSettlementEntryOptions(r *http.Request, maxLimit int) (SaaSPaymentSettlementEntryOptions, error) {
	transactionType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("transactionType")))
	if transactionType == "" {
		transactionType = SaaSPaymentSettlementTransactionAll
	}
	if !saasPaymentSettlementTransactionTypeValid(transactionType, true) {
		return SaaSPaymentSettlementEntryOptions{}, errors.New("transactionType 必须是 all、payment 或 refund")
	}
	reconciliationStatus := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("reconciliationStatus")))
	if reconciliationStatus == "" {
		reconciliationStatus = SaaSPaymentSettlementReconciliationAll
	}
	if !saasPaymentSettlementReconciliationStatusValid(reconciliationStatus, true) {
		return SaaSPaymentSettlementEntryOptions{}, errors.New("reconciliationStatus 格式错误")
	}
	handlingStatus := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("handlingStatus")))
	if handlingStatus == "" {
		handlingStatus = SaaSPaymentSettlementHandlingAll
	}
	if !saasPaymentSettlementHandlingStatusValid(handlingStatus, true) {
		return SaaSPaymentSettlementEntryOptions{}, errors.New("handlingStatus 必须是 all、none、open、resolved 或 ignored")
	}
	limit := positiveQueryInt(r, "limit", 100)
	if limit > maxLimit {
		limit = maxLimit
	}
	return SaaSPaymentSettlementEntryOptions{
		BatchNo: strings.TrimSpace(r.URL.Query().Get("batchNo")), Provider: strings.ToLower(strings.TrimSpace(r.URL.Query().Get("provider"))),
		TransactionType: transactionType, ReconciliationStatus: reconciliationStatus, HandlingStatus: handlingStatus,
		Keyword: strings.TrimSpace(r.URL.Query().Get("keyword")), Limit: limit,
	}, nil
}

type saasPaymentSettlementImportRequest struct {
	BatchNo              string                                    `json:"batchNo"`
	Provider             string                                    `json:"provider"`
	ProviderSettlementNo string                                    `json:"providerSettlementNo"`
	PeriodStart          string                                    `json:"periodStart"`
	PeriodEnd            string                                    `json:"periodEnd"`
	Currency             string                                    `json:"currency"`
	Remark               string                                    `json:"remark"`
	Entries              []saasPaymentSettlementImportEntryRequest `json:"entries"`
}

type saasPaymentSettlementImportEntryRequest struct {
	LineNo                int            `json:"lineNo"`
	ProviderTransactionNo string         `json:"providerTransactionNo"`
	TransactionType       string         `json:"transactionType"`
	OrderNo               string         `json:"orderNo"`
	ProviderOrderNo       string         `json:"providerOrderNo"`
	RefundNo              string         `json:"refundNo"`
	ProviderRefundNo      string         `json:"providerRefundNo"`
	AmountCents           int64          `json:"amountCents"`
	FeeCents              int64          `json:"feeCents"`
	NetAmountCents        *int64         `json:"netAmountCents"`
	Currency              string         `json:"currency"`
	OccurredAt            string         `json:"occurredAt"`
	Raw                   map[string]any `json:"raw"`
}

func parseSaaSPaymentSettlementImport(r *http.Request) (SaaSPaymentSettlementImport, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, saasPaymentSettlementImportMaxBytes+1))
	if err != nil {
		return SaaSPaymentSettlementImport{}, err
	}
	if len(body) == 0 {
		return SaaSPaymentSettlementImport{}, errors.New("结算内容不能为空")
	}
	if len(body) > saasPaymentSettlementImportMaxBytes {
		return SaaSPaymentSettlementImport{}, errors.New("结算内容超过 8MB")
	}
	digest := sha256.Sum256(body)
	var req saasPaymentSettlementImportRequest
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.Contains(contentType, "csv") || strings.Contains(contentType, "text/plain") {
		req, err = parseSaaSPaymentSettlementCSV(body)
	} else {
		if err = json.Unmarshal(body, &req); err != nil {
			return SaaSPaymentSettlementImport{}, errors.New("JSON 格式错误")
		}
	}
	if err != nil {
		return SaaSPaymentSettlementImport{}, err
	}
	return normalizeSaaSPaymentSettlementImport(req, hex.EncodeToString(digest[:]))
}

func normalizeSaaSPaymentSettlementImport(req saasPaymentSettlementImportRequest, digest string) (SaaSPaymentSettlementImport, error) {
	batchNo := strings.TrimSpace(req.BatchNo)
	if batchNo != "" && (!saasPaymentIdentifierPattern.MatchString(batchNo) || len(batchNo) > 64) {
		return SaaSPaymentSettlementImport{}, errors.New("batchNo 格式错误")
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" || !saasPaymentIdentifierPattern.MatchString(provider) || len(provider) > 32 {
		return SaaSPaymentSettlementImport{}, errors.New("provider 格式错误")
	}
	providerSettlementNo := strings.TrimSpace(req.ProviderSettlementNo)
	if providerSettlementNo == "" || len(providerSettlementNo) > 128 {
		return SaaSPaymentSettlementImport{}, errors.New("providerSettlementNo required")
	}
	currency, err := normalizeSaaSPaymentCurrency(req.Currency)
	if err != nil {
		return SaaSPaymentSettlementImport{}, err
	}
	periodStart, err := normalizeSaaSPaymentOptionalTimestamp(req.PeriodStart, "periodStart")
	if err != nil {
		return SaaSPaymentSettlementImport{}, err
	}
	periodEnd, err := normalizeSaaSPaymentOptionalTimestamp(req.PeriodEnd, "periodEnd")
	if err != nil {
		return SaaSPaymentSettlementImport{}, err
	}
	if periodStart != "" && periodEnd != "" && periodStart > periodEnd {
		return SaaSPaymentSettlementImport{}, errors.New("periodEnd 不能早于 periodStart")
	}
	if len(req.Entries) == 0 || len(req.Entries) > saasPaymentSettlementImportMaxEntries {
		return SaaSPaymentSettlementImport{}, fmt.Errorf("entries 数量必须在 1 到 %d 之间", saasPaymentSettlementImportMaxEntries)
	}
	lineNos := make(map[int]struct{}, len(req.Entries))
	transactionNos := make(map[string]struct{}, len(req.Entries))
	entries := make([]SaaSPaymentSettlementImportEntry, 0, len(req.Entries))
	for index, item := range req.Entries {
		lineNo := item.LineNo
		if lineNo == 0 {
			lineNo = index + 1
		}
		if lineNo <= 0 {
			return SaaSPaymentSettlementImport{}, fmt.Errorf("entries[%d].lineNo 格式错误", index)
		}
		if _, exists := lineNos[lineNo]; exists {
			return SaaSPaymentSettlementImport{}, fmt.Errorf("lineNo %d 重复", lineNo)
		}
		lineNos[lineNo] = struct{}{}
		providerTransactionNo := strings.TrimSpace(item.ProviderTransactionNo)
		if providerTransactionNo == "" || len(providerTransactionNo) > 128 {
			return SaaSPaymentSettlementImport{}, fmt.Errorf("entries[%d].providerTransactionNo required", index)
		}
		transactionType := strings.ToLower(strings.TrimSpace(item.TransactionType))
		if !saasPaymentSettlementTransactionTypeValid(transactionType, false) {
			return SaaSPaymentSettlementImport{}, fmt.Errorf("entries[%d].transactionType 必须是 payment 或 refund", index)
		}
		transactionKey := transactionType + "\x00" + providerTransactionNo
		if _, exists := transactionNos[transactionKey]; exists {
			return SaaSPaymentSettlementImport{}, fmt.Errorf("渠道流水 %s 重复", providerTransactionNo)
		}
		transactionNos[transactionKey] = struct{}{}
		if transactionType == SaaSPaymentSettlementTransactionPayment && item.AmountCents <= 0 {
			return SaaSPaymentSettlementImport{}, fmt.Errorf("entries[%d] 收款金额必须大于 0", index)
		}
		if transactionType == SaaSPaymentSettlementTransactionRefund && item.AmountCents >= 0 {
			return SaaSPaymentSettlementImport{}, fmt.Errorf("entries[%d] 退款金额必须小于 0", index)
		}
		orderNo, providerOrderNo := strings.TrimSpace(item.OrderNo), strings.TrimSpace(item.ProviderOrderNo)
		refundNo, providerRefundNo := strings.TrimSpace(item.RefundNo), strings.TrimSpace(item.ProviderRefundNo)
		if transactionType == SaaSPaymentSettlementTransactionPayment && orderNo == "" && providerOrderNo == "" {
			return SaaSPaymentSettlementImport{}, fmt.Errorf("entries[%d] 收款必须提供 orderNo 或 providerOrderNo", index)
		}
		if transactionType == SaaSPaymentSettlementTransactionRefund && refundNo == "" && providerRefundNo == "" {
			return SaaSPaymentSettlementImport{}, fmt.Errorf("entries[%d] 退款必须提供 refundNo 或 providerRefundNo", index)
		}
		entryCurrency := strings.ToUpper(strings.TrimSpace(item.Currency))
		if entryCurrency == "" {
			entryCurrency = currency
		}
		entryCurrency, err = normalizeSaaSPaymentCurrency(entryCurrency)
		if err != nil {
			return SaaSPaymentSettlementImport{}, fmt.Errorf("entries[%d].currency: %w", index, err)
		}
		occurredAt, err := normalizeSaaSPaymentOptionalTimestamp(item.OccurredAt, fmt.Sprintf("entries[%d].occurredAt", index))
		if err != nil {
			return SaaSPaymentSettlementImport{}, err
		}
		netAmountCents := item.AmountCents + item.FeeCents
		if item.NetAmountCents != nil && *item.NetAmountCents != netAmountCents {
			return SaaSPaymentSettlementImport{}, fmt.Errorf("entries[%d].netAmountCents 必须等于 amountCents + feeCents", index)
		}
		rawJSON := ""
		if len(item.Raw) > 0 {
			raw, marshalErr := json.Marshal(item.Raw)
			if marshalErr != nil {
				return SaaSPaymentSettlementImport{}, marshalErr
			}
			rawJSON = string(raw)
		}
		entries = append(entries, SaaSPaymentSettlementImportEntry{
			LineNo: lineNo, ProviderTransactionNo: providerTransactionNo, TransactionType: transactionType,
			OrderNo: orderNo, ProviderOrderNo: providerOrderNo, RefundNo: refundNo, ProviderRefundNo: providerRefundNo,
			AmountCents: item.AmountCents, FeeCents: item.FeeCents, NetAmountCents: netAmountCents,
			Currency: entryCurrency, OccurredAt: occurredAt, RawJSON: rawJSON,
		})
	}
	return SaaSPaymentSettlementImport{
		BatchNo: batchNo, Provider: provider, ProviderSettlementNo: providerSettlementNo,
		PeriodStart: periodStart, PeriodEnd: periodEnd, Currency: currency, SourceSHA256: digest,
		Remark: strings.TrimSpace(req.Remark), Entries: entries,
	}, nil
}

func parseSaaSPaymentSettlementCSV(body []byte) (saasPaymentSettlementImportRequest, error) {
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(body), "\ufeff")))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return saasPaymentSettlementImportRequest{}, fmt.Errorf("CSV 格式错误: %w", err)
	}
	if len(records) < 2 {
		return saasPaymentSettlementImportRequest{}, errors.New("CSV 至少需要表头和一条明细")
	}
	header := make(map[string]int, len(records[0]))
	for index, raw := range records[0] {
		name := normalizeSaaSPaymentSettlementCSVHeader(raw)
		if name != "" {
			header[name] = index
		}
	}
	for _, required := range []string{"provider", "providersettlementno", "providertransactionno", "transactiontype", "amountcents"} {
		if _, ok := header[required]; !ok {
			return saasPaymentSettlementImportRequest{}, fmt.Errorf("CSV 缺少列 %s", required)
		}
	}
	var req saasPaymentSettlementImportRequest
	for rowIndex, row := range records[1:] {
		if saasPaymentSettlementCSVRowEmpty(row) {
			continue
		}
		rowNo := rowIndex + 2
		if len(req.Entries) == 0 {
			req.BatchNo = saasPaymentSettlementCSVValue(row, header, "batchno")
			req.Provider = saasPaymentSettlementCSVValue(row, header, "provider")
			req.ProviderSettlementNo = saasPaymentSettlementCSVValue(row, header, "providersettlementno")
			req.PeriodStart = saasPaymentSettlementCSVValue(row, header, "periodstart")
			req.PeriodEnd = saasPaymentSettlementCSVValue(row, header, "periodend")
			req.Currency = saasPaymentSettlementCSVValue(row, header, "currency")
			req.Remark = saasPaymentSettlementCSVValue(row, header, "remark")
		} else {
			if value := saasPaymentSettlementCSVValue(row, header, "provider"); value != "" && !strings.EqualFold(value, req.Provider) {
				return saasPaymentSettlementImportRequest{}, fmt.Errorf("CSV 第 %d 行 provider 与批次不一致", rowNo)
			}
			if value := saasPaymentSettlementCSVValue(row, header, "providersettlementno"); value != "" && value != req.ProviderSettlementNo {
				return saasPaymentSettlementImportRequest{}, fmt.Errorf("CSV 第 %d 行 providerSettlementNo 与批次不一致", rowNo)
			}
		}
		lineNo, err := saasPaymentSettlementCSVInt(row, header, "lineno", len(req.Entries)+1)
		if err != nil {
			return req, fmt.Errorf("CSV 第 %d 行 lineNo: %w", rowNo, err)
		}
		amountCents, err := saasPaymentSettlementCSVInt64(row, header, "amountcents", true)
		if err != nil {
			return req, fmt.Errorf("CSV 第 %d 行 amountCents: %w", rowNo, err)
		}
		feeCents, err := saasPaymentSettlementCSVInt64(row, header, "feecents", false)
		if err != nil {
			return req, fmt.Errorf("CSV 第 %d 行 feeCents: %w", rowNo, err)
		}
		var netAmountCents *int64
		if raw := saasPaymentSettlementCSVValue(row, header, "netamountcents"); raw != "" {
			value, parseErr := strconv.ParseInt(raw, 10, 64)
			if parseErr != nil {
				return req, fmt.Errorf("CSV 第 %d 行 netAmountCents 格式错误", rowNo)
			}
			netAmountCents = &value
		}
		rawMap := make(map[string]any, len(records[0]))
		for index, name := range records[0] {
			if index < len(row) && strings.TrimSpace(row[index]) != "" {
				rawMap[strings.TrimSpace(name)] = strings.TrimSpace(row[index])
			}
		}
		req.Entries = append(req.Entries, saasPaymentSettlementImportEntryRequest{
			LineNo: lineNo, ProviderTransactionNo: saasPaymentSettlementCSVValue(row, header, "providertransactionno"),
			TransactionType: saasPaymentSettlementCSVValue(row, header, "transactiontype"),
			OrderNo:         saasPaymentSettlementCSVValue(row, header, "orderno"), ProviderOrderNo: saasPaymentSettlementCSVValue(row, header, "providerorderno"),
			RefundNo: saasPaymentSettlementCSVValue(row, header, "refundno"), ProviderRefundNo: saasPaymentSettlementCSVValue(row, header, "providerrefundno"),
			AmountCents: amountCents, FeeCents: feeCents, NetAmountCents: netAmountCents,
			Currency: saasPaymentSettlementCSVValue(row, header, "currency"), OccurredAt: saasPaymentSettlementCSVValue(row, header, "occurredat"), Raw: rawMap,
		})
	}
	if len(req.Entries) == 0 {
		return req, errors.New("CSV 没有有效明细")
	}
	return req, nil
}

func normalizeSaaSPaymentSettlementCSVHeader(raw string) string {
	value := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff")))
	value = strings.NewReplacer("_", "", "-", "", " ", "", ".", "").Replace(value)
	return value
}

func saasPaymentSettlementCSVValue(row []string, header map[string]int, name string) string {
	index, ok := header[name]
	if !ok || index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func saasPaymentSettlementCSVInt(row []string, header map[string]int, name string, fallback int) (int, error) {
	raw := saasPaymentSettlementCSVValue(row, header, name)
	if raw == "" {
		return fallback, nil
	}
	return strconv.Atoi(raw)
}

func saasPaymentSettlementCSVInt64(row []string, header map[string]int, name string, required bool) (int64, error) {
	raw := saasPaymentSettlementCSVValue(row, header, name)
	if raw == "" {
		if required {
			return 0, errors.New("required")
		}
		return 0, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}

func saasPaymentSettlementCSVRowEmpty(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

type saasPaymentSettlementReconcileRequest struct {
	BatchNo         string `json:"batchNo"`
	ExpectedVersion int    `json:"expectedVersion"`
	DryRun          *bool  `json:"dryRun"`
}

func parseSaaSPaymentSettlementReconcile(r *http.Request) (SaaSPaymentSettlementReconcile, error) {
	var req saasPaymentSettlementReconcileRequest
	if err := decodeSaaSPaymentSettlementJSON(r, &req); err != nil {
		return SaaSPaymentSettlementReconcile{}, err
	}
	if strings.TrimSpace(req.BatchNo) == "" || req.ExpectedVersion <= 0 {
		return SaaSPaymentSettlementReconcile{}, errors.New("batchNo and expectedVersion are required")
	}
	dryRun := true
	if req.DryRun != nil {
		dryRun = *req.DryRun
	}
	return SaaSPaymentSettlementReconcile{BatchNo: strings.TrimSpace(req.BatchNo), ExpectedVersion: req.ExpectedVersion, DryRun: dryRun}, nil
}

type saasPaymentSettlementResolveRequest struct {
	EntryID         int64  `json:"entryId"`
	ExpectedVersion int    `json:"expectedVersion"`
	HandlingStatus  string `json:"handlingStatus"`
	Reason          string `json:"reason"`
}

func parseSaaSPaymentSettlementEntryResolve(r *http.Request) (SaaSPaymentSettlementEntryResolve, error) {
	var req saasPaymentSettlementResolveRequest
	if err := decodeSaaSPaymentSettlementJSON(r, &req); err != nil {
		return SaaSPaymentSettlementEntryResolve{}, err
	}
	status := strings.ToLower(strings.TrimSpace(req.HandlingStatus))
	if req.EntryID <= 0 || req.ExpectedVersion <= 0 || (status != SaaSPaymentSettlementHandlingResolved && status != SaaSPaymentSettlementHandlingIgnored && status != SaaSPaymentSettlementHandlingOpen) {
		return SaaSPaymentSettlementEntryResolve{}, errors.New("entryId、expectedVersion 和 handlingStatus=open/resolved/ignored 必填")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return SaaSPaymentSettlementEntryResolve{}, errors.New("reason required")
	}
	return SaaSPaymentSettlementEntryResolve{EntryID: req.EntryID, ExpectedVersion: req.ExpectedVersion, HandlingStatus: status, Reason: strings.TrimSpace(req.Reason)}, nil
}

type saasPaymentSettlementTransitionRequest struct {
	BatchNo         string `json:"batchNo"`
	ExpectedVersion int    `json:"expectedVersion"`
	Action          string `json:"action"`
	Reason          string `json:"reason"`
}

func parseSaaSPaymentSettlementTransition(r *http.Request) (SaaSPaymentSettlementTransition, error) {
	var req saasPaymentSettlementTransitionRequest
	if err := decodeSaaSPaymentSettlementJSON(r, &req); err != nil {
		return SaaSPaymentSettlementTransition{}, err
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if strings.TrimSpace(req.BatchNo) == "" || req.ExpectedVersion <= 0 || (action != SaaSPaymentSettlementTransitionClose && action != SaaSPaymentSettlementTransitionReopen) {
		return SaaSPaymentSettlementTransition{}, errors.New("batchNo、expectedVersion 和 action=close/reopen 必填")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return SaaSPaymentSettlementTransition{}, errors.New("reason required")
	}
	return SaaSPaymentSettlementTransition{BatchNo: strings.TrimSpace(req.BatchNo), ExpectedVersion: req.ExpectedVersion, Action: action, Reason: strings.TrimSpace(req.Reason)}, nil
}

func decodeSaaSPaymentSettlementJSON(r *http.Request, target any) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return err
	}
	if len(body) == 0 || json.Unmarshal(body, target) != nil {
		return errors.New("JSON 格式错误")
	}
	return nil
}

func saasPaymentSettlementTransactionTypeValid(value string, allowAll bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case SaaSPaymentSettlementTransactionPayment, SaaSPaymentSettlementTransactionRefund:
		return true
	case SaaSPaymentSettlementTransactionAll:
		return allowAll
	default:
		return false
	}
}

func saasPaymentSettlementReconciliationStatusValid(value string, allowAll bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case SaaSPaymentSettlementReconciliationMatched, SaaSPaymentSettlementReconciliationMissingInternal,
		SaaSPaymentSettlementReconciliationIdentifierConflict, SaaSPaymentSettlementReconciliationStatusMismatch,
		SaaSPaymentSettlementReconciliationCurrencyMismatch, SaaSPaymentSettlementReconciliationAmountMismatch:
		return true
	case SaaSPaymentSettlementReconciliationAll:
		return allowAll
	default:
		return false
	}
}

func saasPaymentSettlementHandlingStatusValid(value string, allowAll bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case SaaSPaymentSettlementHandlingNone, SaaSPaymentSettlementHandlingOpen, SaaSPaymentSettlementHandlingResolved, SaaSPaymentSettlementHandlingIgnored:
		return true
	case SaaSPaymentSettlementHandlingAll:
		return allowAll
	default:
		return false
	}
}

func newSaaSPaymentSettlementBatchNo() (string, error) {
	buffer := make([]byte, 5)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return "SET-" + time.Now().UTC().Format("20060102-150405") + "-" + strings.ToUpper(hex.EncodeToString(buffer)), nil
}

func saasPaymentSettlementBatchReportPayload(report SaaSPaymentSettlementBatchReport) map[string]any {
	batches := make([]map[string]any, 0, len(report.Batches))
	for _, item := range report.Batches {
		batches = append(batches, saasPaymentSettlementBatchPayload(item))
	}
	return map[string]any{
		"filters": map[string]any{"batchNo": report.Options.BatchNo, "provider": report.Options.Provider, "status": report.Options.Status, "currency": report.Options.Currency, "keyword": report.Options.Keyword, "limit": report.Options.Limit},
		"summary": saasPaymentSettlementBatchSummaryPayload(report.Summary), "returnedCount": len(batches), "batches": batches,
	}
}

func saasPaymentSettlementBatchPayload(item SaaSPaymentSettlementBatch) map[string]any {
	return map[string]any{
		"id": item.ID, "batchNo": item.BatchNo, "provider": item.Provider, "providerSettlementNo": item.ProviderSettlementNo,
		"periodStart": item.PeriodStart, "periodEnd": item.PeriodEnd, "currency": item.Currency, "status": item.Status,
		"sourceSha256": item.SourceSHA256, "entryCount": item.EntryCount, "paymentCount": item.PaymentCount, "refundCount": item.RefundCount,
		"matchedCount": item.MatchedCount, "issueCount": item.IssueCount, "openIssueCount": item.OpenIssueCount,
		"resolvedIssueCount": item.ResolvedIssueCount, "ignoredIssueCount": item.IgnoredIssueCount,
		"totalAmountCents": item.TotalAmountCents, "totalFeeCents": item.TotalFeeCents, "totalNetCents": item.TotalNetCents,
		"differenceAmountCents": item.DifferenceAmountCents, "importedByUserId": item.ImportedByUserID,
		"importedByTenantId": item.ImportedByTenantID, "importedAt": item.ImportedAt, "reconciledAt": item.ReconciledAt,
		"closedByUserId": item.ClosedByUserID, "closedAt": item.ClosedAt, "closeReason": item.CloseReason,
		"version": item.Version, "remark": item.Remark, "createdAt": item.CreatedAt, "updatedAt": item.UpdatedAt,
	}
}

func saasPaymentSettlementBatchSummaryPayload(summary SaaSPaymentSettlementBatchSummary) map[string]any {
	return map[string]any{
		"batchCount": summary.BatchCount, "reconciledCount": summary.ReconciledCount, "closedCount": summary.ClosedCount,
		"entryCount": summary.EntryCount, "paymentCount": summary.PaymentCount, "refundCount": summary.RefundCount,
		"matchedCount": summary.MatchedCount, "issueCount": summary.IssueCount, "openIssueCount": summary.OpenIssueCount,
		"resolvedIssueCount": summary.ResolvedIssueCount, "ignoredIssueCount": summary.IgnoredIssueCount,
		"totalAmountCents": summary.TotalAmountCents, "totalFeeCents": summary.TotalFeeCents, "totalNetCents": summary.TotalNetCents,
		"differenceAmountCents": summary.DifferenceAmountCents, "providerCount": summary.ProviderCount, "currencyCount": summary.CurrencyCount,
	}
}

func saasPaymentSettlementEntryReportPayload(report SaaSPaymentSettlementEntryReport) map[string]any {
	entries := make([]map[string]any, 0, len(report.Entries))
	for _, item := range report.Entries {
		entries = append(entries, saasPaymentSettlementEntryPayload(item))
	}
	return map[string]any{
		"filters": map[string]any{
			"batchNo": report.Options.BatchNo, "provider": report.Options.Provider, "transactionType": report.Options.TransactionType,
			"reconciliationStatus": report.Options.ReconciliationStatus, "handlingStatus": report.Options.HandlingStatus,
			"keyword": report.Options.Keyword, "limit": report.Options.Limit,
		},
		"summary": saasPaymentSettlementEntrySummaryPayload(report.Summary), "returnedCount": len(entries), "entries": entries,
	}
}

func saasPaymentSettlementEntryPayload(item SaaSPaymentSettlementEntry) map[string]any {
	return map[string]any{
		"id": item.ID, "batchId": item.BatchID, "batchNo": item.BatchNo, "batchStatus": item.BatchStatus, "lineNo": item.LineNo,
		"provider": item.Provider, "providerTransactionNo": item.ProviderTransactionNo, "transactionType": item.TransactionType,
		"orderNo": item.OrderNo, "providerOrderNo": item.ProviderOrderNo, "refundNo": item.RefundNo, "providerRefundNo": item.ProviderRefundNo,
		"amountCents": item.AmountCents, "feeCents": item.FeeCents, "netAmountCents": item.NetAmountCents,
		"currency": item.Currency, "occurredAt": item.OccurredAt,
		"matchedPaymentOrderId": item.MatchedPaymentOrderID, "matchedRefundId": item.MatchedRefundID,
		"matchedTenantId": item.MatchedTenantID, "matchedTenantName": item.MatchedTenantName, "matchedInternalNo": item.MatchedInternalNo,
		"expectedAmountCents": item.ExpectedAmountCents, "expectedCurrency": item.ExpectedCurrency, "expectedStatus": item.ExpectedStatus,
		"reconciliationStatus": item.ReconciliationStatus, "differenceAmountCents": item.DifferenceAmountCents,
		"issueCodesJson": item.IssueCodesJSON, "issueMessage": item.IssueMessage, "handlingStatus": item.HandlingStatus,
		"handledByUserId": item.HandledByUserID, "handledByTenantId": item.HandledByTenantID, "handledAt": item.HandledAt,
		"handlingReason": item.HandlingReason, "version": item.Version, "rawJson": item.RawJSON,
		"createdAt": item.CreatedAt, "updatedAt": item.UpdatedAt,
	}
}

func saasPaymentSettlementEntrySummaryPayload(summary SaaSPaymentSettlementEntrySummary) map[string]any {
	return map[string]any{
		"entryCount": summary.EntryCount, "paymentCount": summary.PaymentCount, "refundCount": summary.RefundCount,
		"matchedCount": summary.MatchedCount, "missingInternalCount": summary.MissingInternalCount,
		"identifierConflictCount": summary.IdentifierConflictCount, "statusMismatchCount": summary.StatusMismatchCount,
		"currencyMismatchCount": summary.CurrencyMismatchCount, "amountMismatchCount": summary.AmountMismatchCount,
		"openIssueCount": summary.OpenIssueCount, "resolvedIssueCount": summary.ResolvedIssueCount, "ignoredIssueCount": summary.IgnoredIssueCount,
		"totalAmountCents": summary.TotalAmountCents, "totalFeeCents": summary.TotalFeeCents, "totalNetCents": summary.TotalNetCents,
		"expectedAmountCents": summary.ExpectedAmountCents, "differenceAmountCents": summary.DifferenceAmountCents, "tenantCount": summary.TenantCount,
	}
}

func saasPaymentSettlementImportResultPayload(result SaaSPaymentSettlementImportResult) map[string]any {
	return map[string]any{
		"batch": saasPaymentSettlementBatchPayload(result.Batch), "entries": saasPaymentSettlementEntryReportPayload(result.Entries),
		"operationId": result.OperationID, "idempotent": result.Idempotent,
	}
}

func saasPaymentSettlementReconcileResultPayload(result SaaSPaymentSettlementReconcileResult) map[string]any {
	return map[string]any{
		"batch": saasPaymentSettlementBatchPayload(result.Batch), "entries": saasPaymentSettlementEntryReportPayload(result.Entries),
		"operationId": result.OperationID, "dryRun": result.DryRun,
	}
}

func writeSaaSPaymentSettlementBatchCSV(writer *csv.Writer, report SaaSPaymentSettlementBatchReport) {
	_ = writer.Write([]string{"batchNo", "provider", "providerSettlementNo", "status", "periodStart", "periodEnd", "currency", "entryCount", "paymentCount", "refundCount", "matchedCount", "issueCount", "openIssueCount", "resolvedIssueCount", "ignoredIssueCount", "totalAmountCents", "totalFeeCents", "totalNetCents", "differenceAmountCents", "version", "importedAt", "reconciledAt", "closedAt", "closeReason", "remark"})
	for _, item := range report.Batches {
		_ = writer.Write([]string{
			item.BatchNo, item.Provider, item.ProviderSettlementNo, item.Status, item.PeriodStart, item.PeriodEnd, item.Currency,
			strconv.Itoa(item.EntryCount), strconv.Itoa(item.PaymentCount), strconv.Itoa(item.RefundCount), strconv.Itoa(item.MatchedCount),
			strconv.Itoa(item.IssueCount), strconv.Itoa(item.OpenIssueCount), strconv.Itoa(item.ResolvedIssueCount), strconv.Itoa(item.IgnoredIssueCount),
			strconv.FormatInt(item.TotalAmountCents, 10), strconv.FormatInt(item.TotalFeeCents, 10), strconv.FormatInt(item.TotalNetCents, 10),
			strconv.FormatInt(item.DifferenceAmountCents, 10), strconv.Itoa(item.Version), item.ImportedAt, item.ReconciledAt, item.ClosedAt, item.CloseReason, item.Remark,
		})
	}
}

func writeSaaSPaymentSettlementEntryCSV(writer *csv.Writer, report SaaSPaymentSettlementEntryReport) {
	_ = writer.Write([]string{"batchNo", "batchStatus", "lineNo", "provider", "providerTransactionNo", "transactionType", "orderNo", "providerOrderNo", "refundNo", "providerRefundNo", "amountCents", "feeCents", "netAmountCents", "currency", "occurredAt", "matchedTenantId", "matchedTenantName", "matchedInternalNo", "expectedAmountCents", "expectedCurrency", "expectedStatus", "reconciliationStatus", "differenceAmountCents", "handlingStatus", "handlingReason", "version", "issueMessage"})
	for _, item := range report.Entries {
		_ = writer.Write([]string{
			item.BatchNo, item.BatchStatus, strconv.Itoa(item.LineNo), item.Provider, item.ProviderTransactionNo, item.TransactionType,
			item.OrderNo, item.ProviderOrderNo, item.RefundNo, item.ProviderRefundNo,
			strconv.FormatInt(item.AmountCents, 10), strconv.FormatInt(item.FeeCents, 10), strconv.FormatInt(item.NetAmountCents, 10),
			item.Currency, item.OccurredAt, strconv.Itoa(item.MatchedTenantID), item.MatchedTenantName, item.MatchedInternalNo,
			strconv.FormatInt(item.ExpectedAmountCents, 10), item.ExpectedCurrency, item.ExpectedStatus, item.ReconciliationStatus,
			strconv.FormatInt(item.DifferenceAmountCents, 10), item.HandlingStatus, item.HandlingReason, strconv.Itoa(item.Version), item.IssueMessage,
		})
	}
}
