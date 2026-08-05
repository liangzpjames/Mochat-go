package http

import (
	"context"
	"encoding/json"
	"jiyi/mochat-go/internal/modules/scrm/adapters/mysql"
	nethttp "net/http"
	"strconv"
)

type settingsRepository interface {
	List(context.Context, int64, int64, string) ([]mysql.SCRMSetting, error)
	Upsert(context.Context, mysql.SCRMSetting, int64) (mysql.SCRMSetting, error)
}
type SettingsHandler struct {
	repo       settingsRepository
	principal  PrincipalResolver
	authorizer LeadAuthorizer
}

func NewSettingsHandler(repo settingsRepository, p PrincipalResolver, a LeadAuthorizer) *SettingsHandler {
	return &SettingsHandler{repo: repo, principal: p, authorizer: a}
}
func (h *SettingsHandler) ServeHTTP(w nethttp.ResponseWriter, r *nethttp.Request) {
	p, e := h.principal.Resolve(r)
	if e != nil {
		nethttp.Error(w, "principal unauthorized", 401)
		return
	}
	// PUT clients send corpId in the JSON payload; accept either location while
	// keeping GET query-only semantics. Decode before authorization so the
	// principal is always checked against the requested corp.
	var payload mysql.SCRMSetting
	if r.Method != nethttp.MethodGet {
		if json.NewDecoder(r.Body).Decode(&payload) != nil {
			nethttp.Error(w, "invalid json", 400)
			return
		}
	}
	corp, _ := strconv.ParseInt(r.URL.Query().Get("corpId"), 10, 64)
	if corp <= 0 {
		corp = payload.CorpID
	}
	if corp <= 0 {
		nethttp.Error(w, "corpId required", 400)
		return
	}
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
		json.NewEncoder(w).Encode(map[string]any{"code": nethttp.StatusOK, "msg": "success", "data": v})
		return
	}
	s := payload
	s.TenantID = p.TenantID
	s.CorpID = corp
	v, e := h.repo.Upsert(r.Context(), s, p.UserID)
	if e != nil {
		nethttp.Error(w, e.Error(), 409)
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"code": nethttp.StatusOK, "msg": "success", "data": v})
}
