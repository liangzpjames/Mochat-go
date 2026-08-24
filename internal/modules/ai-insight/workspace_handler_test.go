package aiinsight

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	settingsports "jiyi/mochat-go/internal/modules/ai-settings/ports"
)

type workspaceTestResolver struct {
	principal WorkspacePrincipal
	err       error
}

type workspaceAssistantStub struct {
	assistant settingsports.SessionAssistantContext
	ensureErr error
	loadErr   error
}

type workspaceSystemAssistantStub struct {
	contexts map[string]settingsports.SystemAssistantContext
	loaded   []string
}

func (s *workspaceSystemAssistantStub) EnsureSystemAssistants(context.Context, int64, int64, int64, string, string) ([]settingsports.Agent, error) {
	return nil, nil
}
func (s *workspaceSystemAssistantStub) LoadSystemAssistantContext(_ context.Context, _, _ int64, key string) (settingsports.SystemAssistantContext, error) {
	s.loaded = append(s.loaded, key)
	return s.contexts[key], nil
}

func (s workspaceAssistantStub) EnsureSessionAssistant(context.Context, int64, int64, int64, string) (settingsports.Agent, error) {
	return settingsports.Agent{}, s.ensureErr
}
func (s workspaceAssistantStub) LoadSessionAssistantContext(context.Context, int64, int64) (settingsports.SessionAssistantContext, error) {
	return s.assistant, s.loadErr
}

func TestWorkspaceStatusReturnsFailureWhenSessionAssistantIsUnavailable(t *testing.T) {
	tests := []struct {
		name      string
		assistant workspaceAssistantStub
	}{
		{name: "ensure fails", assistant: workspaceAssistantStub{ensureErr: errors.New("ensure failed")}},
		{name: "load fails", assistant: workspaceAssistantStub{loadErr: errors.New("load failed")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewWorkspaceHandler(
				workspaceTestResolver{principal: WorkspacePrincipal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, workspaceTestRepo{}, nil,
				test.assistant,
			)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/session-analysis/status", nil))

			if recorder.Code != http.StatusInternalServerError || !strings.Contains(recorder.Body.String(), `"msg":"AI 洞察请求失败"`) {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestWorkspaceSessionStatusExposesRuntimeAssistantSummary(t *testing.T) {
	handler := NewWorkspaceHandler(
		workspaceTestResolver{principal: WorkspacePrincipal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, workspaceTestRepo{}, nil,
		workspaceAssistantStub{assistant: settingsports.SessionAssistantContext{Name: settingsports.SessionAnalysisAssistantName, Enabled: true, KnowledgeBaseCount: 2, ReadyDocumentCount: 5, UpdatedAt: "2026-08-23T12:00:00Z"}},
	)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/session-analysis/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"会话分析助手"`) || !strings.Contains(rec.Body.String(), `"readyDocumentCount":5`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWorkspaceSmartStatusDoesNotExposeLegacySessionAssistant(t *testing.T) {
	handler := NewWorkspaceHandler(
		workspaceTestResolver{principal: WorkspacePrincipal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, workspaceTestRepo{}, nil,
		workspaceAssistantStub{assistant: settingsports.SessionAssistantContext{Name: settingsports.SessionAnalysisAssistantName, Enabled: true, KnowledgeBaseCount: 2, ReadyDocumentCount: 5, UpdatedAt: "2026-08-23T12:00:00Z"}},
	)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/smart-analysis/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `"assistant"`) || strings.Contains(rec.Body.String(), `"name":"会话分析助手"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWorkspaceStatusMapsEachPageToItsOwnAssistant(t *testing.T) {
	assistants := &workspaceSystemAssistantStub{contexts: map[string]settingsports.SystemAssistantContext{
		settingsports.SessionAnalysisSystemKey: {Name: "会话助手独立", Enabled: true},
		settingsports.SmartAnalysisSystemKey:   {Name: "智能助手独立", Enabled: true},
	}}
	handler := NewWorkspaceHandler(workspaceTestResolver{principal: WorkspacePrincipal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, workspaceTestRepo{}, nil, assistants)
	for _, test := range []struct {
		path     string
		expected string
	}{
		{path: "/dashboard/ai-insight/session-analysis/status", expected: "会话助手独立"},
		{path: "/dashboard/ai-insight/smart-analysis/status", expected: "智能助手独立"},
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), test.expected) {
			t.Fatalf("path=%s status=%d body=%s", test.path, recorder.Code, recorder.Body.String())
		}
	}
	if len(assistants.loaded) != 2 || assistants.loaded[0] != settingsports.SessionAnalysisSystemKey || assistants.loaded[1] != settingsports.SmartAnalysisSystemKey {
		t.Fatalf("loaded keys = %#v", assistants.loaded)
	}
}

func TestWorkspaceRejectsIndependentSmartRuleWrites(t *testing.T) {
	handler := NewWorkspaceHandler(workspaceTestResolver{principal: WorkspacePrincipal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, workspaceTestRepo{}, nil)
	requests := []*http.Request{
		httptest.NewRequest(http.MethodPost, "/dashboard/ai-insight/smart-analysis/rules", strings.NewReader(`{"name":"无入口规则"}`)),
		httptest.NewRequest(http.MethodPut, "/dashboard/ai-insight/smart-analysis/rules", strings.NewReader(`{"id":1}`)),
		httptest.NewRequest(http.MethodDelete, "/dashboard/ai-insight/smart-analysis/rules?id=1", nil),
		httptest.NewRequest(http.MethodPost, "/dashboard/ai-insight/smart-analysis/rules/status", strings.NewReader(`{"id":1,"status":"enabled"}`)),
	}
	for _, request := range requests {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s %s status=%d body=%s", request.Method, request.URL.Path, recorder.Code, recorder.Body.String())
		}
	}
}

func (r workspaceTestResolver) Resolve(*http.Request) (WorkspacePrincipal, error) {
	return r.principal, r.err
}

type workspaceTestAuthorizer struct {
	err        error
	permission string
}

func (a *workspaceTestAuthorizer) Authorize(_ context.Context, _ WorkspacePrincipal, _ int64, permission string) error {
	a.permission = permission
	return a.err
}

type workspaceTestRepo struct{}

type workspaceInsightRepo struct {
	workspaceTestRepo
	item ConversationInsight
}

func (r workspaceInsightRepo) InsightPage(context.Context, InsightFilter) (InsightPage, error) {
	return InsightPage{Page: 1, PageSize: 20, Total: 1, Items: []ConversationInsight{r.item}}, nil
}

func (r workspaceInsightRepo) InsightDetail(context.Context, InsightDetailFilter) (ConversationInsight, error) {
	return r.item, nil
}

func (workspaceTestRepo) ConversationCandidates(context.Context, CandidateQuery) ([]ConversationCandidate, error) {
	return nil, nil
}
func (workspaceTestRepo) ConversationMessages(context.Context, ConversationWindowQuery) ([]SourceMessage, error) {
	return nil, nil
}
func (workspaceTestRepo) LatestSucceededFingerprint(context.Context, int64, int64, AnalysisType, int64, string) (string, error) {
	return "", nil
}
func (workspaceTestRepo) SaveInsight(context.Context, ConversationInsight) error   { return nil }
func (workspaceTestRepo) CreateRun(context.Context, InsightRun) (int64, error)     { return 1, nil }
func (workspaceTestRepo) FinishRun(context.Context, int64, InsightRunResult) error { return nil }
func (workspaceTestRepo) InsightPage(context.Context, InsightFilter) (InsightPage, error) {
	return InsightPage{Page: 1, PageSize: 20, Items: []ConversationInsight{}}, nil
}
func (workspaceTestRepo) EmployeeOptions(context.Context, EmployeeOptionFilter) ([]EmployeeOption, error) {
	return []EmployeeOption{}, nil
}
func (workspaceTestRepo) InsightDetail(context.Context, InsightDetailFilter) (ConversationInsight, error) {
	return ConversationInsight{}, nil
}
func (workspaceTestRepo) LatestRun(context.Context, int64, int64, AnalysisType) (*InsightRun, error) {
	return nil, nil
}
func (workspaceTestRepo) RulePage(context.Context, RuleFilter) (RulePage, error) {
	return RulePage{Page: 1, PageSize: 20, Items: []AnalysisRule{}}, nil
}
func (workspaceTestRepo) RuleByID(context.Context, int64, int64, int64) (AnalysisRule, error) {
	return AnalysisRule{}, nil
}
func (workspaceTestRepo) CreateRule(context.Context, RuleWrite) (AnalysisRule, error) {
	return AnalysisRule{}, nil
}
func (workspaceTestRepo) UpdateRule(context.Context, RuleWrite) (AnalysisRule, error) {
	return AnalysisRule{}, nil
}
func (workspaceTestRepo) SetRuleStatus(context.Context, RuleStatusWrite) error { return nil }
func (workspaceTestRepo) DeleteRule(context.Context, RuleDelete) error         { return nil }
func (workspaceTestRepo) EnabledRuleVersions(context.Context, int64, int64) ([]AnalysisRuleVersion, error) {
	return nil, nil
}
func (workspaceTestRepo) CurrentEnabledRuleVersion(context.Context, int64, int64, string) (*AnalysisRuleVersion, error) {
	return nil, nil
}

func TestWorkspaceDetailHidesUnauthorizedRecordAsNotFound(t *testing.T) {
	handler := NewWorkspaceHandler(workspaceTestResolver{principal: WorkspacePrincipal{UserID: 7, TenantID: 1, CorpID: 2, EmployeeScopeRestricted: true, AllowedEmployeeIDs: []int64{1001}}}, nil, workspaceTestRepo{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/session-analysis/detail?id=99", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("fake repository status=%d; scope enforcement belongs to repository", rec.Code)
	}
}

func TestWorkspaceRejectsInvalidPageBeforeRepository(t *testing.T) {
	handler := NewWorkspaceHandler(workspaceTestResolver{principal: WorkspacePrincipal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, workspaceTestRepo{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/session-analysis/records?page=0", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d,want 400", rec.Code)
	}
}

func TestWorkspaceAuthorizationIsCheckedBeforeRead(t *testing.T) {
	authorizer := &workspaceTestAuthorizer{err: errors.New("denied")}
	handler := NewWorkspaceHandler(workspaceTestResolver{principal: WorkspacePrincipal{UserID: 7, TenantID: 1, CorpID: 2}}, authorizer, workspaceTestRepo{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/smart-analysis/records", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d,want 403", rec.Code)
	}
}

type workspaceEmployeeOptionsRepo struct {
	workspaceTestRepo
	filter  EmployeeOptionFilter
	options []EmployeeOption
	err     error
}

func (r *workspaceEmployeeOptionsRepo) EmployeeOptions(_ context.Context, filter EmployeeOptionFilter) ([]EmployeeOption, error) {
	r.filter = filter
	return r.options, r.err
}

func TestWorkspaceFilterOptionsMapsEachPageAndPreservesPrincipalScope(t *testing.T) {
	for _, test := range []struct {
		path string
		typ  AnalysisType
	}{
		{path: "/dashboard/ai-insight/session-analysis/filter-options?employeeKeyword=%E7%8E%8B&limit=25", typ: AnalysisTypeSession},
		{path: "/dashboard/ai-insight/smart-analysis/filter-options?employeeKeyword=%E7%8E%8B&limit=25", typ: AnalysisTypeSmart},
	} {
		repo := &workspaceEmployeeOptionsRepo{options: []EmployeeOption{{ID: 1001, Name: "王甲", Avatar: "avatar"}}}
		authorizer := &workspaceTestAuthorizer{}
		handler := NewWorkspaceHandler(workspaceTestResolver{principal: WorkspacePrincipal{
			UserID: 7, TenantID: 11, CorpID: 22, EmployeeScopeRestricted: true, AllowedEmployeeIDs: []int64{1001, 1002},
		}}, authorizer, repo, nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"employees":[{"avatar":"avatar","id":1001,"name":"王甲"}]`) {
			t.Fatalf("path=%s status=%d body=%s", test.path, recorder.Code, recorder.Body.String())
		}
		if repo.filter.TenantID != 11 || repo.filter.CorpID != 22 || repo.filter.AnalysisType != test.typ || repo.filter.EmployeeKeyword != "王" || repo.filter.Limit != 25 || !repo.filter.Restricted || len(repo.filter.AllowedEmployeeIDs) != 2 {
			t.Fatalf("path=%s filter=%#v", test.path, repo.filter)
		}
		page := map[AnalysisType]string{AnalysisTypeSession: "session-analysis", AnalysisTypeSmart: "smart-analysis"}[test.typ]
		if authorizer.permission != "/ai-insight/"+page+"#read" {
			t.Fatalf("permission=%q", authorizer.permission)
		}
	}
}

func TestWorkspaceFilterOptionsRejectsInvalidLimitBeforeRepository(t *testing.T) {
	for _, limit := range []string{"0", "101", "abc"} {
		repo := &workspaceEmployeeOptionsRepo{}
		handler := NewWorkspaceHandler(workspaceTestResolver{principal: WorkspacePrincipal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, repo, nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/session-analysis/filter-options?limit="+limit, nil))
		if recorder.Code != http.StatusBadRequest || repo.filter.TenantID != 0 {
			t.Fatalf("limit=%s status=%d filter=%#v", limit, recorder.Code, repo.filter)
		}
	}
}

func TestWorkspaceRecordsParsesCustomerName(t *testing.T) {
	filter, err := parseWorkspaceFilter(httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/session-analysis/records?customerName=%E5%AE%A2%E6%88%B7%E7%94%B2", nil), WorkspacePrincipal{TenantID: 1, CorpID: 2}, "session-analysis")
	if err != nil {
		t.Fatal(err)
	}
	if filter.CustomerName != "客户甲" {
		t.Fatalf("customerName=%q", filter.CustomerName)
	}
}

func TestWorkspaceFilterOptionsDoesNotLeakRepositoryError(t *testing.T) {
	repo := &workspaceEmployeeOptionsRepo{err: errors.New("SELECT failed: password=secret")}
	handler := NewWorkspaceHandler(workspaceTestResolver{principal: WorkspacePrincipal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, repo, nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/session-analysis/filter-options", nil))
	if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "password") || !strings.Contains(recorder.Body.String(), "AI 洞察请求失败") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestWorkspaceRecordsAndDetailExposeInsightFailureReason(t *testing.T) {
	item := ConversationInsight{
		ID: 9, AnalysisType: AnalysisTypeSession, ConversationKey: "1001:1:2001",
		EmployeeID: 1001, EmployeeName: "员工甲", TargetType: "1", TargetID: "2001", TargetName: "客户甲",
		SourceMessageCount: 3, SourceFingerprint: strings.Repeat("a", 64), Status: AnalysisStatusFailed,
		ErrorSummary: "模型响应超时",
	}
	handler := NewWorkspaceHandler(workspaceTestResolver{principal: WorkspacePrincipal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, workspaceInsightRepo{item: item}, nil)
	for _, path := range []string{
		"/dashboard/ai-insight/session-analysis/records",
		"/dashboard/ai-insight/session-analysis/detail?id=9",
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"errorSummary":"模型响应超时"`) {
			t.Fatalf("path=%s status=%d body=%s", path, recorder.Code, recorder.Body.String())
		}
	}
}
