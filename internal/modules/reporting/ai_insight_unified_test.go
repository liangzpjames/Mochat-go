package reporting

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestOverviewAIInsightReadsLatestUnifiedTenantCorpProjection(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	generatedAt := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	query := `SELECT summary, provider, generated_at
		FROM mochat_go_ai_conversation_insights
		WHERE tenant_id = ? AND corp_id = ? AND analysis_type = 'smart' AND status = 'succeeded'
		ORDER BY analysis_date DESC, generated_at DESC, id DESC
		LIMIT 1`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(int64(7), int64(8)).
		WillReturnRows(sqlmock.NewRows([]string{"summary", "provider", "generated_at"}).AddRow("统一智能洞察摘要", "openai-compatible", generatedAt))

	got, err := NewSQLRepository(db, OverviewReport).queryAIInsight(context.Background(), 7, 8)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Capability != "ready" || got.Provider != "openai-compatible" || got.Summary != "统一智能洞察摘要" || got.GeneratedAt != generatedAt.Format(time.RFC3339) {
		t.Fatalf("AI insight summary=%#v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
