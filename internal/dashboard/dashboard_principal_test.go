package dashboard

import (
	"io"
	"net/http"
	"net/http/httptest"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

// authenticatedDashboardRequestForTest is an explicit test opt-in for a
// handler that production RequestGuard would call with a server-owned
// DashboardPrincipal. It is intentionally never used by SaaS, callback,
// worker, or public-route tests.
func authenticatedDashboardRequestForTest(method, target string, body io.Reader) *http.Request {
	return authenticatedDashboardRequestForTestAs(method, target, body, 1, 1, 7, 99)
}

func authenticatedDashboardRequestForTestAs(method, target string, body io.Reader, userID, tenantID, corpID, workEmployeeID int) *http.Request {
	request := httptest.NewRequest(method, target, body)
	return withDashboardPrincipalForTest(request, userID, tenantID, corpID, workEmployeeID)
}

func withDashboardPrincipalForTest(request *http.Request, userID, tenantID, corpID, workEmployeeID int) *http.Request {
	principal := dashboardprincipal.DashboardPrincipal{
		UserID:       userID,
		TenantID:     tenantID,
		CorpID:       corpID,
		CorpStatus:   dashboardprincipal.CorpBindingStatusActive,
		AuthVersion:  1,
		IsSuperAdmin: false,
	}
	ctx := dashboardprincipal.WithPrincipal(request.Context(), principal)
	ctx = WithDashboardAccessContext(ctx, DashboardAccessContext{
		UserID:         userID,
		TenantID:       tenantID,
		CorpID:         corpID,
		WorkEmployeeID: workEmployeeID,
		Scope:          DataScopeTenant,
	})
	return request.WithContext(ctx)
}
