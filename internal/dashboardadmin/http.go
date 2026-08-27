package dashboardadmin

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/saasauth"
)

const (
	codeInvalidRequest      = "INVALID_REQUEST"
	codeSessionInvalid      = "SESSION_INVALID"
	codePermissionDenied    = "PERMISSION_DENIED"
	codeLoginConflict       = "LOGIN_IDENTIFIER_CONFLICT"
	codeIdempotencyConflict = "IDEMPOTENCY_CONFLICT"
	codeTargetNotFound      = "TARGET_NOT_FOUND"
	codeVersionConflict     = "VERSION_CONFLICT"
	codeActivationRequired  = "ACTIVATION_REQUIRED"
	codeLastSuperAdmin      = "LAST_SUPER_ADMIN"
	codeGovernanceOnly      = "SUPER_ADMIN_GOVERNANCE_ONLY"
	codeApprovalRequired    = "APPROVAL_REQUIRED"
	codeUnavailable         = "DASHBOARD_ADMIN_UNAVAILABLE"
)

type HTTPHandler struct {
	service          *Service
	approvalGate     ApprovalGate
	weComIntegration *WeComIntegrationService
}

func (handler *HTTPHandler) WithWeComIntegration(service *WeComIntegrationService) *HTTPHandler {
	if handler != nil {
		handler.weComIntegration = service
	}
	return handler
}

func actorFromSaaSPrincipal(principal saasauth.Principal) Actor {
	return Actor{UserID: principal.UserID, Active: true}
}

func NewHTTPHandler(service *Service) *HTTPHandler {
	return &HTTPHandler{service: service}
}

func (handler *HTTPHandler) WithApprovalGate(gate ApprovalGate) *HTTPHandler {
	if handler != nil {
		handler.approvalGate = gate
	}
	return handler
}

func (handler *HTTPHandler) requireDirectApproval(w http.ResponseWriter, r *http.Request, actionType string) bool {
	if handler == nil || handler.approvalGate == nil {
		return false
	}
	decision, err := handler.approvalGate(r.Context(), actionType)
	if err != nil {
		writeDashboardAdminServiceError(w, err)
		return true
	}
	if !decision.Required {
		return false
	}
	writeDashboardAdminApprovalRequired(w, decision)
	return true
}

func (handler *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if handler == nil || handler.service == nil {
		writeDashboardAdminError(w, http.StatusServiceUnavailable, codeUnavailable)
		return
	}
	if tenantID, action, ok := dashboardAdminWeComIntegrationPath(r.URL.Path); ok && handler.weComIntegration != nil {
		handler.weComIntegrationHTTP(w, r, tenantID, action)
		return
	}
	if r.Method == http.MethodGet {
		if tenantID, ok := dashboardAdminTenantActionPath(r.URL.Path, "dashboard-admins"); ok {
			handler.dashboardAdminGovernance(w, r, tenantID)
			return
		}
	}
	if r.Method == http.MethodPost {
		if tenantID, ok := dashboardAdminTenantActionPath(r.URL.Path, "activation/resend"); ok {
			handler.resendActivation(w, r, tenantID)
			return
		}
		if tenantID, ok := dashboardAdminTenantActionPath(r.URL.Path, "super-admin/replace"); ok {
			handler.replaceSuperAdmin(w, r, tenantID)
			return
		}
		if tenantID, ok := dashboardAdminTenantActionPath(r.URL.Path, "super-admin/status"); ok {
			handler.superAdminStatus(w, r, tenantID)
			return
		}
	}
	if r.URL.Path != "/dashboard/saasAdmin/tenants/provision" || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	handler.provision(w, r)
}

func dashboardAdminWeComIntegrationPath(path string) (int, string, bool) {
	const prefix = "/dashboard/saasAdmin/tenants/"
	if !strings.HasPrefix(path, prefix) {
		return 0, "", false
	}
	rest := strings.TrimPrefix(path, prefix)
	idText, suffix, ok := strings.Cut(rest, "/")
	if !ok {
		return 0, "", false
	}
	tenantID, err := strconv.Atoi(idText)
	if err != nil || tenantID <= 0 || strconv.Itoa(tenantID) != idText {
		return 0, "", false
	}
	if suffix == "wecom-integration" || suffix == "wecom-integration/audits" {
		return tenantID, suffix, true
	}
	return 0, "", false
}

func (handler *HTTPHandler) weComIntegrationHTTP(w http.ResponseWriter, r *http.Request, tenantID int, action string) {
	principal, err := saasauth.PrincipalFromContext(r.Context())
	if err != nil {
		writeDashboardAdminError(w, http.StatusUnauthorized, codeSessionInvalid)
		return
	}
	actor := actorFromSaaSPrincipal(principal)
	switch {
	case action == "wecom-integration" && r.Method == http.MethodGet:
		result, err := handler.weComIntegration.Get(r.Context(), actor, tenantID)
		if err != nil {
			writeDashboardAdminServiceError(w, err)
			return
		}
		writeDashboardAdminJSON(w, http.StatusOK, result)
	case action == "wecom-integration" && r.Method == http.MethodPut:
		var input WeComIntegrationCandidateInput
		if decodeDashboardAdminJSON(r, &input) != nil {
			writeDashboardAdminError(w, http.StatusBadRequest, codeInvalidRequest)
			return
		}
		result, err := handler.weComIntegration.SaveCurrent(r.Context(), actor, tenantID, input)
		if err != nil {
			writeDashboardAdminServiceError(w, err)
			return
		}
		writeDashboardAdminJSON(w, http.StatusOK, result)
	case action == "wecom-integration/audits" && r.Method == http.MethodGet:
		result, err := handler.weComIntegration.Audits(r.Context(), actor, tenantID)
		if err != nil {
			writeDashboardAdminServiceError(w, err)
			return
		}
		writeDashboardAdminJSON(w, http.StatusOK, result)
	default:
		http.NotFound(w, r)
	}
}

func (handler *HTTPHandler) dashboardAdminGovernance(w http.ResponseWriter, r *http.Request, tenantID int) {
	principal, err := saasauth.PrincipalFromContext(r.Context())
	if err != nil {
		writeDashboardAdminError(w, http.StatusUnauthorized, codeSessionInvalid)
		return
	}
	view, err := handler.service.DashboardAdminGovernance(r.Context(), actorFromSaaSPrincipal(principal), tenantID)
	if err != nil {
		writeDashboardAdminServiceError(w, err)
		return
	}
	identities := make([]map[string]any, 0, len(view.Identities))
	for _, identity := range view.Identities {
		identities = append(identities, map[string]any{
			"id": identity.ID, "name": identity.Name, "loginIdentifier": identity.LoginIdentifier,
			"userStatus": identity.UserStatus, "identityStatus": identity.IdentityStatus,
			"activatedAt": identity.ActivatedAt, "isSuperAdmin": identity.IsSuperAdmin,
			"availableActions": identity.AvailableActions, "blockedReasons": identity.BlockedReasons,
		})
	}
	writeDashboardAdminJSON(w, http.StatusOK, map[string]any{
		"tenantId": view.TenantID, "bindingVersion": view.BindingVersion, "identities": identities,
	})
}

func (handler *HTTPHandler) replaceSuperAdmin(w http.ResponseWriter, r *http.Request, tenantID int) {
	principal, err := saasauth.PrincipalFromContext(r.Context())
	if err != nil {
		writeDashboardAdminError(w, http.StatusUnauthorized, codeSessionInvalid)
		return
	}
	var input ReplaceSuperAdminInput
	if err := decodeDashboardAdminJSON(r, &input); err != nil {
		writeDashboardAdminError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	input.TenantID = tenantID
	input.RequestID = r.Header.Get("X-Request-ID")
	if handler.requireDirectApproval(w, r, ApprovalActionSuperAdminReplace) {
		return
	}
	result, err := handler.service.ReplaceDashboardSuperAdmin(r.Context(), actorFromSaaSPrincipal(principal), input)
	if err != nil {
		writeDashboardAdminServiceError(w, err)
		return
	}
	writeDashboardAdminJSON(w, http.StatusOK, map[string]any{
		"tenantId": result.TenantID, "dashboardUserId": result.DashboardUserID, "version": result.Version, "idempotent": result.Idempotent,
	})
}

func (handler *HTTPHandler) superAdminStatus(w http.ResponseWriter, r *http.Request, tenantID int) {
	principal, err := saasauth.PrincipalFromContext(r.Context())
	if err != nil {
		writeDashboardAdminError(w, http.StatusUnauthorized, codeSessionInvalid)
		return
	}
	var input SuperAdminStatusInput
	if err := decodeDashboardAdminJSON(r, &input); err != nil {
		writeDashboardAdminError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	input.TenantID = tenantID
	input.RequestID = r.Header.Get("X-Request-ID")
	if handler.requireDirectApproval(w, r, ApprovalActionSuperAdminStatus) {
		return
	}
	result, err := handler.service.SetDashboardSuperAdminStatus(r.Context(), actorFromSaaSPrincipal(principal), input)
	if err != nil {
		writeDashboardAdminServiceError(w, err)
		return
	}
	writeDashboardAdminJSON(w, http.StatusOK, map[string]any{
		"tenantId": result.TenantID, "dashboardUserId": result.DashboardUserID, "version": result.Version, "idempotent": result.Idempotent,
	})
}

func (handler *HTTPHandler) resendActivation(w http.ResponseWriter, r *http.Request, tenantID int) {
	principal, err := saasauth.PrincipalFromContext(r.Context())
	if err != nil {
		writeDashboardAdminError(w, http.StatusUnauthorized, codeSessionInvalid)
		return
	}
	var input ResendActivationInput
	if err := decodeDashboardAdminJSON(r, &input); err != nil {
		writeDashboardAdminError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	input.TenantID = tenantID
	input.RequestID = r.Header.Get("X-Request-ID")
	if handler.requireDirectApproval(w, r, ApprovalActionActivationResend) {
		return
	}
	result, err := handler.service.ResendActivation(r.Context(), actorFromSaaSPrincipal(principal), input)
	if err != nil {
		writeDashboardAdminServiceError(w, err)
		return
	}
	status := http.StatusCreated
	if result.Idempotent {
		status = http.StatusOK
	}
	writeDashboardAdminJSON(w, status, map[string]any{
		"tenantId":            result.TenantID,
		"dashboardUserId":     result.DashboardUserID,
		"version":             result.Version,
		"activationToken":     result.ActivationToken,
		"activationPath":      result.ActivationPath,
		"activationExpiresAt": dashboardActivationExpiry(result.ActivationExpiresAt),
		"idempotent":          result.Idempotent,
	})
}

func dashboardAdminTenantActionPath(path, action string) (int, bool) {
	const prefix = "/dashboard/saasAdmin/tenants/"
	if !strings.HasPrefix(path, prefix) {
		return 0, false
	}
	rest := strings.TrimPrefix(path, prefix)
	idText, suffix, ok := strings.Cut(rest, "/")
	if !ok || suffix != action {
		return 0, false
	}
	tenantID, err := strconv.Atoi(idText)
	if err != nil || tenantID <= 0 || strconv.Itoa(tenantID) != idText {
		return 0, false
	}
	return tenantID, true
}

func (handler *HTTPHandler) provision(w http.ResponseWriter, r *http.Request) {
	principal, err := saasauth.PrincipalFromContext(r.Context())
	if err != nil {
		writeDashboardAdminError(w, http.StatusUnauthorized, codeSessionInvalid)
		return
	}
	var input ProvisionDashboardTenant
	if err := decodeDashboardAdminJSON(r, &input); err != nil {
		writeDashboardAdminError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	input.RequestID = r.Header.Get("X-Request-ID")
	if handler.requireDirectApproval(w, r, ApprovalActionTenantProvision) {
		return
	}
	result, err := handler.service.ProvisionDashboardTenant(r.Context(), actorFromSaaSPrincipal(principal), input)
	if err != nil {
		writeDashboardAdminServiceError(w, err)
		return
	}
	writeDashboardAdminJSON(w, http.StatusCreated, map[string]any{
		"tenantId":            result.TenantID,
		"dashboardUserId":     result.DashboardUserID,
		"bindingCorpId":       result.BindingCorpID,
		"activationToken":     result.ActivationToken,
		"activationPath":      result.ActivationPath,
		"activationExpiresAt": dashboardActivationExpiry(result.ActivationExpiresAt),
		"idempotent":          result.Idempotent,
	})
}

func dashboardActivationExpiry(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func decodeDashboardAdminJSON(r *http.Request, target any) error {
	if r == nil || r.Body == nil {
		return io.ErrUnexpectedEOF
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain exactly one JSON value")
		}
		return err
	}
	return nil
}

func writeDashboardAdminJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Code int `json:"code"`
		Data any `json:"data"`
	}{Code: status, Data: data})
}

func writeDashboardAdminError(w http.ResponseWriter, status int, errorCode string) {
	writeDashboardAdminErrorData(w, status, errorCode, nil)
}

func writeDashboardAdminApprovalRequired(w http.ResponseWriter, decision ApprovalGateResult) {
	writeDashboardAdminErrorData(w, http.StatusPreconditionRequired, codeApprovalRequired, map[string]any{
		"actionType":        decision.ActionType,
		"requestPath":       "/dashboard/saasAdmin/approvalRequest",
		"required":          true,
		"requiredApprovals": decision.RequiredApprovals,
		"expiryHours":       decision.ExpiryHours,
	})
}

func writeDashboardAdminErrorData(w http.ResponseWriter, status int, errorCode string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Code      int    `json:"code"`
		ErrorCode string `json:"errorCode"`
		Data      any    `json:"data"`
	}{Code: status, ErrorCode: errorCode, Data: data})
}

func writeDashboardAdminServiceError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := codeUnavailable
	switch {
	case errors.Is(err, saasauth.ErrSessionInvalid):
		status, code = http.StatusUnauthorized, codeSessionInvalid
	case errors.Is(err, ErrInvalidRequest):
		status, code = http.StatusBadRequest, codeInvalidRequest
	case errors.Is(err, ErrPermissionDenied):
		status, code = http.StatusForbidden, codePermissionDenied
	case errors.Is(err, ErrLoginIdentifierConflict):
		status, code = http.StatusConflict, codeLoginConflict
	case errors.Is(err, ErrIdempotencyConflict):
		status, code = http.StatusConflict, codeIdempotencyConflict
	case errors.Is(err, ErrTargetNotFound):
		status, code = http.StatusNotFound, codeTargetNotFound
	case errors.Is(err, ErrVersionConflict):
		status, code = http.StatusConflict, codeVersionConflict
	case errors.Is(err, ErrReplacementRequiresActivation):
		status, code = http.StatusConflict, codeActivationRequired
	case errors.Is(err, ErrActivationAlreadyComplete):
		status, code = http.StatusConflict, "ACTIVATION_ALREADY_COMPLETE"
	case errors.Is(err, ErrLastSuperAdmin):
		status, code = http.StatusConflict, codeLastSuperAdmin
	case errors.Is(err, ErrSuperAdminGovernanceOnly):
		status, code = http.StatusForbidden, codeGovernanceOnly
	case errors.Is(err, ErrStoreUnavailable):
		status, code = http.StatusServiceUnavailable, codeUnavailable
	case errors.Is(err, ErrWeComOnlineVerificationUnavailable):
		status, code = http.StatusServiceUnavailable, "WECOM_ONLINE_VERIFICATION_UNAVAILABLE"
	case errors.Is(err, ErrWeComCorpMismatch):
		status, code = http.StatusConflict, "WECOM_CORP_MISMATCH"
	case errors.Is(err, ErrWeComMissingCapabilities):
		status, code = http.StatusConflict, "WECOM_MISSING_CAPABILITIES"
	case errors.Is(err, ErrWeComCredentialDecrypt):
		status, code = http.StatusConflict, "WECOM_CREDENTIAL_DECRYPT_FAILED"
	case errors.Is(err, ErrWeComActiveMediaLease):
		status, code = http.StatusConflict, "WECOM_ACTIVE_MEDIA_LEASE"
	case errors.Is(err, ErrWeComCandidateNotVerified):
		status, code = http.StatusConflict, "WECOM_CANDIDATE_NOT_VERIFIED"
	case errors.Is(err, ErrWeComModeImmutable):
		status, code = http.StatusConflict, "WECOM_MODE_IMMUTABLE"
	}
	writeDashboardAdminError(w, status, code)
}
