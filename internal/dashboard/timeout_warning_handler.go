package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type TimeoutWarningHandler struct {
	provider   TimeoutWarningProvider
	authorizer CorpAdminAuthorizer
}

func NewTimeoutWarningHandler(provider TimeoutWarningProvider, _ LoginCache, _ UserIDResolver, authorizer CorpAdminAuthorizer) *TimeoutWarningHandler {
	return &TimeoutWarningHandler{provider: provider, authorizer: authorizer}
}
func (h *TimeoutWarningHandler) resolve(w http.ResponseWriter, r *http.Request, permission string) (int, int, bool) {
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
func (h *TimeoutWarningHandler) Rules(w http.ResponseWriter, r *http.Request) {
	tenant, corp, ok := h.resolve(w, r, "/ai-insight/v2/timeout#read")
	if !ok {
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	per, _ := strconv.Atoi(r.URL.Query().Get("perPage"))
	v, err := h.provider.TimeoutRulePage(r.Context(), TimeoutRuleFilter{TenantID: tenant, CorpID: corp, Name: r.URL.Query().Get("name"), Page: page, PerPage: per})
	if err != nil {
		writeEnvelope(w, 500, 500, err.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", v)
}
func (h *TimeoutWarningHandler) Records(w http.ResponseWriter, r *http.Request) {
	tenant, corp, ok := h.resolve(w, r, "/ai-insight/v2/timeout#read")
	if !ok {
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	per, _ := strconv.Atoi(r.URL.Query().Get("perPage"))
	ruleID, _ := strconv.ParseInt(r.URL.Query().Get("ruleId"), 10, 64)
	access, _ := DashboardAccessFromContext(r.Context())
	v, err := h.provider.TimeoutRecordPage(r.Context(), TimeoutRecordFilter{TenantID: tenant, CorpID: corp, Customer: r.URL.Query().Get("customer"), RiskLevel: r.URL.Query().Get("riskLevel"), ConversationType: r.URL.Query().Get("conversationType"), AuditStatus: r.URL.Query().Get("auditStatus"), RuleID: ruleID, Page: page, PerPage: per, AllowedEmployeeIDs: append([]int(nil), access.AllowedEmployeeIDs...), RestrictEmployeeIDs: access.ScopeRequired && access.Scope != DataScopeTenant})
	if err != nil {
		writeEnvelope(w, 500, 500, err.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", v)
}
func (h *TimeoutWarningHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	tenant, corp, ok := h.resolve(w, r, "/ai-insight/v2/timeout#manage")
	if !ok {
		return
	}
	writer, ok := h.provider.(TimeoutRuleProviderWriter)
	if !ok {
		writeEnvelope(w, 501, 501, "超时规则写入能力未启用", nil)
		return
	}
	var v TimeoutRule
	if json.NewDecoder(r.Body).Decode(&v) != nil {
		writeEnvelope(w, 400, 400, "规则参数无效", nil)
		return
	}
	v.TenantID = int64(tenant)
	v.CorpID = int64(corp)
	id, err := writer.CreateTimeoutRule(r.Context(), v)
	if err != nil {
		writeEnvelope(w, 400, 400, err.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"id": id})
}
func (h *TimeoutWarningHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	tenant, corp, ok := h.resolve(w, r, "/ai-insight/v2/timeout#manage")
	if !ok {
		return
	}
	writer, ok := h.provider.(TimeoutRuleProviderWriter)
	if !ok {
		writeEnvelope(w, 501, 501, "超时规则写入能力未启用", nil)
		return
	}
	var v TimeoutRule
	if json.NewDecoder(r.Body).Decode(&v) != nil {
		writeEnvelope(w, 400, 400, "规则参数无效", nil)
		return
	}
	v.TenantID = int64(tenant)
	v.CorpID = int64(corp)
	updated, err := writer.UpdateTimeoutRule(r.Context(), v)
	if err != nil {
		writeEnvelope(w, 400, 400, err.Error(), nil)
		return
	}
	if !updated {
		writeEnvelope(w, 404, 404, "规则不存在", nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"updated": true})
}
func (h *TimeoutWarningHandler) RuleStatus(w http.ResponseWriter, r *http.Request) {
	tenant, corp, ok := h.resolve(w, r, "/ai-insight/v2/timeout#manage")
	if !ok {
		return
	}
	writer, ok := h.provider.(TimeoutRuleProviderWriter)
	if !ok {
		writeEnvelope(w, 501, 501, "超时规则写入能力未启用", nil)
		return
	}
	var input struct {
		ID     int64             `json:"id"`
		Status TimeoutRuleStatus `json:"status"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		writeEnvelope(w, 400, 400, "规则参数无效", nil)
		return
	}
	updated, err := writer.SetTimeoutRuleStatus(r.Context(), tenant, corp, input.ID, input.Status)
	if err != nil {
		writeEnvelope(w, 400, 400, err.Error(), nil)
		return
	}
	if !updated {
		writeEnvelope(w, 404, 404, "规则不存在", nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"updated": true})
}
func (h *TimeoutWarningHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	tenant, corp, ok := h.resolve(w, r, "/ai-insight/v2/timeout#manage")
	if !ok {
		return
	}
	writer, ok := h.provider.(TimeoutRuleProviderWriter)
	if !ok {
		writeEnvelope(w, 501, 501, "超时规则写入能力未启用", nil)
		return
	}
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	deleted, err := writer.DeleteTimeoutRule(r.Context(), tenant, corp, id)
	if err != nil {
		writeEnvelope(w, 409, 409, err.Error(), nil)
		return
	}
	if !deleted {
		writeEnvelope(w, 404, 404, "规则不存在", nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"deleted": true})
}
func (h *TimeoutWarningHandler) Settings(w http.ResponseWriter, r *http.Request) {
	tenant, corp, ok := h.resolve(w, r, "/ai-insight/v2/timeout#read")
	if !ok {
		return
	}
	v, err := h.provider.TimeoutSettings(r.Context(), tenant, corp)
	if err != nil {
		writeEnvelope(w, 500, 500, err.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", v)
}
func (h *TimeoutWarningHandler) SaveSettings(w http.ResponseWriter, r *http.Request) {
	tenant, corp, ok := h.resolve(w, r, "/ai-insight/v2/timeout#manage")
	if !ok {
		return
	}
	writer, ok := h.provider.(TimeoutSettingsProviderWriter)
	if !ok {
		writeEnvelope(w, 501, 501, "超时设置写入能力未启用", nil)
		return
	}
	var v TimeoutSettings
	if json.NewDecoder(r.Body).Decode(&v) != nil {
		writeEnvelope(w, 400, 400, "设置参数无效", nil)
		return
	}
	v.TenantID = int64(tenant)
	v.CorpID = int64(corp)
	if err := writer.SaveTimeoutSettings(r.Context(), v); err != nil {
		writeEnvelope(w, 400, 400, err.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"updated": true})
}
func (h *TimeoutWarningHandler) AuditRecords(w http.ResponseWriter, r *http.Request) {
	if access, ok := DashboardAccessFromContext(r.Context()); ok && access.ScopeRequired && access.Scope != DataScopeTenant {
		writeMachineEnvelope(w, http.StatusForbidden, DashboardPermissionDeniedCode, "dashboard permission denied", nil)
		return
	}
	tenant, corp, ok := h.resolve(w, r, "/ai-insight/v2/timeout#manage")
	if !ok {
		return
	}
	writer, ok := h.provider.(TimeoutRecordProviderWriter)
	if !ok {
		writeEnvelope(w, 501, 501, "超时记录处置能力未启用", nil)
		return
	}
	identity, _ := ResolveDashboardHandlerIdentity(r.Context())
	actor := identity.UserID
	var input struct {
		IDs    []int64 `json:"ids"`
		Action string  `json:"action"`
		Remark string  `json:"remark"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		writeEnvelope(w, 400, 400, "处置参数无效", nil)
		return
	}
	count, err := writer.AuditTimeoutRecords(r.Context(), tenant, corp, actor, input.IDs, input.Action, input.Remark)
	if err != nil {
		writeEnvelope(w, 400, 400, err.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"updated": count})
}
func (h *TimeoutWarningHandler) AssignRecords(w http.ResponseWriter, r *http.Request) {
	if access, ok := DashboardAccessFromContext(r.Context()); ok && access.ScopeRequired && access.Scope != DataScopeTenant {
		writeMachineEnvelope(w, http.StatusForbidden, DashboardPermissionDeniedCode, "dashboard permission denied", nil)
		return
	}
	tenant, corp, ok := h.resolve(w, r, "/ai-insight/v2/timeout#manage")
	if !ok {
		return
	}
	writer, ok := h.provider.(TimeoutRecordProviderWriter)
	if !ok {
		writeEnvelope(w, 501, 501, "超时记录处置能力未启用", nil)
		return
	}
	identity, _ := ResolveDashboardHandlerIdentity(r.Context())
	actor := identity.UserID
	var input struct {
		IDs        []int64 `json:"ids"`
		EmployeeID int64   `json:"employeeId"`
		Remark     string  `json:"remark"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		writeEnvelope(w, 400, 400, "分派参数无效", nil)
		return
	}
	count, err := writer.AssignTimeoutRecords(r.Context(), tenant, corp, actor, input.IDs, input.EmployeeID, input.Remark)
	if err != nil {
		writeEnvelope(w, 400, 400, err.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"updated": count})
}
func (h *TimeoutWarningHandler) Evaluate(w http.ResponseWriter, r *http.Request) {
	tenant, corp, ok := h.resolve(w, r, "/ai-insight/v2/timeout#manage")
	if !ok {
		return
	}
	provider, ok := h.provider.(TimeoutEvaluatorProvider)
	if !ok {
		writeEnvelope(w, 501, 501, "超时评估能力未启用", nil)
		return
	}
	var v TimeoutEvaluation
	if json.NewDecoder(r.Body).Decode(&v) != nil {
		writeEnvelope(w, 400, 400, "评估参数无效", nil)
		return
	}
	v.TenantID = tenant
	v.CorpID = corp
	created, err := provider.EvaluateTimeoutMessage(r.Context(), v)
	if err != nil {
		writeEnvelope(w, 400, 400, err.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"created": created})
}
