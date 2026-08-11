package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardprincipal"
	scrmhttp "jiyi/mochat-go/internal/modules/scrm/transport/http"
)

func TestDashboardModulePrincipalResolverUsesOnlyContextPrincipalAndScope(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/dashboard/scrm/contacts", nil)
	ctx := dashboardprincipal.WithPrincipal(request.Context(), dashboardprincipal.DashboardPrincipal{
		UserID: 7, TenantID: 9, CorpID: 22, CorpStatus: dashboardprincipal.CorpBindingStatusActive, AuthVersion: 3,
	})
	ctx = dashboard.WithDashboardAccessContext(ctx, dashboard.DashboardAccessContext{
		UserID: 7, TenantID: 9, CorpID: 22, WorkEmployeeID: 41,
		Scope: dashboard.DataScopeDepartment, ScopeRequired: true, AllowedEmployeeIDs: []int{41, 42},
	})
	request = request.WithContext(ctx)

	principal, err := (dashboardModulePrincipalResolver{}).Resolve(request)
	if err != nil {
		t.Fatal(err)
	}
	if principal.UserID != 7 || principal.TenantID != 9 || principal.CorpID != 22 || principal.WorkEmployeeID != 41 {
		t.Fatalf("principal = %#v", principal)
	}
	if !principal.EmployeeScopeRestricted || len(principal.AllowedEmployeeIDs) != 2 || principal.AllowedEmployeeIDs[1] != 42 {
		t.Fatalf("employee scope = %#v", principal)
	}
}

func TestDashboardModulePrincipalResolverFailsClosedWithoutContext(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/dashboard/scrm/contacts", nil)
	request.Header.Set("X-Mochat-Go-User-ID", "7")

	if _, err := (dashboardModulePrincipalResolver{}).Resolve(request); err == nil {
		t.Fatal("missing DashboardPrincipal context was accepted")
	}
}

var _ scrmhttp.PrincipalResolver = dashboardModulePrincipalResolver{}
