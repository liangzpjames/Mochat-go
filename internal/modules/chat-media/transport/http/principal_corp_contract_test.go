package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMediaListRejectsCorpAssertionDifferentFromDashboardPrincipal(t *testing.T) {
	handler, _, _ := newTestHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/dashboard/chat/media?corpId=99", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", response.Code, response.Body.String())
	}
}
