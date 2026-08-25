package aiinsight

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	settingsports "jiyi/mochat-go/internal/modules/ai-settings/ports"
	"jiyi/mochat-go/internal/modules/providers"
)

func TestDailyLoopTopLevelFailureLogNeverContainsDatabaseError(t *testing.T) {
	var output bytes.Buffer
	logDailyRunFailure(log.New(&output, "", 0), errors.New("fixture database secret"))
	if strings.Contains(output.String(), "fixture") || strings.Contains(output.String(), "secret") || !strings.Contains(output.String(), "AI_DAILY_RUN_FAILED") {
		t.Fatalf("unsafe daily failure log: %s", output.String())
	}
}

func TestDailyRunnerRecordsConversationFailuresBeforeReportingUnavailableProvider(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT c.tenant_id, c.id
		FROM mc_corp c
		JOIN mochat_go_tenant_corp_bindings b ON b.tenant_id = c.tenant_id AND b.corp_id = c.id
		WHERE c.deleted_at IS NULL AND b.status = 2
		ORDER BY c.id`)).WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "id"}).AddRow(1, 2))

	repo := &runnerRepoStub{rules: []AnalysisRuleVersion{{ID: 22, RuleID: 12, Version: 1}}}
	provider := unavailableAIProvider{}
	runner := &DailyAnalysisRunner{
		db:           db,
		conversation: newConversationAnalysisRunnerForTest(repo, provider, RunnerConfig{}, log.New(io.Discard, "", 0)),
		logger:       log.New(io.Discard, "", 0),
	}

	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repo.runs) != 2 || repo.runs[0].AnalysisType != AnalysisTypeSession || repo.runs[1].AnalysisType != AnalysisTypeSmart {
		t.Fatalf("runs = %#v, want session and default smart failures", repo.runs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDailyRunnerContinuesAfterOneCorpProviderResolutionFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT c.tenant_id, c.id
		FROM mc_corp c
		JOIN mochat_go_tenant_corp_bindings b ON b.tenant_id = c.tenant_id AND b.corp_id = c.id
		WHERE c.deleted_at IS NULL AND b.status = 2
		ORDER BY c.id`)).WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "id"}).AddRow(1, 2).AddRow(3, 4))
	repo := &runnerRepoStub{sessionRule: &AnalysisRuleVersion{ID: 11, RuleID: 1, Version: 1, MinimumMessages: 1}}
	provider := &capturingAIProvider{}
	resolver := &scopedResolver{providers: map[string]providers.AIProvider{"3/4": provider}, errors: map[string]error{"1/2": errors.New("fixture-secret-must-not-persist")}}
	runner := NewDailyAnalysisRunnerWithResolver(db, repo, resolver, log.New(io.Discard, "", 0), nil)
	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if resolver.calls["1/2"] != 1 || resolver.calls["3/4"] != 1 || provider.calls != 1 {
		t.Fatalf("resolver calls=%#v chat=%d", resolver.calls, provider.calls)
	}
	for _, finished := range repo.finished {
		if strings.Contains(finished.ErrorSummary, "fixture-secret-must-not-persist") {
			t.Fatalf("unsafe resolver error persisted: %#v", finished)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type scopedResolver struct {
	providers map[string]providers.AIProvider
	errors    map[string]error
	calls     map[string]int
}

func TestDailyRunnerUsesAssistantEnablementAndTenantGuidance(t *testing.T) {
	for _, test := range []struct {
		name        string
		assistants  *systemAssistantStub
		wantCalls   int
		wantContent []string
	}{
		{name: "disabled assistants never chat", assistants: &systemAssistantStub{contexts: map[string]settingsports.SystemAssistantContext{settingsports.SessionAnalysisSystemKey: {Enabled: false}, settingsports.SmartAnalysisSystemKey: {Enabled: false}}, loadErrs: map[string]error{}}, wantCalls: 0},
		{name: "enabled assistants contribute settings", assistants: &systemAssistantStub{contexts: map[string]settingsports.SystemAssistantContext{
			settingsports.SessionAnalysisSystemKey: {Enabled: true, Instructions: "每日会话说明", SettingsFingerprint: "daily-session", KnowledgeChunks: []settingsports.KnowledgeChunk{{Content: "每日会话知识"}}},
			settingsports.SmartAnalysisSystemKey:   {Enabled: true, Instructions: "每日智能说明", SettingsFingerprint: "daily-smart", KnowledgeChunks: []settingsports.KnowledgeChunk{{Content: "每日智能知识"}}},
		}, loadErrs: map[string]error{}}, wantCalls: 2, wantContent: []string{"每日会话说明", "每日会话知识", "每日智能说明", "每日智能知识"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectQuery("SELECT c.tenant_id, c.id").WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "id"}).AddRow(1, 2))
			repo := &runnerRepoStub{sessionRule: &AnalysisRuleVersion{ID: 11, RuleID: 1, Version: 1, ConversationTypes: []string{"direct"}, MinimumMessages: 1}, rules: []AnalysisRuleVersion{{ID: 22, RuleID: 2, Version: 1, Objective: "智能目标", ConversationTypes: []string{"direct"}, MinimumMessages: 1}}}
			provider := &capturingAIProvider{}
			resolver := &scopedResolver{providers: map[string]providers.AIProvider{"1/2": provider}, errors: map[string]error{}}
			runner := NewDailyAnalysisRunnerWithResolver(db, repo, resolver, log.New(io.Discard, "", 0), test.assistants)
			if err := runner.RunOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			if provider.calls != test.wantCalls {
				t.Fatalf("chat calls=%d want=%d", provider.calls, test.wantCalls)
			}
			joined := ""
			for _, request := range provider.requests {
				joined += request.System + request.Prompt
			}
			for _, content := range test.wantContent {
				if !strings.Contains(joined, content) {
					t.Fatalf("requests missing expected assistant content category")
				}
			}
			if test.wantCalls > 0 && (len(repo.saved) != 2 || repo.saved[0].SourceFingerprint == "messages-v1" || repo.saved[1].SourceFingerprint == "messages-v1") {
				t.Fatalf("assistant settings fingerprint not applied")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func (r *scopedResolver) Resolve(_ context.Context, tenantID, corpID int64) (providers.AIProvider, error) {
	if r.calls == nil {
		r.calls = map[string]int{}
	}
	key := fmt.Sprintf("%d/%d", tenantID, corpID)
	r.calls[key]++
	return r.providers[key], r.errors[key]
}
