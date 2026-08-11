package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLeadRejectsCorpAssertionDifferentFromDashboardPrincipal(t *testing.T) {
	service := &fakeLeadService{}
	handler := NewLeadHandler(service, fakePrincipalResolver{principal: Principal{UserID: 7, TenantID: 41, CorpID: 41}})
	response := httptest.NewRecorder()

	handler.List(response, httptest.NewRequest(http.MethodGet, LeadsPath+"?corpId=99", nil))

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", response.Code, response.Body.String())
	}
	if service.listCalls != 0 {
		t.Fatalf("service list calls = %d, want 0", service.listCalls)
	}
}
