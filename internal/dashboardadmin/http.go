package dashboardadmin

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

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
	codeUnavailable         = "DASHBOARD_ADMIN_UNAVAILABLE"
)

type HTTPHandler struct {
	service *Service
}

func actorFromSaaSPrincipal(principal saasauth.Principal) Actor {
	return Actor{UserID: principal.UserID, Active: true}
}

func NewHTTPHandler(service *Service) *HTTPHandler {
	return &HTTPHandler{service: service}
}

func (handler *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if handler == nil || handler.service == nil {
		writeDashboardAdminError(w, http.StatusServiceUnavailable, codeUnavailable)
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
		"tenantId":        result.TenantID,
		"dashboardUserId": result.DashboardUserID,
		"version":         result.Version,
		"activationToken": result.ActivationToken,
		"idempotent":      result.Idempotent,
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
	result, err := handler.service.ProvisionDashboardTenant(r.Context(), actorFromSaaSPrincipal(principal), input)
	if err != nil {
		writeDashboardAdminServiceError(w, err)
		return
	}
	writeDashboardAdminJSON(w, http.StatusCreated, map[string]any{
		"tenantId":        result.TenantID,
		"dashboardUserId": result.DashboardUserID,
		"bindingCorpId":   result.BindingCorpID,
		"activationToken": result.ActivationToken,
		"idempotent":      result.Idempotent,
	})
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
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Code      int    `json:"code"`
		ErrorCode string `json:"errorCode"`
		Data      any    `json:"data"`
	}{Code: status, ErrorCode: errorCode, Data: nil})
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
	}
	writeDashboardAdminError(w, status, code)
}
