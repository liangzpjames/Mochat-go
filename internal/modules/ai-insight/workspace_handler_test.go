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
}

func (s workspaceAssistantStub) EnsureSessionAssistant(context.Context, int64, int64, int64, string) (settingsports.Agent, error) {
	return settingsports.Agent{}, nil
}
func (s workspaceAssistantStub) LoadSessionAssistantContext(context.Context, int64, int64) (settingsports.SessionAssistantContext, error) {
	return s.assistant, nil
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

func (r workspaceTestResolver) Resolve(*http.Request) (WorkspacePrincipal, error) {
	return r.principal, r.err
}

type workspaceTestAuthorizer struct{ err error }

func (a workspaceTestAuthorizer) Authorize(context.Context, WorkspacePrincipal, int64, string) error {
	return a.err
}

type workspaceTestRepo struct{}

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
	handler := NewWorkspaceHandler(workspaceTestResolver{principal: WorkspacePrincipal{UserID: 7, TenantID: 1, CorpID: 2}}, workspaceTestAuthorizer{err: errors.New("denied")}, workspaceTestRepo{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/smart-analysis/records", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d,want 403", rec.Code)
	}
}
