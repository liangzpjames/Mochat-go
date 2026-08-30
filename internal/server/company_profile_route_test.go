package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/dashboard"
)

func TestCompanyProfileRoutesReachTheDedicatedHandler(t *testing.T) {
	called := make([]string, 0, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = append(called, r.Method+" "+r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	})
	srv, err := New(config.Config{}, WithCompanyProfileHandler(handler))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/dashboard/company/profile"},
		{http.MethodPut, "/dashboard/company/profile"},
		{http.MethodPut, "/dashboard/company/wecom-credentials"},
		{http.MethodPut, "/dashboard/company/agent-credentials"},
		{http.MethodPut, "/dashboard/company/application-credentials"},
		{http.MethodPut, "/dashboard/company/archive-credentials"},
		{http.MethodGet, "/dashboard/company/callback-configuration"},
		{http.MethodPost, "/dashboard/company/callback-configuration/regenerate"},
		{http.MethodPost, "/dashboard/company/verify"},
		{http.MethodPost, "/dashboard/company/employee-sync"},
		{http.MethodGet, "/dashboard/company/sync-status"},
		{http.MethodPost, "/dashboard/company/archive-sync"},
		{http.MethodGet, "/dashboard/company/archive-sync-status"},
		{http.MethodGet, "/dashboard/company/audits"},
		{http.MethodGet, "/dashboard/company/callback-side-effects"},
		{http.MethodGet, "/dashboard/company/callback-side-effects/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/fission.employee_reminder"},
		{http.MethodPost, "/dashboard/company/callback-side-effects/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/fission.employee_reminder/reconcile"},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		srv.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
		if response.Code != http.StatusNoContent {
			t.Fatalf("%s %s status=%d, want 204", test.method, test.path, response.Code)
		}
	}
	if len(called) != len(tests) {
		t.Fatalf("handler calls=%v, want %d", called, len(tests))
	}
}

func TestCompanyProfileRouteDoesNotBroadenMethods(t *testing.T) {
	called := false
	srv, err := New(config.Config{}, WithCompanyProfileHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	})))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	srv.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/dashboard/company/profile", strings.NewReader(`{}`)))
	if called {
		t.Fatal("POST /dashboard/company/profile reached a handler")
	}
	if response.Code == http.StatusNoContent {
		t.Fatalf("unmatched method returned handler status 204")
	}
}

type companyProfileRouteGuard struct {
	calls   int
	allowed bool
}

func (g *companyProfileRouteGuard) Authorize(w http.ResponseWriter, _ *http.Request) bool {
	g.calls++
	if !g.allowed {
		w.WriteHeader(http.StatusUnauthorized)
		return false
	}
	return true
}

func TestCompanyProfileRoutesRequireDashboardIdentityGuard(t *testing.T) {
	called := false
	guard := &companyProfileRouteGuard{}
	srv, err := New(config.Config{},
		WithDashboardRequestGuard(guard),
		WithCompanyProfileHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			called = true
		})),
	)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	srv.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/dashboard/company/profile", nil))
	if response.Code != http.StatusUnauthorized || called || guard.calls != 1 {
		t.Fatalf("status=%d called=%t guardCalls=%d, want 401/false/1", response.Code, called, guard.calls)
	}
}

func TestCompanyProfileRoutesAreGrantableDashboardContracts(t *testing.T) {
	denyOnly := make(map[string]struct{})
	for _, contract := range dashboard.DenyOnlyDashboardRouteContracts() {
		denyOnly[contract] = struct{}{}
	}
	for _, contract := range []string{
		"GET /dashboard/company/profile",
		"PUT /dashboard/company/profile",
		"PUT /dashboard/company/wecom-credentials",
		"PUT /dashboard/company/agent-credentials",
		"PUT /dashboard/company/application-credentials",
		"PUT /dashboard/company/archive-credentials",
		"GET /dashboard/company/callback-configuration",
		"POST /dashboard/company/callback-configuration/regenerate",
		"POST /dashboard/company/verify",
		"POST /dashboard/company/employee-sync",
		"GET /dashboard/company/sync-status",
		"POST /dashboard/company/archive-sync",
		"GET /dashboard/company/archive-sync-status",
		"GET /dashboard/company/audits",
		"GET /dashboard/company/callback-side-effects",
		"GET /dashboard/company/callback-side-effects/{eventKey}/{actionKey}",
		"POST /dashboard/company/callback-side-effects/{eventKey}/{actionKey}/reconcile",
		"GET /dashboard/providers/status",
	} {
		if _, ok := denyOnly[contract]; ok {
			t.Fatalf("grantable company route %q must not remain dashboard deny-only", contract)
		}
	}
}
