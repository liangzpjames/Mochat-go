package reporting

import (
	"context"
	"testing"
	"time"
)

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

func validQuery() ReportQuery {
	return ReportQuery{TenantID: 1, CorpID: 2, Timezone: "Asia/Shanghai", StartAt: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), EndAt: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC), Page: 1, PageSize: 20}
}
