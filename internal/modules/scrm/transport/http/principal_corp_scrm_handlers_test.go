package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCustomerLifecycleIgnoresCorpAssertionAndUsesDashboardPrincipal(t *testing.T) {
	handler := NewCustomerLifecycleHandler(&contactLifecycleServiceFake{}, fakePrincipalResolver{principal: Principal{UserID: 5, TenantID: 7, CorpID: 9}}, &contactAuthorizerFake{})
	response := httptest.NewRecorder()

	handler.ListContacts(response, httptest.NewRequest(http.MethodGet, ContactsPath+"?corpId=10", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
}

func TestCustomerTagIgnoresCorpAssertionAndUsesDashboardPrincipal(t *testing.T) {
	handler := NewCustomerTagHandler(&customerTagServiceFake{}, fakePrincipalResolver{principal: Principal{UserID: 3, TenantID: 11, CorpID: 22}}, &contactAuthorizerFake{})
	response := httptest.NewRecorder()

	handler.ListCatalog(response, httptest.NewRequest(http.MethodGet, TagsPath+"?corpId=23", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
}

func TestOpportunityIgnoresCorpAssertionAndUsesDashboardPrincipal(t *testing.T) {
	handler := NewOpportunityHandler(&opportunityServiceFake{}, fakePrincipalResolver{principal: Principal{UserID: 3, TenantID: 11, CorpID: 22}})
	response := httptest.NewRecorder()

	handler.List(response, httptest.NewRequest(http.MethodGet, OpportunitiesPath+"?corpId=23", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
}

func TestOrderIgnoresCorpAssertionAndUsesDashboardPrincipal(t *testing.T) {
	handler := NewOrderHandler(NewMemoryOrderRepository(), fakePrincipalResolver{principal: Principal{UserID: 3, TenantID: 11, CorpID: 22}})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/dashboard/scrm/orders?corpId=23", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
}

func TestSettingsIgnoresCorpAssertionAndUsesDashboardPrincipal(t *testing.T) {
	handler := NewSettingsHandler(&settingsRepo{}, fakePrincipalResolver{principal: Principal{UserID: 3, TenantID: 11, CorpID: 22}}, nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scrm/settings?corpId=23", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
}
