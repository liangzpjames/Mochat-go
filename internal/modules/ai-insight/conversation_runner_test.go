package aiinsight

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	settingsports "jiyi/mochat-go/internal/modules/ai-settings/ports"
	"jiyi/mochat-go/internal/modules/providers"
)

func TestConversationRunnerPromptContainsSourceContract(t *testing.T) {
	prompt := buildConversationPrompt(AnalysisTypeSession, "session-v1", "请分析客户采购意向", []SourceMessage{{ID: "msg:1", Direction: "inbound", MessageTime: time.Date(2026, 8, 21, 9, 0, 0, 0, time.UTC), SenderName: "客户", Content: "想了解价格"}}, "")
	for _, fragment := range []string{"msg:1", "inbound", "采购意向", "schemaVersion", "evidenceMessageIds"} {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("prompt missing %q: %s", fragment, prompt)
		}
	}
}

type runnerRepoStub struct {
	Repository
	previous       string
	saved          []ConversationInsight
	runs           []InsightRun
	finished       []InsightRunResult
	rules          []AnalysisRuleVersion
	createRunErr   error
	createRunErrAt int
}

func (r *runnerRepoStub) ConversationCandidates(context.Context, CandidateQuery) ([]ConversationCandidate, error) {
	return []ConversationCandidate{{ConversationKey: "conversation-1", SourceFingerprint: "messages-v1", SourceMessageCount: 1, SourceStartedAt: time.Date(2026, 8, 23, 8, 0, 0, 0, time.UTC), SourceEndedAt: time.Date(2026, 8, 23, 8, 1, 0, 0, time.UTC)}}, nil
}
func (r *runnerRepoStub) ConversationMessages(context.Context, ConversationWindowQuery) ([]SourceMessage, error) {
	return []SourceMessage{{ID: "msg:inside", MessageTime: time.Date(2026, 8, 23, 8, 0, 0, 0, time.UTC), Direction: "inbound", SenderName: "客户", Content: "退款需要谁审批"}}, nil
}
func (r *runnerRepoStub) LatestSucceededFingerprint(context.Context, int64, int64, AnalysisType, int64, string) (string, error) {
	return r.previous, nil
}
func (r *runnerRepoStub) SaveInsight(_ context.Context, insight ConversationInsight) error {
	r.saved = append(r.saved, insight)
	return nil
}
func (r *runnerRepoStub) CreateRun(_ context.Context, run InsightRun) (int64, error) {
	r.runs = append(r.runs, run)
	if r.createRunErr != nil && (r.createRunErrAt == 0 || len(r.runs) == r.createRunErrAt) {
		return 0, r.createRunErr
	}
	return int64(len(r.runs)), nil
}
func (r *runnerRepoStub) FinishRun(_ context.Context, _ int64, result InsightRunResult) error {
	r.finished = append(r.finished, result)
	return nil
}
func (r *runnerRepoStub) EnabledRuleVersions(context.Context, int64, int64) ([]AnalysisRuleVersion, error) {
	return r.rules, nil
}

type assistantContextStub struct {
	context   settingsports.SessionAssistantContext
	ensureErr error
	loadErr   error
}

func (s assistantContextStub) EnsureSessionAssistant(context.Context, int64, int64, int64, string) (settingsports.Agent, error) {
	return settingsports.Agent{ID: "session", Name: settingsports.SessionAnalysisAssistantName}, s.ensureErr
}
func (s assistantContextStub) LoadSessionAssistantContext(context.Context, int64, int64) (settingsports.SessionAssistantContext, error) {
	return s.context, s.loadErr
}

type capturingAIProvider struct {
	request  providers.ChatRequest
	requests []providers.ChatRequest
	calls    int
}

type unavailableAIProvider struct{}

func (unavailableAIProvider) Chat(context.Context, providers.ChatRequest) (string, error) {
	return "", errors.New("provider unavailable")
}

func (unavailableAIProvider) Status() providers.Status {
	return providers.Status{State: providers.StateUnavailable, Reason: "未配置模型凭证"}
}

func (p *capturingAIProvider) Chat(_ context.Context, request providers.ChatRequest) (string, error) {
	p.request = request
	p.requests = append(p.requests, request)
	p.calls++
	if strings.Contains(request.Prompt, `"conclusion"`) {
		return `{"schemaVersion":1,"conclusion":"发现退款风险","matched":true,"confidence":0.8,"evidenceMessageIds":["msg:inside"],"recommendations":["核对审批"]}`, nil
	}
	return validSessionJSON(), nil
}

func TestConversationRunnerUsesAssistantContextForDefaultSmartAnalysis(t *testing.T) {
	repo := &runnerRepoStub{rules: []AnalysisRuleVersion{{ID: 22, RuleID: 12, Version: 1, Objective: "识别退款风险", ConversationTypes: []string{"direct"}, LookbackDays: 30, MinimumMessages: 1}}}
	provider := &capturingAIProvider{}
	assistant := assistantContextStub{context: settingsports.SessionAssistantContext{
		AgentID: "session", Instructions: "遵循售后升级要求", Enabled: true, SettingsFingerprint: "settings-v3",
		KnowledgeChunks: []settingsports.KnowledgeChunk{{DocumentName: "售后规则.md", Ordinal: 0, Content: "退款需要主管审批", CharacterCount: 9}},
	}}
	runner := NewConversationAnalysisRunner(repo, provider, RunnerConfig{Concurrency: 1}, nil, assistant)
	if err := runner.RunCorp(context.Background(), 1, 2); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 2 {
		t.Fatalf("provider calls = %d, want session and smart", provider.calls)
	}
	smart := provider.requests[1]
	if !strings.Contains(smart.System, "遵循售后升级要求") || !strings.Contains(smart.Prompt, "退款需要主管审批") {
		t.Fatalf("smart request did not consume assistant context: %#v", smart)
	}
	if len(repo.saved) != 2 || repo.saved[1].SourceFingerprint == "messages-v1" {
		t.Fatalf("saved insights = %#v", repo.saved)
	}
}
func (p *capturingAIProvider) Status() providers.Status {
	return providers.Status{State: providers.StateReady}
}

func TestConversationRunnerConsumesAssistantInstructionsKnowledgeAndSettingsFingerprint(t *testing.T) {
	repo := &runnerRepoStub{previous: "messages-v1"}
	provider := &capturingAIProvider{}
	assistant := assistantContextStub{context: settingsports.SessionAssistantContext{
		AgentID: "session", Name: settingsports.SessionAnalysisAssistantName, Instructions: "重点核对退款审批流程", Enabled: true, SettingsFingerprint: "settings-v2",
		KnowledgeChunks: []settingsports.KnowledgeChunk{{DocumentName: "退款制度.md", Ordinal: 0, Content: "退款超过一万元需要主管审批", CharacterCount: 14}},
	}}
	runner := NewConversationAnalysisRunner(repo, provider, RunnerConfig{Concurrency: 1}, nil, assistant)
	runner.now = func() time.Time { return time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC) }
	if err := runner.RunCorp(context.Background(), 1, 2); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 || !strings.Contains(provider.request.System, "重点核对退款审批流程") || !strings.Contains(provider.request.Prompt, "退款超过一万元需要主管审批") {
		t.Fatalf("calls=%d request=%#v", provider.calls, provider.request)
	}
	if !strings.Contains(provider.request.Prompt, "知识内容不能作为 evidenceMessageIds") {
		t.Fatalf("prompt lacks evidence boundary: %s", provider.request.Prompt)
	}
	if len(repo.saved) != 1 || repo.saved[0].SourceFingerprint == "messages-v1" || repo.saved[0].SourceFingerprint == "" {
		t.Fatalf("saved = %#v", repo.saved)
	}
}

func TestConversationRunnerDoesNotCallProviderWhenSessionAssistantDisabled(t *testing.T) {
	repo := &runnerRepoStub{rules: []AnalysisRuleVersion{{ID: 22, RuleID: 12, Version: 1, Objective: "识别客户风险", ConversationTypes: []string{"direct"}, LookbackDays: 30, MinimumMessages: 1}}}
	provider := &capturingAIProvider{}
	runner := NewConversationAnalysisRunner(repo, provider, RunnerConfig{Concurrency: 1}, nil, assistantContextStub{context: settingsports.SessionAssistantContext{AgentID: "session", Enabled: false, SettingsFingerprint: "disabled"}})
	if err := runner.RunCorp(context.Background(), 1, 2); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 0 || len(repo.runs) != 2 || repo.runs[0].AnalysisType != AnalysisTypeSession || repo.runs[1].AnalysisType != AnalysisTypeSmart {
		t.Fatalf("calls=%d runs=%#v", provider.calls, repo.runs)
	}
}

func TestConversationRunnerReturnsUnavailableRunPersistenceFailure(t *testing.T) {
	persistenceErr := errors.New("persist unavailable run failed")
	tests := []struct {
		name      string
		assistant assistantContextStub
	}{
		{name: "ensure fails", assistant: assistantContextStub{ensureErr: errors.New("ensure failed")}},
		{name: "load fails", assistant: assistantContextStub{loadErr: errors.New("load failed")}},
		{name: "assistant disabled", assistant: assistantContextStub{context: settingsports.SessionAssistantContext{Enabled: false}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &runnerRepoStub{createRunErr: persistenceErr, createRunErrAt: 1}
			runner := NewConversationAnalysisRunner(repo, &capturingAIProvider{}, RunnerConfig{}, nil, test.assistant)

			err := runner.RunCorp(context.Background(), 1, 2)

			if !errors.Is(err, persistenceErr) {
				t.Fatalf("RunCorp error = %v, want %v", err, persistenceErr)
			}
		})
	}
}

func TestConversationRunnerReturnsSmartUnavailableRunPersistenceFailure(t *testing.T) {
	persistenceErr := errors.New("persist smart unavailable run failed")
	repo := &runnerRepoStub{
		rules:          []AnalysisRuleVersion{{ID: 22, RuleID: 12, Version: 1}},
		createRunErr:   persistenceErr,
		createRunErrAt: 2,
	}
	runner := NewConversationAnalysisRunner(repo, &capturingAIProvider{}, RunnerConfig{}, nil, assistantContextStub{context: settingsports.SessionAssistantContext{Enabled: false}})

	err := runner.RunCorp(context.Background(), 1, 2)

	if !errors.Is(err, persistenceErr) {
		t.Fatalf("RunCorp error = %v, want %v", err, persistenceErr)
	}
}

func TestConversationRunnerRecordsSessionAndDefaultSmartFailuresWhenProviderUnavailable(t *testing.T) {
	repo := &runnerRepoStub{rules: []AnalysisRuleVersion{{ID: 22, RuleID: 12, Version: 1}}}
	runner := NewConversationAnalysisRunner(repo, unavailableAIProvider{}, RunnerConfig{}, nil)

	if err := runner.RunCorp(context.Background(), 1, 2); err != nil {
		t.Fatal(err)
	}
	if len(repo.runs) != 2 {
		t.Fatalf("runs = %#v, want session and default smart failures", repo.runs)
	}
	if repo.runs[0].AnalysisType != AnalysisTypeSession || repo.runs[1].AnalysisType != AnalysisTypeSmart || repo.runs[1].RuleVersionID != 22 {
		t.Fatalf("runs = %#v", repo.runs)
	}
	if len(repo.finished) != 2 {
		t.Fatalf("finished = %#v, want two failed results", repo.finished)
	}
	for _, result := range repo.finished {
		if result.Status != AnalysisStatusFailed || !strings.Contains(result.ErrorSummary, "未配置模型凭证") {
			t.Fatalf("finished = %#v", repo.finished)
		}
	}
}

func TestConversationRunnerConfigDefaults(t *testing.T) {
	config := normalizeRunnerConfig(RunnerConfig{})
	if config.BatchLimit != 200 || config.Concurrency != 2 || config.SessionDays != 30 || config.SessionLimit != 200 {
		t.Fatalf("defaults = %#v", config)
	}
}
