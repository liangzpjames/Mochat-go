package http

import (
	"context"
	"encoding/json"
	nethttp "net/http"
	"regexp"
	"strings"
)

var acceptanceEnvironmentPattern = regexp.MustCompile(`^P35-ACCEPT-[A-Za-z0-9-]{1,35}$`)

const (
	AcceptancePath              = "/dashboard/acceptance/phase35"
	AcceptancePrefix            = "P35-ACCEPT-"
	AcceptanceEnvironmentHeader = "X-Phase35-Acceptance-Environment"
)

type AcceptanceScope struct {
	TenantID, CorpID, ActorID int64
	EnvironmentID             string
}
type AcceptanceResult struct {
	Prefix        string   `json:"prefix"`
	EnvironmentID string   `json:"environmentId,omitempty"`
	ResourceIDs   []string `json:"resourceIds,omitempty"`
	Count         int      `json:"count"`
}
type AcceptanceStore interface {
	Create(context.Context, AcceptanceScope) (AcceptanceResult, error)
	Verify(context.Context, AcceptanceScope) (AcceptanceResult, error)
	Cleanup(context.Context, AcceptanceScope) (AcceptanceResult, error)
}
type AcceptanceHandler struct {
	enabled       bool
	environmentID string
	store         AcceptanceStore
	principal     PrincipalResolver
	authorizer    LeadAuthorizer
}

func NewAcceptanceHandler(enabled bool, environmentID string, store AcceptanceStore, principal PrincipalResolver, authorizer LeadAuthorizer) *AcceptanceHandler {
	return &AcceptanceHandler{enabled: enabled && acceptanceEnvironmentPattern.MatchString(environmentID), environmentID: environmentID, store: store, principal: principal, authorizer: authorizer}
}

func (h *AcceptanceHandler) ServeHTTP(w nethttp.ResponseWriter, r *nethttp.Request) {
	if !h.enabled {
		nethttp.NotFound(w, r)
		return
	}
	if h.store == nil || h.principal == nil {
		nethttp.Error(w, "acceptance lifecycle unavailable", nethttp.StatusServiceUnavailable)
		return
	}
	principal, err := h.principal.Resolve(r)
	if err != nil || principal.UserID <= 0 || principal.TenantID <= 0 || principal.CorpID <= 0 {
		nethttp.Error(w, "principal unauthorized", nethttp.StatusUnauthorized)
		return
	}
	corpID := principal.CorpID
	if h.authorizer != nil {
		if err = h.authorizer.Authorize(r.Context(), principal, corpID, "/acceptance/phase35#"+strings.ToLower(r.Method)); err != nil {
			nethttp.Error(w, "forbidden", nethttp.StatusForbidden)
			return
		}
	}
	environmentID := strings.TrimSpace(r.Header.Get(AcceptanceEnvironmentHeader))
	prefix := r.URL.Query().Get("prefix")
	if r.Method != nethttp.MethodGet {
		var body struct {
			EnvironmentID string `json:"environmentId"`
			Prefix        string `json:"prefix"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			nethttp.Error(w, "invalid json", 400)
			return
		}
		if environmentID != body.EnvironmentID || prefix != body.Prefix {
			nethttp.Error(w, "acceptance scope mismatch", 400)
			return
		}
	}
	if prefix != AcceptancePrefix || !acceptanceEnvironmentPattern.MatchString(environmentID) {
		nethttp.Error(w, "invalid acceptance prefix", nethttp.StatusBadRequest)
		return
	}
	if environmentID != h.environmentID {
		nethttp.Error(w, "acceptance environment rejected", nethttp.StatusForbidden)
		return
	}
	scope := AcceptanceScope{TenantID: principal.TenantID, CorpID: corpID, ActorID: principal.UserID, EnvironmentID: environmentID}
	var result AcceptanceResult
	switch r.Method {
	case nethttp.MethodPost:
		result, err = h.store.Create(r.Context(), scope)
	case nethttp.MethodGet:
		result, err = h.store.Verify(r.Context(), scope)
	case nethttp.MethodDelete:
		result, err = h.store.Cleanup(r.Context(), scope)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		nethttp.Error(w, "method not allowed", 405)
		return
	}
	if err != nil {
		nethttp.Error(w, "acceptance lifecycle failed", nethttp.StatusConflict)
		return
	}
	result.Prefix = AcceptancePrefix
	result.EnvironmentID = environmentID
	writeJSON(w, nethttp.StatusOK, result)
}
