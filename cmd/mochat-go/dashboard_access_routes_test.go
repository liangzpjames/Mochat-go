package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	appmodules "jiyi/mochat-go/internal/app/modules"
)

func TestRegisterDashboardAccessRoutesInstallsExactContracts(t *testing.T) {
	router := appmodules.NewRouter()
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	if err := registerDashboardAccessRoutes(router, handler); err != nil {
		t.Fatal(err)
	}
	for _, contract := range []struct{ method, path string }{
		{http.MethodGet, "/dashboard/access/profile"},
		{http.MethodGet, "/dashboard/access/catalog"},
		{http.MethodGet, "/dashboard/access/users"},
		{http.MethodGet, "/dashboard/access/users/7"},
		{http.MethodPut, "/dashboard/access/users/7"},
		{http.MethodGet, "/dashboard/access/roles"},
		{http.MethodPost, "/dashboard/access/roles"},
		{http.MethodPut, "/dashboard/access/roles/8"},
		{http.MethodPut, "/dashboard/access/roles/8/status"},
		{http.MethodDelete, "/dashboard/access/roles/8"},
		{http.MethodGet, "/dashboard/access/audits"},
	} {
		request := httptest.NewRequest(contract.method, contract.path, nil)
		if matched, ok := router.Match(request); !ok || matched == nil {
			t.Fatalf("missing %s %s", contract.method, contract.path)
		}
	}
	if matched, ok := router.Match(httptest.NewRequest(http.MethodPost, "/dashboard/access/catalog", nil)); ok || matched != nil {
		t.Fatal("unregistered method became reachable")
	}
}
