package aiinsight

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestLatestSucceededFingerprintOnlyChecksCurrentAnalysisDate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	analysisDate := time.Date(2026, 8, 25, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT source_fingerprint FROM mochat_go_ai_conversation_insights WHERE tenant_id=? AND corp_id=? AND analysis_type=? AND rule_version_id=? AND conversation_key=? AND analysis_date=? AND status='succeeded' ORDER BY generated_at DESC, id DESC LIMIT 1")).
		WithArgs(int64(7), int64(8), AnalysisTypeSession, int64(9), "conversation-1", "2026-08-25").
		WillReturnRows(sqlmock.NewRows([]string{"source_fingerprint"}).AddRow("today-fingerprint"))

	got, err := NewSQLRepository(db).LatestSucceededFingerprint(context.Background(), 7, 8, AnalysisTypeSession, 9, "conversation-1", analysisDate)
	if err != nil || got != "today-fingerprint" {
		t.Fatalf("fingerprint=%q err=%v", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPreviousSucceededInsightSelectsLatestPriorDayWithinFullScope(t *testing.T) {
	tests := []struct {
		name         string
		analysisType AnalysisType
		scorePath    string
		score        float64
	}{
		{name: "session employee score", analysisType: AnalysisTypeSession, scorePath: "$.employeeQa.score", score: 86},
		{name: "smart match score", analysisType: AnalysisTypeSmart, scorePath: "$.matchScore", score: 72.5},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			analysisDate := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
			generatedAt := time.Date(2026, 8, 24, 15, 30, 0, 0, time.UTC)
			query := "SELECT id,CASE WHEN JSON_VALID(result_json) AND JSON_TYPE(JSON_EXTRACT(result_json,'" + test.scorePath + "')) IN ('INTEGER','DOUBLE') THEN CAST(JSON_UNQUOTE(JSON_EXTRACT(result_json,'" + test.scorePath + "')) AS DECIMAL(6,2)) END,summary,generated_at,analysis_date FROM mochat_go_ai_conversation_insights WHERE tenant_id=? AND corp_id=? AND analysis_type=? AND rule_version_id=? AND conversation_key=? AND status='succeeded' AND analysis_date<? ORDER BY analysis_date DESC,generated_at DESC,id DESC LIMIT 1"
			mock.ExpectQuery(regexp.QuoteMeta(query)).
				WithArgs(int64(7), int64(8), test.analysisType, int64(9), "conversation-1", "2026-08-25").
				WillReturnRows(sqlmock.NewRows([]string{"id", "score", "summary", "generated_at", "analysis_date"}).AddRow(41, test.score, "上次结果摘要 msg:historical-only", generatedAt, analysisDate.AddDate(0, 0, -1)))

			got, err := NewSQLRepository(db).PreviousSucceededInsight(context.Background(), 7, 8, test.analysisType, 9, "conversation-1", analysisDate)
			if err != nil {
				t.Fatal(err)
			}
			if got == nil || got.ID != 41 || got.Score == nil || *got.Score != test.score || got.Summary != "上次结果摘要 msg:historical-only" || !got.GeneratedAt.Equal(generatedAt) {
				t.Fatalf("previous=%#v", got)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPreviousSucceededInsightAllowsSummaryWithoutScore(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	analysisDate := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	query := "SELECT id,CASE WHEN JSON_VALID(result_json) AND JSON_TYPE(JSON_EXTRACT(result_json,'$.matchScore')) IN ('INTEGER','DOUBLE') THEN CAST(JSON_UNQUOTE(JSON_EXTRACT(result_json,'$.matchScore')) AS DECIMAL(6,2)) END,summary,generated_at,analysis_date FROM mochat_go_ai_conversation_insights WHERE tenant_id=? AND corp_id=? AND analysis_type=? AND rule_version_id=? AND conversation_key=? AND status='succeeded' AND analysis_date<? ORDER BY analysis_date DESC,generated_at DESC,id DESC LIMIT 1"
	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(int64(7), int64(8), AnalysisTypeSmart, int64(9), "conversation-1", "2026-08-25").
		WillReturnRows(sqlmock.NewRows([]string{"id", "score", "summary", "generated_at", "analysis_date"}).AddRow(41, nil, "只有摘要仍可连续", nil, analysisDate.AddDate(0, 0, -1)))

	got, err := NewSQLRepository(db).PreviousSucceededInsight(context.Background(), 7, 8, AnalysisTypeSmart, 9, "conversation-1", analysisDate)
	if err != nil || got == nil || got.Score != nil || got.Summary != "只有摘要仍可连续" {
		t.Fatalf("previous=%#v err=%v", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSaveInsightPersistsDailyKeyAndPreviousSnapshot(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	analysisDate := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	generatedAt := analysisDate.Add(10 * time.Hour)
	previousGeneratedAt := analysisDate.Add(-14 * time.Hour)
	previousScore := 84.5
	insight := ConversationInsight{
		TenantID: 7, CorpID: 8, AnalysisType: AnalysisTypeSession, RuleID: 9, RuleVersionID: 10,
		ConversationKey: "11:1:customer", AnalysisDate: analysisDate, EmployeeID: 11,
		SourceFingerprint: "source-fingerprint", Status: AnalysisStatusSucceeded, Summary: "摘要", ResultJSON: []byte(`{"schemaVersion":2}`),
		PreviousInsightID: 40, PreviousScore: &previousScore, PreviousSummary: "昨日摘要", PreviousGeneratedAt: &previousGeneratedAt,
		GeneratedAt: &generatedAt,
	}
	mock.ExpectExec("INSERT INTO mochat_go_ai_conversation_insights").
		WithArgs(
			insight.TenantID, insight.CorpID, insight.AnalysisType, insight.RuleID, insight.RuleVersionID,
			insight.ConversationKey, "2026-08-25", insight.EmployeeID, insight.EmployeeName, insight.EmployeeAvatar,
			insight.TargetType, insight.TargetID, insight.TargetName, insight.TargetAvatar, nil, nil,
			insight.SourceMessageCount, insight.SourceFingerprint, insight.Status, insight.Summary, string(insight.ResultJSON),
			insight.ErrorSummary, insight.Provider, insight.Model, insight.PromptVersion, insight.PreviousInsightID,
			previousScore, insight.PreviousSummary, previousGeneratedAt, generatedAt,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := NewSQLRepository(db).SaveInsight(context.Background(), insight); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAnalysisDateSQLValuePreservesShanghaiCalendarDay(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	value := time.Date(2026, 8, 25, 0, 0, 0, 0, shanghai)
	if got := analysisDateSQLValue(value); got != "2026-08-25" {
		t.Fatalf("analysis date SQL value=%q, want Shanghai calendar date", got)
	}
}

func TestSaveInsightRejectsMissingAnalysisDate(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	err = NewSQLRepository(db).SaveInsight(context.Background(), ConversationInsight{ResultJSON: []byte(`{}`)})
	if err == nil || !strings.Contains(err.Error(), "analysis date") {
		t.Fatalf("missing analysis date error=%v", err)
	}
}
