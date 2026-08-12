package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardprincipal"
)

func TestAIInsightPrincipalResolverCarriesRestrictedDashboardScope(t *testing.T) {
	resolver := aiInsightPrincipalResolver{}
	ctx := dashboardprincipal.WithPrincipal(context.Background(), dashboardprincipal.DashboardPrincipal{
		UserID: 7, TenantID: 9, CorpID: 13, CorpStatus: dashboardprincipal.CorpBindingStatusActive, AuthVersion: 1,
	})
	request := httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/employee-score", nil).WithContext(dashboard.WithDashboardAccessContext(ctx, dashboard.DashboardAccessContext{
		UserID: 7, TenantID: 9, CorpID: 13, Scope: dashboard.DataScopeSelf, ScopeRequired: true, AllowedEmployeeIDs: []int{81},
	}))
	principal, err := resolver.Resolve(request)
	if err != nil || !principal.EmployeeScopeRestricted || len(principal.AllowedEmployeeIDs) != 1 || principal.AllowedEmployeeIDs[0] != 81 {
		t.Fatalf("principal=%+v err=%v", principal, err)
	}
}
