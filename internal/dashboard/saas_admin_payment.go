package dashboard

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	SaaSAdminExportKindPaymentOrders = "paymentOrders"

	SaaSPaymentOrderStatusAll        = "all"
	SaaSPaymentOrderStatusPending    = "pending"
	SaaSPaymentOrderStatusProcessing = "processing"
	SaaSPaymentOrderStatusPaid       = "paid"
	SaaSPaymentOrderStatusFailed     = "failed"
	SaaSPaymentOrderStatusCanceled   = "canceled"

	SaaSPaymentWebhookEventStatusAll       = "all"
	SaaSPaymentWebhookEventStatusReceived  = "received"
	SaaSPaymentWebhookEventStatusProcessed = "processed"
	SaaSPaymentWebhookEventStatusIgnored   = "ignored"
	SaaSPaymentWebhookEventStatusFailed    = "failed"

	SaaSPaymentWebhookTypeProcessing = "payment.processing"
	SaaSPaymentWebhookTypeSucceeded  = "payment.succeeded"
	SaaSPaymentWebhookTypeFailed     = "payment.failed"
	SaaSPaymentWebhookTypeCanceled   = "payment.canceled"

	SaaSPaymentWebhookTimestampHeader = "X-Mochat-Go-Payment-Timestamp"
	SaaSPaymentWebhookSignatureHeader = "X-Mochat-Go-Payment-Signature"

	SaaSAdminOperationActionPaymentOrderCreate  = "payment.order.create"
	SaaSAdminOperationActionPaymentOrderCancel  = "payment.order.cancel"
	SaaSAdminOperationActionPaymentOrderPaid    = "payment.order.paid"
	SaaSAdminOperationActionPaymentOrderFailed  = "payment.order.failed"
	SaaSAdminOperationActionPaymentOrderProcess = "payment.order.processing"
	SaaSAdminOperationActionPaymentDunning      = "payment.order.dunning"
	SaaSAdminOperationTargetPaymentOrder        = "payment_order"
)

var saasPaymentIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)

type SaaSAdminPaymentOrderOptions struct {
	TenantID    int
	Status      string
	Provider    string
	PackageCode string
	Keyword     string
	Limit       int
}

type SaaSAdminPaymentOrder struct {
	ID                    int64
	OrderNo               string
	TenantID              int
	TenantName            string
	Provider              string
	ProviderOrderNo       string
	IdempotencyKey        string
	Status                string
	PackageCode           string
	PackageName           string
	PackageVersion        int
	PackageLimits         SaaSAdminPackageLimits
	BillingCycle          string
	ServiceExpiresAt      string
	AmountCents           int64
	RefundPendingCents    int64
	RefundedAmountCents   int64
	RefundableAmountCents int64
	RefundStatus          string
	InvoicePendingCents   int64
	InvoicedAmountCents   int64
	CreditPendingCents    int64
	CreditedAmountCents   int64
	NetInvoicedCents      int64
	InvoiceAvailableCents int64
	CreditNoteDueCents    int64
	InvoiceStatus         string
	Currency              string
	CheckoutURL           string
	CheckoutExpiresAt     string
	AttemptCount          int
	DunningAttempts       int
	MaxDunningAttempts    int
	NextDunningAt         string
	LastDunningAt         string
	LatestWebhookEventID  int64
	BillingEventID        int64
	LatestRefundID        int64
	LatestInvoiceID       int64
	PaidAt                string
	FailedAt              string
	CanceledAt            string
	FailureCode           string
	FailureMessage        string
	Version               int
	CreatedByUserID       int
	CreatedByTenantID     int
	Remark                string
	MetadataJSON          string
	CreatedAt             string
	UpdatedAt             string
	DunningDue            bool
}

type SaaSAdminPaymentOrderSummary struct {
	OrderCount          int
	PendingCount        int
	ProcessingCount     int
	PaidCount           int
	FailedCount         int
	CanceledCount       int
	DunningDueCount     int
	CollectibleCount    int
	OrderAmountCents    int64
	PaidAmountCents     int64
	RefundPendingCents  int64
	RefundedAmountCents int64
	NetPaidAmountCents  int64
	InvoicePendingCents int64
	InvoicedAmountCents int64
	CreditPendingCents  int64
	CreditedAmountCents int64
	NetInvoicedCents    int64
	InvoiceAvailable    int64
	CreditNoteDueCents  int64
	OutstandingCents    int64
	TenantCount         int
	ProviderCount       int
	PackageCount        int
}

type SaaSAdminPaymentOrderReport struct {
	Options SaaSAdminPaymentOrderOptions
	Summary SaaSAdminPaymentOrderSummary
	Orders  []SaaSAdminPaymentOrder
}

type SaaSAdminPaymentOrderCreate struct {
	OrderNo                  string `json:"orderNo"`
	TenantID                 int    `json:"tenantId"`
	Provider                 string `json:"provider"`
	ProviderOrderNo          string `json:"providerOrderNo"`
	IdempotencyKey           string `json:"idempotencyKey"`
	PackageCode              string `json:"packageCode"`
	BillingCycle             string `json:"billingCycle"`
	ServiceExpiresAt         string `json:"serviceExpiresAt"`
	AmountCents              int64  `json:"amountCents"`
	Currency                 string `json:"currency"`
	CheckoutURL              string `json:"checkoutUrl"`
	CheckoutExpiresAt        string `json:"checkoutExpiresAt"`
	MaxDunningAttempts       int    `json:"maxDunningAttempts"`
	Remark                   string `json:"remark"`
	MetadataJSON             string `json:"metadataJson"`
	ExpectedTenantStatus     int    `json:"expectedTenantStatus"`
	ExpectedPackageVersion   int    `json:"expectedPackageVersion"`
	ActorUserID              int    `json:"-"`
	ActorTenantID            int    `json:"-"`
	ApprovalExecutionID      int64  `json:"-"`
	ApprovalExecutionVersion int    `json:"-"`
}

type SaaSAdminPaymentOrderTenantSnapshot struct {
	TenantID     int    `json:"tenantId"`
	TenantName   string `json:"tenantName"`
	TenantStatus int    `json:"tenantStatus"`
}

type SaaSAdminPaymentOrderCreateApprovalPlan struct {
	Create  SaaSAdminPaymentOrderCreate         `json:"create"`
	Tenant  SaaSAdminPaymentOrderTenantSnapshot `json:"tenant"`
	Package SaaSAdminPackage                    `json:"package"`
}

type SaaSAdminPaymentOrderCreateResult struct {
	Order       SaaSAdminPaymentOrder
	OperationID int64
	Idempotent  bool
}

type SaaSAdminPaymentOrderCancel struct {
	OrderNo         string
	ExpectedVersion int
	Reason          string
	ActorUserID     int
	ActorTenantID   int
}

type SaaSAdminPaymentOrderCancelResult struct {
	Order          SaaSAdminPaymentOrder
	PreviousStatus string
	OperationID    int64
}

type SaaSAdminPaymentWebhookEventOptions struct {
	TenantID  int
	Provider  string
	Status    string
	EventType string
	Keyword   string
	Limit     int
}

type SaaSAdminPaymentWebhookEvent struct {
	ID                 int64
	Provider           string
	EventID            string
	EventType          string
	OrderID            int64
	OrderNo            string
	ProviderOrderNo    string
	RefundID           int64
	RefundNo           string
	ProviderRefundNo   string
	Status             string
	PayloadSHA256      string
	SignatureTimestamp int64
	Attempts           int
	OccurredAt         string
	ProcessedAt        string
	LastError          string
	PayloadJSON        string
	CreatedAt          string
	UpdatedAt          string
}

type SaaSPaymentWebhookInput struct {
	Provider           string
	EventID            string
	EventType          string
	OrderNo            string
	ProviderOrderNo    string
	RefundNo           string
	ProviderRefundNo   string
	AmountCents        int64
	Currency           string
	PaidAt             string
	RefundedAt         string
	OccurredAt         string
	FailureCode        string
	FailureMessage     string
	MetadataJSON       string
	PayloadJSON        string
	PayloadSHA256      string
	SignatureTimestamp int64
}

type SaaSPaymentWebhookResult struct {
	Event            SaaSAdminPaymentWebhookEvent
	Order            SaaSAdminPaymentOrder
	Refund           SaaSAdminPaymentRefund
	Duplicate        bool
	Ignored          bool
	BillingEventID   int64
	MetricsRefreshed int
	Warning          string
}

type SaaSPaymentDunningOptions struct {
	Limit                   int
	DryRun                  bool
	RetryDelaySeconds       int
	NotificationMaxAttempts int
	ActorUserID             int
	ActorTenantID           int
}

type SaaSPaymentDunningResult struct {
	MatchedCount   int
	EnqueuedCount  int
	ExhaustedCount int
	SkippedCount   int
	FailedCount    int
	DryRun         bool
	OrderNos       []string
	Errors         []SaaSPaymentDunningError
}

type SaaSPaymentDunningError struct {
	OrderNo string
	Message string
}

type SaaSAdminPaymentStore interface {
	SaaSAdminPaymentOrders(ctx context.Context, options SaaSAdminPaymentOrderOptions) (SaaSAdminPaymentOrderReport, error)
	SaaSAdminPaymentWebhookEvents(ctx context.Context, options SaaSAdminPaymentWebhookEventOptions) ([]SaaSAdminPaymentWebhookEvent, error)
	CreateSaaSAdminPaymentOrder(ctx context.Context, create SaaSAdminPaymentOrderCreate) (SaaSAdminPaymentOrderCreateResult, error)
	CancelSaaSAdminPaymentOrder(ctx context.Context, cancel SaaSAdminPaymentOrderCancel) (SaaSAdminPaymentOrderCancelResult, error)
	ProcessSaaSPaymentDunning(ctx context.Context, options SaaSPaymentDunningOptions) (SaaSPaymentDunningResult, error)
}

type SaaSPaymentWebhookStore interface {
	ProcessSaaSPaymentWebhook(ctx context.Context, input SaaSPaymentWebhookInput) (SaaSPaymentWebhookResult, error)
}

type SaaSPaymentDunningStore interface {
	ProcessSaaSPaymentDunning(ctx context.Context, options SaaSPaymentDunningOptions) (SaaSPaymentDunningResult, error)
}

func SaaSPaymentOrderStatusValid(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case SaaSPaymentOrderStatusPending, SaaSPaymentOrderStatusProcessing, SaaSPaymentOrderStatusPaid,
		SaaSPaymentOrderStatusFailed, SaaSPaymentOrderStatusCanceled:
		return true
	default:
		return false
	}
}

func SaaSPaymentWebhookEventStatusValid(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case SaaSPaymentWebhookEventStatusReceived, SaaSPaymentWebhookEventStatusProcessed,
		SaaSPaymentWebhookEventStatusIgnored, SaaSPaymentWebhookEventStatusFailed:
		return true
	default:
		return false
	}
}

func SaaSPaymentWebhookTypeValid(eventType string) bool {
	if SaaSPaymentWebhookRefundType(eventType) {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case SaaSPaymentWebhookTypeProcessing, SaaSPaymentWebhookTypeSucceeded,
		SaaSPaymentWebhookTypeFailed, SaaSPaymentWebhookTypeCanceled:
		return true
	default:
		return false
	}
}

func (h *SaaSAdminHandler) PaymentOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.paymentStore(w)
	if !ok {
		return
	}
	options, err := parseSaaSAdminPaymentOrderOptions(r, saasAdminListMaxLimit)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	report, err := store.SaaSAdminPaymentOrders(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", saasAdminPaymentOrderReportPayload(report))
}

func (h *SaaSAdminHandler) PaymentWebhookEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.paymentStore(w)
	if !ok {
		return
	}
	options, err := parseSaaSAdminPaymentWebhookEventOptions(r, saasAdminListMaxLimit)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	events, err := store.SaaSAdminPaymentWebhookEvents(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{
		"filters":       saasAdminPaymentWebhookEventOptionsPayload(options),
		"returnedCount": len(events),
		"events":        saasAdminPaymentWebhookEventPayloads(events),
	})
}

func (h *SaaSAdminHandler) CreatePaymentOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.paymentStore(w)
	if !ok {
		return
	}
	create, err := parseSaaSAdminPaymentOrderCreate(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionPaymentOrderCreate, 0) {
		return
	}
	if create.OrderNo == "" {
		create.OrderNo, err = newSaaSPaymentOrderNo()
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
	}
	create.ActorUserID = user.ID
	create.ActorTenantID = user.TenantID
	result, err := store.CreateSaaSAdminPaymentOrder(r.Context(), create)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", saasAdminPaymentOrderCreateResultPayload(result))
}

func (h *SaaSAdminHandler) planSaaSAdminPaymentOrderCreate(ctx context.Context, create SaaSAdminPaymentOrderCreate) (SaaSAdminPaymentOrderCreateApprovalPlan, error) {
	overview, err := h.saasAdminOverview(ctx, SaaSAdminOverviewOptions{
		Scope: SaaSAdminScopeTenant, TenantID: create.TenantID, Limit: 1, DueState: SaaSAdminDueStateAll, ExpiringDays: 30,
	})
	if err != nil {
		return SaaSAdminPaymentOrderCreateApprovalPlan{}, err
	}
	if len(overview.Tenants) == 0 {
		return SaaSAdminPaymentOrderCreateApprovalPlan{}, NewSaaSAdminNotFound("tenant not found")
	}
	tenant := overview.Tenants[0]
	if tenant.TenantStatus != 1 {
		return SaaSAdminPaymentOrderCreateApprovalPlan{}, &SaaSAdminOperationError{Status: http.StatusConflict, Message: "只能为正常租户创建支付订单"}
	}
	packages, err := h.store.SaaSAdminPackages(ctx)
	if err != nil {
		return SaaSAdminPaymentOrderCreateApprovalPlan{}, err
	}
	var target SaaSAdminPackage
	found := false
	for _, item := range packages {
		if item.Code == create.PackageCode {
			target = item
			found = true
			break
		}
	}
	if !found {
		return SaaSAdminPaymentOrderCreateApprovalPlan{}, NewSaaSAdminNotFound("package not found")
	}
	if target.Status != 1 || target.Version <= 0 {
		return SaaSAdminPaymentOrderCreateApprovalPlan{}, &SaaSAdminOperationError{Status: http.StatusConflict, Message: "套餐已停用或版本无效"}
	}
	create.ExpectedTenantStatus = tenant.TenantStatus
	create.ExpectedPackageVersion = target.Version
	return SaaSAdminPaymentOrderCreateApprovalPlan{
		Create:  create,
		Tenant:  SaaSAdminPaymentOrderTenantSnapshot{TenantID: tenant.TenantID, TenantName: tenant.TenantName, TenantStatus: tenant.TenantStatus},
		Package: target,
	}, nil
}

func (h *SaaSAdminHandler) CancelPaymentOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.paymentStore(w)
	if !ok {
		return
	}
	cancel, err := parseSaaSAdminPaymentOrderCancel(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	cancel.ActorUserID = user.ID
	cancel.ActorTenantID = user.TenantID
	result, err := store.CancelSaaSAdminPaymentOrder(r.Context(), cancel)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{
		"order":          saasAdminPaymentOrderPayload(result.Order),
		"previousStatus": result.PreviousStatus,
		"operationId":    result.OperationID,
	})
}

func (h *SaaSAdminHandler) PaymentDunning(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.paymentStore(w)
	if !ok {
		return
	}
	options, err := parseSaaSPaymentDunningOptions(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	options.ActorUserID = user.ID
	options.ActorTenantID = user.TenantID
	result, err := store.ProcessSaaSPaymentDunning(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", saasPaymentDunningResultPayload(result))
}

func (h *SaaSAdminHandler) paymentStore(w http.ResponseWriter) (SaaSAdminPaymentStore, bool) {
	store, ok := h.store.(SaaSAdminPaymentStore)
	if !ok || store == nil {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "payment collection store is not configured", nil)
		return nil, false
	}
	return store, true
}

type SaaSPaymentWebhookHandler struct {
	store     SaaSPaymentWebhookStore
	secret    string
	tolerance time.Duration
	now       func() time.Time
}

func NewSaaSPaymentWebhookHandler(store SaaSPaymentWebhookStore, secret string, tolerance time.Duration) *SaaSPaymentWebhookHandler {
	if tolerance <= 0 {
		tolerance = 5 * time.Minute
	}
	return &SaaSPaymentWebhookHandler{store: store, secret: strings.TrimSpace(secret), tolerance: tolerance, now: time.Now}
}

func (h *SaaSPaymentWebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if h == nil || h.store == nil || h.secret == "" {
		writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "payment webhook is not configured", nil)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "read payment webhook body failed", nil)
		return
	}
	timestamp, err := strconv.ParseInt(strings.TrimSpace(r.Header.Get(SaaSPaymentWebhookTimestampHeader)), 10, 64)
	if err != nil || timestamp <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "payment webhook timestamp invalid", nil)
		return
	}
	now := h.now()
	delta := now.Sub(time.Unix(timestamp, 0))
	if delta < 0 {
		delta = -delta
	}
	if delta > h.tolerance {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "payment webhook timestamp expired", nil)
		return
	}
	gotSignature := strings.TrimSpace(r.Header.Get(SaaSPaymentWebhookSignatureHeader))
	wantSignature := "v1=" + SaaSPaymentWebhookSignature(h.secret, timestamp, body)
	if !hmac.Equal([]byte(gotSignature), []byte(wantSignature)) {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "payment webhook signature invalid", nil)
		return
	}
	input, err := parseSaaSPaymentWebhookInput(body, timestamp)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	result, err := h.store.ProcessSaaSPaymentWebhook(r.Context(), input)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 0, "ok", saasPaymentWebhookResultPayload(result))
}

func SaaSPaymentWebhookSignature(secret string, timestamp int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

type saasAdminPaymentOrderCreateRequest struct {
	OrderNo            string         `json:"orderNo"`
	TenantID           int            `json:"tenantId"`
	Provider           string         `json:"provider"`
	ProviderOrderNo    string         `json:"providerOrderNo"`
	IdempotencyKey     string         `json:"idempotencyKey"`
	PackageCode        string         `json:"packageCode"`
	BillingCycle       string         `json:"billingCycle"`
	ServiceExpiresAt   string         `json:"serviceExpiresAt"`
	AmountCents        int64          `json:"amountCents"`
	Amount             string         `json:"amount"`
	Currency           string         `json:"currency"`
	CheckoutURL        string         `json:"checkoutUrl"`
	CheckoutExpiresAt  string         `json:"checkoutExpiresAt"`
	MaxDunningAttempts int            `json:"maxDunningAttempts"`
	Remark             string         `json:"remark"`
	Metadata           map[string]any `json:"metadata"`
}

func parseSaaSAdminPaymentOrderCreate(r *http.Request) (SaaSAdminPaymentOrderCreate, error) {
	req := saasAdminPaymentOrderCreateRequest{Provider: "gateway", BillingCycle: "custom", Currency: "CNY", MaxDunningAttempts: 3}
	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "json") {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminPaymentOrderCreate{}, err
		}
		if err := json.Unmarshal(body, &req); err != nil {
			return SaaSAdminPaymentOrderCreate{}, errors.New("JSON 格式错误")
		}
	} else {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminPaymentOrderCreate{}, err
		}
		req.OrderNo = r.FormValue("orderNo")
		req.TenantID, _ = strconv.Atoi(strings.TrimSpace(r.FormValue("tenantId")))
		req.Provider = r.FormValue("provider")
		req.ProviderOrderNo = r.FormValue("providerOrderNo")
		req.IdempotencyKey = r.FormValue("idempotencyKey")
		req.PackageCode = r.FormValue("packageCode")
		req.BillingCycle = r.FormValue("billingCycle")
		req.ServiceExpiresAt = r.FormValue("serviceExpiresAt")
		req.AmountCents, _ = strconv.ParseInt(strings.TrimSpace(r.FormValue("amountCents")), 10, 64)
		req.Amount = r.FormValue("amount")
		req.Currency = r.FormValue("currency")
		req.CheckoutURL = r.FormValue("checkoutUrl")
		req.CheckoutExpiresAt = r.FormValue("checkoutExpiresAt")
		req.MaxDunningAttempts, _ = strconv.Atoi(strings.TrimSpace(r.FormValue("maxDunningAttempts")))
		req.Remark = r.FormValue("remark")
	}
	orderNo := strings.TrimSpace(req.OrderNo)
	if orderNo != "" && (!saasPaymentIdentifierPattern.MatchString(orderNo) || len(orderNo) > 64) {
		return SaaSAdminPaymentOrderCreate{}, errors.New("orderNo 格式错误")
	}
	if req.TenantID <= 0 {
		return SaaSAdminPaymentOrderCreate{}, errors.New("tenantId required")
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" {
		provider = "gateway"
	}
	if !saasPaymentIdentifierPattern.MatchString(provider) || len(provider) > 32 {
		return SaaSAdminPaymentOrderCreate{}, errors.New("provider 格式错误")
	}
	providerOrderNo := strings.TrimSpace(req.ProviderOrderNo)
	if len(providerOrderNo) > 128 {
		return SaaSAdminPaymentOrderCreate{}, errors.New("providerOrderNo too long")
	}
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if len(idempotencyKey) > 128 {
		return SaaSAdminPaymentOrderCreate{}, errors.New("idempotencyKey too long")
	}
	packageCode := strings.TrimSpace(req.PackageCode)
	if packageCode == "" || len(packageCode) > 64 {
		return SaaSAdminPaymentOrderCreate{}, errors.New("packageCode required")
	}
	billingCycle := strings.ToLower(strings.TrimSpace(req.BillingCycle))
	if billingCycle == "" {
		billingCycle = "custom"
	}
	if billingCycle != "monthly" && billingCycle != "yearly" && billingCycle != "custom" && billingCycle != "lifetime" {
		return SaaSAdminPaymentOrderCreate{}, errors.New("billingCycle 必须是 monthly、yearly、custom 或 lifetime")
	}
	serviceExpiresAt, err := normalizeSaaSPaymentOptionalTimestamp(req.ServiceExpiresAt, "serviceExpiresAt")
	if err != nil {
		return SaaSAdminPaymentOrderCreate{}, err
	}
	if billingCycle == "lifetime" {
		serviceExpiresAt = ""
	} else if serviceExpiresAt == "" {
		return SaaSAdminPaymentOrderCreate{}, errors.New("serviceExpiresAt required")
	}
	amountCents := req.AmountCents
	if strings.TrimSpace(req.Amount) != "" {
		amountCents, err = parseSaaSAdminAmountCents(req.Amount)
		if err != nil {
			return SaaSAdminPaymentOrderCreate{}, err
		}
	}
	if amountCents <= 0 {
		return SaaSAdminPaymentOrderCreate{}, errors.New("amountCents must be positive")
	}
	currency, err := normalizeSaaSPaymentCurrency(req.Currency)
	if err != nil {
		return SaaSAdminPaymentOrderCreate{}, err
	}
	checkoutURL := strings.TrimSpace(req.CheckoutURL)
	if checkoutURL != "" {
		parsed, err := url.Parse(checkoutURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
			return SaaSAdminPaymentOrderCreate{}, errors.New("checkoutUrl 必须是绝对 HTTP(S) URL")
		}
	}
	checkoutExpiresAt, err := normalizeSaaSPaymentOptionalTimestamp(req.CheckoutExpiresAt, "checkoutExpiresAt")
	if err != nil {
		return SaaSAdminPaymentOrderCreate{}, err
	}
	if checkoutExpiresAt == "" {
		checkoutExpiresAt = time.Now().Add(30 * time.Minute).Format("2006-01-02 15:04:05")
	}
	if req.MaxDunningAttempts == 0 {
		req.MaxDunningAttempts = 3
	}
	if req.MaxDunningAttempts < 1 || req.MaxDunningAttempts > 10 {
		return SaaSAdminPaymentOrderCreate{}, errors.New("maxDunningAttempts 必须在 1 至 10 之间")
	}
	if len([]rune(strings.TrimSpace(req.Remark))) > 255 {
		return SaaSAdminPaymentOrderCreate{}, errors.New("remark too long")
	}
	metadataJSON := ""
	if req.Metadata != nil {
		raw, err := json.Marshal(req.Metadata)
		if err != nil {
			return SaaSAdminPaymentOrderCreate{}, errors.New("metadata invalid")
		}
		metadataJSON = string(raw)
	}
	return SaaSAdminPaymentOrderCreate{
		OrderNo: orderNo, TenantID: req.TenantID, Provider: provider, ProviderOrderNo: providerOrderNo,
		IdempotencyKey: idempotencyKey, PackageCode: packageCode, BillingCycle: billingCycle,
		ServiceExpiresAt: serviceExpiresAt, AmountCents: amountCents, Currency: currency,
		CheckoutURL: checkoutURL, CheckoutExpiresAt: checkoutExpiresAt,
		MaxDunningAttempts: req.MaxDunningAttempts, Remark: strings.TrimSpace(req.Remark), MetadataJSON: metadataJSON,
	}, nil
}

type saasAdminPaymentOrderCancelRequest struct {
	OrderNo         string `json:"orderNo"`
	ExpectedVersion int    `json:"expectedVersion"`
	Reason          string `json:"reason"`
}

func parseSaaSAdminPaymentOrderCancel(r *http.Request) (SaaSAdminPaymentOrderCancel, error) {
	var req saasAdminPaymentOrderCancelRequest
	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "json") {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminPaymentOrderCancel{}, err
		}
		if err := json.Unmarshal(body, &req); err != nil {
			return SaaSAdminPaymentOrderCancel{}, errors.New("JSON 格式错误")
		}
	} else {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminPaymentOrderCancel{}, err
		}
		req.OrderNo = r.FormValue("orderNo")
		req.ExpectedVersion, _ = strconv.Atoi(strings.TrimSpace(r.FormValue("expectedVersion")))
		req.Reason = r.FormValue("reason")
	}
	req.OrderNo = strings.TrimSpace(req.OrderNo)
	if req.OrderNo == "" || len(req.OrderNo) > 64 {
		return SaaSAdminPaymentOrderCancel{}, errors.New("orderNo required")
	}
	if req.ExpectedVersion <= 0 {
		return SaaSAdminPaymentOrderCancel{}, errors.New("expectedVersion required")
	}
	reason := strings.TrimSpace(req.Reason)
	if len([]rune(reason)) > 255 {
		return SaaSAdminPaymentOrderCancel{}, errors.New("reason too long")
	}
	return SaaSAdminPaymentOrderCancel{OrderNo: req.OrderNo, ExpectedVersion: req.ExpectedVersion, Reason: reason}, nil
}

func parseSaaSAdminPaymentOrderOptions(r *http.Request, maxLimit int) (SaaSAdminPaymentOrderOptions, error) {
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status == "" {
		status = SaaSPaymentOrderStatusAll
	}
	if status != SaaSPaymentOrderStatusAll && !SaaSPaymentOrderStatusValid(status) {
		return SaaSAdminPaymentOrderOptions{}, errors.New("status invalid")
	}
	provider := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("provider")))
	if len(provider) > 32 {
		return SaaSAdminPaymentOrderOptions{}, errors.New("provider too long")
	}
	packageCode := strings.TrimSpace(r.URL.Query().Get("packageCode"))
	if len(packageCode) > 64 {
		return SaaSAdminPaymentOrderOptions{}, errors.New("packageCode too long")
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if len([]rune(keyword)) > 80 {
		return SaaSAdminPaymentOrderOptions{}, errors.New("keyword too long")
	}
	limit := positiveQueryInt(r, "limit", 100)
	if limit > maxLimit {
		limit = maxLimit
	}
	return SaaSAdminPaymentOrderOptions{TenantID: saasAdminQueryInt(r, "tenantId", 0), Status: status, Provider: provider, PackageCode: packageCode, Keyword: keyword, Limit: limit}, nil
}

func parseSaaSAdminPaymentWebhookEventOptions(r *http.Request, maxLimit int) (SaaSAdminPaymentWebhookEventOptions, error) {
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status == "" {
		status = SaaSPaymentWebhookEventStatusAll
	}
	if status != SaaSPaymentWebhookEventStatusAll && !SaaSPaymentWebhookEventStatusValid(status) {
		return SaaSAdminPaymentWebhookEventOptions{}, errors.New("status invalid")
	}
	eventType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("eventType")))
	if eventType != "" && !SaaSPaymentWebhookTypeValid(eventType) {
		return SaaSAdminPaymentWebhookEventOptions{}, errors.New("eventType invalid")
	}
	provider := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("provider")))
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if len(provider) > 32 || len([]rune(keyword)) > 80 {
		return SaaSAdminPaymentWebhookEventOptions{}, errors.New("filter too long")
	}
	limit := positiveQueryInt(r, "limit", 50)
	if limit > maxLimit {
		limit = maxLimit
	}
	return SaaSAdminPaymentWebhookEventOptions{TenantID: saasAdminQueryInt(r, "tenantId", 0), Provider: provider, Status: status, EventType: eventType, Keyword: keyword, Limit: limit}, nil
}

type saasPaymentDunningRequest struct {
	Limit                   int  `json:"limit"`
	DryRun                  bool `json:"dryRun"`
	RetryDelaySeconds       int  `json:"retryDelaySeconds"`
	NotificationMaxAttempts int  `json:"notificationMaxAttempts"`
}

func parseSaaSPaymentDunningOptions(r *http.Request) (SaaSPaymentDunningOptions, error) {
	req := saasPaymentDunningRequest{Limit: 100, RetryDelaySeconds: 86400, NotificationMaxAttempts: 3}
	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "json") {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSPaymentDunningOptions{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSPaymentDunningOptions{}, errors.New("JSON 格式错误")
			}
		}
	} else {
		if err := r.ParseForm(); err != nil {
			return SaaSPaymentDunningOptions{}, err
		}
		if raw := strings.TrimSpace(r.FormValue("limit")); raw != "" {
			req.Limit, _ = strconv.Atoi(raw)
		}
		if raw := strings.TrimSpace(r.FormValue("dryRun")); raw != "" {
			value, err := strconv.ParseBool(raw)
			if err != nil {
				return SaaSPaymentDunningOptions{}, errors.New("dryRun 必须是布尔值")
			}
			req.DryRun = value
		}
		if raw := strings.TrimSpace(r.FormValue("retryDelaySeconds")); raw != "" {
			req.RetryDelaySeconds, _ = strconv.Atoi(raw)
		}
		if raw := strings.TrimSpace(r.FormValue("notificationMaxAttempts")); raw != "" {
			req.NotificationMaxAttempts, _ = strconv.Atoi(raw)
		}
	}
	if req.Limit <= 0 || req.Limit > 5000 {
		return SaaSPaymentDunningOptions{}, errors.New("limit 必须在 1 至 5000 之间")
	}
	if req.RetryDelaySeconds < 60 || req.RetryDelaySeconds > 2592000 {
		return SaaSPaymentDunningOptions{}, errors.New("retryDelaySeconds 必须在 60 至 2592000 之间")
	}
	if req.NotificationMaxAttempts < 1 || req.NotificationMaxAttempts > 20 {
		return SaaSPaymentDunningOptions{}, errors.New("notificationMaxAttempts 必须在 1 至 20 之间")
	}
	return SaaSPaymentDunningOptions{Limit: req.Limit, DryRun: req.DryRun, RetryDelaySeconds: req.RetryDelaySeconds, NotificationMaxAttempts: req.NotificationMaxAttempts}, nil
}

type saasPaymentWebhookRequest struct {
	Provider         string         `json:"provider"`
	EventID          string         `json:"eventId"`
	EventType        string         `json:"eventType"`
	OrderNo          string         `json:"orderNo"`
	ProviderOrderNo  string         `json:"providerOrderNo"`
	RefundNo         string         `json:"refundNo"`
	ProviderRefundNo string         `json:"providerRefundNo"`
	AmountCents      int64          `json:"amountCents"`
	Currency         string         `json:"currency"`
	PaidAt           string         `json:"paidAt"`
	RefundedAt       string         `json:"refundedAt"`
	OccurredAt       string         `json:"occurredAt"`
	FailureCode      string         `json:"failureCode"`
	FailureMessage   string         `json:"failureMessage"`
	Metadata         map[string]any `json:"metadata"`
}

func parseSaaSPaymentWebhookInput(body []byte, signatureTimestamp int64) (SaaSPaymentWebhookInput, error) {
	var req saasPaymentWebhookRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return SaaSPaymentWebhookInput{}, errors.New("JSON 格式错误")
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" || len(provider) > 32 || !saasPaymentIdentifierPattern.MatchString(provider) {
		return SaaSPaymentWebhookInput{}, errors.New("provider invalid")
	}
	eventID := strings.TrimSpace(req.EventID)
	if eventID == "" || len(eventID) > 128 || !saasPaymentIdentifierPattern.MatchString(eventID) {
		return SaaSPaymentWebhookInput{}, errors.New("eventId invalid")
	}
	eventType := strings.ToLower(strings.TrimSpace(req.EventType))
	if !SaaSPaymentWebhookTypeValid(eventType) {
		return SaaSPaymentWebhookInput{}, errors.New("eventType invalid")
	}
	orderNo := strings.TrimSpace(req.OrderNo)
	if orderNo == "" || len(orderNo) > 64 || !saasPaymentIdentifierPattern.MatchString(orderNo) {
		return SaaSPaymentWebhookInput{}, errors.New("orderNo invalid")
	}
	providerOrderNo := strings.TrimSpace(req.ProviderOrderNo)
	if len(providerOrderNo) > 128 {
		return SaaSPaymentWebhookInput{}, errors.New("providerOrderNo too long")
	}
	refundNo := strings.TrimSpace(req.RefundNo)
	if SaaSPaymentWebhookRefundType(eventType) && (refundNo == "" || len(refundNo) > 64 || !saasPaymentIdentifierPattern.MatchString(refundNo)) {
		return SaaSPaymentWebhookInput{}, errors.New("refundNo invalid")
	}
	providerRefundNo := strings.TrimSpace(req.ProviderRefundNo)
	if len(providerRefundNo) > 128 {
		return SaaSPaymentWebhookInput{}, errors.New("providerRefundNo too long")
	}
	currency := ""
	var err error
	if eventType == SaaSPaymentWebhookTypeSucceeded || eventType == SaaSPaymentWebhookTypeRefundSucceeded {
		if req.AmountCents <= 0 {
			return SaaSPaymentWebhookInput{}, errors.New("amountCents must be positive for succeeded event")
		}
		currency, err = normalizeSaaSPaymentCurrency(req.Currency)
		if err != nil {
			return SaaSPaymentWebhookInput{}, err
		}
	} else if strings.TrimSpace(req.Currency) != "" {
		currency, err = normalizeSaaSPaymentCurrency(req.Currency)
		if err != nil {
			return SaaSPaymentWebhookInput{}, err
		}
	}
	paidAt, err := normalizeSaaSPaymentOptionalTimestamp(req.PaidAt, "paidAt")
	if err != nil {
		return SaaSPaymentWebhookInput{}, err
	}
	refundedAt, err := normalizeSaaSPaymentOptionalTimestamp(req.RefundedAt, "refundedAt")
	if err != nil {
		return SaaSPaymentWebhookInput{}, err
	}
	occurredAt, err := normalizeSaaSPaymentOptionalTimestamp(req.OccurredAt, "occurredAt")
	if err != nil {
		return SaaSPaymentWebhookInput{}, err
	}
	if occurredAt == "" {
		occurredAt = time.Unix(signatureTimestamp, 0).Format("2006-01-02 15:04:05")
	}
	failureCode := strings.TrimSpace(req.FailureCode)
	failureMessage := strings.TrimSpace(req.FailureMessage)
	if len(failureCode) > 64 || len([]rune(failureMessage)) > 255 {
		return SaaSPaymentWebhookInput{}, errors.New("failure detail too long")
	}
	metadataJSON := ""
	if req.Metadata != nil {
		raw, err := json.Marshal(req.Metadata)
		if err != nil {
			return SaaSPaymentWebhookInput{}, errors.New("metadata invalid")
		}
		metadataJSON = string(raw)
	}
	digest := sha256.Sum256(body)
	return SaaSPaymentWebhookInput{
		Provider: provider, EventID: eventID, EventType: eventType, OrderNo: orderNo,
		ProviderOrderNo: providerOrderNo, RefundNo: refundNo, ProviderRefundNo: providerRefundNo,
		AmountCents: req.AmountCents, Currency: currency, PaidAt: paidAt, RefundedAt: refundedAt,
		OccurredAt: occurredAt, FailureCode: failureCode, FailureMessage: failureMessage,
		MetadataJSON: metadataJSON, PayloadJSON: string(body), PayloadSHA256: hex.EncodeToString(digest[:]),
		SignatureTimestamp: signatureTimestamp,
	}, nil
}

func normalizeSaaSPaymentCurrency(raw string) (string, error) {
	currency := strings.ToUpper(strings.TrimSpace(raw))
	if len(currency) != 3 {
		return "", errors.New("currency 必须是 3 位字母")
	}
	for _, char := range currency {
		if char < 'A' || char > 'Z' {
			return "", errors.New("currency 必须是 3 位字母")
		}
	}
	return currency, nil
}

func normalizeSaaSPaymentOptionalTimestamp(raw string, field string) (string, error) {
	value, err := normalizeSaaSAdminOptionalDateTime(raw)
	if errors.Is(err, errSaaSAdminMySQLTimestampRange) {
		return "", fmt.Errorf("%s 超出 MySQL 5.7 TIMESTAMP 范围（1970-01-02 至 2038-01-18）", field)
	}
	if err != nil || value == "" {
		return value, err
	}
	parsed, ok := parseSaaSAdminNormalizedDateTime(value)
	if !ok {
		return "", fmt.Errorf("%s format invalid", field)
	}
	minValue := time.Date(1970, 1, 2, 0, 0, 0, 0, time.Local)
	maxValue := time.Date(2038, 1, 18, 23, 59, 59, 0, time.Local)
	if parsed.Before(minValue) || parsed.After(maxValue) {
		return "", fmt.Errorf("%s 超出 MySQL 5.7 TIMESTAMP 范围（1970-01-02 至 2038-01-18）", field)
	}
	return value, nil
}

func newSaaSPaymentOrderNo() (string, error) {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate payment order number: %w", err)
	}
	return "PAY-" + time.Now().UTC().Format("20060102T150405") + "-" + strings.ToUpper(hex.EncodeToString(random)), nil
}

func saasAdminPaymentOrderReportPayload(report SaaSAdminPaymentOrderReport) map[string]any {
	return map[string]any{
		"filters":       saasAdminPaymentOrderOptionsPayload(report.Options),
		"summary":       saasAdminPaymentOrderSummaryPayload(report.Summary),
		"returnedCount": len(report.Orders),
		"orders":        saasAdminPaymentOrderPayloads(report.Orders),
	}
}

func saasAdminPaymentOrderOptionsPayload(options SaaSAdminPaymentOrderOptions) map[string]any {
	return map[string]any{"tenantId": options.TenantID, "status": options.Status, "provider": options.Provider, "packageCode": options.PackageCode, "keyword": options.Keyword, "limit": options.Limit}
}

func saasAdminPaymentOrderSummaryPayload(summary SaaSAdminPaymentOrderSummary) map[string]any {
	return map[string]any{
		"orderCount": summary.OrderCount, "pendingCount": summary.PendingCount,
		"processingCount": summary.ProcessingCount, "paidCount": summary.PaidCount,
		"failedCount": summary.FailedCount, "canceledCount": summary.CanceledCount,
		"dunningDueCount": summary.DunningDueCount, "collectibleCount": summary.CollectibleCount,
		"orderAmountCents": summary.OrderAmountCents, "paidAmountCents": summary.PaidAmountCents,
		"refundPendingCents": summary.RefundPendingCents, "refundedAmountCents": summary.RefundedAmountCents,
		"netPaidAmountCents":  summary.NetPaidAmountCents,
		"invoicePendingCents": summary.InvoicePendingCents, "invoicedAmountCents": summary.InvoicedAmountCents,
		"creditPendingCents": summary.CreditPendingCents, "creditedAmountCents": summary.CreditedAmountCents,
		"netInvoicedCents": summary.NetInvoicedCents, "invoiceAvailableCents": summary.InvoiceAvailable,
		"creditNoteDueCents": summary.CreditNoteDueCents,
		"outstandingCents":   summary.OutstandingCents, "tenantCount": summary.TenantCount,
		"providerCount": summary.ProviderCount, "packageCount": summary.PackageCount,
	}
}

func saasAdminPaymentOrderPayloads(orders []SaaSAdminPaymentOrder) []map[string]any {
	items := make([]map[string]any, 0, len(orders))
	for _, order := range orders {
		items = append(items, saasAdminPaymentOrderPayload(order))
	}
	return items
}

func saasAdminPaymentOrderPayload(order SaaSAdminPaymentOrder) map[string]any {
	return map[string]any{
		"id": order.ID, "orderNo": order.OrderNo, "tenantId": order.TenantID, "tenantName": order.TenantName,
		"provider": order.Provider, "providerOrderNo": order.ProviderOrderNo, "idempotencyKey": order.IdempotencyKey,
		"status": order.Status, "packageCode": order.PackageCode, "packageName": order.PackageName,
		"packageVersion": order.PackageVersion, "packageLimits": saasAdminPackageLimitsPayload(order.PackageLimits),
		"billingCycle": order.BillingCycle, "serviceExpiresAt": order.ServiceExpiresAt,
		"amountCents": order.AmountCents, "refundPendingCents": order.RefundPendingCents,
		"refundedAmountCents": order.RefundedAmountCents, "refundableAmountCents": order.RefundableAmountCents,
		"refundStatus": order.RefundStatus, "invoicePendingCents": order.InvoicePendingCents,
		"invoicedAmountCents": order.InvoicedAmountCents, "creditPendingCents": order.CreditPendingCents,
		"creditedAmountCents": order.CreditedAmountCents, "netInvoicedCents": order.NetInvoicedCents,
		"invoiceAvailableCents": order.InvoiceAvailableCents, "creditNoteDueCents": order.CreditNoteDueCents,
		"invoiceStatus": order.InvoiceStatus, "currency": order.Currency, "checkoutUrl": order.CheckoutURL,
		"checkoutExpiresAt": order.CheckoutExpiresAt, "attemptCount": order.AttemptCount,
		"dunningAttempts": order.DunningAttempts, "maxDunningAttempts": order.MaxDunningAttempts,
		"nextDunningAt": order.NextDunningAt, "lastDunningAt": order.LastDunningAt,
		"latestWebhookEventId": order.LatestWebhookEventID, "billingEventId": order.BillingEventID, "latestRefundId": order.LatestRefundID,
		"latestInvoiceDocumentId": order.LatestInvoiceID,
		"paidAt":                  order.PaidAt, "failedAt": order.FailedAt, "canceledAt": order.CanceledAt,
		"failureCode": order.FailureCode, "failureMessage": order.FailureMessage,
		"version": order.Version, "createdByUserId": order.CreatedByUserID, "createdByTenantId": order.CreatedByTenantID,
		"remark": order.Remark, "metadataJson": order.MetadataJSON,
		"createdAt": order.CreatedAt, "updatedAt": order.UpdatedAt, "dunningDue": order.DunningDue,
	}
}

func saasAdminPaymentOrderCreateResultPayload(result SaaSAdminPaymentOrderCreateResult) map[string]any {
	return map[string]any{"order": saasAdminPaymentOrderPayload(result.Order), "operationId": result.OperationID, "idempotent": result.Idempotent}
}

func saasAdminPaymentWebhookEventOptionsPayload(options SaaSAdminPaymentWebhookEventOptions) map[string]any {
	return map[string]any{"tenantId": options.TenantID, "provider": options.Provider, "status": options.Status, "eventType": options.EventType, "keyword": options.Keyword, "limit": options.Limit}
}

func saasAdminPaymentWebhookEventPayloads(events []SaaSAdminPaymentWebhookEvent) []map[string]any {
	items := make([]map[string]any, 0, len(events))
	for _, event := range events {
		items = append(items, saasAdminPaymentWebhookEventPayload(event))
	}
	return items
}

func saasAdminPaymentWebhookEventPayload(event SaaSAdminPaymentWebhookEvent) map[string]any {
	return map[string]any{
		"id": event.ID, "provider": event.Provider, "eventId": event.EventID, "eventType": event.EventType,
		"orderId": event.OrderID, "orderNo": event.OrderNo, "providerOrderNo": event.ProviderOrderNo,
		"refundId": event.RefundID, "refundNo": event.RefundNo, "providerRefundNo": event.ProviderRefundNo,
		"status": event.Status, "payloadSha256": event.PayloadSHA256,
		"signatureTimestamp": event.SignatureTimestamp, "attempts": event.Attempts,
		"occurredAt": event.OccurredAt, "processedAt": event.ProcessedAt, "lastError": event.LastError,
		"payloadJson": event.PayloadJSON, "createdAt": event.CreatedAt, "updatedAt": event.UpdatedAt,
	}
}

func saasPaymentWebhookResultPayload(result SaaSPaymentWebhookResult) map[string]any {
	return map[string]any{
		"duplicate": result.Duplicate, "ignored": result.Ignored,
		"billingEventId": result.BillingEventID, "metricsRefreshed": result.MetricsRefreshed,
		"warning": result.Warning, "event": saasAdminPaymentWebhookEventPayload(result.Event),
		"order": saasAdminPaymentOrderPayload(result.Order), "refund": saasAdminPaymentRefundPayload(result.Refund),
	}
}

func saasPaymentDunningResultPayload(result SaaSPaymentDunningResult) map[string]any {
	errorsPayload := make([]map[string]any, 0, len(result.Errors))
	for _, item := range result.Errors {
		errorsPayload = append(errorsPayload, map[string]any{"orderNo": item.OrderNo, "message": item.Message})
	}
	return map[string]any{
		"matchedCount": result.MatchedCount, "enqueuedCount": result.EnqueuedCount,
		"exhaustedCount": result.ExhaustedCount, "skippedCount": result.SkippedCount,
		"failedCount": result.FailedCount, "dryRun": result.DryRun,
		"orderNos": result.OrderNos, "errors": errorsPayload,
	}
}

func writeSaaSAdminPaymentOrderCSV(writer *csv.Writer, report SaaSAdminPaymentOrderReport) {
	_ = writer.Write([]string{
		"orderNo", "tenantId", "tenantName", "provider", "providerOrderNo", "status",
		"packageCode", "packageName", "billingCycle", "serviceExpiresAt", "amountCents", "currency",
		"refundPendingCents", "refundedAmountCents", "refundableAmountCents", "refundStatus", "latestRefundId",
		"invoicePendingCents", "invoicedAmountCents", "creditPendingCents", "creditedAmountCents",
		"netInvoicedCents", "invoiceAvailableCents", "creditNoteDueCents", "invoiceStatus", "latestInvoiceDocumentId",
		"checkoutExpiresAt", "attemptCount", "dunningAttempts", "maxDunningAttempts", "nextDunningAt",
		"billingEventId", "paidAt", "failedAt", "failureCode", "failureMessage", "version", "remark", "createdAt", "updatedAt",
	})
	for _, order := range report.Orders {
		_ = writer.Write([]string{
			order.OrderNo, strconv.Itoa(order.TenantID), order.TenantName, order.Provider, order.ProviderOrderNo,
			order.Status, order.PackageCode, order.PackageName, order.BillingCycle, order.ServiceExpiresAt,
			strconv.FormatInt(order.AmountCents, 10), order.Currency,
			strconv.FormatInt(order.RefundPendingCents, 10), strconv.FormatInt(order.RefundedAmountCents, 10),
			strconv.FormatInt(order.RefundableAmountCents, 10), order.RefundStatus, strconv.FormatInt(order.LatestRefundID, 10),
			strconv.FormatInt(order.InvoicePendingCents, 10), strconv.FormatInt(order.InvoicedAmountCents, 10),
			strconv.FormatInt(order.CreditPendingCents, 10), strconv.FormatInt(order.CreditedAmountCents, 10),
			strconv.FormatInt(order.NetInvoicedCents, 10), strconv.FormatInt(order.InvoiceAvailableCents, 10),
			strconv.FormatInt(order.CreditNoteDueCents, 10), order.InvoiceStatus, strconv.FormatInt(order.LatestInvoiceID, 10),
			order.CheckoutExpiresAt,
			strconv.Itoa(order.AttemptCount), strconv.Itoa(order.DunningAttempts), strconv.Itoa(order.MaxDunningAttempts),
			order.NextDunningAt, strconv.FormatInt(order.BillingEventID, 10), order.PaidAt, order.FailedAt,
			order.FailureCode, order.FailureMessage, strconv.Itoa(order.Version), order.Remark, order.CreatedAt, order.UpdatedAt,
		})
	}
}
