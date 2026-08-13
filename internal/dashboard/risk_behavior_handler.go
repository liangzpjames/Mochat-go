package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type RiskBehaviorHandler struct {
	provider   RiskBehaviorProvider
	authorizer CorpAdminAuthorizer
}

func NewRiskBehaviorHandler(provider RiskBehaviorProvider, _ LoginCache, _ UserIDResolver, authorizer CorpAdminAuthorizer) *RiskBehaviorHandler {
	return &RiskBehaviorHandler{provider: provider, authorizer: authorizer}
}
func (h *RiskBehaviorHandler) resolve(w http.ResponseWriter, r *http.Request, permission string) (int, int, bool) {
	identity, err := ResolveDashboardHandlerIdentity(r.Context())
	if err != nil {
		writeMachineEnvelope(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized", nil)
		return 0, 0, false
	}
	if h.authorizer != nil {
		if _, err = h.authorizer.Resolve(r.Context(), identity.UserID, permission, identity.CorpID, identity.WorkEmployeeID); err != nil {
			writeMachineEnvelope(w, http.StatusForbidden, DashboardPermissionDeniedCode, "dashboard permission denied", nil)
			return 0, 0, false
		}
	}
	return identity.TenantID, identity.CorpID, true
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
	access, _ := DashboardAccessFromContext(r.Context())
	result, err := h.provider.RiskRecordPage(r.Context(), RiskRecordFilter{TenantID: tenant, CorpID: corp, RiskLevel: r.URL.Query().Get("riskLevel"), Behavior: r.URL.Query().Get("behavior"), ConversationType: r.URL.Query().Get("conversationType"), RuleID: ruleID, Page: page, PerPage: per, AllowedEmployeeIDs: append([]int(nil), access.AllowedEmployeeIDs...), RestrictEmployeeIDs: access.ScopeRequired && access.Scope != DataScopeTenant})
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

func (h *RiskBehaviorHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	_, corp, ok := h.resolve(w, r, "/ai-insight/v2/risk#manage")
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
	rule.CorpID = int64(corp)
	updated, err := writer.UpdateRiskRule(r.Context(), rule)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if !updated {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "规则不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 0, "ok", map[string]any{"updated": true})
}

func (h *RiskBehaviorHandler) RuleStatus(w http.ResponseWriter, r *http.Request) {
	_, corp, ok := h.resolve(w, r, "/ai-insight/v2/risk#manage")
	if !ok {
		return
	}
	writer, ok := h.provider.(RiskRuleProviderWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "风险规则写入能力未启用", nil)
		return
	}
	var input struct {
		ID     int64          `json:"id"`
		Status RiskRuleStatus `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "规则参数无效", nil)
		return
	}
	updated, err := writer.SetRiskRuleStatus(r.Context(), corp, input.ID, input.Status)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if !updated {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "规则不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 0, "ok", map[string]any{"updated": true})
}

func (h *RiskBehaviorHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	_, corp, ok := h.resolve(w, r, "/ai-insight/v2/risk#manage")
	if !ok {
		return
	}
	writer, ok := h.provider.(RiskRuleProviderWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "风险规则写入能力未启用", nil)
		return
	}
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	deleted, err := writer.DeleteRiskRule(r.Context(), corp, id)
	if err != nil {
		writeEnvelope(w, http.StatusConflict, http.StatusConflict, err.Error(), nil)
		return
	}
	if !deleted {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "规则不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 0, "ok", map[string]any{"deleted": true})
}

func (h *RiskBehaviorHandler) AuditRecords(w http.ResponseWriter, r *http.Request) {
	if access, scoped := DashboardAccessFromContext(r.Context()); scoped && access.ScopeRequired && access.Scope != DataScopeTenant {
		writeMachineEnvelope(w, http.StatusForbidden, DashboardPermissionDeniedCode, "dashboard permission denied", nil)
		return
	}
	tenant, corp, ok := h.resolve(w, r, "/ai-insight/v2/risk#manage")
	if !ok {
		return
	}
	writer, ok := h.provider.(RiskRecordProviderWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "风险审计能力未启用", nil)
		return
	}
	identity, _ := ResolveDashboardHandlerIdentity(r.Context())
	actor := identity.UserID
	var input struct {
		IDs    []int64 `json:"ids"`
		Action string  `json:"action"`
		Remark string  `json:"remark"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "审计参数无效", nil)
		return
	}
	count, err := writer.AuditRiskRecords(r.Context(), tenant, corp, actor, input.IDs, input.Action, input.Remark)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 0, "ok", map[string]any{"updated": count})
}

func (h *RiskBehaviorHandler) Evaluate(w http.ResponseWriter, r *http.Request) {
	tenant, corp, ok := h.resolve(w, r, "/ai-insight/v2/risk#manage")
	if !ok {
		return
	}
	evaluator, ok := h.provider.(RiskEvaluatorProvider)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "风险评估能力未启用", nil)
		return
	}
	var message RiskMessage
	if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "消息参数无效", nil)
		return
	}
	message.TenantID = tenant
	message.CorpID = corp
	created, err := evaluator.EvaluateRiskMessage(r.Context(), message)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 0, "ok", map[string]any{"created": created})
}
