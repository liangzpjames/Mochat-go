package dashboard

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
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
	assignedEmployeeID, _ := strconv.ParseInt(r.URL.Query().Get("assignedEmployeeId"), 10, 64)
	v, e := h.provider.SilentRecordPage(r.Context(), SilentRecordFilter{TenantID: t, CorpID: c, Customer: r.URL.Query().Get("customer"), Status: r.URL.Query().Get("status"), AssignedEmployeeID: assignedEmployeeID, RuleID: rid, Page: p, PerPage: n, AllowedEmployeeIDs: append([]int(nil), access.AllowedEmployeeIDs...), RestrictEmployeeIDs: access.ScopeRequired && access.Scope != DataScopeTenant})
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
	if access, scoped := DashboardAccessFromContext(r.Context()); scoped && access.ScopeRequired && access.Scope != DataScopeTenant {
		writeMachineEnvelope(w, http.StatusForbidden, DashboardPermissionDeniedCode, "dashboard permission denied", nil)
		return
	}
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
	refusedFrom, refusedTo, dateErr := refuseArchiveDateRange(r)
	if dateErr != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, dateErr.Error(), nil)
		return
	}
	employeeID := int64(0)
	if rawEmployeeID := strings.TrimSpace(r.URL.Query().Get("employeeId")); rawEmployeeID != "" {
		parsed, err := strconv.ParseInt(rawEmployeeID, 10, 64)
		if err != nil || parsed <= 0 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "关联员工参数无效", nil)
			return
		}
		employeeID = parsed
	}
	v, e := h.provider.RefuseArchivePage(r.Context(), RefuseArchiveFilter{
		TenantID: t, CorpID: c, Subject: r.URL.Query().Get("subject"), SubjectType: r.URL.Query().Get("subjectType"), EmployeeID: employeeID,
		RefusedFrom: refusedFrom, RefusedTo: refusedTo, AuthorizationStatus: r.URL.Query().Get("authorizationStatus"), FollowUpStatus: r.URL.Query().Get("followUpStatus"), Page: p, PerPage: n,
	})
	if e != nil {
		writeEnvelope(w, 500, 500, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", v)
}

func refuseArchiveDateRange(r *http.Request) (string, string, error) {
	from, to := strings.TrimSpace(r.URL.Query().Get("refusedFrom")), strings.TrimSpace(r.URL.Query().Get("refusedTo"))
	for _, value := range []string{from, to} {
		if value == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return "", "", errors.New("拒绝日期格式必须为 YYYY-MM-DD")
		}
	}
	if from != "" && to != "" && from > to {
		return "", "", errors.New("拒绝开始日期不能晚于结束日期")
	}
	return from, to, nil
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
