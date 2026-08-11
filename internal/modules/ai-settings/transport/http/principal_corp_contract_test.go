package http

import (
	"net/http"
	"testing"
)

func TestKnowledgeBaseRejectsCorpAssertionDifferentFromDashboardPrincipal(t *testing.T) {
	handler := newTestKBs(nil)
	response := perform(handler, http.MethodGet, "/dashboard/ai-settings/knowledge-bases?corpId=99", "")

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", response.Code, response.Body.String())
	}
}
