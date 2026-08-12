package main

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/store"

	reportinghttp "jiyi/mochat-go/internal/modules/reporting/transport/http"
	transporthttp "jiyi/mochat-go/internal/modules/scrm/transport/http"
)

func TestReportingPrincipalResolverCarriesDashboardEmployeeScope(t *testing.T) {
	resolver := reportingPrincipalResolver{}
	request := httptest.NewRequest(http.MethodGet, "/dashboard/reports/customer", nil)
	ctx := dashboardprincipal.WithPrincipal(context.Background(), dashboardprincipal.DashboardPrincipal{
		UserID: 7, TenantID: 9, CorpID: 13, CorpStatus: dashboardprincipal.CorpBindingStatusActive, AuthVersion: 1,
	})
	request = request.WithContext(dashboard.WithDashboardAccessContext(ctx, dashboard.DashboardAccessContext{
		UserID: 7, TenantID: 9, CorpID: 13, Scope: dashboard.DataScopeDepartment, ScopeRequired: true, AllowedEmployeeIDs: []int{81, 82},
	}))
	principal, err := resolver.Resolve(request)
	if err != nil {
		t.Fatal(err)
	}
	if !principal.EmployeeScopeRestricted || len(principal.AllowedEmployeeIDs) != 2 || principal.AllowedEmployeeIDs[1] != 82 {
		t.Fatalf("scope not carried: %+v", principal)
	}
}

func TestReportingPrincipalResolverTenantScopeIsUnrestricted(t *testing.T) {
	resolver := reportingPrincipalResolver{}
	ctx := dashboardprincipal.WithPrincipal(context.Background(), dashboardprincipal.DashboardPrincipal{
		UserID: 7, TenantID: 9, CorpID: 13, CorpStatus: dashboardprincipal.CorpBindingStatusActive, AuthVersion: 1,
	})
	request := httptest.NewRequest(http.MethodGet, "/dashboard/reports/customer", nil).WithContext(dashboard.WithDashboardAccessContext(ctx, dashboard.DashboardAccessContext{UserID: 7, TenantID: 9, CorpID: 13, Scope: dashboard.DataScopeTenant, ScopeRequired: true}))
	principal, err := resolver.Resolve(request)
	if err != nil || principal.EmployeeScopeRestricted {
		t.Fatalf("tenant scope should be unrestricted: %+v, %v", principal, err)
	}
}

func TestReportingPrincipalResolverRejectsContextIdentityMismatch(t *testing.T) {
	resolver := reportingPrincipalResolver{}
	ctx := dashboardprincipal.WithPrincipal(context.Background(), dashboardprincipal.DashboardPrincipal{
		UserID: 7, TenantID: 9, CorpID: 13, CorpStatus: dashboardprincipal.CorpBindingStatusActive, AuthVersion: 1,
	})
	request := httptest.NewRequest(http.MethodGet, "/dashboard/reports/customer", nil).WithContext(dashboard.WithDashboardAccessContext(ctx, dashboard.DashboardAccessContext{UserID: 8, TenantID: 9, CorpID: 13}))
	if _, err := resolver.Resolve(request); err == nil {
		t.Fatal("identity mismatch must fail closed")
	}
}

var _ reportinghttp.PrincipalResolver = reportingPrincipalResolver{}

func TestNewSCRMModuleRouterDisabledDoesNotResolveDependencies(t *testing.T) {
	router, err := newSCRMModuleRouter(config.Config{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertMainSCRMRouteMissing(t, router, http.MethodPost)
	assertMainSCRMRouteMissing(t, router, http.MethodGet)
}

func TestNewSCRMModuleRouterEnabledInstallsBothRoutes(t *testing.T) {
	db := mainTestDB(t)
	mysqlStore := store.NewMySQLStore(db)
	router, err := newSCRMModuleRouter(
		config.Config{EnablePhase22SCRMPilot: true},
		func() *store.MySQLStore { return mysqlStore },
		dashboardModulePrincipalResolver{},
	)
	if err != nil {
		t.Fatal(err)
	}
	assertMainSCRMRouteInstalled(t, router, http.MethodPost)
	assertMainSCRMRouteInstalled(t, router, http.MethodGet)
}

func assertMainSCRMRouteInstalled(t *testing.T, router routeMatcher, method string) {
	t.Helper()
	request := httptest.NewRequest(method, transporthttp.LeadsPath, nil)
	if handler, ok := router.Match(request); !ok || handler == nil {
		t.Fatalf("%s %s was not installed", method, transporthttp.LeadsPath)
	}
}

func assertMainSCRMRouteMissing(t *testing.T, router routeMatcher, method string) {
	t.Helper()
	request := httptest.NewRequest(method, transporthttp.LeadsPath, nil)
	if handler, ok := router.Match(request); ok || handler != nil {
		t.Fatalf("%s %s was installed", method, transporthttp.LeadsPath)
	}
}

type routeMatcher interface {
	Match(*http.Request) (http.Handler, bool)
}

func mainTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("mysql", "unused:unused@tcp(localhost:3306)/unused")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

type mainFixedUserIDResolver struct {
	userID int
}

func (r mainFixedUserIDResolver) UserID(*http.Request) (int, error) {
	return r.userID, nil
}
