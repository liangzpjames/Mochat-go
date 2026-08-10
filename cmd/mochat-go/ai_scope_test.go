package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
	scrmhttp "jiyi/mochat-go/internal/modules/scrm/transport/http"
)

func TestAIInsightPrincipalResolverCarriesRestrictedDashboardScope(t *testing.T) {
	resolver := aiInsightPrincipalResolver{delegate: fixedMainPrincipalResolver{principal: scrmhttp.Principal{UserID: 7, TenantID: 9}}}
	request := httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/employee-score", nil).WithContext(dashboard.WithDashboardAccessContext(context.Background(), dashboard.DashboardAccessContext{
		UserID: 7, TenantID: 9, Scope: dashboard.DataScopeSelf, ScopeRequired: true, AllowedEmployeeIDs: []int{81},
	}))
	principal, err := resolver.Resolve(request)
	if err != nil || !principal.EmployeeScopeRestricted || len(principal.AllowedEmployeeIDs) != 1 || principal.AllowedEmployeeIDs[0] != 81 {
		t.Fatalf("principal=%+v err=%v", principal, err)
	}
}
