package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReportRejectsCorpAssertionDifferentFromDashboardPrincipal(t *testing.T) {
	handler := NewHandler(serviceStub{}, resolverStub{}, nil)
	request := httptest.NewRequest(http.MethodGet, "/dashboard/reports/customer?corpId=99&timezone=Asia%2FShanghai&startAt=2026-08-01T00:00:00Z&endAt=2026-08-02T00:00:00Z", nil)
	request.SetPathValue("kind", "customer")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", response.Code, response.Body.String())
	}
}
