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
	CanRunAnalysis          bool
}
type WorkspacePrincipalResolver interface {
	Resolve(*http.Request) (WorkspacePrincipal, error)
}
type WorkspaceAuthorizer interface {
	Authorize(context.Context, WorkspacePrincipal, int64, string) error
}

type directoryOptionRepository interface {
	DirectoryOptions(context.Context, DirectoryOptionFilter) (DirectoryOptions, error)
}

type WorkspaceHandler struct {
	principal       WorkspacePrincipalResolver
	authorize       WorkspaceAuthorizer
	repo            Repository
	resolver        providers.AIProviderResolver
	assistant       AssistantContextProvider
	systemAssistant SystemAssistantContextProvider
	now             func() time.Time
}

func NewWorkspaceHandler(principal WorkspacePrincipalResolver, authorize WorkspaceAuthorizer, repo Repository, resolver providers.AIProviderResolver, assistants ...any) *WorkspaceHandler {
	var assistant AssistantContextProvider
	var systemAssistant SystemAssistantContextProvider
	if len(assistants) > 0 {
		assistant, _ = assistants[0].(AssistantContextProvider)
		systemAssistant, _ = assistants[0].(SystemAssistantContextProvider)
	}
	return &WorkspaceHandler{principal: principal, authorize: authorize, repo: repo, resolver: resolver, assistant: assistant, systemAssistant: systemAssistant, now: time.Now}
}

func newWorkspaceHandlerForTest(principal WorkspacePrincipalResolver, authorize WorkspaceAuthorizer, repo Repository, ai providers.AIProvider, assistants ...any) *WorkspaceHandler {
	return NewWorkspaceHandler(principal, authorize, repo, providers.StaticAIProviderResolver{Provider: ai}, assistants...)
}

// NewWorkspaceHandlerWithResolver creates the tenant-scoped runtime path.
// Read-only endpoints do not resolve or call the model; status resolves only
// the authenticated principal's tenant/corp and never accepts request scope.
func NewWorkspaceHandlerWithResolver(principal WorkspacePrincipalResolver, authorize WorkspaceAuthorizer, repo Repository, resolver providers.AIProviderResolver, assistants ...any) *WorkspaceHandler {
	return NewWorkspaceHandler(principal, authorize, repo, resolver, assistants...)
}

func (h *WorkspaceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal.Resolve(r)
	if err != nil || p.UserID <= 0 || p.TenantID <= 0 || p.CorpID <= 0 {
		workspaceEnvelope(w, 401, "principal unauthorized", nil)
		return
	}
	if r.URL.Path == "/dashboard/ai-insight/run" {
		h.runNow(w, r, p)
		return
	}
	page, action := workspacePath(r.URL.Path)
	if !workspacePageAllowed(page) {
		workspaceEnvelope(w, 404, "page not found", nil)
		return
	}
	if workspaceDerivedPage(page) && !workspaceDerivedActionAllowed(action) {
		workspaceEnvelope(w, 404, "resource not found", nil)
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
	case "filter-options":
		h.filterOptions(w, r, p, page)
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

func (h *WorkspaceHandler) runNow(w http.ResponseWriter, r *http.Request, p WorkspacePrincipal) {
	if r.Method != http.MethodPost || !p.CanRunAnalysis {
		workspaceEnvelope(w, http.StatusForbidden, "forbidden", nil)
		return
	}
	if h.authorize != nil {
		if err := h.authorize.Authorize(r.Context(), p, p.CorpID, "/ai-insight/run#run"); err != nil {
			workspaceEnvelope(w, http.StatusForbidden, "forbidden", nil)
			return
		}
	}
	if h.resolver == nil {
		workspaceEnvelope(w, http.StatusServiceUnavailable, "AI_PROVIDER_UNAVAILABLE", nil)
		return
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("CST", 8*3600)
	}
	nowFn := h.now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn().In(location)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	runner := NewConversationAnalysisRunner(h.repo, h.resolver, RunnerConfig{}, nil, h.systemAssistant)
	if err := runner.RunCorpWindow(r.Context(), p.TenantID, p.CorpID, start, now); err != nil {
		code := "AI_ANALYSIS_FAILED"
		var safe providers.AIProviderResolveError
		if errors.As(err, &safe) && strings.TrimSpace(safe.SafeCode()) != "" {
			code = strings.TrimSpace(safe.SafeCode())
		}
		workspaceEnvelope(w, http.StatusServiceUnavailable, code, nil)
		return
	}
	workspaceEnvelope(w, http.StatusOK, "success", map[string]any{"tenantId": p.TenantID, "corpId": p.CorpID})
}

func (h *WorkspaceHandler) filterOptions(w http.ResponseWriter, r *http.Request, p WorkspacePrincipal, page string) {
	limit := 50
	if value := r.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			workspaceEnvelope(w, 400, "数量限制无效", nil)
			return
		}
		limit = parsed
	}
	typ := AnalysisTypeSession
	if page == "smart-analysis" {
		typ = AnalysisTypeSmart
	}
	if directoryRepo, ok := h.repo.(directoryOptionRepository); ok {
		options, err := directoryRepo.DirectoryOptions(r.Context(), DirectoryOptionFilter{
			TenantID: p.TenantID, CorpID: p.CorpID, AnalysisType: typ,
			EmployeeKeyword: strings.TrimSpace(r.URL.Query().Get("employeeKeyword")),
			CustomerKeyword: strings.TrimSpace(r.URL.Query().Get("customerKeyword")), Limit: limit,
			AllowedEmployeeIDs: p.AllowedEmployeeIDs, Restricted: p.EmployeeScopeRestricted,
		})
		if err != nil {
			workspaceRepoError(w, err)
			return
		}
		employees := make([]map[string]any, 0, len(options.Employees))
		for _, option := range options.Employees {
			employees = append(employees, map[string]any{"id": option.ID, "name": option.Name, "avatar": option.Avatar})
		}
		customers := make([]map[string]any, 0, len(options.Customers))
		for _, option := range options.Customers {
			customers = append(customers, map[string]any{"id": option.ID, "name": option.Name, "avatar": option.Avatar})
		}
		coverage := map[string]any{
			"availableEmployeeCount": options.Coverage.AvailableEmployeeCount,
			"availableCustomerCount": options.Coverage.AvailableCustomerCount,
			"analyzedEmployeeCount":  options.Coverage.AnalyzedEmployeeCount,
			"analyzedCustomerCount":  options.Coverage.AnalyzedCustomerCount,
		}
		workspaceEnvelope(w, 200, "success", map[string]any{"employees": employees, "customers": customers, "coverage": coverage})
		return
	}
	options, err := h.repo.EmployeeOptions(r.Context(), EmployeeOptionFilter{
		TenantID: p.TenantID, CorpID: p.CorpID, AnalysisType: typ,
		EmployeeKeyword: strings.TrimSpace(r.URL.Query().Get("employeeKeyword")), Limit: limit,
		AllowedEmployeeIDs: p.AllowedEmployeeIDs, Restricted: p.EmployeeScopeRestricted,
	})
	if err != nil {
		workspaceRepoError(w, err)
		return
	}
	employees := make([]map[string]any, 0, len(options))
	for _, option := range options {
		employees = append(employees, map[string]any{"id": option.ID, "name": option.Name, "avatar": option.Avatar})
	}
	workspaceEnvelope(w, 200, "success", map[string]any{"employees": employees})
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
	messages, err := h.repo.ConversationMessages(r.Context(), ConversationWindowQuery{TenantID: p.TenantID, CorpID: p.CorpID, ConversationKey: item.ConversationKey, StartAt: item.SourceStartedAt, EndAt: item.SourceEndedAt, Limit: 200, AllowedEmployeeIDs: p.AllowedEmployeeIDs, Restricted: p.EmployeeScopeRestricted})
	if err != nil {
		workspaceRepoError(w, err)
		return
	}
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
	var ai providers.AIProvider
	if h.resolver != nil {
		resolved, err := h.resolver.Resolve(r.Context(), p.TenantID, p.CorpID)
		if err != nil {
			code, message := "AI_PROVIDER_UNAVAILABLE", safeProviderFailure(err)
			var resolveErr providers.AIProviderResolveError
			if errors.As(err, &resolveErr) && strings.TrimSpace(resolveErr.SafeCode()) != "" {
				code, message = strings.TrimSpace(resolveErr.SafeCode()), strings.TrimSpace(resolveErr.SafeReason())
			}
			provider = map[string]any{"state": "unavailable", "code": code, "message": message}
		} else {
			ai = resolved
		}
	}
	if ai != nil {
		status := ai.Status()
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
	} else if h.assistant != nil && page != "smart-analysis" {
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
	filter.Page, filter.PageSize, filter.Export = 1, 10000, true
	result, err := h.repo.InsightPage(r.Context(), filter)
	if err != nil {
		workspaceRepoError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"ai-insight.csv\"")
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(w)
	if workspaceDerivedPage(page) {
		workspaceWriteProjectionCSV(writer, page, result.Items)
		writer.Flush()
		return
	}
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
	} else if workspaceDerivedPage(page) {
		filter.View = page
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
	_, hasKeyword := q["keyword"]
	_, hasEmotion := q["emotion"]
	_, hasMinScore := q["minScore"]
	_, hasMaxScore := q["maxScore"]
	if hasEmotion && page != "emotion" {
		return filter, errors.New("客户情绪筛选无效")
	}
	if (hasMinScore || hasMaxScore) && page != "employee-score" {
		return filter, errors.New("员工评分筛选无效")
	}
	if hasKeyword && workspaceDerivedPage(page) && page != "communication-keyword" {
		return filter, errors.New("沟通关键词筛选无效")
	}
	if page == "emotion" && hasEmotion {
		filter.Emotion = strings.TrimSpace(q.Get("emotion"))
		if !workspaceEmotionAllowed(filter.Emotion) {
			return filter, errors.New("客户情绪筛选无效")
		}
	}
	if page == "employee-score" {
		var err error
		if hasMinScore {
			filter.MinScore, err = workspaceScore(q.Get("minScore"))
			if err != nil {
				return filter, err
			}
		}
		if hasMaxScore {
			filter.MaxScore, err = workspaceScore(q.Get("maxScore"))
			if err != nil {
				return filter, err
			}
		}
		if filter.MinScore != nil && filter.MaxScore != nil && *filter.MinScore > *filter.MaxScore {
			return filter, errors.New("员工评分范围无效")
		}
	}
	filter.Keyword = strings.TrimSpace(q.Get("keyword"))
	filter.CustomerName = strings.TrimSpace(q.Get("customerName"))
	if value := q.Get("customerId"); value != "" {
		parsed, e := strconv.ParseInt(value, 10, 64)
		if e != nil || parsed < 1 {
			return filter, errors.New("客户 ID 无效")
		}
		filter.CustomerID = parsed
	}
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
	if filter.EndAt != nil {
		endExclusive := filter.EndAt.AddDate(0, 0, 1)
		filter.EndAt = &endExclusive
	}
	if filter.StartAt != nil && filter.EndAt != nil && !filter.StartAt.Before(*filter.EndAt) {
		return filter, errors.New("日期范围无效")
	}
	return filter, nil
}

func workspacePageAllowed(page string) bool {
	return page == "session-analysis" || page == "smart-analysis" || workspaceDerivedPage(page)
}

func workspaceDerivedPage(page string) bool {
	return page == "emotion" || page == "employee-score" || page == "communication-keyword"
}

func workspaceDerivedActionAllowed(action string) bool {
	return action == "records" || action == "detail" || action == "status" || action == "filter-options" || action == "export"
}

func workspaceEmotionAllowed(emotion string) bool {
	switch emotion {
	case "positive", "neutral", "negative", "mixed", "unknown":
		return true
	default:
		return false
	}
}

func workspaceScore(value string) (*int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 || parsed > 100 {
		return nil, errors.New("员工评分筛选无效")
	}
	return &parsed, nil
}

func workspaceWriteProjectionCSV(writer *csv.Writer, page string, items []ConversationInsight) {
	headers := map[string][]string{
		"emotion":               {"员工", "对象", "客户情绪", "情绪依据", "状态", "失败原因", "来源开始时间", "来源结束时间", "来源消息数", "分析时间", "Provider", "Model", "Prompt"},
		"employee-score":        {"员工", "对象", "员工评分", "评分摘要", "优点", "问题", "建议", "状态", "失败原因", "来源开始时间", "来源结束时间", "来源消息数", "分析时间", "Provider", "Model", "Prompt"},
		"communication-keyword": {"员工", "对象", "沟通关键词", "摘要", "状态", "失败原因", "来源开始时间", "来源结束时间", "来源消息数", "分析时间", "Provider", "Model", "Prompt"},
	}
	_ = writer.Write(headers[page])
	for _, item := range items {
		row := workspaceProjectionCSVRow(page, item)
		for index := range row {
			row[index] = workspaceCSV(row[index])
		}
		_ = writer.Write(row)
	}
}

func workspaceProjectionCSVRow(page string, item ConversationInsight) []string {
	commonTail := []string{string(item.Status), item.ErrorSummary, workspaceCSVTime(item.SourceStartedAt), workspaceCSVTime(item.SourceEndedAt), strconv.Itoa(item.SourceMessageCount), workspaceTime(item.GeneratedAt), item.Provider, item.Model, item.PromptVersion}
	result, recordedScore := workspaceSessionResult(item)
	switch page {
	case "emotion":
		label, reason := "", ""
		if result != nil {
			label = result.Customer.Emotion.Label
			reason = result.Customer.Emotion.Reason
		}
		return append([]string{item.EmployeeName, item.TargetName, label, reason}, commonTail...)
	case "employee-score":
		score, strengths, issues, suggestions := "", "", "", ""
		if result != nil {
			if recordedScore != nil {
				score = strconv.Itoa(*recordedScore)
			}
			strengths = strings.Join(result.EmployeeQA.Strengths, "；")
			issues = strings.Join(result.EmployeeQA.Issues, "；")
			suggestions = strings.Join(result.EmployeeQA.Suggestions, "；")
		}
		return append([]string{item.EmployeeName, item.TargetName, score, item.Summary, strengths, issues, suggestions}, commonTail...)
	default:
		keywords := ""
		if result != nil {
			keywords = strings.Join(result.Customer.Keywords, "；")
		}
		return append([]string{item.EmployeeName, item.TargetName, keywords, item.Summary}, commonTail...)
	}
}

func workspaceSessionResult(item ConversationInsight) (*SessionAnalysisResult, *int) {
	if len(item.ResultJSON) > 0 {
		var wire struct {
			EmployeeQA struct {
				Score *int `json:"score"`
			} `json:"employeeQa"`
		}
		var result SessionAnalysisResult
		if json.Unmarshal(item.ResultJSON, &result) == nil && json.Unmarshal(item.ResultJSON, &wire) == nil {
			return &result, wire.EmployeeQA.Score
		}
		return nil, nil
	}
	if item.SessionResult == nil {
		return nil, nil
	}
	score := item.SessionResult.EmployeeQA.Score
	return item.SessionResult, &score
}

func workspaceCSVTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02 15:04")
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
		items = append(items, map[string]any{"id": message.ID, "legacyId": message.LegacyID, "time": message.MessageTime, "direction": message.Direction, "senderName": message.SenderName, "content": message.Content})
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
