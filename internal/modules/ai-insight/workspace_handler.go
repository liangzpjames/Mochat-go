package aiinsight

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	settingsports "jiyi/mochat-go/internal/modules/ai-settings/ports"
	"jiyi/mochat-go/internal/modules/providers"
)

type WorkspacePrincipal struct {
	UserID                  int64
	TenantID                int64
	CorpID                  int64
	AllowedEmployeeIDs      []int64
	EmployeeScopeRestricted bool
}
type WorkspacePrincipalResolver interface {
	Resolve(*http.Request) (WorkspacePrincipal, error)
}
type WorkspaceAuthorizer interface {
	Authorize(context.Context, WorkspacePrincipal, int64, string) error
}

type WorkspaceHandler struct {
	principal       WorkspacePrincipalResolver
	authorize       WorkspaceAuthorizer
	repo            Repository
	ai              providers.AIProvider
	assistant       AssistantContextProvider
	systemAssistant SystemAssistantContextProvider
}

func NewWorkspaceHandler(principal WorkspacePrincipalResolver, authorize WorkspaceAuthorizer, repo Repository, ai providers.AIProvider, assistants ...any) *WorkspaceHandler {
	var assistant AssistantContextProvider
	var systemAssistant SystemAssistantContextProvider
	if len(assistants) > 0 {
		assistant, _ = assistants[0].(AssistantContextProvider)
		systemAssistant, _ = assistants[0].(SystemAssistantContextProvider)
	}
	return &WorkspaceHandler{principal: principal, authorize: authorize, repo: repo, ai: ai, assistant: assistant, systemAssistant: systemAssistant}
}

func (h *WorkspaceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal.Resolve(r)
	if err != nil || p.UserID <= 0 || p.TenantID <= 0 || p.CorpID <= 0 {
		workspaceEnvelope(w, 401, "principal unauthorized", nil)
		return
	}
	page, action := workspacePath(r.URL.Path)
	if page != "session-analysis" && page != "smart-analysis" {
		workspaceEnvelope(w, 404, "page not found", nil)
		return
	}
	if h.authorize != nil {
		permission := "/ai-insight/" + page + "#read"
		if action == "export" {
			permission += ".export"
		}
		if strings.HasPrefix(action, "rule") {
			permission += ".rule.manage"
		}
		if err := h.authorize.Authorize(r.Context(), p, p.CorpID, permission); err != nil {
			workspaceEnvelope(w, 403, "forbidden", nil)
			return
		}
	}
	if h.repo == nil {
		workspaceEnvelope(w, 503, "AI 洞察数据服务暂不可用", nil)
		return
	}
	switch action {
	case "records", "":
		h.records(w, r, p, page)
	case "detail":
		h.detail(w, r, p, page)
	case "status":
		h.status(w, r, p, page)
	case "export":
		h.export(w, r, p, page)
	case "rules":
		h.rules(w, r, p)
	case "rule-status":
		h.ruleStatus(w, r, p)
	default:
		workspaceEnvelope(w, 404, "resource not found", nil)
	}
}

func (h *WorkspaceHandler) records(w http.ResponseWriter, r *http.Request, p WorkspacePrincipal, page string) {
	filter, err := parseWorkspaceFilter(r, p, page)
	if err != nil {
		workspaceEnvelope(w, 400, err.Error(), nil)
		return
	}
	result, err := h.repo.InsightPage(r.Context(), filter)
	if err != nil {
		workspaceRepoError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, workspaceInsightJSON(item))
	}
	workspaceEnvelope(w, 200, "success", map[string]any{"page": result.Page, "pageSize": result.PageSize, "total": result.Total, "items": items})
}

func (h *WorkspaceHandler) detail(w http.ResponseWriter, r *http.Request, p WorkspacePrincipal, page string) {
	id, err := workspacePositiveID(r, "id")
	if err != nil {
		workspaceEnvelope(w, 400, "详情 ID 无效", nil)
		return
	}
	typ := AnalysisTypeSession
	if page == "smart-analysis" {
		typ = AnalysisTypeSmart
	}
	item, err := h.repo.InsightDetail(r.Context(), InsightDetailFilter{TenantID: p.TenantID, CorpID: p.CorpID, AnalysisType: typ, ID: id, AllowedEmployeeIDs: p.AllowedEmployeeIDs, Restricted: p.EmployeeScopeRestricted})
	if err != nil {
		workspaceRepoError(w, err)
		return
	}
	messages, _ := h.repo.ConversationMessages(r.Context(), ConversationWindowQuery{TenantID: p.TenantID, CorpID: p.CorpID, ConversationKey: item.ConversationKey, StartAt: item.SourceStartedAt, EndAt: item.SourceEndedAt, Limit: 200, AllowedEmployeeIDs: p.AllowedEmployeeIDs, Restricted: p.EmployeeScopeRestricted})
	data := workspaceInsightJSON(item)
	data["messages"] = workspaceMessagesJSON(messages)
	data["conversationUrl"] = fmt.Sprintf("/chat/v2-%s?employeeId=%d&conversationId=%s", map[bool]string{true: "customer", false: "staff"}[item.TargetType == "1"], item.EmployeeID, item.ConversationKey)
	workspaceEnvelope(w, 200, "success", data)
}

func (h *WorkspaceHandler) status(w http.ResponseWriter, r *http.Request, p WorkspacePrincipal, page string) {
	typ := AnalysisTypeSession
	if page == "smart-analysis" {
		typ = AnalysisTypeSmart
	}
	run, err := h.repo.LatestRun(r.Context(), p.TenantID, p.CorpID, typ)
	if err != nil {
		workspaceRepoError(w, err)
		return
	}
	provider := map[string]any{"state": "unavailable", "message": "尚未运行"}
	if h.ai != nil {
		status := h.ai.Status()
		provider = map[string]any{"state": string(status.State), "source": string(status.Source), "code": status.Code, "message": status.Reason}
	}
	data := map[string]any{"provider": provider}
	if h.systemAssistant != nil {
		if _, err := h.systemAssistant.EnsureSystemAssistants(r.Context(), p.TenantID, p.CorpID, p.UserID, fmt.Sprintf("session-%d-%d", p.TenantID, p.CorpID), fmt.Sprintf("smart-%d-%d", p.TenantID, p.CorpID)); err != nil {
			workspaceRepoError(w, err)
			return
		}
		key := settingsports.SessionAnalysisSystemKey
		if page == "smart-analysis" {
			key = settingsports.SmartAnalysisSystemKey
		}
		assistant, err := h.systemAssistant.LoadSystemAssistantContext(r.Context(), p.TenantID, p.CorpID, key)
		if err != nil {
			workspaceRepoError(w, err)
			return
		}
		data["assistant"] = workspaceAssistantJSON(assistant)
	} else if h.assistant != nil && page == "session-analysis" {
		if _, err := h.assistant.EnsureSessionAssistant(r.Context(), p.TenantID, p.CorpID, p.UserID, fmt.Sprintf("session-%d-%d", p.TenantID, p.CorpID)); err != nil {
			workspaceRepoError(w, err)
			return
		}
		assistant, err := h.assistant.LoadSessionAssistantContext(r.Context(), p.TenantID, p.CorpID)
		if err != nil {
			workspaceRepoError(w, err)
			return
		}
		data["assistant"] = workspaceAssistantJSON(assistant)
	}
	if run != nil {
		data["run"] = map[string]any{"status": run.Status, "candidateCount": run.CandidateCount, "successCount": run.SuccessCount, "failureCount": run.FailureCount, "backlogCount": run.BacklogCount, "errorSummary": run.ErrorSummary, "createdAt": run.CreatedAt}
	}
	workspaceEnvelope(w, 200, "success", data)
}

func workspaceAssistantJSON(assistant settingsports.SystemAssistantContext) map[string]any {
	return map[string]any{"name": assistant.Name, "enabled": assistant.Enabled, "knowledgeBaseCount": assistant.KnowledgeBaseCount, "readyDocumentCount": assistant.ReadyDocumentCount, "updatedAt": assistant.UpdatedAt}
}

func (h *WorkspaceHandler) export(w http.ResponseWriter, r *http.Request, p WorkspacePrincipal, page string) {
	filter, err := parseWorkspaceFilter(r, p, page)
	if err != nil {
		workspaceEnvelope(w, 400, err.Error(), nil)
		return
	}
	filter.Page, filter.PageSize = 1, 10000
	result, err := h.repo.InsightPage(r.Context(), filter)
	if err != nil {
		workspaceRepoError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"ai-insight.csv\"")
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"沟通人", "对象", "分析结果", "来源消息数", "来源时间", "分析时间"})
	for _, item := range result.Items {
		_ = writer.Write([]string{workspaceCSV(item.EmployeeName), workspaceCSV(item.TargetName), workspaceCSV(item.Summary), strconv.Itoa(item.SourceMessageCount), item.SourceEndedAt.Format("2006-01-02 15:04"), workspaceTime(item.GeneratedAt)})
	}
	writer.Flush()
}

func (h *WorkspaceHandler) rules(w http.ResponseWriter, r *http.Request, p WorkspacePrincipal) {
	switch r.Method {
	case http.MethodGet:
		page, err := h.repo.RulePage(r.Context(), RuleFilter{TenantID: p.TenantID, CorpID: p.CorpID, Page: 1, PageSize: 20, Status: r.URL.Query().Get("status"), Keyword: r.URL.Query().Get("keyword")})
		if err != nil {
			workspaceRepoError(w, err)
			return
		}
		items := make([]map[string]any, 0, len(page.Items))
		for _, rule := range page.Items {
			items = append(items, workspaceRuleJSON(rule))
		}
		workspaceEnvelope(w, 200, "success", map[string]any{"page": page.Page, "pageSize": page.PageSize, "total": page.Total, "items": items})
	default:
		workspaceEnvelope(w, 405, "method not allowed", nil)
	}
}

func (h *WorkspaceHandler) ruleStatus(w http.ResponseWriter, r *http.Request, p WorkspacePrincipal) {
	workspaceEnvelope(w, 405, "method not allowed", nil)
}

type workspaceRuleInput struct {
	ID                int64    `json:"id"`
	Name              string   `json:"name"`
	Objective         string   `json:"objective"`
	ConversationTypes []string `json:"conversationTypes"`
	TargetScope       string   `json:"targetScope"`
	TargetIDs         []int64  `json:"targetIds"`
	LookbackDays      int      `json:"lookbackDays"`
	MinimumMessages   int      `json:"minimumMessages"`
	Status            string   `json:"status"`
}

func (i workspaceRuleInput) toWrite(p WorkspacePrincipal) RuleWrite {
	return RuleWrite{TenantID: p.TenantID, CorpID: p.CorpID, ID: i.ID, Name: i.Name, Objective: i.Objective, ConversationTypes: i.ConversationTypes, TargetScope: i.TargetScope, TargetIDs: i.TargetIDs, LookbackDays: i.LookbackDays, MinimumMessages: i.MinimumMessages, Status: i.Status, ActorID: p.UserID}
}

func parseWorkspaceFilter(r *http.Request, p WorkspacePrincipal, page string) (InsightFilter, error) {
	filter := InsightFilter{TenantID: p.TenantID, CorpID: p.CorpID, Page: 1, PageSize: 20, AllowedEmployeeIDs: p.AllowedEmployeeIDs, Restricted: p.EmployeeScopeRestricted}
	filter.AnalysisType = AnalysisTypeSession
	if page == "smart-analysis" {
		filter.AnalysisType = AnalysisTypeSmart
	}
	q := r.URL.Query()
	if value := q.Get("page"); value != "" {
		parsed, e := strconv.Atoi(value)
		if e != nil || parsed < 1 {
			return filter, errors.New("页码无效")
		}
		filter.Page = parsed
	}
	if value := q.Get("employeeId"); value != "" {
		parsed, e := strconv.ParseInt(value, 10, 64)
		if e != nil || parsed < 1 {
			return filter, errors.New("员工 ID 无效")
		}
		filter.EmployeeID = parsed
	}
	filter.TargetID = strings.TrimSpace(q.Get("targetId"))
	filter.ConversationType = strings.TrimSpace(q.Get("conversationType"))
	if filter.ConversationType == "direct" {
		filter.ConversationType = "1"
	}
	if filter.ConversationType == "group" {
		filter.ConversationType = "2"
	}
	if filter.ConversationType != "" && filter.ConversationType != "1" && filter.ConversationType != "2" {
		return filter, errors.New("会话类型无效")
	}
	filter.Keyword = strings.TrimSpace(q.Get("keyword"))
	filter.Status = AnalysisStatus(q.Get("status"))
	if filter.Status != "" && filter.Status != AnalysisStatusSucceeded && filter.Status != AnalysisStatusFailed {
		return filter, errors.New("分析状态无效")
	}
	if value := q.Get("ruleVersionId"); value != "" {
		parsed, e := strconv.ParseInt(value, 10, 64)
		if e != nil || parsed < 1 {
			return filter, errors.New("规则版本无效")
		}
		filter.RuleVersionID = parsed
	}
	for key, dest := range map[string]**time.Time{"startDate": &filter.StartAt, "endDate": &filter.EndAt} {
		if value := q.Get(key); value != "" {
			parsed, e := time.Parse("2006-01-02", value)
			if e != nil {
				return filter, errors.New("日期无效")
			}
			*dest = &parsed
		}
	}
	return filter, nil
}

func workspacePath(path string) (string, string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 {
		return "", ""
	}
	page := parts[2]
	if len(parts) == 3 {
		return page, "records"
	}
	if parts[3] == "rule-status" || (parts[3] == "rules" && len(parts) > 4 && parts[4] == "status") {
		return page, "rule-status"
	}
	return page, parts[3]
}
func workspaceInsightJSON(item ConversationInsight) map[string]any {
	data := map[string]any{"id": item.ID, "conversationKey": item.ConversationKey, "employee": map[string]any{"id": item.EmployeeID, "name": item.EmployeeName, "avatar": item.EmployeeAvatar}, "target": map[string]any{"type": workspaceTypeName(item.TargetType), "id": item.TargetID, "name": item.TargetName, "avatar": item.TargetAvatar}, "sourceWindow": map[string]any{"startedAt": item.SourceStartedAt, "endedAt": item.SourceEndedAt, "messageCount": item.SourceMessageCount, "fingerprint": item.SourceFingerprint}, "status": item.Status, "summary": item.Summary, "errorSummary": item.ErrorSummary, "analysisAt": workspaceTime(item.GeneratedAt), "provider": item.Provider, "model": item.Model, "promptVersion": item.PromptVersion}
	if item.RuleID > 0 {
		data["rule"] = map[string]any{"id": item.RuleID, "name": item.RuleNameSnapshot, "version": item.RuleVersion}
	}
	if len(item.ResultJSON) > 0 {
		var raw any
		if json.Unmarshal(item.ResultJSON, &raw) == nil {
			data["result"] = raw
		}
	}
	return data
}
func workspaceMessagesJSON(messages []SourceMessage) []map[string]any {
	items := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		items = append(items, map[string]any{"id": message.ID, "time": message.MessageTime, "direction": message.Direction, "senderName": message.SenderName, "content": message.Content})
	}
	return items
}
func workspaceRuleJSON(rule AnalysisRule) map[string]any {
	return map[string]any{"id": rule.ID, "name": rule.Name, "objective": rule.Objective, "conversationTypes": rule.ConversationTypes, "targetScope": rule.TargetScope, "targetIds": rule.TargetIDs, "lookbackDays": rule.LookbackDays, "minimumMessages": rule.MinimumMessages, "status": rule.Status, "currentVersion": rule.CurrentVersion, "createdAt": rule.CreatedAt, "updatedAt": rule.UpdatedAt}
}
func workspaceTypeName(value string) string {
	if value == "1" {
		return "direct"
	}
	if value == "2" {
		return "group"
	}
	return value
}
func workspacePositiveID(r *http.Request, key string) (int64, error) {
	value := r.URL.Query().Get(key)
	id, e := strconv.ParseInt(value, 10, 64)
	if e != nil || id < 1 {
		return 0, errors.New("invalid")
	}
	return id, nil
}
func workspaceTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format(time.RFC3339)
}
func workspaceCSV(value string) string {
	if strings.HasPrefix(value, "=") || strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") || strings.HasPrefix(value, "@") {
		return "'" + value
	}
	return value
}
func workspaceRuleJSONBytes(value any) []byte { encoded, _ := json.Marshal(value); return encoded }
func workspaceRepoError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		workspaceEnvelope(w, 404, "记录不存在", nil)
		return
	}
	workspaceEnvelope(w, 500, "AI 洞察请求失败", nil)
}
func workspaceEnvelope(w http.ResponseWriter, status int, message string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": status, "msg": message, "data": data})
}
