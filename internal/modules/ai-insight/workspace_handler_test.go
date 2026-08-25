package aiinsight

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	settingsports "jiyi/mochat-go/internal/modules/ai-settings/ports"
	"jiyi/mochat-go/internal/modules/providers"
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
	filter           EmployeeOptionFilter
	options          []EmployeeOption
	directoryFilter  DirectoryOptionFilter
	directoryOptions DirectoryOptions
	err              error
}

func (r *workspaceEmployeeOptionsRepo) EmployeeOptions(_ context.Context, filter EmployeeOptionFilter) ([]EmployeeOption, error) {
	r.filter = filter
	return r.options, r.err
}

func (r *workspaceEmployeeOptionsRepo) DirectoryOptions(_ context.Context, filter DirectoryOptionFilter) (DirectoryOptions, error) {
	r.directoryFilter = filter
	return r.directoryOptions, r.err
}

func TestWorkspaceFilterOptionsMapsEachPageAndPreservesPrincipalScope(t *testing.T) {
	for _, test := range []struct {
		path string
		typ  AnalysisType
	}{
		{path: "/dashboard/ai-insight/session-analysis/filter-options?employeeKeyword=%E7%8E%8B&limit=25", typ: AnalysisTypeSession},
		{path: "/dashboard/ai-insight/smart-analysis/filter-options?employeeKeyword=%E7%8E%8B&limit=25", typ: AnalysisTypeSmart},
	} {
		repo := &workspaceEmployeeOptionsRepo{directoryOptions: DirectoryOptions{Employees: []EmployeeOption{{ID: 1001, Name: "王甲", Avatar: "avatar"}}}}
		authorizer := &workspaceTestAuthorizer{}
		handler := NewWorkspaceHandler(workspaceTestResolver{principal: WorkspacePrincipal{
			UserID: 7, TenantID: 11, CorpID: 22, EmployeeScopeRestricted: true, AllowedEmployeeIDs: []int64{1001, 1002},
		}}, authorizer, repo, nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"employees":[{"avatar":"avatar","id":1001,"name":"王甲"}]`) {
			t.Fatalf("path=%s status=%d body=%s", test.path, recorder.Code, recorder.Body.String())
		}
		if repo.directoryFilter.TenantID != 11 || repo.directoryFilter.CorpID != 22 || repo.directoryFilter.AnalysisType != test.typ || repo.directoryFilter.EmployeeKeyword != "王" || repo.directoryFilter.Limit != 25 || !repo.directoryFilter.Restricted || len(repo.directoryFilter.AllowedEmployeeIDs) != 2 {
			t.Fatalf("path=%s filter=%#v", test.path, repo.directoryFilter)
		}
		page := map[AnalysisType]string{AnalysisTypeSession: "session-analysis", AnalysisTypeSmart: "smart-analysis"}[test.typ]
		if authorizer.permission != "/ai-insight/"+page+"#read" {
			t.Fatalf("permission=%q", authorizer.permission)
		}
	}
}

func TestWorkspaceDerivedFilterOptionsReturnsAuthorityCustomersAndCoverage(t *testing.T) {
	repo := &workspaceEmployeeOptionsRepo{directoryOptions: DirectoryOptions{
		Employees: []EmployeeOption{{ID: 1001, Name: "王甲", Avatar: "employee-avatar"}},
		Customers: []CustomerOption{{ID: 2001, Name: "客户甲", Avatar: "customer-avatar"}},
		Coverage:  InsightDirectoryCoverage{AvailableEmployeeCount: 12, AvailableCustomerCount: 16, AnalyzedEmployeeCount: 1, AnalyzedCustomerCount: 1},
	}}
	handler := NewWorkspaceHandler(workspaceTestResolver{principal: WorkspacePrincipal{
		UserID: 7, TenantID: 11, CorpID: 22, EmployeeScopeRestricted: true, AllowedEmployeeIDs: []int64{1001, 1002},
	}}, nil, repo, nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/emotion/filter-options?employeeKeyword=%E7%8E%8B&customerKeyword=%E5%AE%A2&limit=25", nil))
	body := recorder.Body.String()
	for _, fragment := range []string{
		`"employees":[{"avatar":"employee-avatar","id":1001,"name":"王甲"}]`,
		`"customers":[{"avatar":"customer-avatar","id":2001,"name":"客户甲"}]`,
		`"availableEmployeeCount":12`, `"availableCustomerCount":16`, `"analyzedEmployeeCount":1`, `"analyzedCustomerCount":1`,
	} {
		if recorder.Code != http.StatusOK || !strings.Contains(body, fragment) {
			t.Fatalf("status=%d body=%s missing=%s", recorder.Code, body, fragment)
		}
	}
	filter := repo.directoryFilter
	if filter.TenantID != 11 || filter.CorpID != 22 || filter.AnalysisType != AnalysisTypeSession || filter.EmployeeKeyword != "王" || filter.CustomerKeyword != "客" || filter.Limit != 25 || !filter.Restricted || len(filter.AllowedEmployeeIDs) != 2 {
		t.Fatalf("filter=%#v", filter)
	}
}

func TestWorkspaceFilterOptionsRejectsInvalidLimitBeforeRepository(t *testing.T) {
	for _, limit := range []string{"0", "101", "abc"} {
		repo := &workspaceEmployeeOptionsRepo{}
		handler := NewWorkspaceHandler(workspaceTestResolver{principal: WorkspacePrincipal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, repo, nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/session-analysis/filter-options?limit="+limit, nil))
		if recorder.Code != http.StatusBadRequest || repo.directoryFilter.TenantID != 0 {
			t.Fatalf("limit=%s status=%d filter=%#v", limit, recorder.Code, repo.directoryFilter)
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

func TestWorkspaceRecordsParsesPositiveCustomerID(t *testing.T) {
	filter, err := parseWorkspaceFilter(httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/emotion/records?customerId=2001", nil), WorkspacePrincipal{TenantID: 1, CorpID: 2}, "emotion")
	if err != nil {
		t.Fatal(err)
	}
	if filter.CustomerID != 2001 {
		t.Fatalf("customerId=%d", filter.CustomerID)
	}
	for _, value := range []string{"0", "-1", "abc"} {
		_, err := parseWorkspaceFilter(httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/emotion/records?customerId="+value, nil), WorkspacePrincipal{TenantID: 1, CorpID: 2}, "emotion")
		if err == nil {
			t.Fatalf("customerId=%q should fail", value)
		}
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

type projectionWorkspaceRepo struct {
	workspaceTestRepo
	pageFilters     []InsightFilter
	detailFilters   []InsightDetailFilter
	messageQueries  []ConversationWindowQuery
	employeeFilters []EmployeeOptionFilter
	latestRunTypes  []AnalysisType
	item            ConversationInsight
	detailErr       error
}

func (r *projectionWorkspaceRepo) InsightPage(_ context.Context, filter InsightFilter) (InsightPage, error) {
	r.pageFilters = append(r.pageFilters, filter)
	return InsightPage{Page: 1, PageSize: 20, Items: []ConversationInsight{}}, nil
}

func (r *projectionWorkspaceRepo) InsightDetail(_ context.Context, filter InsightDetailFilter) (ConversationInsight, error) {
	r.detailFilters = append(r.detailFilters, filter)
	if r.detailErr != nil {
		return ConversationInsight{}, r.detailErr
	}
	return r.item, nil
}

func (r *projectionWorkspaceRepo) ConversationMessages(_ context.Context, query ConversationWindowQuery) ([]SourceMessage, error) {
	r.messageQueries = append(r.messageQueries, query)
	return []SourceMessage{}, nil
}

func (r *projectionWorkspaceRepo) EmployeeOptions(_ context.Context, filter EmployeeOptionFilter) ([]EmployeeOption, error) {
	r.employeeFilters = append(r.employeeFilters, filter)
	return []EmployeeOption{}, nil
}

func (r *projectionWorkspaceRepo) LatestRun(_ context.Context, _, _ int64, typ AnalysisType) (*InsightRun, error) {
	r.latestRunTypes = append(r.latestRunTypes, typ)
	return nil, nil
}

type projectionAIProvider struct {
	chatCalls   int
	statusCalls int
}

func (p *projectionAIProvider) Chat(context.Context, providers.ChatRequest) (string, error) {
	p.chatCalls++
	return "unexpected", nil
}

func (p *projectionAIProvider) Status() providers.Status {
	p.statusCalls++
	return providers.Status{State: providers.StateReady}
}

func projectionFilterString(t *testing.T, filter InsightFilter, name string) string {
	t.Helper()
	field := reflect.ValueOf(filter).FieldByName(name)
	if !field.IsValid() {
		t.Fatalf("InsightFilter 缺少投影字段 %s", name)
	}
	if field.Kind() != reflect.String {
		t.Fatalf("InsightFilter.%s 类型=%s，want string", name, field.Type())
	}
	return field.String()
}

func projectionFilterIntPointer(t *testing.T, filter InsightFilter, name string) *int {
	t.Helper()
	field := reflect.ValueOf(filter).FieldByName(name)
	if !field.IsValid() {
		t.Fatalf("InsightFilter 缺少投影字段 %s", name)
	}
	want := reflect.TypeOf((*int)(nil))
	if field.Type() != want {
		t.Fatalf("InsightFilter.%s 类型=%s，want *int", name, field.Type())
	}
	if field.IsNil() {
		return nil
	}
	value := int(field.Elem().Int())
	return &value
}

func TestProjectionRecordsMapEveryDerivedViewToSessionRepository(t *testing.T) {
	for _, view := range []string{"emotion", "employee-score", "communication-keyword"} {
		t.Run(view, func(t *testing.T) {
			repo := &projectionWorkspaceRepo{}
			handler := NewWorkspaceHandler(workspaceTestResolver{principal: WorkspacePrincipal{
				UserID: 7, TenantID: 11, CorpID: 22, EmployeeScopeRestricted: true, AllowedEmployeeIDs: []int64{1002, 1001},
			}}, nil, repo, nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/"+view+"/records", nil))
			if recorder.Code != http.StatusOK {
				t.Fatalf("GET %s records status=%d body=%s; derived view route must reach repository", view, recorder.Code, recorder.Body.String())
			}
			if len(repo.pageFilters) != 1 {
				t.Fatalf("GET %s repository calls=%d，want 1", view, len(repo.pageFilters))
			}
			filter := repo.pageFilters[0]
			if filter.AnalysisType != AnalysisTypeSession || projectionFilterString(t, filter, "View") != view || !filter.Restricted || !reflect.DeepEqual(filter.AllowedEmployeeIDs, []int64{1002, 1001}) {
				t.Fatalf("GET %s filter=%#v", view, filter)
			}
		})
	}
}

func TestProjectionQueryValidationPreservesEmotionStatesAndScoreZero(t *testing.T) {
	valid := []struct {
		view  string
		query string
		check func(*testing.T, InsightFilter)
	}{
		{view: "emotion", query: "emotion=positive", check: func(t *testing.T, f InsightFilter) {
			if projectionFilterString(t, f, "Emotion") != "positive" {
				t.Fatal("emotion 未保留")
			}
		}},
		{view: "emotion", query: "emotion=neutral", check: func(t *testing.T, f InsightFilter) {
			if projectionFilterString(t, f, "Emotion") != "neutral" {
				t.Fatal("emotion 未保留")
			}
		}},
		{view: "emotion", query: "emotion=negative", check: func(t *testing.T, f InsightFilter) {
			if projectionFilterString(t, f, "Emotion") != "negative" {
				t.Fatal("emotion 未保留")
			}
		}},
		{view: "emotion", query: "emotion=mixed", check: func(t *testing.T, f InsightFilter) {
			if projectionFilterString(t, f, "Emotion") != "mixed" {
				t.Fatal("emotion 未保留")
			}
		}},
		{view: "emotion", query: "emotion=unknown", check: func(t *testing.T, f InsightFilter) {
			if projectionFilterString(t, f, "Emotion") != "unknown" {
				t.Fatal("emotion 未保留")
			}
		}},
		{view: "employee-score", query: "minScore=0&maxScore=100", check: func(t *testing.T, f InsightFilter) {
			min, max := projectionFilterIntPointer(t, f, "MinScore"), projectionFilterIntPointer(t, f, "MaxScore")
			if min == nil || *min != 0 || max == nil || *max != 100 {
				t.Fatalf("score range=%v..%v，必须保留真实 0", min, max)
			}
		}},
		{view: "communication-keyword", query: "keyword=采购", check: func(t *testing.T, f InsightFilter) {
			if f.Keyword != "采购" {
				t.Fatalf("keyword=%q", f.Keyword)
			}
		}},
	}
	for _, test := range valid {
		t.Run(test.view+"_"+test.query, func(t *testing.T) {
			repo := &projectionWorkspaceRepo{}
			handler := NewWorkspaceHandler(workspaceTestResolver{principal: WorkspacePrincipal{UserID: 7, TenantID: 11, CorpID: 22}}, nil, repo, nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/"+test.view+"/records?"+test.query, nil))
			if recorder.Code != http.StatusOK || len(repo.pageFilters) != 1 {
				t.Fatalf("valid %s?%s status=%d calls=%d body=%s", test.view, test.query, recorder.Code, len(repo.pageFilters), recorder.Body.String())
			}
			test.check(t, repo.pageFilters[0])
		})
	}

	invalid := []struct{ view, query string }{
		{view: "emotion", query: "emotion=happy"},
		{view: "employee-score", query: "minScore=-1"},
		{view: "employee-score", query: "maxScore=101"},
		{view: "employee-score", query: "minScore=80&maxScore=20"},
		{view: "employee-score", query: "emotion=positive"},
		{view: "employee-score", query: "keyword=采购"},
		{view: "emotion", query: "minScore=0"},
		{view: "emotion", query: "keyword=采购"},
		{view: "communication-keyword", query: "maxScore=100"},
		{view: "communication-keyword", query: "emotion=neutral"},
	}
	for _, test := range invalid {
		t.Run("reject_"+test.view+"_"+test.query, func(t *testing.T) {
			repo := &projectionWorkspaceRepo{}
			handler := NewWorkspaceHandler(workspaceTestResolver{principal: WorkspacePrincipal{UserID: 7, TenantID: 11, CorpID: 22}}, nil, repo, nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/"+test.view+"/records?"+test.query, nil))
			if recorder.Code != http.StatusBadRequest || len(repo.pageFilters) != 0 {
				t.Fatalf("invalid %s?%s status=%d calls=%d body=%s", test.view, test.query, recorder.Code, len(repo.pageFilters), recorder.Body.String())
			}
		})
	}
}

func TestProjectionDateRangeUsesInclusiveStartAndExclusiveNextDayEnd(t *testing.T) {
	filter, err := parseWorkspaceFilter(
		httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/emotion/records?startDate=2026-08-20&endDate=2026-08-24", nil),
		WorkspacePrincipal{TenantID: 11, CorpID: 22}, "emotion",
	)
	if err != nil {
		t.Fatal(err)
	}
	wantStart := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	if filter.StartAt == nil || !filter.StartAt.Equal(wantStart) {
		t.Fatalf("startDate=%v，want inclusive %s", filter.StartAt, wantStart)
	}
	if filter.EndAt == nil || !filter.EndAt.Equal(wantEnd) {
		t.Fatalf("endDate=%v，want exclusive next-day boundary %s", filter.EndAt, wantEnd)
	}
}

func TestProjectionDetailUsesSessionIdentityAndSameEmployeeScopeForMessages(t *testing.T) {
	started := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)
	ended := started.Add(10 * time.Minute)
	for _, view := range []string{"emotion", "employee-score", "communication-keyword"} {
		t.Run(view, func(t *testing.T) {
			repo := &projectionWorkspaceRepo{item: ConversationInsight{
				ID: 91, TenantID: 11, CorpID: 22, AnalysisType: AnalysisTypeSession, ConversationKey: "1001:1:2001",
				EmployeeID: 1001, EmployeeName: "员工甲", TargetType: "1", TargetID: "2001", TargetName: "客户甲",
				SourceStartedAt: started, SourceEndedAt: ended, SourceMessageCount: 2, SourceFingerprint: strings.Repeat("a", 64), Status: AnalysisStatusSucceeded,
			}}
			handler := NewWorkspaceHandler(workspaceTestResolver{principal: WorkspacePrincipal{
				UserID: 7, TenantID: 11, CorpID: 22, EmployeeScopeRestricted: true, AllowedEmployeeIDs: []int64{1001},
			}}, nil, repo, nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/"+view+"/detail?id=91", nil))
			if recorder.Code != http.StatusOK || len(repo.detailFilters) != 1 || len(repo.messageQueries) != 1 {
				t.Fatalf("view=%s status=%d detailCalls=%d messageCalls=%d body=%s", view, recorder.Code, len(repo.detailFilters), len(repo.messageQueries), recorder.Body.String())
			}
			detail, messages := repo.detailFilters[0], repo.messageQueries[0]
			if detail.TenantID != 11 || detail.CorpID != 22 || detail.AnalysisType != AnalysisTypeSession || detail.ID != 91 || !detail.Restricted || !reflect.DeepEqual(detail.AllowedEmployeeIDs, []int64{1001}) {
				t.Fatalf("detail filter=%#v", detail)
			}
			if messages.TenantID != 11 || messages.CorpID != 22 || messages.ConversationKey != "1001:1:2001" || !messages.Restricted || !reflect.DeepEqual(messages.AllowedEmployeeIDs, []int64{1001}) {
				t.Fatalf("message query=%#v", messages)
			}
			if !messages.StartAt.Equal(started) || !messages.EndAt.Equal(ended) || messages.Limit != 200 {
				t.Fatalf("message source window=%s..%s limit=%d, want %s..%s limit=200", messages.StartAt, messages.EndAt, messages.Limit, started, ended)
			}
		})
	}
}

func TestProjectionDetailDoesNotDowngradeCrossScopeLookup(t *testing.T) {
	for _, view := range []string{"emotion", "employee-score", "communication-keyword"} {
		t.Run(view, func(t *testing.T) {
			notFoundRepo := &projectionWorkspaceRepo{detailErr: sql.ErrNoRows}
			handler := NewWorkspaceHandler(workspaceTestResolver{principal: WorkspacePrincipal{UserID: 7, TenantID: 11, CorpID: 22}}, nil, notFoundRepo, nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/"+view+"/detail?id=91", nil))
			if recorder.Code != http.StatusNotFound || len(notFoundRepo.detailFilters) != 1 || len(notFoundRepo.messageQueries) != 0 {
				t.Fatalf("view=%s status=%d detailCalls=%d messageCalls=%d body=%s", view, recorder.Code, len(notFoundRepo.detailFilters), len(notFoundRepo.messageQueries), recorder.Body.String())
			}
		})
	}
}

func TestProjectionReadsNeverChatAndStatusOnlyReadsProviderStatus(t *testing.T) {
	for _, view := range []string{"emotion", "employee-score", "communication-keyword"} {
		t.Run(view, func(t *testing.T) {
			provider := &projectionAIProvider{}
			repo := &projectionWorkspaceRepo{item: ConversationInsight{ID: 1, ConversationKey: "1001:1:2001", EmployeeID: 1001, TargetType: "1"}}
			handler := NewWorkspaceHandler(workspaceTestResolver{principal: WorkspacePrincipal{
				UserID: 7, TenantID: 11, CorpID: 22, EmployeeScopeRestricted: true, AllowedEmployeeIDs: []int64{1001},
			}}, nil, repo, provider)
			for _, action := range []string{"records", "detail?id=1", "filter-options", "export"} {
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/"+view+"/"+action, nil))
				if recorder.Code != http.StatusOK {
					t.Fatalf("GET %s/%s status=%d body=%s", view, action, recorder.Code, recorder.Body.String())
				}
			}
			if provider.chatCalls != 0 || provider.statusCalls != 0 {
				t.Fatalf("ordinary reads chat=%d status=%d", provider.chatCalls, provider.statusCalls)
			}
			if len(repo.employeeFilters) != 1 || repo.employeeFilters[0].AnalysisType != AnalysisTypeSession || !repo.employeeFilters[0].Restricted || !reflect.DeepEqual(repo.employeeFilters[0].AllowedEmployeeIDs, []int64{1001}) {
				t.Fatalf("derived filter-options scope=%#v", repo.employeeFilters)
			}
			if len(repo.pageFilters) != 2 || projectionFilterString(t, repo.pageFilters[0], "View") != view || projectionFilterString(t, repo.pageFilters[1], "View") != view {
				t.Fatalf("records/export projection filters=%#v", repo.pageFilters)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/"+view+"/status", nil))
			if recorder.Code != http.StatusOK || provider.chatCalls != 0 || provider.statusCalls != 1 || len(repo.latestRunTypes) != 1 || repo.latestRunTypes[0] != AnalysisTypeSession {
				t.Fatalf("status=%d chat=%d statusCalls=%d runTypes=%#v body=%s", recorder.Code, provider.chatCalls, provider.statusCalls, repo.latestRunTypes, recorder.Body.String())
			}
		})
	}
}

func TestWorkspaceStatusResolvesAuthenticatedTenantCorpWithoutChat(t *testing.T) {
	provider := &projectionAIProvider{}
	resolver := &workspaceProviderResolver{provider: provider}
	handler := NewWorkspaceHandlerWithResolver(workspaceTestResolver{principal: WorkspacePrincipal{UserID: 7, TenantID: 11, CorpID: 22}}, nil, workspaceTestRepo{}, resolver)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/session-analysis/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || resolver.calls != 1 || resolver.tenantID != 11 || resolver.corpID != 22 || provider.chatCalls != 0 {
		t.Fatalf("status=%d resolver=%#v chat=%d", rec.Code, resolver, provider.chatCalls)
	}
}

type workspaceProviderResolver struct {
	provider         providers.AIProvider
	calls            int
	tenantID, corpID int64
}

func (r *workspaceProviderResolver) Resolve(_ context.Context, tenantID, corpID int64) (providers.AIProvider, error) {
	r.calls++
	r.tenantID, r.corpID = tenantID, corpID
	return r.provider, nil
}
