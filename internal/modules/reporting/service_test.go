package reporting

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestReportResultMarshalsOverviewModules(t *testing.T) {
	result := ReportResult{
		AIMetrics: &AIMetrics{AnalysisCount: intPtr(5)},
		Quality: &QualityStats{
			SensitiveWords: intPtr(2), RiskBehavior: intPtr(4), CustomerLoss: intPtr(0), TimeoutWarning: intPtr(3),
			Trend: []QualityTrendPoint{{Date: "2026-08-15", SensitiveWords: intPtr(1), RiskBehavior: intPtr(2), CustomerLoss: intPtr(0), TimeoutWarning: intPtr(2)}},
		},
		EmployeeRanking: []EmployeeRankingItem{{EmployeeID: 1001, EmployeeName: "张伟", Sessions: 1, Messages: 13}},
		Trajectory:      []ConversationTrajectoryItem{{ID: "customer:2001", TargetType: "customer", TargetID: "2001", EmployeeName: "张伟", MessageCount: 13, LatestAt: "2026-08-16 13:50:08"}},
	}
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"aiMetrics"`, `"quality"`, `"employeeRanking"`, `"trajectory"`} {
		if !strings.Contains(string(payload), field) {
			t.Fatalf("overview response is missing %s: %s", field, payload)
		}
	}
	if !strings.Contains(string(payload), `"trend":[{"date":"2026-08-15","sensitiveWords":1,"riskBehavior":2,"customerLoss":0,"timeoutWarning":2}]`) {
		t.Fatalf("overview quality trend is missing: %s", payload)
	}
}

type sourceStub struct {
	result ReportResult
	err    error
	query  ReportQuery
}

func (s *sourceStub) Query(_ context.Context, query ReportQuery) (ReportResult, error) {
	s.query = query
	return s.result, s.err
}

func TestServiceValidatesTimezoneAndHalfOpenWindow(t *testing.T) {
	service := NewService(map[ReportKind]Source{CustomerReport: &sourceStub{}})
	_, err := service.Query(context.Background(), CustomerReport, ReportQuery{
		TenantID: 1, CorpID: 2, Timezone: "Bad/Zone", StartAt: time.Now(), EndAt: time.Now(),
	})
	if err == nil {
		t.Fatal("expected invalid query")
	}
}

func TestServiceIntersectsRequestedEmployeesWithScope(t *testing.T) {
	source := &sourceStub{result: ReportResult{Summary: map[string]*float64{}}}
	service := NewService(map[ReportKind]Source{CustomerReport: source})
	_, err := service.Query(context.Background(), CustomerReport, ReportQuery{
		TenantID: 1, CorpID: 2, Timezone: "Asia/Shanghai",
		StartAt: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), EndAt: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC),
		EmployeeIDs: []int64{2, 3, 4}, AllowedEmployeeIDs: []int64{1, 3}, Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(source.query.EmployeeIDs) != 1 || source.query.EmployeeIDs[0] != 3 {
		t.Fatalf("employee scope = %#v", source.query.EmployeeIDs)
	}
}

func TestServiceRejectsUnknownConversionStage(t *testing.T) {
	service := NewService(map[ReportKind]Source{ConversionReport: &sourceStub{}})
	query := validQuery()
	query.Stage = "unknown"
	if _, err := service.Query(context.Background(), ConversionReport, query); err == nil {
		t.Fatal("expected invalid query for unknown stage")
	}
	query.Stage = "won"
	if _, err := service.Query(context.Background(), ConversionReport, query); err != nil {
		t.Fatalf("valid stage rejected: %v", err)
	}
}

func TestRatioReturnsNilForZeroDenominator(t *testing.T) {
	if Ratio(3, 0) != nil {
		t.Fatal("zero denominator must return null")
	}
}

func TestUnavailableProviderBecomesLimitation(t *testing.T) {
	service := NewService(map[ReportKind]Source{EmployeeReport: UnavailableSource("conversation_archive", "会话存档 Provider 不可用")})
	result, err := service.Query(context.Background(), EmployeeReport, validQuery())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Limitations) != 1 || result.Limitations[0].Provider != "conversation_archive" {
		t.Fatalf("limitations = %#v", result.Limitations)
	}
}

func TestParseKindAcceptsOverview(t *testing.T) {
	kind, ok := ParseKind("overview")
	if !ok || kind != OverviewReport {
		t.Fatalf("ParseKind(overview) = %q, %v", kind, ok)
	}
}

func TestServiceRunsOverviewThroughItsSource(t *testing.T) {
	source := &sourceStub{result: ReportResult{Summary: map[string]*float64{"customer": floatPtr(3)}}}
	service := NewService(map[ReportKind]Source{OverviewReport: source})
	if _, err := service.Query(context.Background(), OverviewReport, validQuery()); err != nil {
		t.Fatalf("overview query failed: %v", err)
	}
	if source.query.CorpID != 2 || source.query.Page != 1 {
		t.Fatalf("query passed to source = %#v", source.query)
	}
}

func floatPtr(value float64) *float64 { return &value }

func intPtr(value int) *int { return &value }

func validQuery() ReportQuery {
	return ReportQuery{TenantID: 1, CorpID: 2, Timezone: "Asia/Shanghai", StartAt: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), EndAt: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC), Page: 1, PageSize: 20}
}
