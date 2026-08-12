package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLeadIgnoresCorpAssertionAndUsesDashboardPrincipal(t *testing.T) {
	service := &fakeLeadService{}
	handler := NewLeadHandler(service, fakePrincipalResolver{principal: Principal{UserID: 7, TenantID: 41, CorpID: 41}})
	response := httptest.NewRecorder()

	handler.List(response, httptest.NewRequest(http.MethodGet, LeadsPath+"?corpId=99", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
	if service.listCalls != 1 {
		t.Fatalf("service list calls = %d, want 1", service.listCalls)
	}
}
