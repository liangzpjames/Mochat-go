package dashboard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	SaaSInvoiceKindAll        = "all"
	SaaSInvoiceKindInvoice    = "invoice"
	SaaSInvoiceKindCreditNote = "credit_note"

	SaaSInvoiceStatusAll        = "all"
	SaaSInvoiceStatusRequested  = "requested"
	SaaSInvoiceStatusProcessing = "processing"
	SaaSInvoiceStatusIssued     = "issued"
	SaaSInvoiceStatusFailed     = "failed"
	SaaSInvoiceStatusCanceled   = "canceled"

	SaaSInvoiceTypeNormal  = "normal"
	SaaSInvoiceTypeSpecial = "special"

	SaaSAdminExportKindInvoiceDocuments = "invoiceDocuments"

	SaaSAdminOperationActionBillingProfileUpdate = "billing.profile.update"
	SaaSAdminOperationActionInvoiceRequest       = "billing.invoice.request"
	SaaSAdminOperationActionInvoiceTransition    = "billing.invoice.transition"
	SaaSAdminOperationActionCreditNoteRequest    = "billing.credit_note.request"
	SaaSAdminOperationTargetBillingProfile       = "billing_profile"
	SaaSAdminOperationTargetInvoiceDocument      = "invoice_document"
)

var saasInvoiceTaxIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9-]{8,64}$`)

type SaaSBillingProfile struct {
	ID                int64
	TenantID          int
	TenantName        string
	InvoiceType       string
	InvoiceTitle      string
	TaxIdentifier     string
	Email             string
	Phone             string
	RegisteredAddress string
	BankName          string
	BankAccount       string
	RecipientName     string
	Status            int
	Version           int
	UpdatedByUserID   int
	UpdatedByTenantID int
	Remark            string
	CreatedAt         string
	UpdatedAt         string
}

type SaaSBillingProfileUpdate struct {
	TenantID          int
	InvoiceType       string
	InvoiceTitle      string
	TaxIdentifier     string
	Email             string
	Phone             string
	RegisteredAddress string
	BankName          string
	BankAccount       string
	RecipientName     string
	ExpectedVersion   int
	Remark            string
	ActorUserID       int
	ActorTenantID     int
}

type SaaSBillingProfileUpdateResult struct {
	Profile     SaaSBillingProfile
	OperationID int64
	Created     bool
}

type SaaSInvoiceDocumentOptions struct {
	TenantID   int
	Kind       string
	Status     string
	OrderNo    string
	DocumentNo string
	Keyword    string
	Limit      int
}

type SaaSInvoiceDocument struct {
	ID                    int64
	DocumentNo            string
	TenantID              int
	TenantName            string
	PaymentOrderID        int64
	OrderNo               string
	OrderStatus           string
	OrderVersion          int
	Kind                  string
	OriginalDocumentID    int64
	OriginalDocumentNo    string
	IdempotencyKey        string
	Status                string
	AmountCents           int64
	Currency              string
	InvoiceType           string
	InvoiceTitle          string
	TaxIdentifier         string
	Email                 string
	Phone                 string
	RegisteredAddress     string
	BankName              string
	BankAccount           string
	RecipientName         string
	Provider              string
	ProviderDocumentNo    string
	DocumentURL           string
	OperationID           int64
	RequestedByUserID     int
	RequestedByTenantID   int
	ProcessedByUserID     int
	ProcessedByTenantID   int
	RequestedAt           string
	ProcessingAt          string
	IssuedAt              string
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
	OrderRefundPending    int64
	OrderRefundedCents    int64
	OrderNetPaidCents     int64
	OrderInvoicePending   int64
	OrderInvoicedCents    int64
	OrderCreditPending    int64
	OrderCreditedCents    int64
	OrderNetInvoicedCents int64
	OrderInvoiceAvailable int64
	OrderCreditNoteDue    int64
}

type SaaSInvoiceDocumentSummary struct {
	DocumentCount       int
	InvoiceCount        int
	CreditNoteCount     int
	RequestedCount      int
	ProcessingCount     int
	IssuedCount         int
	FailedCount         int
	CanceledCount       int
	RequestedAmount     int64
	IssuedInvoiceAmount int64
	IssuedCreditAmount  int64
	NetIssuedAmount     int64
	TenantCount         int
	OrderCount          int
}

type SaaSInvoiceDocumentReport struct {
	Options   SaaSInvoiceDocumentOptions
	Summary   SaaSInvoiceDocumentSummary
	Documents []SaaSInvoiceDocument
}

type SaaSInvoiceDocumentCreate struct {
	DocumentNo         string
	TenantID           int
	OrderNo            string
	Kind               string
	OriginalDocumentNo string
	IdempotencyKey     string
	AmountCents        int64
	Currency           string
	Provider           string
	Remark             string
	MetadataJSON       string
	ActorUserID        int
	ActorTenantID      int
}

type SaaSInvoiceDocumentCreateResult struct {
	Document    SaaSInvoiceDocument
	Order       SaaSAdminPaymentOrder
	OperationID int64
	Idempotent  bool
}

type SaaSInvoiceDocumentTransition struct {
	DocumentNo               string
	TenantID                 int
	ExpectedVersion          int
	Status                   string
	Provider                 string
	ProviderDocumentNo       string
	DocumentURL              string
	IssuedAt                 string
	FailureCode              string
	FailureMessage           string
	Remark                   string
	ActorUserID              int
	ActorTenantID            int
	ApprovalPlan             *SaaSInvoiceIssueApprovalPlan `json:"-"`
	ApprovalExecutionID      int64                         `json:"-"`
	ApprovalExecutionVersion int                           `json:"-"`
}

type SaaSInvoiceDocumentTransitionResult struct {
	Document       SaaSInvoiceDocument
	Order          SaaSAdminPaymentOrder
	PreviousStatus string
	OperationID    int64
}

type SaaSInvoiceIssueDocumentSnapshot struct {
	ID                 int64  `json:"id"`
	DocumentNo         string `json:"documentNo"`
	TenantID           int    `json:"tenantId"`
	TenantName         string `json:"tenantName"`
	PaymentOrderID     int64  `json:"paymentOrderId"`
	OrderNo            string `json:"orderNo"`
	Kind               string `json:"kind"`
	OriginalDocumentID int64  `json:"originalDocumentId"`
	OriginalDocumentNo string `json:"originalDocumentNo"`
	Status             string `json:"status"`
	AmountCents        int64  `json:"amountCents"`
	Currency           string `json:"currency"`
	InvoiceType        string `json:"invoiceType"`
	InvoiceTitle       string `json:"invoiceTitle"`
	TaxIdentifier      string `json:"taxIdentifier"`
	Provider           string `json:"provider"`
	Version            int    `json:"version"`
}

type SaaSInvoiceIssueOrderSnapshot struct {
	ID                  int64  `json:"id"`
	OrderNo             string `json:"orderNo"`
	TenantID            int    `json:"tenantId"`
	Status              string `json:"status"`
	AmountCents         int64  `json:"amountCents"`
	RefundPendingCents  int64  `json:"refundPendingCents"`
	RefundedAmountCents int64  `json:"refundedAmountCents"`
	InvoicePendingCents int64  `json:"invoicePendingCents"`
	InvoicedAmountCents int64  `json:"invoicedAmountCents"`
	CreditPendingCents  int64  `json:"creditPendingCents"`
	CreditedAmountCents int64  `json:"creditedAmountCents"`
	Version             int    `json:"version"`
}

type SaaSInvoiceIssueApprovalPlan struct {
	Transition SaaSInvoiceDocumentTransition    `json:"transition"`
	Document   SaaSInvoiceIssueDocumentSnapshot `json:"document"`
	Order      SaaSInvoiceIssueOrderSnapshot    `json:"order"`
}

type SaaSInvoiceStore interface {
	SaaSBillingProfile(ctx context.Context, tenantID int) (SaaSBillingProfile, bool, error)
	SaveSaaSBillingProfile(ctx context.Context, update SaaSBillingProfileUpdate) (SaaSBillingProfileUpdateResult, error)
	SaaSInvoiceDocuments(ctx context.Context, options SaaSInvoiceDocumentOptions) (SaaSInvoiceDocumentReport, error)
	CreateSaaSInvoiceDocument(ctx context.Context, create SaaSInvoiceDocumentCreate) (SaaSInvoiceDocumentCreateResult, error)
	TransitionSaaSInvoiceDocument(ctx context.Context, transition SaaSInvoiceDocumentTransition) (SaaSInvoiceDocumentTransitionResult, error)
}

func SaaSInvoiceKindValid(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case SaaSInvoiceKindInvoice, SaaSInvoiceKindCreditNote:
		return true
	default:
		return false
	}
}

func SaaSInvoiceStatusValid(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case SaaSInvoiceStatusRequested, SaaSInvoiceStatusProcessing, SaaSInvoiceStatusIssued,
		SaaSInvoiceStatusFailed, SaaSInvoiceStatusCanceled:
		return true
	default:
		return false
	}
}

func SaaSInvoiceTypeValid(invoiceType string) bool {
	switch strings.ToLower(strings.TrimSpace(invoiceType)) {
	case SaaSInvoiceTypeNormal, SaaSInvoiceTypeSpecial:
		return true
	default:
		return false
	}
}

func SaaSInvoiceTransitionAllowed(from string, to string) bool {
	from = strings.ToLower(strings.TrimSpace(from))
	to = strings.ToLower(strings.TrimSpace(to))
	if from == to {
		return true
	}
	switch from {
	case SaaSInvoiceStatusRequested, SaaSInvoiceStatusProcessing:
		return to == SaaSInvoiceStatusProcessing || to == SaaSInvoiceStatusIssued ||
			to == SaaSInvoiceStatusFailed || to == SaaSInvoiceStatusCanceled
	default:
		return false
	}
}

func validateSaaSBillingProfileUpdate(update SaaSBillingProfileUpdate) error {
	update.InvoiceType = strings.ToLower(strings.TrimSpace(update.InvoiceType))
	if !SaaSInvoiceTypeValid(update.InvoiceType) {
		return errors.New("invoiceType 必须是 normal 或 special")
	}
	if strings.TrimSpace(update.InvoiceTitle) == "" || len([]rune(update.InvoiceTitle)) > 255 {
		return errors.New("invoiceTitle 必填且不能超过 255 个字符")
	}
	taxID := strings.TrimSpace(update.TaxIdentifier)
	if !saasInvoiceTaxIdentifierPattern.MatchString(taxID) {
		return errors.New("taxIdentifier 格式错误")
	}
	address, err := mail.ParseAddress(strings.TrimSpace(update.Email))
	if err != nil || !strings.EqualFold(address.Address, strings.TrimSpace(update.Email)) {
		return errors.New("email 格式错误")
	}
	for name, spec := range map[string]struct {
		value string
		limit int
	}{
		"phone": {update.Phone, 32}, "registeredAddress": {update.RegisteredAddress, 255},
		"bankName": {update.BankName, 255}, "bankAccount": {update.BankAccount, 128},
		"recipientName": {update.RecipientName, 128}, "remark": {update.Remark, 255},
	} {
		if len([]rune(spec.value)) > spec.limit {
			return fmt.Errorf("%s 不能超过 %d 个字符", name, spec.limit)
		}
	}
	if update.InvoiceType == SaaSInvoiceTypeSpecial &&
		(strings.TrimSpace(update.RegisteredAddress) == "" || strings.TrimSpace(update.BankName) == "" || strings.TrimSpace(update.BankAccount) == "") {
		return errors.New("special 发票必须填写注册地址、开户行和银行账号")
	}
	return nil
}

func validateSaaSInvoiceDocumentCreate(create SaaSInvoiceDocumentCreate) error {
	if strings.TrimSpace(create.OrderNo) == "" || len(create.OrderNo) > 64 || !saasPaymentIdentifierPattern.MatchString(strings.TrimSpace(create.OrderNo)) {
		return errors.New("orderNo 格式错误")
	}
	if !SaaSInvoiceKindValid(create.Kind) {
		return errors.New("kind 必须是 invoice 或 credit_note")
	}
	if create.Kind == SaaSInvoiceKindCreditNote && strings.TrimSpace(create.OriginalDocumentNo) == "" {
		return errors.New("credit_note 必须指定 originalDocumentNo")
	}
	if create.AmountCents <= 0 {
		return errors.New("amountCents 必须为正整数")
	}
	if strings.TrimSpace(create.Currency) != "" {
		if _, err := normalizeSaaSPaymentCurrency(create.Currency); err != nil {
			return err
		}
	}
	if len(create.DocumentNo) > 64 || (create.DocumentNo != "" && !saasPaymentIdentifierPattern.MatchString(create.DocumentNo)) {
		return errors.New("documentNo 格式错误")
	}
	if len(create.IdempotencyKey) > 128 || len(create.Provider) > 32 || len([]rune(create.Remark)) > 255 {
		return errors.New("idempotencyKey、provider 或 remark 过长")
	}
	return nil
}

func validateSaaSInvoiceDocumentTransition(transition SaaSInvoiceDocumentTransition) error {
	if strings.TrimSpace(transition.DocumentNo) == "" || len(transition.DocumentNo) > 64 || !saasPaymentIdentifierPattern.MatchString(strings.TrimSpace(transition.DocumentNo)) {
		return errors.New("documentNo 格式错误")
	}
	if transition.ExpectedVersion <= 0 {
		return errors.New("expectedVersion 必填")
	}
	if !SaaSInvoiceStatusValid(transition.Status) || transition.Status == SaaSInvoiceStatusRequested {
		return errors.New("status 必须是 processing、issued、failed 或 canceled")
	}
	if len(transition.Provider) > 32 || len(transition.ProviderDocumentNo) > 128 || len(transition.FailureCode) > 64 || len([]rune(transition.FailureMessage)) > 255 || len([]rune(transition.Remark)) > 255 {
		return errors.New("发票处理字段过长")
	}
	if transition.Status == SaaSInvoiceStatusIssued && strings.TrimSpace(transition.ProviderDocumentNo) == "" {
		return errors.New("issued 状态必须填写 providerDocumentNo")
	}
	if transition.Status == SaaSInvoiceStatusFailed && strings.TrimSpace(transition.FailureMessage) == "" {
		return errors.New("failed 状态必须填写 failureMessage")
	}
	if strings.TrimSpace(transition.DocumentURL) != "" {
		parsed, err := url.ParseRequestURI(strings.TrimSpace(transition.DocumentURL))
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return errors.New("documentUrl 必须是 HTTPS URL")
		}
	}
	if strings.TrimSpace(transition.IssuedAt) != "" {
		if _, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(transition.IssuedAt), time.Local); err != nil {
			return errors.New("issuedAt 格式必须是 YYYY-MM-DD HH:MM:SS")
		}
	}
	return nil
}

func newSaaSInvoiceDocumentNo(kind string) (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	prefix := "INV"
	if kind == SaaSInvoiceKindCreditNote {
		prefix = "CRN"
	}
	return prefix + "-" + time.Now().Format("20060102150405") + "-" + strings.ToUpper(hex.EncodeToString(buf)), nil
}

func parseSaaSInvoiceAmount(params map[string]any) (int64, error) {
	if raw := strings.TrimSpace(stringParam(params, "amount")); raw != "" {
		return parseSaaSAdminAmountCents(raw)
	}
	value := int64Param(params, "amountCents")
	if value <= 0 {
		return 0, errors.New("amountCents 必须为正整数")
	}
	return value, nil
}

func int64Param(params map[string]any, key string) int64 {
	value, ok := params[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return parsed
	default:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(fmt.Sprint(value)), 10, 64)
		return parsed
	}
}

func saasBillingProfilePayload(profile SaaSBillingProfile, exists bool) map[string]any {
	return map[string]any{
		"exists": exists, "id": profile.ID, "tenantId": profile.TenantID, "tenantName": profile.TenantName,
		"invoiceType": profile.InvoiceType, "invoiceTitle": profile.InvoiceTitle,
		"taxIdentifier": profile.TaxIdentifier, "email": profile.Email, "phone": profile.Phone,
		"registeredAddress": profile.RegisteredAddress, "bankName": profile.BankName,
		"bankAccount": profile.BankAccount, "recipientName": profile.RecipientName,
		"status": profile.Status, "version": profile.Version, "updatedByUserId": profile.UpdatedByUserID,
		"updatedByTenantId": profile.UpdatedByTenantID, "remark": profile.Remark,
		"createdAt": profile.CreatedAt, "updatedAt": profile.UpdatedAt,
	}
}

func saasInvoiceDocumentPayload(item SaaSInvoiceDocument) map[string]any {
	return map[string]any{
		"id": item.ID, "documentNo": item.DocumentNo, "tenantId": item.TenantID, "tenantName": item.TenantName,
		"paymentOrderId": item.PaymentOrderID, "orderNo": item.OrderNo,
		"orderStatus": item.OrderStatus, "orderVersion": item.OrderVersion, "kind": item.Kind,
		"originalDocumentId": item.OriginalDocumentID, "originalDocumentNo": item.OriginalDocumentNo,
		"idempotencyKey": item.IdempotencyKey, "status": item.Status, "amountCents": item.AmountCents,
		"currency": item.Currency, "invoiceType": item.InvoiceType, "invoiceTitle": item.InvoiceTitle,
		"taxIdentifier": item.TaxIdentifier, "email": item.Email, "phone": item.Phone,
		"registeredAddress": item.RegisteredAddress, "bankName": item.BankName,
		"bankAccount": item.BankAccount, "recipientName": item.RecipientName,
		"provider": item.Provider, "providerDocumentNo": item.ProviderDocumentNo, "documentUrl": item.DocumentURL,
		"operationId": item.OperationID, "requestedByUserId": item.RequestedByUserID,
		"requestedByTenantId": item.RequestedByTenantID, "processedByUserId": item.ProcessedByUserID,
		"processedByTenantId": item.ProcessedByTenantID, "requestedAt": item.RequestedAt,
		"processingAt": item.ProcessingAt, "issuedAt": item.IssuedAt, "failedAt": item.FailedAt,
		"canceledAt": item.CanceledAt, "failureCode": item.FailureCode, "failureMessage": item.FailureMessage,
		"version": item.Version, "remark": item.Remark, "metadataJson": item.MetadataJSON,
		"createdAt": item.CreatedAt, "updatedAt": item.UpdatedAt,
		"orderAmountCents": item.OrderAmountCents, "orderRefundPendingCents": item.OrderRefundPending,
		"orderRefundedCents": item.OrderRefundedCents, "orderNetPaidCents": item.OrderNetPaidCents,
		"orderInvoicePendingCents": item.OrderInvoicePending, "orderInvoicedCents": item.OrderInvoicedCents,
		"orderCreditPendingCents": item.OrderCreditPending, "orderCreditedCents": item.OrderCreditedCents,
		"orderNetInvoicedCents": item.OrderNetInvoicedCents, "orderInvoiceAvailableCents": item.OrderInvoiceAvailable,
		"orderCreditNoteDueCents": item.OrderCreditNoteDue,
	}
}

func saasInvoiceDocumentPayloads(items []SaaSInvoiceDocument) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, saasInvoiceDocumentPayload(item))
	}
	return payloads
}

func saasInvoiceDocumentSummaryPayload(summary SaaSInvoiceDocumentSummary) map[string]any {
	return map[string]any{
		"documentCount": summary.DocumentCount, "invoiceCount": summary.InvoiceCount,
		"creditNoteCount": summary.CreditNoteCount, "requestedCount": summary.RequestedCount,
		"processingCount": summary.ProcessingCount, "issuedCount": summary.IssuedCount,
		"failedCount": summary.FailedCount, "canceledCount": summary.CanceledCount,
		"requestedAmountCents": summary.RequestedAmount, "issuedInvoiceAmountCents": summary.IssuedInvoiceAmount,
		"issuedCreditAmountCents": summary.IssuedCreditAmount, "netIssuedAmountCents": summary.NetIssuedAmount,
		"tenantCount": summary.TenantCount, "orderCount": summary.OrderCount,
	}
}
