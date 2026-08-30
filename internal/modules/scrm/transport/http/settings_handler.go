package http

import (
	"jiyi/mochat-go/internal/modules/scrm/ports"
	nethttp "net/http"
)

type SettingsHandler struct {
	repo       ports.SettingsRepository
	principal  PrincipalResolver
	authorizer LeadAuthorizer
}

func NewSettingsHandler(repo ports.SettingsRepository, p PrincipalResolver, a LeadAuthorizer) *SettingsHandler {
	return &SettingsHandler{repo: repo, principal: p, authorizer: a}
}
func (h *SettingsHandler) ServeHTTP(w nethttp.ResponseWriter, r *nethttp.Request) {
	p, e := h.principal.Resolve(r)
	if e != nil || p.UserID <= 0 || p.TenantID <= 0 || p.CorpID <= 0 {
		nethttp.Error(w, "principal unauthorized", 401)
		return
	}
	var payload struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Key     string `json:"key"`
		Label   string `json:"label"`
		Value   any    `json:"value"`
		Enabled bool   `json:"enabled"`
	}
	if r.Method != nethttp.MethodGet {
		if decodeRequestJSON(w, r, &payload) != nil {
			nethttp.Error(w, "invalid json", 400)
			return
		}
	}
	corp := p.CorpID
	perm := "/scrm/settings#get"
	if r.Method != nethttp.MethodGet {
		perm = "/scrm/settings@edit#put"
	}
	if h.authorizer != nil {
		if e = h.authorizer.Authorize(r.Context(), p, corp, perm); e != nil {
			nethttp.Error(w, "forbidden", 403)
			return
		}
	}
	if r.Method == nethttp.MethodGet {
		v, e := h.repo.List(r.Context(), p.TenantID, corp, r.URL.Query().Get("type"))
		if e != nil {
			nethttp.Error(w, e.Error(), 500)
			return
		}
		writeJSON(w, nethttp.StatusOK, map[string]any{"code": nethttp.StatusOK, "msg": "success", "data": v})
		return
	}
	s := ports.SCRMSetting{ID: payload.ID, Type: payload.Type, Key: payload.Key, Label: payload.Label, Value: payload.Value, Enabled: payload.Enabled}
	s.TenantID = p.TenantID
	s.CorpID = corp
	v, e := h.repo.Upsert(r.Context(), s, p.UserID)
	if e != nil {
		nethttp.Error(w, e.Error(), 409)
		return
	}
	writeJSON(w, nethttp.StatusOK, map[string]any{"code": nethttp.StatusOK, "msg": "success", "data": v})
}
