package aiinsight

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"jiyi/mochat-go/internal/modules/providers"
)

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
		ai:           provider,
		conversation: NewConversationAnalysisRunner(repo, provider, RunnerConfig{}, log.New(io.Discard, "", 0)),
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
	runner := NewDailyAnalysisRunnerWithResolver(db, repo, resolver, log.New(io.Discard, "", 0))
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

func (r *scopedResolver) Resolve(_ context.Context, tenantID, corpID int64) (providers.AIProvider, error) {
	if r.calls == nil {
		r.calls = map[string]int{}
	}
	key := fmt.Sprintf("%d/%d", tenantID, corpID)
	r.calls[key]++
	return r.providers[key], r.errors[key]
}
