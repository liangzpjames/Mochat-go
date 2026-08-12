package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMediaListIgnoresCorpAssertionAndUsesDashboardPrincipal(t *testing.T) {
	handler, _, _ := newTestHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/dashboard/chat/media?corpId=99", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
}
