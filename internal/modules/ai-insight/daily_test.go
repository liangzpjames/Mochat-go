package aiinsight

import (
	"context"
	"io"
	"log"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
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

	if err := runner.RunOnce(context.Background()); err == nil {
		t.Fatal("expected unavailable provider error")
	}
	if len(repo.runs) != 2 || repo.runs[0].AnalysisType != AnalysisTypeSession || repo.runs[1].AnalysisType != AnalysisTypeSmart {
		t.Fatalf("runs = %#v, want session and default smart failures", repo.runs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
