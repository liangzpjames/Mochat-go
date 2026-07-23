package dashboard

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (h *SaaSAdminHandler) InvoiceProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.invoiceStore(w)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		tenantID, err := positiveRequiredQueryInt(r, "tenantId")
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return
		}
		profile, exists, err := store.SaaSBillingProfile(r.Context(), tenantID)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"profile": saasBillingProfilePayload(profile, exists)})
	case http.MethodPost, http.MethodPut:
		update, err := parseSaaSBillingProfileUpdate(r)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return
		}
		if update.TenantID <= 0 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "tenantId 必填", nil)
			return
		}
		update.ActorUserID = user.ID
		update.ActorTenantID = user.TenantID
		result, err := store.SaveSaaSBillingProfile(r.Context(), update)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeEnvelope(w, http.StatusOK, http.StatusOK, "success", saasBillingProfileUpdateResultPayload(result))
	default:
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
	}
}

func (h *SaaSAdminHandler) InvoiceDocuments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.invoiceStore(w)
	if !ok {
		return
	}
	options, err := parseSaaSInvoiceDocumentOptions(r, saasAdminListMaxLimit)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	report, err := store.SaaSInvoiceDocuments(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", saasInvoiceDocumentReportPayload(report))
}

func (h *SaaSAdminHandler) CreateInvoiceDocument(w http.ResponseWriter, r *http.Request) {
	h.createInvoiceDocument(w, r, SaaSInvoiceKindInvoice)
}

func (h *SaaSAdminHandler) CreateCreditNote(w http.ResponseWriter, r *http.Request) {
	h.createInvoiceDocument(w, r, SaaSInvoiceKindCreditNote)
}

func (h *SaaSAdminHandler) createInvoiceDocument(w http.ResponseWriter, r *http.Request, kind string) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.invoiceStore(w)
	if !ok {
		return
	}
	create, err := parseSaaSInvoiceDocumentCreate(r, kind)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if create.TenantID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "tenantId 必填", nil)
		return
	}
	if create.DocumentNo == "" {
		create.DocumentNo, err = newSaaSInvoiceDocumentNo(kind)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
	}
	create.ActorUserID = user.ID
	create.ActorTenantID = user.TenantID
	result, err := store.CreateSaaSInvoiceDocument(r.Context(), create)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", saasInvoiceDocumentCreateResultPayload(result))
}

func (h *SaaSAdminHandler) TransitionInvoiceDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.invoiceStore(w)
	if !ok {
		return
	}
	transition, err := parseSaaSInvoiceDocumentTransition(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if transition.Status == SaaSInvoiceStatusIssued &&
		h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionInvoiceIssue, 0) {
		return
	}
	transition.ActorUserID = user.ID
	transition.ActorTenantID = user.TenantID
	result, err := store.TransitionSaaSInvoiceDocument(r.Context(), transition)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", saasInvoiceDocumentTransitionResultPayload(result))
}

func (h *SaaSAdminHandler) planSaaSInvoiceIssue(ctx context.Context, transition SaaSInvoiceDocumentTransition) (SaaSInvoiceIssueApprovalPlan, error) {
	if transition.Status != SaaSInvoiceStatusIssued {
		return SaaSInvoiceIssueApprovalPlan{}, NewSaaSAdminBadRequest("开具审批只支持 issued 状态")
	}
	store, ok := h.store.(SaaSInvoiceStore)
	if !ok || store == nil {
		return SaaSInvoiceIssueApprovalPlan{}, errors.New("invoice store is not configured")
	}
	report, err := store.SaaSInvoiceDocuments(ctx, SaaSInvoiceDocumentOptions{
		DocumentNo: transition.DocumentNo,
		Kind:       SaaSInvoiceKindAll,
		Status:     SaaSInvoiceStatusAll,
		Limit:      2,
	})
	if err != nil {
		return SaaSInvoiceIssueApprovalPlan{}, err
	}
	var current SaaSInvoiceDocument
	found := false
	for _, item := range report.Documents {
		if item.DocumentNo == transition.DocumentNo {
			if found {
				return SaaSInvoiceIssueApprovalPlan{}, &SaaSAdminOperationError{
					Status: http.StatusConflict, Message: "发票单据编号不唯一，请先修复数据",
				}
			}
			current, found = item, true
		}
	}
	if !found {
		return SaaSInvoiceIssueApprovalPlan{}, NewSaaSAdminNotFound("invoice document not found")
	}
	if transition.ExpectedVersion != current.Version {
		return SaaSInvoiceIssueApprovalPlan{}, &SaaSAdminOperationError{
			Status: http.StatusConflict, Message: fmt.Sprintf("发票单据版本已变化，请刷新后重试: current=%d", current.Version),
		}
	}
	if !SaaSInvoiceTransitionAllowed(current.Status, transition.Status) || current.Status == transition.Status {
		return SaaSInvoiceIssueApprovalPlan{}, &SaaSAdminOperationError{
			Status: http.StatusConflict, Message: "发票单据状态不允许申请开具审批: " + current.Status + " -> " + transition.Status,
		}
	}
	if current.ID <= 0 || current.TenantID <= 0 || current.PaymentOrderID <= 0 ||
		current.AmountCents <= 0 || current.Version <= 0 || current.OrderVersion <= 0 ||
		current.OrderNo == "" || current.OrderStatus != SaaSPaymentOrderStatusPaid ||
		!SaaSInvoiceKindValid(current.Kind) {
		return SaaSInvoiceIssueApprovalPlan{}, &SaaSAdminOperationError{
			Status: http.StatusConflict, Message: "发票或支付订单快照不完整，请刷新后重试",
		}
	}
	if current.Kind == SaaSInvoiceKindInvoice {
		if current.OrderInvoicePending < current.AmountCents {
			return SaaSInvoiceIssueApprovalPlan{}, &SaaSAdminOperationError{
				Status: http.StatusConflict, Message: "蓝票预占金额不足，不能申请开具",
			}
		}
	} else {
		if current.OriginalDocumentID <= 0 || current.OriginalDocumentNo == "" ||
			current.OrderCreditPending < current.AmountCents ||
			current.OrderRefundedCents < current.OrderCreditedCents+current.AmountCents ||
			current.OrderInvoicedCents < current.OrderCreditedCents+current.AmountCents {
			return SaaSInvoiceIssueApprovalPlan{}, &SaaSAdminOperationError{
				Status: http.StatusConflict, Message: "红票关联或红冲预占已变化，不能申请开具",
			}
		}
	}
	transition.ExpectedVersion = current.Version
	return SaaSInvoiceIssueApprovalPlan{
		Transition: transition,
		Document: SaaSInvoiceIssueDocumentSnapshot{
			ID: current.ID, DocumentNo: current.DocumentNo, TenantID: current.TenantID, TenantName: current.TenantName,
			PaymentOrderID: current.PaymentOrderID, OrderNo: current.OrderNo, Kind: current.Kind,
			OriginalDocumentID: current.OriginalDocumentID, OriginalDocumentNo: current.OriginalDocumentNo,
			Status: current.Status, AmountCents: current.AmountCents, Currency: current.Currency,
			InvoiceType: current.InvoiceType, InvoiceTitle: current.InvoiceTitle,
			TaxIdentifier: current.TaxIdentifier, Provider: current.Provider, Version: current.Version,
		},
		Order: SaaSInvoiceIssueOrderSnapshot{
			ID: current.PaymentOrderID, OrderNo: current.OrderNo, TenantID: current.TenantID, Status: current.OrderStatus,
			AmountCents: current.OrderAmountCents, RefundPendingCents: current.OrderRefundPending,
			RefundedAmountCents: current.OrderRefundedCents, InvoicePendingCents: current.OrderInvoicePending,
			InvoicedAmountCents: current.OrderInvoicedCents, CreditPendingCents: current.OrderCreditPending,
			CreditedAmountCents: current.OrderCreditedCents, Version: current.OrderVersion,
		},
	}, nil
}

func (h *SaaSAdminHandler) invoiceStore(w http.ResponseWriter) (SaaSInvoiceStore, bool) {
	store, ok := h.store.(SaaSInvoiceStore)
	if !ok || store == nil {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "invoice store is not configured", nil)
		return nil, false
	}
	return store, true
}

func parseSaaSBillingProfileUpdate(r *http.Request) (SaaSBillingProfileUpdate, error) {
	params, err := parseRequestParams(r)
	if err != nil {
		return SaaSBillingProfileUpdate{}, errors.New("请求格式错误")
	}
	tenantID, _, err := intParam(params, "tenantId")
	if err != nil {
		return SaaSBillingProfileUpdate{}, errors.New("tenantId 格式错误")
	}
	expectedVersion, _, err := intParam(params, "expectedVersion")
	if err != nil {
		return SaaSBillingProfileUpdate{}, errors.New("expectedVersion 格式错误")
	}
	update := SaaSBillingProfileUpdate{
		TenantID: tenantID, InvoiceType: strings.ToLower(strings.TrimSpace(stringParam(params, "invoiceType"))),
		InvoiceTitle:  strings.TrimSpace(stringParam(params, "invoiceTitle")),
		TaxIdentifier: strings.ToUpper(strings.TrimSpace(stringParam(params, "taxIdentifier"))),
		Email:         strings.TrimSpace(stringParam(params, "email")), Phone: strings.TrimSpace(stringParam(params, "phone")),
		RegisteredAddress: strings.TrimSpace(stringParam(params, "registeredAddress")),
		BankName:          strings.TrimSpace(stringParam(params, "bankName")), BankAccount: strings.TrimSpace(stringParam(params, "bankAccount")),
		RecipientName: strings.TrimSpace(stringParam(params, "recipientName")), ExpectedVersion: expectedVersion,
		Remark: strings.TrimSpace(stringParam(params, "remark")),
	}
	if update.InvoiceType == "" {
		update.InvoiceType = SaaSInvoiceTypeNormal
	}
	if err := validateSaaSBillingProfileUpdate(update); err != nil {
		return SaaSBillingProfileUpdate{}, err
	}
	return update, nil
}

func parseSaaSInvoiceDocumentCreate(r *http.Request, kind string) (SaaSInvoiceDocumentCreate, error) {
	params, err := parseRequestParams(r)
	if err != nil {
		return SaaSInvoiceDocumentCreate{}, errors.New("请求格式错误")
	}
	tenantID, _, err := intParam(params, "tenantId")
	if err != nil {
		return SaaSInvoiceDocumentCreate{}, errors.New("tenantId 格式错误")
	}
	amountCents, err := parseSaaSInvoiceAmount(params)
	if err != nil {
		return SaaSInvoiceDocumentCreate{}, err
	}
	currency := strings.ToUpper(strings.TrimSpace(stringParam(params, "currency")))
	if currency != "" {
		currency, err = normalizeSaaSPaymentCurrency(currency)
		if err != nil {
			return SaaSInvoiceDocumentCreate{}, err
		}
	}
	metadataJSON := ""
	if metadata, ok := params["metadata"].(map[string]any); ok {
		raw, err := json.Marshal(metadata)
		if err != nil {
			return SaaSInvoiceDocumentCreate{}, errors.New("metadata 格式错误")
		}
		metadataJSON = string(raw)
	}
	create := SaaSInvoiceDocumentCreate{
		DocumentNo: strings.TrimSpace(stringParam(params, "documentNo")), TenantID: tenantID,
		OrderNo: strings.TrimSpace(stringParam(params, "orderNo")), Kind: kind,
		OriginalDocumentNo: strings.TrimSpace(stringParam(params, "originalDocumentNo")),
		IdempotencyKey:     strings.TrimSpace(stringParam(params, "idempotencyKey")), AmountCents: amountCents,
		Currency: currency, Provider: strings.TrimSpace(stringParam(params, "provider")),
		Remark: strings.TrimSpace(stringParam(params, "remark")), MetadataJSON: metadataJSON,
	}
	if err := validateSaaSInvoiceDocumentCreate(create); err != nil {
		return SaaSInvoiceDocumentCreate{}, err
	}
	return create, nil
}

func parseSaaSInvoiceDocumentTransition(r *http.Request) (SaaSInvoiceDocumentTransition, error) {
	params, err := parseRequestParams(r)
	if err != nil {
		return SaaSInvoiceDocumentTransition{}, errors.New("请求格式错误")
	}
	expectedVersion, _, err := intParam(params, "expectedVersion")
	if err != nil {
		return SaaSInvoiceDocumentTransition{}, errors.New("expectedVersion 格式错误")
	}
	transition := SaaSInvoiceDocumentTransition{
		DocumentNo: strings.TrimSpace(stringParam(params, "documentNo")), ExpectedVersion: expectedVersion,
		Status:             strings.ToLower(strings.TrimSpace(stringParam(params, "status"))),
		Provider:           strings.TrimSpace(stringParam(params, "provider")),
		ProviderDocumentNo: strings.TrimSpace(stringParam(params, "providerDocumentNo")),
		DocumentURL:        strings.TrimSpace(stringParam(params, "documentUrl")),
		IssuedAt:           strings.TrimSpace(stringParam(params, "issuedAt")),
		FailureCode:        strings.TrimSpace(stringParam(params, "failureCode")),
		FailureMessage:     strings.TrimSpace(stringParam(params, "failureMessage")),
		Remark:             strings.TrimSpace(stringParam(params, "remark")),
	}
	if err := validateSaaSInvoiceDocumentTransition(transition); err != nil {
		return SaaSInvoiceDocumentTransition{}, err
	}
	return transition, nil
}

func parseSaaSInvoiceDocumentOptions(r *http.Request, maxLimit int) (SaaSInvoiceDocumentOptions, error) {
	query := r.URL.Query()
	options := SaaSInvoiceDocumentOptions{
		Kind: strings.ToLower(strings.TrimSpace(query.Get("kind"))), Status: strings.ToLower(strings.TrimSpace(query.Get("status"))),
		OrderNo: strings.TrimSpace(query.Get("orderNo")), DocumentNo: strings.TrimSpace(query.Get("documentNo")),
		Keyword: strings.TrimSpace(query.Get("keyword")),
		Limit:   positiveQueryInt(r, "limit", 100),
	}
	if raw := strings.TrimSpace(query.Get("tenantId")); raw != "" {
		tenantID, err := strconv.Atoi(raw)
		if err != nil || tenantID <= 0 {
			return SaaSInvoiceDocumentOptions{}, errors.New("tenantId 格式错误")
		}
		options.TenantID = tenantID
	}
	if options.Kind == "" {
		options.Kind = SaaSInvoiceKindAll
	}
	if options.Kind != SaaSInvoiceKindAll && !SaaSInvoiceKindValid(options.Kind) {
		return SaaSInvoiceDocumentOptions{}, errors.New("kind 格式错误")
	}
	if options.Status == "" {
		options.Status = SaaSInvoiceStatusAll
	}
	if options.Status != SaaSInvoiceStatusAll && !SaaSInvoiceStatusValid(options.Status) {
		return SaaSInvoiceDocumentOptions{}, errors.New("status 格式错误")
	}
	if options.OrderNo != "" && (len(options.OrderNo) > 64 || !saasPaymentIdentifierPattern.MatchString(options.OrderNo)) {
		return SaaSInvoiceDocumentOptions{}, errors.New("orderNo 格式错误")
	}
	if options.DocumentNo != "" && (len(options.DocumentNo) > 64 || !saasPaymentIdentifierPattern.MatchString(options.DocumentNo)) {
		return SaaSInvoiceDocumentOptions{}, errors.New("documentNo 格式错误")
	}
	if len([]rune(options.Keyword)) > 128 {
		return SaaSInvoiceDocumentOptions{}, errors.New("keyword 过长")
	}
	if maxLimit <= 0 {
		maxLimit = saasAdminListMaxLimit
	}
	if options.Limit > maxLimit {
		options.Limit = maxLimit
	}
	return options, nil
}

func positiveRequiredQueryInt(r *http.Request, key string) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, errors.New(key + " 必须为正整数")
	}
	return value, nil
}

func saasBillingProfileUpdateResultPayload(result SaaSBillingProfileUpdateResult) map[string]any {
	return map[string]any{"profile": saasBillingProfilePayload(result.Profile, true), "operationId": result.OperationID, "created": result.Created}
}

func saasInvoiceDocumentReportPayload(report SaaSInvoiceDocumentReport) map[string]any {
	return map[string]any{
		"filters": map[string]any{"tenantId": report.Options.TenantID, "kind": report.Options.Kind, "status": report.Options.Status, "orderNo": report.Options.OrderNo, "documentNo": report.Options.DocumentNo, "keyword": report.Options.Keyword, "limit": report.Options.Limit},
		"summary": saasInvoiceDocumentSummaryPayload(report.Summary), "returnedCount": len(report.Documents),
		"documents": saasInvoiceDocumentPayloads(report.Documents),
	}
}

func saasInvoiceDocumentCreateResultPayload(result SaaSInvoiceDocumentCreateResult) map[string]any {
	return map[string]any{"document": saasInvoiceDocumentPayload(result.Document), "order": saasAdminPaymentOrderPayload(result.Order), "operationId": result.OperationID, "idempotent": result.Idempotent}
}

func saasInvoiceDocumentTransitionResultPayload(result SaaSInvoiceDocumentTransitionResult) map[string]any {
	return map[string]any{"document": saasInvoiceDocumentPayload(result.Document), "order": saasAdminPaymentOrderPayload(result.Order), "previousStatus": result.PreviousStatus, "operationId": result.OperationID}
}

func writeSaaSInvoiceDocumentsCSV(writer *csv.Writer, report SaaSInvoiceDocumentReport) {
	_ = writer.Write([]string{
		"documentNo", "kind", "status", "tenantId", "tenantName", "orderNo", "originalDocumentNo",
		"amountCents", "currency", "invoiceType", "invoiceTitle", "taxIdentifier", "email", "phone",
		"provider", "providerDocumentNo", "documentUrl", "requestedAt", "processingAt", "issuedAt",
		"failedAt", "canceledAt", "failureCode", "failureMessage", "version", "remark",
		"orderNetPaidCents", "orderNetInvoicedCents", "orderInvoiceAvailableCents", "orderCreditNoteDueCents",
	})
	for _, item := range report.Documents {
		_ = writer.Write([]string{
			item.DocumentNo, item.Kind, item.Status, strconv.Itoa(item.TenantID), item.TenantName, item.OrderNo,
			item.OriginalDocumentNo, strconv.FormatInt(item.AmountCents, 10), item.Currency, item.InvoiceType,
			item.InvoiceTitle, item.TaxIdentifier, item.Email, item.Phone, item.Provider, item.ProviderDocumentNo,
			item.DocumentURL, item.RequestedAt, item.ProcessingAt, item.IssuedAt, item.FailedAt, item.CanceledAt,
			item.FailureCode, item.FailureMessage, strconv.Itoa(item.Version), item.Remark,
			strconv.FormatInt(item.OrderNetPaidCents, 10), strconv.FormatInt(item.OrderNetInvoicedCents, 10),
			strconv.FormatInt(item.OrderInvoiceAvailable, 10), strconv.FormatInt(item.OrderCreditNoteDue, 10),
		})
	}
}
