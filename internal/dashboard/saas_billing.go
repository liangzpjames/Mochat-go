package dashboard

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"jiyi/mochat-go/internal/saasauth"
)

type SaaSBillingStore interface {
	SaaSInvoiceStore
	UserByID(ctx context.Context, userID int) (User, bool, error)
	SaaSAdminPaymentOrders(ctx context.Context, options SaaSAdminPaymentOrderOptions) (SaaSAdminPaymentOrderReport, error)
	SaaSAdminPaymentRefunds(ctx context.Context, options SaaSAdminPaymentRefundOptions) (SaaSAdminPaymentRefundReport, error)
}

type SaaSBillingHandler struct {
	store                 SaaSBillingStore
	resolver              UserIDResolver
	platformAdminTenantID int
}

func NewSaaSBillingHandler(store SaaSBillingStore, resolver UserIDResolver, platformTenantID ...int) *SaaSBillingHandler {
	platformID := 1
	if len(platformTenantID) > 0 && platformTenantID[0] > 0 {
		platformID = platformTenantID[0]
	}
	return &SaaSBillingHandler{store: store, resolver: resolver, platformAdminTenantID: platformID}
}

func (h *SaaSBillingHandler) Summary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	profile, exists, err := h.store.SaaSBillingProfile(r.Context(), user.TenantID)
	if err != nil {
		writeSaaSBillingError(w, err)
		return
	}
	orders, err := h.store.SaaSAdminPaymentOrders(r.Context(), SaaSAdminPaymentOrderOptions{TenantID: user.TenantID, Status: SaaSPaymentOrderStatusAll, Limit: 100})
	if err != nil {
		writeSaaSBillingError(w, err)
		return
	}
	refunds, err := h.store.SaaSAdminPaymentRefunds(r.Context(), SaaSAdminPaymentRefundOptions{TenantID: user.TenantID, Status: SaaSPaymentRefundStatusAll, Limit: 100})
	if err != nil {
		writeSaaSBillingError(w, err)
		return
	}
	invoices, err := h.store.SaaSInvoiceDocuments(r.Context(), SaaSInvoiceDocumentOptions{TenantID: user.TenantID, Kind: SaaSInvoiceKindAll, Status: SaaSInvoiceStatusAll, Limit: 100})
	if err != nil {
		writeSaaSBillingError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"tenantId": user.TenantID, "profile": saasBillingProfilePayload(profile, exists),
		"paymentOrders":  saasAdminPaymentOrderReportPayload(orders),
		"paymentRefunds": saasAdminPaymentRefundReportPayload(refunds),
		"invoices":       saasInvoiceDocumentReportPayload(invoices),
	})
}

func (h *SaaSBillingHandler) PaymentOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	options, err := parseSaaSAdminPaymentOrderOptions(r, 500)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	options.TenantID = user.TenantID
	report, err := h.store.SaaSAdminPaymentOrders(r.Context(), options)
	if err != nil {
		writeSaaSBillingError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminPaymentOrderReportPayload(report))
}

func (h *SaaSBillingHandler) PaymentRefunds(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	options, err := parseSaaSAdminPaymentRefundOptions(r, 500)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	options.TenantID = user.TenantID
	report, err := h.store.SaaSAdminPaymentRefunds(r.Context(), options)
	if err != nil {
		writeSaaSBillingError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminPaymentRefundReportPayload(report))
}

func (h *SaaSBillingHandler) InvoiceProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		profile, exists, err := h.store.SaaSBillingProfile(r.Context(), user.TenantID)
		if err != nil {
			writeSaaSBillingError(w, err)
			return
		}
		writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"profile": saasBillingProfilePayload(profile, exists)})
	case http.MethodPost, http.MethodPut:
		update, err := parseSaaSBillingProfileUpdate(r)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return
		}
		update.TenantID = user.TenantID
		update.ActorUserID = user.ID
		update.ActorTenantID = user.TenantID
		result, err := h.store.SaveSaaSBillingProfile(r.Context(), update)
		if err != nil {
			writeSaaSBillingError(w, err)
			return
		}
		writeEnvelope(w, http.StatusOK, 200, "success", saasBillingProfileUpdateResultPayload(result))
	default:
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
	}
}

func (h *SaaSBillingHandler) Invoices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	options, err := parseSaaSInvoiceDocumentOptions(r, 500)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	options.TenantID = user.TenantID
	report, err := h.store.SaaSInvoiceDocuments(r.Context(), options)
	if err != nil {
		writeSaaSBillingError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasInvoiceDocumentReportPayload(report))
}

func (h *SaaSBillingHandler) CreateInvoice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	create, err := parseSaaSInvoiceDocumentCreate(r, SaaSInvoiceKindInvoice)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	create.TenantID = user.TenantID
	create.Kind = SaaSInvoiceKindInvoice
	create.OriginalDocumentNo = ""
	if create.DocumentNo == "" {
		create.DocumentNo, err = newSaaSInvoiceDocumentNo(SaaSInvoiceKindInvoice)
		if err != nil {
			writeSaaSBillingError(w, err)
			return
		}
	}
	create.ActorUserID = user.ID
	create.ActorTenantID = user.TenantID
	result, err := h.store.CreateSaaSInvoiceDocument(r.Context(), create)
	if err != nil {
		writeSaaSBillingError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasInvoiceDocumentCreateResultPayload(result))
}

func (h *SaaSBillingHandler) CancelInvoice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请求格式错误", nil)
		return
	}
	version, _, err := intParam(params, "expectedVersion")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "expectedVersion 格式错误", nil)
		return
	}
	reason := strings.TrimSpace(stringParam(params, "reason"))
	transition := SaaSInvoiceDocumentTransition{
		DocumentNo: strings.TrimSpace(stringParam(params, "documentNo")), TenantID: user.TenantID,
		ExpectedVersion: version, Status: SaaSInvoiceStatusCanceled, Remark: reason,
		ActorUserID: user.ID, ActorTenantID: user.TenantID,
	}
	if reason == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "reason 必填", nil)
		return
	}
	if err := validateSaaSInvoiceDocumentTransition(transition); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	result, err := h.store.TransitionSaaSInvoiceDocument(r.Context(), transition)
	if err != nil {
		writeSaaSBillingError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasInvoiceDocumentTransitionResultPayload(result))
}

func (h *SaaSBillingHandler) resolveSuperAdmin(w http.ResponseWriter, r *http.Request) (User, bool) {
	if _, principalErr := saasauth.PrincipalFromContext(r.Context()); principalErr == nil {
		user, found, err := resolveSaaSAdminActor(r.Context(), h.store, h.platformAdminTenantID)
		if errors.Is(err, ErrSaaSAdminActorStoreUnavailable) {
			writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, err.Error(), nil)
			return User{}, false
		}
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return User{}, false
		}
		if !found || user.Status != 1 {
			writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
			return User{}, false
		}
		if user.IsSuperAdmin != 1 {
			writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, ErrPermissionDenied.Error(), nil)
			return User{}, false
		}
		return user, true
	}
	if h.resolver == nil {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return User{}, false
	}
	userID, err := h.resolver.UserID(r)
	if err != nil || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return User{}, false
	}
	user, found, err := h.store.UserByID(r.Context(), userID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return User{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
		return User{}, false
	}
	if user.IsSuperAdmin != 1 || user.TenantID <= 0 {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, ErrPermissionDenied.Error(), nil)
		return User{}, false
	}
	return user, true
}

func writeSaaSBillingError(w http.ResponseWriter, err error) {
	var operationErr *SaaSAdminOperationError
	if errors.As(err, &operationErr) {
		writeEnvelope(w, operationErr.Status, operationErr.Status, operationErr.Message, nil)
		return
	}
	writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
}
