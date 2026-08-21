//go:build integration

package reporting

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Runs against a retained-volume schema when MOCHAT_MYSQL_DSN is provided.
// The test intentionally uses an empty tenant/corp so it proves every report
// kind executes real SQL without leaking data or requiring fixtures.
func TestSQLRepositoryAgainstRetainedMariaDBSchema(t *testing.T) {
	dsn := os.Getenv("MOCHAT_MYSQL_DSN")
	if dsn == "" {
		t.Skip("MOCHAT_MYSQL_DSN not set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	service := NewSQLService(db)
	q := ReportQuery{TenantID: 1, CorpID: 1, StartAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), EndAt: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), Timezone: "Asia/Shanghai", Page: 1, PageSize: 20}
	for _, kind := range []ReportKind{CustomerReport, ConversionReport, EmployeeReport, BehaviorReport, DetailReport, OverviewReport} {
		if _, err := service.Query(ctx, kind, q); err != nil {
			t.Fatalf("kind %s failed against retained schema: %v", kind, err)
		}
	}
	for _, stage := range []string{"lead", "contact", "opportunity", "won", "order"} {
		stageQuery := q
		stageQuery.Stage = stage
		result, err := service.Query(ctx, ConversionReport, stageQuery)
		if err != nil {
			t.Fatalf("conversion stage %s failed against retained schema: %v", stage, err)
		}
		if int(result.Pagination.Total) != int(*result.Summary[stage]) {
			t.Fatalf("stage %s pagination total=%d summary=%v", stage, result.Pagination.Total, *result.Summary[stage])
		}
	}
}

func TestOverviewIncludesConversationAIInsightAndFreshness(t *testing.T) {
	dsn := os.Getenv("MOCHAT_MYSQL_DSN")
	if dsn == "" {
		t.Skip("MOCHAT_MYSQL_DSN not set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	service := NewSQLService(db)
	q := ReportQuery{
		TenantID: 1,
		CorpID:   1,
		StartAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:    time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		Timezone: "Asia/Shanghai",
		Page:     1,
		PageSize: 20,
	}
	result, err := service.Query(ctx, OverviewReport, q)
	if err != nil {
		t.Fatalf("overview failed: %v", err)
	}
	if result.Conversation == nil {
		t.Fatal("overview conversation stats are missing")
	}
	if result.Freshness.DataThrough.Year() < 2026 {
		t.Fatalf("overview dataThrough is not populated: %v", result.Freshness.DataThrough)
	}
	if result.AIInsight == nil {
		t.Log("overview aiInsight nil (no persisted analysis row yet)")
	} else if result.AIInsight.Capability != "ready" {
		t.Fatalf("overview aiInsight capability = %q, want ready", result.AIInsight.Capability)
	}
	if result.AIMetrics == nil || result.AIMetrics.AnalysisCount == nil {
		t.Fatal("overview aiMetrics.analysisCount is not populated from persisted analyses")
	}
	if result.Quality == nil || result.Quality.RiskBehavior == nil || result.Quality.SensitiveWords == nil || result.Quality.TimeoutWarning == nil || result.Quality.CustomerLoss == nil {
		t.Fatalf("overview quality metrics are incomplete: %+v", result.Quality)
	}
	if len(result.EmployeeRanking) == 0 {
		t.Fatal("overview employee ranking is empty despite retained archive data")
	}
	if len(result.Trajectory) == 0 {
		t.Fatal("overview conversation trajectory is empty despite retained archive data")
	}
}

func TestOverviewConversationTrendWindowUsesLocalDays(t *testing.T) {
	dsn := os.Getenv("MOCHAT_MYSQL_DSN")
	if dsn == "" {
		t.Skip("MOCHAT_MYSQL_DSN not set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	service := NewSQLService(db)
	trendEnd := time.Date(2026, 8, 15, 16, 0, 0, 0, time.UTC) // 2026-08-16T00:00+08:00
	trendStart := trendEnd.AddDate(0, 0, -7)
	q := ReportQuery{
		TenantID:     1,
		CorpID:       1,
		StartAt:      trendStart,
		EndAt:        trendEnd,
		TrendStartAt: &trendStart,
		TrendEndAt:   &trendEnd,
		Timezone:     "Asia/Shanghai",
		Page:         1,
		PageSize:     20,
	}
	result, err := service.Query(ctx, OverviewReport, q)
	if err != nil {
		t.Fatalf("overview failed: %v", err)
	}
	if result.Conversation == nil {
		t.Fatal("overview conversation stats are missing")
	}
	if result.Quality == nil {
		t.Fatal("overview quality stats are missing")
	}
	trend := result.Conversation.Trend
	if len(trend) != 7 {
		t.Fatalf("conversation trend has %d points, want 7", len(trend))
	}
	want := []string{"2026-08-09", "2026-08-10", "2026-08-11", "2026-08-12", "2026-08-13", "2026-08-14", "2026-08-15"}
	for i := range want {
		if trend[i].Date != want[i] {
			t.Fatalf("conversation trend[%d].Date = %q, want %q (full %v)", i, trend[i].Date, want[i], datesOf(trend))
		}
	}
	totalCustomerSessions := 0
	for _, point := range trend {
		totalCustomerSessions += point.CustomerSessions
	}
	if totalCustomerSessions == 0 {
		t.Fatalf("conversation trend carries no message data, expected non-zero session counts: %+v", trend)
	}
	qualityTrend := result.Quality.Trend
	if len(qualityTrend) != 7 {
		t.Fatalf("quality trend has %d points, want 7", len(qualityTrend))
	}
	qualitySignals := 0
	for i := range want {
		if qualityTrend[i].Date != want[i] {
			t.Fatalf("quality trend[%d].Date = %q, want %q", i, qualityTrend[i].Date, want[i])
		}
		qualitySignals += pointerValue(qualityTrend[i].RiskBehavior)
		qualitySignals += pointerValue(qualityTrend[i].SensitiveWords)
		qualitySignals += pointerValue(qualityTrend[i].TimeoutWarning)
	}
	if qualitySignals == 0 {
		t.Fatalf("quality trend carries no retained risk data: %+v", qualityTrend)
	}
	if len(result.Series) != 7 {
		t.Fatalf("customer growth series has %d points, want 7 local days: %+v", len(result.Series), result.Series)
	}
	totalGrowth := 0.0
	for _, point := range result.Series {
		totalGrowth += point.Value
	}
	if totalGrowth == 0 {
		t.Fatalf("customer growth series carries no data: %+v", result.Series)
	}
}

func pointerValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func datesOf(points []ConversationTrendPoint) []string {
	dates := make([]string, len(points))
	for i := range points {
		dates[i] = points[i].Date
	}
	return dates
}
