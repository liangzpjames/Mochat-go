package dashboard

import (
	"context"
	"crypto/rand"
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
	SaaSAdminExportKindPaymentRefunds = "paymentRefunds"

	SaaSPaymentRefundStatusAll        = "all"
	SaaSPaymentRefundStatusRequested  = "requested"
	SaaSPaymentRefundStatusProcessing = "processing"
	SaaSPaymentRefundStatusSucceeded  = "succeeded"
	SaaSPaymentRefundStatusFailed     = "failed"
	SaaSPaymentRefundStatusCanceled   = "canceled"

	SaaSPaymentRefundEntitlementKeep    = "keep"
	SaaSPaymentRefundEntitlementSuspend = "suspend"
	SaaSPaymentRefundEntitlementCancel  = "cancel"

	SaaSPaymentWebhookTypeRefundProcessing = "refund.processing"
	SaaSPaymentWebhookTypeRefundSucceeded  = "refund.succeeded"
	SaaSPaymentWebhookTypeRefundFailed     = "refund.failed"
	SaaSPaymentWebhookTypeRefundCanceled   = "refund.canceled"

	SaaSAdminOperationActionPaymentRefundRequest   = "payment.refund.request"
	SaaSAdminOperationActionPaymentRefundProcess   = "payment.refund.processing"
	SaaSAdminOperationActionPaymentRefundSucceeded = "payment.refund.succeeded"
	SaaSAdminOperationActionPaymentRefundFailed    = "payment.refund.failed"
	SaaSAdminOperationActionPaymentRefundCancel    = "payment.refund.cancel"
	SaaSAdminOperationTargetPaymentRefund          = "payment_refund"
)

type SaaSAdminPaymentRefundOptions struct {
	TenantID int
	Status   string
	Provider string
	OrderNo  string
	Keyword  string
	Limit    int
}

type SaaSAdminPaymentRefund struct {
	ID                    int64
	RefundNo              string
	PaymentOrderID        int64
	OrderNo               string
	TenantID              int
	TenantName            string
	Provider              string
	ProviderRefundNo      string
	IdempotencyKey        string
	Status                string
	AmountCents           int64
	Currency              string
	Reason                string
	EntitlementAction     string
	LatestWebhookEventID  int64
	BillingEventID        int64
	RequestedByUserID     int
	RequestedByTenantID   int
	RequestedAt           string
	SucceededAt           string
	FailedAt              string
	CanceledAt            string
	FailureCode           string
	FailureMessage        string
	Version               int
	Remark                string
	MetadataJSON          string
	CreatedAt             string
	UpdatedAt             string
	OrderAmountCents      int64
	OrderRefundedCents    int64
	OrderRefundPending    int64
	OrderRefundableCents  int64
	OrderRefundStatus     string
	OrderPaymentStatus    string
	SubscriptionEventID   int64
	SubscriptionOperation int64
}

type SaaSAdminPaymentRefundSummary struct {
	RefundCount          int
	RequestedCount       int
	ProcessingCount      int
	SucceededCount       int
	FailedCount          int
	CanceledCount        int
	RefundAmountCents    int64
	PendingAmountCents   int64
	SucceededAmountCents int64
	FailedAmountCents    int64
	TenantCount          int
	OrderCount           int
	ProviderCount        int
}

type SaaSAdminPaymentRefundReport struct {
	Options SaaSAdminPaymentRefundOptions
	Summary SaaSAdminPaymentRefundSummary
	Refunds []SaaSAdminPaymentRefund
}

type SaaSAdminPaymentRefundCreate struct {
	RefundNo                 string `json:"refundNo"`
	OrderNo                  string `json:"orderNo"`
	ProviderRefundNo         string `json:"providerRefundNo"`
	IdempotencyKey           string `json:"idempotencyKey"`
	AmountCents              int64  `json:"amountCents"`
	Currency                 string `json:"currency"`
	Reason                   string `json:"reason"`
	EntitlementAction        string `json:"entitlementAction"`
	Remark                   string `json:"remark"`
	MetadataJSON             string `json:"metadataJson"`
	ActorUserID              int    `json:"-"`
	ActorTenantID            int    `json:"-"`
	ApprovalExecutionID      int64  `json:"-"`
	ApprovalExecutionVersion int    `json:"-"`
}

type SaaSAdminPaymentRefundCreateResult struct {
	Refund      SaaSAdminPaymentRefund
	Order       SaaSAdminPaymentOrder
	OperationID int64
	Idempotent  bool
}

type SaaSAdminPaymentRefundCancel struct {
	RefundNo        string
	ExpectedVersion int
	Reason          string
	ActorUserID     int
	ActorTenantID   int
}

type SaaSAdminPaymentRefundCancelResult struct {
	Refund         SaaSAdminPaymentRefund
	Order          SaaSAdminPaymentOrder
	PreviousStatus string
	OperationID    int64
}

type SaaSAdminPaymentRefundStore interface {
	SaaSAdminPaymentRefunds(ctx context.Context, options SaaSAdminPaymentRefundOptions) (SaaSAdminPaymentRefundReport, error)
	CreateSaaSAdminPaymentRefund(ctx context.Context, create SaaSAdminPaymentRefundCreate) (SaaSAdminPaymentRefundCreateResult, error)
	CancelSaaSAdminPaymentRefund(ctx context.Context, cancel SaaSAdminPaymentRefundCancel) (SaaSAdminPaymentRefundCancelResult, error)
}

func SaaSPaymentRefundStatusValid(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case SaaSPaymentRefundStatusRequested, SaaSPaymentRefundStatusProcessing,
		SaaSPaymentRefundStatusSucceeded, SaaSPaymentRefundStatusFailed, SaaSPaymentRefundStatusCanceled:
		return true
	default:
		return false
	}
}

func SaaSPaymentRefundEntitlementActionValid(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case SaaSPaymentRefundEntitlementKeep, SaaSPaymentRefundEntitlementSuspend, SaaSPaymentRefundEntitlementCancel:
		return true
	default:
		return false
	}
}

func SaaSPaymentWebhookRefundType(eventType string) bool {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case SaaSPaymentWebhookTypeRefundProcessing, SaaSPaymentWebhookTypeRefundSucceeded,
		SaaSPaymentWebhookTypeRefundFailed, SaaSPaymentWebhookTypeRefundCanceled:
		return true
	default:
		return false
	}
}

func (h *SaaSAdminHandler) PaymentRefunds(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.paymentRefundStore(w)
	if !ok {
		return
	}
	options, err := parseSaaSAdminPaymentRefundOptions(r, saasAdminListMaxLimit)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	report, err := store.SaaSAdminPaymentRefunds(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", saasAdminPaymentRefundReportPayload(report))
}

func (h *SaaSAdminHandler) CreatePaymentRefund(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.paymentRefundStore(w)
	if !ok {
		return
	}
	create, err := parseSaaSAdminPaymentRefundCreate(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if create.RefundNo == "" {
		create.RefundNo, err = newSaaSPaymentRefundNo()
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
	}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionPaymentRefundCreate, create.AmountCents) {
		return
	}
	create.ActorUserID = user.ID
	create.ActorTenantID = user.TenantID
	result, err := store.CreateSaaSAdminPaymentRefund(r.Context(), create)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", saasAdminPaymentRefundCreateResultPayload(result))
}

func (h *SaaSAdminHandler) CancelPaymentRefund(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.paymentRefundStore(w)
	if !ok {
		return
	}
	cancel, err := parseSaaSAdminPaymentRefundCancel(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	cancel.ActorUserID = user.ID
	cancel.ActorTenantID = user.TenantID
	result, err := store.CancelSaaSAdminPaymentRefund(r.Context(), cancel)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", saasAdminPaymentRefundCancelResultPayload(result))
}

func (h *SaaSAdminHandler) paymentRefundStore(w http.ResponseWriter) (SaaSAdminPaymentRefundStore, bool) {
	store, ok := h.store.(SaaSAdminPaymentRefundStore)
	if !ok || store == nil {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "payment refund store is not configured", nil)
		return nil, false
	}
	return store, true
}

type saasAdminPaymentRefundCreateRequest struct {
	RefundNo          string         `json:"refundNo"`
	OrderNo           string         `json:"orderNo"`
	ProviderRefundNo  string         `json:"providerRefundNo"`
	IdempotencyKey    string         `json:"idempotencyKey"`
	AmountCents       int64          `json:"amountCents"`
	Amount            string         `json:"amount"`
	Currency          string         `json:"currency"`
	Reason            string         `json:"reason"`
	EntitlementAction string         `json:"entitlementAction"`
	Remark            string         `json:"remark"`
	Metadata          map[string]any `json:"metadata"`
}

func parseSaaSAdminPaymentRefundCreate(r *http.Request) (SaaSAdminPaymentRefundCreate, error) {
	req := saasAdminPaymentRefundCreateRequest{EntitlementAction: SaaSPaymentRefundEntitlementKeep}
	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "json") {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminPaymentRefundCreate{}, err
		}
		if err := json.Unmarshal(body, &req); err != nil {
			return SaaSAdminPaymentRefundCreate{}, errors.New("JSON 格式错误")
		}
	} else {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminPaymentRefundCreate{}, err
		}
		req.RefundNo = r.FormValue("refundNo")
		req.OrderNo = r.FormValue("orderNo")
		req.ProviderRefundNo = r.FormValue("providerRefundNo")
		req.IdempotencyKey = r.FormValue("idempotencyKey")
		req.AmountCents, _ = strconv.ParseInt(strings.TrimSpace(r.FormValue("amountCents")), 10, 64)
		req.Amount = r.FormValue("amount")
		req.Currency = r.FormValue("currency")
		req.Reason = r.FormValue("reason")
		req.EntitlementAction = r.FormValue("entitlementAction")
		req.Remark = r.FormValue("remark")
	}
	refundNo := strings.TrimSpace(req.RefundNo)
	if refundNo != "" && (len(refundNo) > 64 || !saasPaymentIdentifierPattern.MatchString(refundNo)) {
		return SaaSAdminPaymentRefundCreate{}, errors.New("refundNo 格式错误")
	}
	orderNo := strings.TrimSpace(req.OrderNo)
	if orderNo == "" || len(orderNo) > 64 || !saasPaymentIdentifierPattern.MatchString(orderNo) {
		return SaaSAdminPaymentRefundCreate{}, errors.New("orderNo invalid")
	}
	providerRefundNo := strings.TrimSpace(req.ProviderRefundNo)
	if len(providerRefundNo) > 128 {
		return SaaSAdminPaymentRefundCreate{}, errors.New("providerRefundNo too long")
	}
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if len(idempotencyKey) > 128 {
		return SaaSAdminPaymentRefundCreate{}, errors.New("idempotencyKey too long")
	}
	amountCents := req.AmountCents
	var err error
	if strings.TrimSpace(req.Amount) != "" {
		amountCents, err = parseSaaSAdminAmountCents(req.Amount)
		if err != nil {
			return SaaSAdminPaymentRefundCreate{}, err
		}
	}
	if amountCents <= 0 {
		return SaaSAdminPaymentRefundCreate{}, errors.New("amountCents must be positive")
	}
	currency := ""
	if strings.TrimSpace(req.Currency) != "" {
		currency, err = normalizeSaaSPaymentCurrency(req.Currency)
		if err != nil {
			return SaaSAdminPaymentRefundCreate{}, err
		}
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return SaaSAdminPaymentRefundCreate{}, errors.New("reason required")
	}
	if len([]rune(reason)) > 255 || len([]rune(req.Remark)) > 255 {
		return SaaSAdminPaymentRefundCreate{}, errors.New("reason or remark too long")
	}
	action := strings.ToLower(strings.TrimSpace(req.EntitlementAction))
	if action == "" {
		action = SaaSPaymentRefundEntitlementKeep
	}
	if !SaaSPaymentRefundEntitlementActionValid(action) {
		return SaaSAdminPaymentRefundCreate{}, errors.New("entitlementAction 必须是 keep、suspend 或 cancel")
	}
	metadataJSON := ""
	if req.Metadata != nil {
		raw, err := json.Marshal(req.Metadata)
		if err != nil {
			return SaaSAdminPaymentRefundCreate{}, errors.New("metadata invalid")
		}
		metadataJSON = string(raw)
	}
	return SaaSAdminPaymentRefundCreate{
		RefundNo: refundNo, OrderNo: orderNo, ProviderRefundNo: providerRefundNo,
		IdempotencyKey: idempotencyKey, AmountCents: amountCents, Currency: currency,
		Reason: reason, EntitlementAction: action, Remark: strings.TrimSpace(req.Remark), MetadataJSON: metadataJSON,
	}, nil
}

type saasAdminPaymentRefundCancelRequest struct {
	RefundNo        string `json:"refundNo"`
	ExpectedVersion int    `json:"expectedVersion"`
	Reason          string `json:"reason"`
}

func parseSaaSAdminPaymentRefundCancel(r *http.Request) (SaaSAdminPaymentRefundCancel, error) {
	var req saasAdminPaymentRefundCancelRequest
	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "json") {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminPaymentRefundCancel{}, err
		}
		if err := json.Unmarshal(body, &req); err != nil {
			return SaaSAdminPaymentRefundCancel{}, errors.New("JSON 格式错误")
		}
	} else {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminPaymentRefundCancel{}, err
		}
		req.RefundNo = r.FormValue("refundNo")
		req.ExpectedVersion, _ = strconv.Atoi(strings.TrimSpace(r.FormValue("expectedVersion")))
		req.Reason = r.FormValue("reason")
	}
	refundNo := strings.TrimSpace(req.RefundNo)
	if refundNo == "" || len(refundNo) > 64 || !saasPaymentIdentifierPattern.MatchString(refundNo) {
		return SaaSAdminPaymentRefundCancel{}, errors.New("refundNo invalid")
	}
	if req.ExpectedVersion <= 0 {
		return SaaSAdminPaymentRefundCancel{}, errors.New("expectedVersion required")
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" || len([]rune(reason)) > 255 {
		return SaaSAdminPaymentRefundCancel{}, errors.New("reason required and must not exceed 255 characters")
	}
	return SaaSAdminPaymentRefundCancel{RefundNo: refundNo, ExpectedVersion: req.ExpectedVersion, Reason: reason}, nil
}

func parseSaaSAdminPaymentRefundOptions(r *http.Request, maxLimit int) (SaaSAdminPaymentRefundOptions, error) {
	if maxLimit <= 0 {
		maxLimit = saasAdminListMaxLimit
	}
	query := r.URL.Query()
	options := SaaSAdminPaymentRefundOptions{
		TenantID: saasAdminQueryInt(r, "tenantId", 0),
		Status:   strings.ToLower(strings.TrimSpace(query.Get("status"))),
		Provider: strings.ToLower(strings.TrimSpace(query.Get("provider"))),
		OrderNo:  strings.TrimSpace(query.Get("orderNo")),
		Keyword:  strings.TrimSpace(query.Get("keyword")),
		Limit:    positiveQueryInt(r, "limit", 100),
	}
	if options.Limit > maxLimit {
		options.Limit = maxLimit
	}
	if options.Status == "" {
		options.Status = SaaSPaymentRefundStatusAll
	}
	if options.Status != SaaSPaymentRefundStatusAll && !SaaSPaymentRefundStatusValid(options.Status) {
		return SaaSAdminPaymentRefundOptions{}, errors.New("status 必须是 all、requested、processing、succeeded、failed 或 canceled")
	}
	if options.Provider != "" && (len(options.Provider) > 32 || !saasPaymentIdentifierPattern.MatchString(options.Provider)) {
		return SaaSAdminPaymentRefundOptions{}, errors.New("provider 格式错误")
	}
	if options.OrderNo != "" && (len(options.OrderNo) > 64 || !saasPaymentIdentifierPattern.MatchString(options.OrderNo)) {
		return SaaSAdminPaymentRefundOptions{}, errors.New("orderNo 格式错误")
	}
	if len([]rune(options.Keyword)) > 100 {
		return SaaSAdminPaymentRefundOptions{}, errors.New("keyword too long")
	}
	return options, nil
}

func newSaaSPaymentRefundNo() (string, error) {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate payment refund number: %w", err)
	}
	return "REF-" + time.Now().UTC().Format("20060102T150405") + "-" + strings.ToUpper(hex.EncodeToString(random)), nil
}

func saasAdminPaymentRefundReportPayload(report SaaSAdminPaymentRefundReport) map[string]any {
	return map[string]any{
		"filters": saasAdminPaymentRefundOptionsPayload(report.Options), "summary": saasAdminPaymentRefundSummaryPayload(report.Summary),
		"returnedCount": len(report.Refunds), "refunds": saasAdminPaymentRefundPayloads(report.Refunds),
	}
}

func saasAdminPaymentRefundOptionsPayload(options SaaSAdminPaymentRefundOptions) map[string]any {
	return map[string]any{"tenantId": options.TenantID, "status": options.Status, "provider": options.Provider, "orderNo": options.OrderNo, "keyword": options.Keyword, "limit": options.Limit}
}

func saasAdminPaymentRefundSummaryPayload(summary SaaSAdminPaymentRefundSummary) map[string]any {
	return map[string]any{
		"refundCount": summary.RefundCount, "requestedCount": summary.RequestedCount,
		"processingCount": summary.ProcessingCount, "succeededCount": summary.SucceededCount,
		"failedCount": summary.FailedCount, "canceledCount": summary.CanceledCount,
		"refundAmountCents": summary.RefundAmountCents, "pendingAmountCents": summary.PendingAmountCents,
		"succeededAmountCents": summary.SucceededAmountCents, "failedAmountCents": summary.FailedAmountCents,
		"tenantCount": summary.TenantCount, "orderCount": summary.OrderCount, "providerCount": summary.ProviderCount,
	}
}

func saasAdminPaymentRefundPayloads(items []SaaSAdminPaymentRefund) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, saasAdminPaymentRefundPayload(item))
	}
	return payloads
}

func saasAdminPaymentRefundPayload(item SaaSAdminPaymentRefund) map[string]any {
	return map[string]any{
		"id": item.ID, "refundNo": item.RefundNo, "paymentOrderId": item.PaymentOrderID, "orderNo": item.OrderNo,
		"tenantId": item.TenantID, "tenantName": item.TenantName, "provider": item.Provider,
		"providerRefundNo": item.ProviderRefundNo, "idempotencyKey": item.IdempotencyKey,
		"status": item.Status, "amountCents": item.AmountCents, "currency": item.Currency,
		"reason": item.Reason, "entitlementAction": item.EntitlementAction,
		"latestWebhookEventId": item.LatestWebhookEventID, "billingEventId": item.BillingEventID,
		"requestedByUserId": item.RequestedByUserID, "requestedByTenantId": item.RequestedByTenantID,
		"requestedAt": item.RequestedAt, "succeededAt": item.SucceededAt, "failedAt": item.FailedAt,
		"canceledAt": item.CanceledAt, "failureCode": item.FailureCode, "failureMessage": item.FailureMessage,
		"version": item.Version, "remark": item.Remark, "metadataJson": item.MetadataJSON,
		"createdAt": item.CreatedAt, "updatedAt": item.UpdatedAt,
		"orderAmountCents": item.OrderAmountCents, "orderRefundedCents": item.OrderRefundedCents,
		"orderRefundPendingCents": item.OrderRefundPending, "orderRefundableCents": item.OrderRefundableCents,
		"orderRefundStatus": item.OrderRefundStatus, "orderPaymentStatus": item.OrderPaymentStatus,
		"subscriptionEventId": item.SubscriptionEventID, "subscriptionOperationId": item.SubscriptionOperation,
	}
}

func saasAdminPaymentRefundCreateResultPayload(result SaaSAdminPaymentRefundCreateResult) map[string]any {
	return map[string]any{
		"refund": saasAdminPaymentRefundPayload(result.Refund), "order": saasAdminPaymentOrderPayload(result.Order),
		"operationId": result.OperationID, "idempotent": result.Idempotent,
	}
}

func saasAdminPaymentRefundCancelResultPayload(result SaaSAdminPaymentRefundCancelResult) map[string]any {
	return map[string]any{
		"refund": saasAdminPaymentRefundPayload(result.Refund), "order": saasAdminPaymentOrderPayload(result.Order),
		"previousStatus": result.PreviousStatus, "operationId": result.OperationID,
	}
}

func writeSaaSAdminPaymentRefundCSV(writer *csv.Writer, report SaaSAdminPaymentRefundReport) {
	_ = writer.Write([]string{
		"refundNo", "orderNo", "tenantId", "tenantName", "provider", "providerRefundNo", "status",
		"amountCents", "currency", "reason", "entitlementAction", "billingEventId", "requestedAt",
		"succeededAt", "failedAt", "canceledAt", "failureCode", "failureMessage", "version",
		"orderAmountCents", "orderRefundedCents", "orderRefundPendingCents", "orderRefundableCents",
		"orderRefundStatus", "remark", "createdAt", "updatedAt",
	})
	for _, item := range report.Refunds {
		_ = writer.Write([]string{
			item.RefundNo, item.OrderNo, strconv.Itoa(item.TenantID), item.TenantName, item.Provider,
			item.ProviderRefundNo, item.Status, strconv.FormatInt(item.AmountCents, 10), item.Currency,
			item.Reason, item.EntitlementAction, strconv.FormatInt(item.BillingEventID, 10), item.RequestedAt,
			item.SucceededAt, item.FailedAt, item.CanceledAt, item.FailureCode, item.FailureMessage,
			strconv.Itoa(item.Version), strconv.FormatInt(item.OrderAmountCents, 10),
			strconv.FormatInt(item.OrderRefundedCents, 10), strconv.FormatInt(item.OrderRefundPending, 10),
			strconv.FormatInt(item.OrderRefundableCents, 10), item.OrderRefundStatus, item.Remark,
			item.CreatedAt, item.UpdatedAt,
		})
	}
}
