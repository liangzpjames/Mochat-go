package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type RiskBehaviorHandler struct {
	provider   RiskBehaviorProvider
	cache      LoginCache
	resolver   UserIDResolver
	authorizer CorpAdminAuthorizer
}

func NewRiskBehaviorHandler(provider RiskBehaviorProvider, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer) *RiskBehaviorHandler {
	return &RiskBehaviorHandler{provider: provider, cache: cache, resolver: resolver, authorizer: authorizer}
}
func (h *RiskBehaviorHandler) resolve(w http.ResponseWriter, r *http.Request, permission string) (int, int, bool) {
	userID, err := h.resolver.UserID(r)
	if err != nil {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, err.Error(), nil)
		return 0, 0, false
	}
	cached, err := h.cache.UserCorpCache(r.Context(), userID)
	if err != nil {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "登录已失效", nil)
		return 0, 0, false
	}
	parts := strings.Split(cached, "-")
	corpID, _ := strconv.Atoi(r.URL.Query().Get("corpId"))
	employeeID := 0
	if len(parts) > 0 {
		if corpID <= 0 {
			corpID, _ = strconv.Atoi(parts[0])
		}
	}
	if len(parts) > 1 {
		employeeID, _ = strconv.Atoi(parts[1])
	}
	if h.authorizer != nil {
		if _, err = h.authorizer.Resolve(r.Context(), userID, permission, corpID, employeeID); err != nil {
			writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "无权访问风险行为", nil)
			return 0, 0, false
		}
	}
	return 0, corpID, true
}
func (h *RiskBehaviorHandler) Rules(w http.ResponseWriter, r *http.Request) {
	tenant, corp, ok := h.resolve(w, r, "/ai-insight/v2/risk#read")
	if !ok {
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	per, _ := strconv.Atoi(r.URL.Query().Get("perPage"))
	result, err := h.provider.RiskRulePage(r.Context(), RiskRuleFilter{TenantID: tenant, CorpID: corp, Name: r.URL.Query().Get("name"), Page: page, PerPage: per})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 0, "ok", result)
}
func (h *RiskBehaviorHandler) Records(w http.ResponseWriter, r *http.Request) {
	tenant, corp, ok := h.resolve(w, r, "/ai-insight/v2/risk#read")
	if !ok {
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	per, _ := strconv.Atoi(r.URL.Query().Get("perPage"))
	ruleID, _ := strconv.ParseInt(r.URL.Query().Get("ruleId"), 10, 64)
	result, err := h.provider.RiskRecordPage(r.Context(), RiskRecordFilter{TenantID: tenant, CorpID: corp, RiskLevel: r.URL.Query().Get("riskLevel"), Behavior: r.URL.Query().Get("behavior"), ConversationType: r.URL.Query().Get("conversationType"), RuleID: ruleID, Page: page, PerPage: per})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 0, "ok", result)
}

func (h *RiskBehaviorHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	tenant, corp, ok := h.resolve(w, r, "/ai-insight/v2/risk#manage")
	if !ok {
		return
	}
	writer, ok := h.provider.(RiskRuleProviderWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "风险规则写入能力未启用", nil)
		return
	}
	var rule RiskRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "规则参数无效", nil)
		return
	}
	rule.TenantID = int64(tenant)
	rule.CorpID = int64(corp)
	id, err := writer.CreateRiskRule(r.Context(), rule)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 0, "ok", map[string]any{"id": id})
}
