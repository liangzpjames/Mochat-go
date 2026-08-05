package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"jiyi/mochat-go/internal/modules/reporting"
)

type resolverStub struct{}

func (resolverStub) Resolve(*http.Request) (Principal, error) { return Principal{TenantID: 7}, nil }

type serviceStub struct{}

func (serviceStub) Query(context.Context, reporting.ReportKind, reporting.ReportQuery) (reporting.ReportResult, error) {
	return reporting.ReportResult{}, nil
}

func TestUnknownReportReturns422(t *testing.T) {
	handler := NewHandler(serviceStub{}, resolverStub{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/reports/unknown?corpId=9&timezone=Asia%2FShanghai&startAt=2026-08-01T00:00:00Z&endAt=2026-08-02T00:00:00Z", nil)
	req.SetPathValue("kind", "unknown")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
