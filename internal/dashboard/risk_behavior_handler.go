package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type RiskBehaviorHandler struct {
	provider       RiskBehaviorProvider
	authorizer     CorpAdminAuthorizer
	scannerEnabled bool
}

func NewRiskBehaviorHandler(provider RiskBehaviorProvider, _ LoginCache, _ UserIDResolver, authorizer CorpAdminAuthorizer) *RiskBehaviorHandler {
	return &RiskBehaviorHandler{provider: provider, authorizer: authorizer}
}

func (h *RiskBehaviorHandler) WithScannerEnabled(enabled bool) *RiskBehaviorHandler {
	h.scannerEnabled = enabled
	return h
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
	result, err := h.provider.RiskRecordPage(r.Context(), RiskRecordFilter{TenantID: tenant, CorpID: corp, RiskLevel: r.URL.Query().Get("riskLevel"), Behavior: r.URL.Query().Get("behavior"), ConversationType: r.URL.Query().Get("conversationType"), AuditStatus: r.URL.Query().Get("auditStatus"), OccurredFrom: r.URL.Query().Get("occurredFrom"), OccurredTo: r.URL.Query().Get("occurredTo"), RuleID: ruleID, Page: page, PerPage: per, EmployeeIDs: parseRiskEmployeeIDs(r.URL.Query().Get("employeeIds")), AllowedEmployeeIDs: append([]int(nil), access.AllowedEmployeeIDs...), RestrictEmployeeIDs: access.ScopeRequired && access.Scope != DataScopeTenant})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 0, "ok", result)
}

func parseRiskEmployeeIDs(value string) []int {
	parts := strings.Split(value, ",")
	result := make([]int, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err == nil && id > 0 {
			result = append(result, id)
		}
	}
	return result
}

func (h *RiskBehaviorHandler) RecordDetail(w http.ResponseWriter, r *http.Request) {
	tenant, corp, ok := h.resolve(w, r, "/ai-insight/v2/risk#read")
	if !ok {
		return
	}
	provider, ok := h.provider.(RiskRecordDetailProvider)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "风险详情能力未启用", nil)
		return
	}
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	access, _ := DashboardAccessFromContext(r.Context())
	result, err := provider.RiskRecordDetail(r.Context(), RiskRecordDetailFilter{TenantID: tenant, CorpID: corp, ID: id, AllowedEmployeeIDs: append([]int(nil), access.AllowedEmployeeIDs...), RestrictEmployeeIDs: access.ScopeRequired && access.Scope != DataScopeTenant})
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "不存在") {
			status = http.StatusNotFound
		}
		writeEnvelope(w, status, status, err.Error(), nil)
		return
	}
	if result.Record.ID <= 0 {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "风险记录不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 0, "ok", result)
}

func (h *RiskBehaviorHandler) ScannerStatus(w http.ResponseWriter, r *http.Request) {
	tenant, corp, ok := h.resolve(w, r, "/ai-insight/v2/risk#read")
	if !ok {
		return
	}
	state := RiskScanStatus{Enabled: h.scannerEnabled, State: "never_run"}
	if !h.scannerEnabled {
		state.State = "disabled"
	}
	if provider, ok := h.provider.(RiskScanStatusProvider); ok {
		stored, err := provider.RiskScanStatus(r.Context(), tenant, corp)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		stored.Enabled = h.scannerEnabled
		if !h.scannerEnabled {
			stored.State = "disabled"
		}
		state = stored
	}
	writeEnvelope(w, http.StatusOK, 0, "ok", state)
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
	if err := ValidateRiskRule(rule); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	id, err := writer.CreateRiskRule(r.Context(), rule)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 0, "ok", map[string]any{"id": id})
}

func (h *RiskBehaviorHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
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
	if err := ValidateRiskRule(rule); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
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
	tenant, corp, ok := h.resolve(w, r, "/ai-insight/v2/risk#manage")
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
	updated, err := writer.SetRiskRuleStatus(r.Context(), tenant, corp, input.ID, input.Status)
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
	tenant, corp, ok := h.resolve(w, r, "/ai-insight/v2/risk#manage")
	if !ok {
		return
	}
	writer, ok := h.provider.(RiskRuleProviderWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "风险规则写入能力未启用", nil)
		return
	}
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	deleted, err := writer.DeleteRiskRule(r.Context(), tenant, corp, id)
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
