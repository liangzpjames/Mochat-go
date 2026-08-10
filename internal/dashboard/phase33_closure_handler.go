package dashboard

import (
	"net/http"
	"strconv"
)

type Phase33ClosureHandler struct {
	base     *MessageInterceptHandler
	provider Phase33ClosureProvider
}

func NewPhase33ClosureHandler(p interface {
	MessageInterceptProvider
	Phase33ClosureProvider
}, c LoginCache, r UserIDResolver, a CorpAdminAuthorizer) *Phase33ClosureHandler {
	return &Phase33ClosureHandler{base: NewMessageInterceptHandler(p, c, r, a), provider: p}
}
func (h *Phase33ClosureHandler) SilentRules(w http.ResponseWriter, r *http.Request) {
	t, c, _, ok := h.base.resolve(w, r, "/ai-insight/v2/silent-customer#read")
	if !ok {
		return
	}
	p, n := pageQuery(r)
	v, e := h.provider.SilentRulePage(r.Context(), SilentRuleFilter{TenantID: t, CorpID: c, Name: r.URL.Query().Get("name"), Status: r.URL.Query().Get("status"), Page: p, PerPage: n})
	if e != nil {
		writeEnvelope(w, 500, 500, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", v)
}
func (h *Phase33ClosureHandler) SaveSilentRule(w http.ResponseWriter, r *http.Request) {
	t, c, _, ok := h.base.resolve(w, r, "/ai-insight/v2/silent-customer#manage")
	if !ok {
		return
	}
	var v SilentCustomerRule
	if !decode(w, r, &v) {
		return
	}
	v.TenantID = int64(t)
	v.CorpID = int64(c)
	writer, ok := h.provider.(Phase33ClosureWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "沉默客户写入能力未启用", nil)
		return
	}
	id, e := writer.SaveSilentRule(r.Context(), v)
	if e != nil {
		writeEnvelope(w, 400, 400, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"id": id})
}
func (h *Phase33ClosureHandler) SilentRuleStatus(w http.ResponseWriter, r *http.Request) {
	t, c, _, ok := h.base.resolve(w, r, "/ai-insight/v2/silent-customer#manage")
	if !ok {
		return
	}
	var v struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	if !decode(w, r, &v) {
		return
	}
	writer, ok := h.provider.(Phase33ClosureWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "沉默客户写入能力未启用", nil)
		return
	}
	yes, e := writer.SetSilentRuleStatus(r.Context(), t, c, v.ID, v.Status)
	if e != nil {
		writeEnvelope(w, 400, 400, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"updated": yes})
}
func (h *Phase33ClosureHandler) DeleteSilentRule(w http.ResponseWriter, r *http.Request) {
	t, c, _, ok := h.base.resolve(w, r, "/ai-insight/v2/silent-customer#manage")
	if !ok {
		return
	}
	writer, ok := h.provider.(Phase33ClosureWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "沉默客户写入能力未启用", nil)
		return
	}
	yes, e := writer.DeleteSilentRule(r.Context(), t, c, idQuery(r))
	if e != nil {
		writeEnvelope(w, 400, 400, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"deleted": yes})
}
func (h *Phase33ClosureHandler) SilentRecords(w http.ResponseWriter, r *http.Request) {
	t, c, _, ok := h.base.resolve(w, r, "/ai-insight/v2/silent-customer#read")
	if !ok {
		return
	}
	p, n := pageQuery(r)
	rid, _ := strconv.ParseInt(r.URL.Query().Get("ruleId"), 10, 64)
	access, _ := DashboardAccessFromContext(r.Context())
	v, e := h.provider.SilentRecordPage(r.Context(), SilentRecordFilter{TenantID: t, CorpID: c, Customer: r.URL.Query().Get("customer"), Status: r.URL.Query().Get("status"), RuleID: rid, Page: p, PerPage: n, AllowedEmployeeIDs: append([]int(nil), access.AllowedEmployeeIDs...), RestrictEmployeeIDs: access.ScopeRequired && access.Scope != DataScopeTenant})
	if e != nil {
		writeEnvelope(w, 500, 500, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", v)
}
func (h *Phase33ClosureHandler) EvaluateSilent(w http.ResponseWriter, r *http.Request) {
	t, c, _, ok := h.base.resolve(w, r, "/ai-insight/v2/silent-customer#evaluate")
	if !ok {
		return
	}
	var v SilentCustomerActivity
	if !decode(w, r, &v) {
		return
	}
	writer, ok := h.provider.(Phase33ClosureWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "沉默客户评估能力未启用", nil)
		return
	}
	n, e := writer.EvaluateSilentCustomer(r.Context(), t, c, v)
	if e != nil {
		writeEnvelope(w, 400, 400, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"matched": n})
}
func (h *Phase33ClosureHandler) ActSilent(w http.ResponseWriter, r *http.Request) {
	t, c, a, ok := h.base.resolve(w, r, "/ai-insight/v2/silent-customer#manage")
	if !ok {
		return
	}
	var v struct {
		IDs                []int64 `json:"ids"`
		Action             string  `json:"action"`
		AssignedEmployeeID int64   `json:"assignedEmployeeId"`
		Remark             string  `json:"remark"`
	}
	if !decode(w, r, &v) {
		return
	}
	writer, ok := h.provider.(Phase33ClosureWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "沉默客户处置能力未启用", nil)
		return
	}
	n, e := writer.ActSilentRecords(r.Context(), t, c, a, v.IDs, v.Action, v.AssignedEmployeeID, v.Remark)
	if e != nil {
		writeEnvelope(w, 400, 400, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"updated": n})
}
func (h *Phase33ClosureHandler) RefuseRecords(w http.ResponseWriter, r *http.Request) {
	t, c, _, ok := h.base.resolve(w, r, "/chat/refuse-archive#read")
	if !ok {
		return
	}
	p, n := pageQuery(r)
	v, e := h.provider.RefuseArchivePage(r.Context(), RefuseArchiveFilter{TenantID: t, CorpID: c, Subject: r.URL.Query().Get("subject"), AuthorizationStatus: r.URL.Query().Get("authorizationStatus"), FollowUpStatus: r.URL.Query().Get("followUpStatus"), Page: p, PerPage: n})
	if e != nil {
		writeEnvelope(w, 500, 500, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", v)
}
func (h *Phase33ClosureHandler) SyncRefuse(w http.ResponseWriter, r *http.Request) {
	t, c, a, ok := h.base.resolve(w, r, "/chat/refuse-archive#sync")
	if !ok {
		return
	}
	var v RefuseArchiveRecord
	if !decode(w, r, &v) {
		return
	}
	writer, ok := h.provider.(Phase33ClosureWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "拒绝存档同步能力未启用", nil)
		return
	}
	id, e := writer.UpsertRefuseArchive(r.Context(), t, c, a, v)
	if e != nil {
		writeEnvelope(w, 400, 400, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"id": id})
}
func (h *Phase33ClosureHandler) FollowRefuse(w http.ResponseWriter, r *http.Request) {
	t, c, a, ok := h.base.resolve(w, r, "/chat/refuse-archive#manage")
	if !ok {
		return
	}
	var v struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if !decode(w, r, &v) {
		return
	}
	writer, ok := h.provider.(Phase33ClosureWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "拒绝存档跟进能力未启用", nil)
		return
	}
	yes, e := writer.FollowUpRefuseArchive(r.Context(), t, c, a, v.ID, v.Status, v.Note)
	if e != nil {
		writeEnvelope(w, 400, 400, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"updated": yes})
}
