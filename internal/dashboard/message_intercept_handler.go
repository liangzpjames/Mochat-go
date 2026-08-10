package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type MessageInterceptHandler struct {
	provider   MessageInterceptProvider
	cache      LoginCache
	resolver   UserIDResolver
	authorizer CorpAdminAuthorizer
}

func NewMessageInterceptHandler(p MessageInterceptProvider, c LoginCache, r UserIDResolver, a CorpAdminAuthorizer) *MessageInterceptHandler {
	return &MessageInterceptHandler{p, c, r, a}
}
func (h *MessageInterceptHandler) resolve(w http.ResponseWriter, r *http.Request, permission string) (int, int, int64, bool) {
	uid, e := h.resolver.UserID(r)
	if e != nil {
		writeEnvelope(w, 401, 401, e.Error(), nil)
		return 0, 0, 0, false
	}
	cached, e := h.cache.UserCorpCache(r.Context(), uid)
	if e != nil {
		writeEnvelope(w, 401, 401, "登录已失效", nil)
		return 0, 0, 0, false
	}
	parts := strings.Split(cached, "-")
	corp, _ := strconv.Atoi(r.URL.Query().Get("corpId"))
	emp := 0
	if corp <= 0 && len(parts) > 0 {
		corp, _ = strconv.Atoi(parts[0])
	}
	if len(parts) > 1 {
		emp, _ = strconv.Atoi(parts[1])
	}
	if h.authorizer != nil {
		if _, e = h.authorizer.Resolve(r.Context(), uid, permission, corp, emp); e != nil {
			writeEnvelope(w, 403, 403, "无权访问消息拦截与关键词库", nil)
			return 0, 0, 0, false
		}
	}
	tenant := 0
	if tr, ok := h.provider.(RiskTenantResolver); ok {
		tenant, e = tr.TenantIDByCorpID(r.Context(), corp)
		if e != nil {
			writeEnvelope(w, 500, 500, e.Error(), nil)
			return 0, 0, 0, false
		}
	}
	return tenant, corp, int64(emp), true
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if e := json.NewDecoder(r.Body).Decode(v); e != nil {
		writeEnvelope(w, 400, 400, "请求参数无效", nil)
		return false
	}
	return true
}
func idQuery(r *http.Request) int64 {
	v, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	return v
}
func pageQuery(r *http.Request) (int, int) {
	p, _ := strconv.Atoi(r.URL.Query().Get("page"))
	n, _ := strconv.Atoi(r.URL.Query().Get("perPage"))
	return p, n
}

func (h *MessageInterceptHandler) Libraries(w http.ResponseWriter, r *http.Request) {
	t, c, _, ok := h.resolve(w, r, "/ai-insight/v2/keyword-library#read")
	if !ok {
		return
	}
	p, n := pageQuery(r)
	v, e := h.provider.KeywordLibraryPage(r.Context(), KeywordLibraryFilter{TenantID: t, CorpID: c, Name: r.URL.Query().Get("name"), Status: r.URL.Query().Get("status"), Page: p, PerPage: n})
	if e != nil {
		writeEnvelope(w, 500, 500, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", v)
}
func (h *MessageInterceptHandler) SaveLibrary(w http.ResponseWriter, r *http.Request) {
	t, c, _, ok := h.resolve(w, r, "/ai-insight/v2/keyword-library#manage")
	if !ok {
		return
	}
	var v KeywordLibrary
	if !decode(w, r, &v) {
		return
	}
	v.TenantID = int64(t)
	v.CorpID = int64(c)
	writer, ok := h.provider.(KeywordLibraryWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "关键词库写入能力未启用", nil)
		return
	}
	id, e := writer.SaveKeywordLibrary(r.Context(), v)
	if e != nil {
		writeEnvelope(w, 400, 400, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"id": id})
}
func (h *MessageInterceptHandler) LibraryStatus(w http.ResponseWriter, r *http.Request) {
	t, c, _, ok := h.resolve(w, r, "/ai-insight/v2/keyword-library#manage")
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
	writer, ok := h.provider.(KeywordLibraryWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "关键词库写入能力未启用", nil)
		return
	}
	yes, e := writer.SetKeywordLibraryStatus(r.Context(), t, c, v.ID, v.Status)
	if e != nil {
		writeEnvelope(w, 400, 400, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"updated": yes})
}
func (h *MessageInterceptHandler) DeleteLibrary(w http.ResponseWriter, r *http.Request) {
	t, c, _, ok := h.resolve(w, r, "/ai-insight/v2/keyword-library#manage")
	if !ok {
		return
	}
	writer, ok := h.provider.(KeywordLibraryWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "关键词库写入能力未启用", nil)
		return
	}
	yes, e := writer.DeleteKeywordLibrary(r.Context(), t, c, idQuery(r))
	if e != nil {
		writeEnvelope(w, 400, 400, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"deleted": yes})
}
func (h *MessageInterceptHandler) PublishLibrary(w http.ResponseWriter, r *http.Request) {
	t, c, a, ok := h.resolve(w, r, "/ai-insight/v2/keyword-library#publish")
	if !ok {
		return
	}
	var v struct {
		ID int64 `json:"id"`
	}
	if !decode(w, r, &v) {
		return
	}
	writer, ok := h.provider.(KeywordLibraryWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "关键词库发布能力未启用", nil)
		return
	}
	version, e := writer.PublishKeywordLibrary(r.Context(), t, c, v.ID, a)
	if e != nil {
		writeEnvelope(w, 400, 400, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"version": version})
}
func (h *MessageInterceptHandler) Entries(w http.ResponseWriter, r *http.Request) {
	t, c, _, ok := h.resolve(w, r, "/ai-insight/v2/keyword-library#read")
	if !ok {
		return
	}
	p, n := pageQuery(r)
	lib, _ := strconv.ParseInt(r.URL.Query().Get("libraryId"), 10, 64)
	v, e := h.provider.KeywordEntryPage(r.Context(), KeywordEntryFilter{TenantID: t, CorpID: c, LibraryID: lib, Keyword: r.URL.Query().Get("keyword"), Status: r.URL.Query().Get("status"), Page: p, PerPage: n})
	if e != nil {
		writeEnvelope(w, 500, 500, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", v)
}
func (h *MessageInterceptHandler) SaveEntry(w http.ResponseWriter, r *http.Request) {
	t, c, _, ok := h.resolve(w, r, "/ai-insight/v2/keyword-library#manage")
	if !ok {
		return
	}
	var v KeywordEntry
	if !decode(w, r, &v) {
		return
	}
	writer, ok := h.provider.(KeywordLibraryWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "关键词库写入能力未启用", nil)
		return
	}
	id, e := writer.SaveKeywordEntry(r.Context(), t, c, v)
	if e != nil {
		writeEnvelope(w, 400, 400, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"id": id})
}
func (h *MessageInterceptHandler) EntryStatus(w http.ResponseWriter, r *http.Request) {
	t, c, _, ok := h.resolve(w, r, "/ai-insight/v2/keyword-library#manage")
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
	writer, ok := h.provider.(KeywordLibraryWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "关键词库写入能力未启用", nil)
		return
	}
	yes, e := writer.SetKeywordEntryStatus(r.Context(), t, c, v.ID, v.Status)
	if e != nil {
		writeEnvelope(w, 400, 400, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"updated": yes})
}
func (h *MessageInterceptHandler) DeleteEntry(w http.ResponseWriter, r *http.Request) {
	t, c, _, ok := h.resolve(w, r, "/ai-insight/v2/keyword-library#manage")
	if !ok {
		return
	}
	writer, ok := h.provider.(KeywordLibraryWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "关键词库写入能力未启用", nil)
		return
	}
	yes, e := writer.DeleteKeywordEntry(r.Context(), t, c, idQuery(r))
	if e != nil {
		writeEnvelope(w, 400, 400, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"deleted": yes})
}
func (h *MessageInterceptHandler) Rules(w http.ResponseWriter, r *http.Request) {
	t, c, _, ok := h.resolve(w, r, "/ai-insight/v2/message-intercept#read")
	if !ok {
		return
	}
	p, n := pageQuery(r)
	v, e := h.provider.MessageInterceptRulePage(r.Context(), MessageInterceptRuleFilter{TenantID: t, CorpID: c, Name: r.URL.Query().Get("name"), Status: r.URL.Query().Get("status"), Page: p, PerPage: n})
	if e != nil {
		writeEnvelope(w, 500, 500, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", v)
}
func (h *MessageInterceptHandler) SaveRule(w http.ResponseWriter, r *http.Request) {
	t, c, _, ok := h.resolve(w, r, "/ai-insight/v2/message-intercept#manage")
	if !ok {
		return
	}
	var v MessageInterceptRule
	if !decode(w, r, &v) {
		return
	}
	v.TenantID = int64(t)
	v.CorpID = int64(c)
	writer, ok := h.provider.(MessageInterceptWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "消息拦截写入能力未启用", nil)
		return
	}
	id, e := writer.SaveMessageInterceptRule(r.Context(), v)
	if e != nil {
		writeEnvelope(w, 400, 400, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"id": id})
}
func (h *MessageInterceptHandler) RuleStatus(w http.ResponseWriter, r *http.Request) {
	t, c, _, ok := h.resolve(w, r, "/ai-insight/v2/message-intercept#manage")
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
	writer, ok := h.provider.(MessageInterceptWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "消息拦截写入能力未启用", nil)
		return
	}
	yes, e := writer.SetMessageInterceptRuleStatus(r.Context(), t, c, v.ID, v.Status)
	if e != nil {
		writeEnvelope(w, 400, 400, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"updated": yes})
}
func (h *MessageInterceptHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	t, c, _, ok := h.resolve(w, r, "/ai-insight/v2/message-intercept#manage")
	if !ok {
		return
	}
	writer, ok := h.provider.(MessageInterceptWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "消息拦截写入能力未启用", nil)
		return
	}
	yes, e := writer.DeleteMessageInterceptRule(r.Context(), t, c, idQuery(r))
	if e != nil {
		writeEnvelope(w, 400, 400, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"deleted": yes})
}
func (h *MessageInterceptHandler) Records(w http.ResponseWriter, r *http.Request) {
	t, c, _, ok := h.resolve(w, r, "/ai-insight/v2/message-intercept#read")
	if !ok {
		return
	}
	if access, scoped := DashboardAccessFromContext(r.Context()); scoped && access.ScopeRequired && access.Scope != DataScopeTenant {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "message intercept records are not safely employee-scoped", map[string]any{"code": DashboardPermissionDeniedCode})
		return
	}
	p, n := pageQuery(r)
	rid, _ := strconv.ParseInt(r.URL.Query().Get("ruleId"), 10, 64)
	v, e := h.provider.MessageInterceptRecordPage(r.Context(), MessageInterceptRecordFilter{TenantID: t, CorpID: c, RuleID: rid, Keyword: r.URL.Query().Get("keyword"), Decision: r.URL.Query().Get("decision"), AuditStatus: r.URL.Query().Get("auditStatus"), Page: p, PerPage: n})
	if e != nil {
		writeEnvelope(w, 500, 500, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", v)
}
func (h *MessageInterceptHandler) Evaluate(w http.ResponseWriter, r *http.Request) {
	t, c, _, ok := h.resolve(w, r, "/ai-insight/v2/message-intercept#evaluate")
	if !ok {
		return
	}
	var v MessageInterceptEvaluation
	if !decode(w, r, &v) {
		return
	}
	writer, ok := h.provider.(MessageInterceptWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "消息拦截评估能力未启用", nil)
		return
	}
	out, e := writer.EvaluateMessageIntercept(r.Context(), t, c, v)
	if e != nil {
		writeEnvelope(w, 400, 400, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", out)
}
func (h *MessageInterceptHandler) Audit(w http.ResponseWriter, r *http.Request) {
	t, c, a, ok := h.resolve(w, r, "/ai-insight/v2/message-intercept#audit")
	if !ok {
		return
	}
	if access, scoped := DashboardAccessFromContext(r.Context()); scoped && access.ScopeRequired && access.Scope != DataScopeTenant {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "message intercept audit is not safely employee-scoped", map[string]any{"code": DashboardPermissionDeniedCode})
		return
	}
	var v struct {
		IDs    []int64 `json:"ids"`
		Action string  `json:"action"`
		Remark string  `json:"remark"`
	}
	if !decode(w, r, &v) {
		return
	}
	writer, ok := h.provider.(MessageInterceptWriter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "消息拦截审计能力未启用", nil)
		return
	}
	n, e := writer.AuditMessageInterceptRecords(r.Context(), t, c, a, v.IDs, v.Action, v.Remark)
	if e != nil {
		writeEnvelope(w, 400, 400, e.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 0, "ok", map[string]any{"updated": n})
}
