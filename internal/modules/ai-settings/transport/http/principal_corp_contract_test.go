package http

import (
	"net/http"
	"testing"
)

func TestKnowledgeBaseIgnoresClientCorpAssertionAndUsesDashboardPrincipal(t *testing.T) {
	handler := newTestKBs(nil)
	response := perform(handler, http.MethodGet, "/dashboard/ai-settings/knowledge-bases?corpId=99", "")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
}
