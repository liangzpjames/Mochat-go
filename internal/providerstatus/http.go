package providerstatus

import (
	"encoding/json"
	"errors"
	"net/http"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

type HTTPHandler struct {
	service *Service
}

func NewHTTPHandler(service *Service) *HTTPHandler {
	return &HTTPHandler{service: service}
}

func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r == nil || r.URL == nil || r.Method != http.MethodGet || r.URL.Path != "/dashboard/providers/status" {
		writeEnvelope(w, http.StatusNotFound, CodeInternal, "not found", nil)
		return
	}
	if h == nil || h.service == nil {
		writeEnvelope(w, http.StatusServiceUnavailable, CodeSourceUnavailable, "provider status source unavailable", nil)
		return
	}
	principal, err := dashboardprincipal.DashboardPrincipalFromContext(r.Context())
	if err != nil {
		writeEnvelope(w, http.StatusUnauthorized, CodeSessionInvalid, "session invalid", nil)
		return
	}
	view, err := h.service.Resolve(r.Context(), principal)
	if err != nil {
		status, code, message := errorResponse(err)
		writeEnvelope(w, status, code, message, nil)
		return
	}
	writeEnvelope(w, http.StatusOK, "OK", "success", view)
}

func errorResponse(err error) (int, string, string) {
	switch {
	case errors.Is(err, ErrScopeDenied):
		return http.StatusForbidden, CodeScopeDenied, "provider status scope denied"
	case errors.Is(err, ErrSourceUnavailable):
		return http.StatusServiceUnavailable, CodeSourceUnavailable, "provider status source unavailable"
	default:
		return http.StatusInternalServerError, CodeInternal, "provider status internal error"
	}
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
