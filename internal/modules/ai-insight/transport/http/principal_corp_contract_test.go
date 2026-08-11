package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInsightRejectsCorpAssertionDifferentFromDashboardPrincipal(t *testing.T) {
	handler := NewInsightHandler(insightResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil)
	request := httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/emotion?corpId=99", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", response.Code, response.Body.String())
	}
}
