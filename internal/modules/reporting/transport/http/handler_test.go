package http

import (
	"context"
	"encoding/json"
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

func TestReportResponseUsesSharedMsgEnvelope(t *testing.T) {
	handler := NewHandler(serviceStub{}, resolverStub{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/reports/customer?corpId=9&timezone=Asia%2FShanghai&startAt=2026-08-01T00:00:00Z&endAt=2026-08-02T00:00:00Z", nil)
	req.SetPathValue("kind", "customer")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	var envelope map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["msg"] != "success" {
		t.Fatalf("msg=%v body=%s", envelope["msg"], recorder.Body.String())
	}
	if _, ok := envelope["message"]; ok {
		t.Fatalf("legacy message field present: %s", recorder.Body.String())
	}
}
