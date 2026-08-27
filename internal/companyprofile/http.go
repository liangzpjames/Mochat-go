package companyprofile

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

type HTTPHandler struct {
	service *Service
}

func NewHTTPHandler(service *Service) *HTTPHandler {
	return &HTTPHandler{service: service}
}

func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r == nil || r.URL == nil {
		writeEnvelope(w, http.StatusBadRequest, CodeInvalidRequest, "invalid request", nil)
		return
	}
	principal, err := dashboardprincipal.DashboardPrincipalFromContext(r.Context())
	if err != nil {
		writeEnvelope(w, http.StatusUnauthorized, "SESSION_INVALID", "session invalid", nil)
		return
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/dashboard/company/profile":
		profile, callErr := h.service.GetProfile(r.Context(), principal)
		h.writeCallResult(w, profile, callErr)
	case r.Method == http.MethodPut && r.URL.Path == "/dashboard/company/profile":
		var input UpdateProfileInput
		if !decodeJSON(r, &input) {
			writeEnvelope(w, http.StatusBadRequest, CodeInvalidRequest, "invalid request", nil)
			return
		}
		profile, callErr := h.service.UpdateProfile(r.Context(), principal, input)
		h.writeCallResult(w, profile, callErr)
	case r.Method == http.MethodPut && r.URL.Path == "/dashboard/company/wecom-credentials":
		var input WeComCredentialsInput
		if !decodeJSON(r, &input) {
			writeEnvelope(w, http.StatusBadRequest, CodeInvalidRequest, "invalid request", nil)
			return
		}
		profile, callErr := h.service.RotateWeComCredentials(r.Context(), principal, input)
		h.writeCallResult(w, profile, callErr)
	case r.Method == http.MethodPut && r.URL.Path == "/dashboard/company/agent-credentials":
		var input AgentCredentialsInput
		if !decodeJSON(r, &input) {
			writeEnvelope(w, http.StatusBadRequest, CodeInvalidRequest, "invalid request", nil)
			return
		}
		profile, callErr := h.service.RotateAgentCredentials(r.Context(), principal, input)
		h.writeCallResult(w, profile, callErr)
	case r.Method == http.MethodPut && r.URL.Path == "/dashboard/company/application-credentials":
		var input ApplicationCredentialsInput
		if !decodeJSON(r, &input) {
			writeEnvelope(w, http.StatusBadRequest, CodeInvalidRequest, "invalid request", nil)
			return
		}
		profile, callErr := h.service.ConfigureApplication(r.Context(), principal, input)
		h.writeCallResult(w, profile, callErr)
	case r.Method == http.MethodPut && r.URL.Path == "/dashboard/company/archive-credentials":
		var input ArchiveCredentialsInput
		if !decodeJSON(r, &input) {
			writeEnvelope(w, http.StatusBadRequest, CodeInvalidRequest, "invalid request", nil)
			return
		}
		profile, callErr := h.service.RotateArchiveCredentials(r.Context(), principal, input)
		h.writeCallResult(w, profile, callErr)
	case r.Method == http.MethodGet && r.URL.Path == "/dashboard/company/callback-configuration":
		configuration, callErr := h.service.GetCallbackConfiguration(r.Context(), principal)
		if callErr == nil {
			configuration.CallbackURL = callbackURLForRequest(r, configuration.CorpID)
		}
		writeSecretResponseHeaders(w)
		h.writeCallResult(w, configuration, callErr)
	case r.Method == http.MethodPost && r.URL.Path == "/dashboard/company/callback-configuration/regenerate":
		var input CallbackConfigurationInput
		if !decodeJSON(r, &input) {
			writeEnvelope(w, http.StatusBadRequest, CodeInvalidRequest, "invalid request", nil)
			return
		}
		configuration, callErr := h.service.RegenerateCallbackConfiguration(r.Context(), principal, input)
		if callErr == nil {
			configuration.CallbackURL = callbackURLForRequest(r, configuration.CorpID)
		}
		writeSecretResponseHeaders(w)
		h.writeCallResult(w, configuration, callErr)
	case r.Method == http.MethodPost && r.URL.Path == "/dashboard/company/verify":
		var input VerifyInput
		if !decodeJSON(r, &input) {
			writeEnvelope(w, http.StatusBadRequest, CodeInvalidRequest, "invalid request", nil)
			return
		}
		profile, callErr := h.service.Verify(r.Context(), principal, input)
		h.writeCallResult(w, profile, callErr)
	case r.Method == http.MethodPost && r.URL.Path == "/dashboard/company/employee-sync":
		if !decodeEmptyJSON(r) {
			writeEnvelope(w, http.StatusBadRequest, CodeInvalidRequest, "invalid request", nil)
			return
		}
		result, callErr := h.service.StartEmployeeSync(r.Context(), principal)
		h.writeCallResult(w, result, callErr)
	case r.Method == http.MethodGet && r.URL.Path == "/dashboard/company/sync-status":
		status, callErr := h.service.GetSyncStatus(r.Context(), principal)
		h.writeCallResult(w, status, callErr)
	case r.Method == http.MethodGet && r.URL.Path == "/dashboard/company/audits":
		page, callErr := h.service.ListAudits(r.Context(), principal, auditFilterFromQuery(r.URL.Query()))
		h.writeCallResult(w, page, callErr)
	default:
		writeEnvelope(w, http.StatusNotFound, CodeNotFound, "not found", nil)
	}
}

func writeSecretResponseHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
}

func callbackURLForRequest(r *http.Request, corpID int) string {
	scheme := "http"
	if r != nil && r.TLS != nil {
		scheme = "https"
	}
	if r != nil && r.URL != nil && (r.URL.Scheme == "http" || r.URL.Scheme == "https") {
		scheme = r.URL.Scheme
	}
	host := ""
	if r != nil {
		host = strings.TrimSpace(r.Host)
	}
	if host == "" || corpID <= 0 {
		return ""
	}
	return scheme + "://" + host + "/weWork/callback?cid=" + strconv.Itoa(corpID)
}

func (h *HTTPHandler) writeCallResult(w http.ResponseWriter, data any, err error) {
	if err == nil {
		writeEnvelope(w, http.StatusOK, "OK", "success", data)
		return
	}
	status, code, message := errorResponse(err)
	writeEnvelope(w, status, code, message, nil)
}

func errorResponse(err error) (int, string, string) {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		return http.StatusBadRequest, CodeInvalidRequest, "invalid request"
	case errors.Is(err, ErrPermissionDenied):
		return http.StatusForbidden, CodePermissionDenied, "permission denied"
	case errors.Is(err, ErrTenantAccessDenied):
		return http.StatusForbidden, CodeTenantAccessDenied, "tenant access denied"
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound, CodeNotFound, "not found"
	case errors.Is(err, ErrVersionConflict):
		return http.StatusConflict, CodeVersionConflict, "version conflict"
	case errors.Is(err, ErrCorpIDImmutable):
		return http.StatusConflict, CodeCorpIDImmutable, "company CorpID is immutable"
	case errors.Is(err, ErrIntegrationMode):
		return http.StatusConflict, CodeIntegrationMode, "WeCom integration is managed by SaaS"
	case errors.Is(err, ErrCredentialInvalid):
		return http.StatusUnprocessableEntity, CodeWeComCredentialError, "WeCom credential invalid"
	default:
		return http.StatusInternalServerError, CodeInternal, "internal error"
	}
}

func decodeJSON(r *http.Request, target any) bool {
	if r == nil || r.Body == nil {
		return false
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return false
	}
	var trailing json.RawMessage
	return errors.Is(decoder.Decode(&trailing), io.EOF)
}

func decodeEmptyJSON(r *http.Request) bool {
	if r == nil || r.Body == nil {
		return true
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var input struct{}
	if err := decoder.Decode(&input); err != nil {
		return errors.Is(err, io.EOF)
	}
	var trailing json.RawMessage
	return errors.Is(decoder.Decode(&trailing), io.EOF)
}

func auditFilterFromQuery(values url.Values) AuditFilter {
	page, _ := strconv.Atoi(strings.TrimSpace(values.Get("page")))
	perPage, _ := strconv.Atoi(strings.TrimSpace(values.Get("perPage")))
	return AuditFilter{Page: page, PerPage: perPage}
}

func writeEnvelope(w http.ResponseWriter, status int, errorCode, message string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Code      int    `json:"code"`
		ErrorCode string `json:"errorCode,omitempty"`
		Msg       string `json:"msg"`
		Data      any    `json:"data"`
	}{Code: status, ErrorCode: errorCode, Msg: message, Data: data})
}
