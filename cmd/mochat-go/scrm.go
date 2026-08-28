package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	appbootstrap "jiyi/mochat-go/internal/app/bootstrap"
	appmodules "jiyi/mochat-go/internal/app/modules"
	"jiyi/mochat-go/internal/authjwt"
	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/modules/reporting"
	reportinghttp "jiyi/mochat-go/internal/modules/reporting/transport/http"
	scrmhttp "jiyi/mochat-go/internal/modules/scrm/transport/http"
	"jiyi/mochat-go/internal/store"
)

func newUserResolverBuilder(
	cfg config.Config,
	identitySessionChecker authjwt.SessionChecker,
	getRedisStore func() *store.RedisStore,
) func(string) (dashboard.UserIDResolver, dashboard.LoginCache) {
	return func(routeName string) (dashboard.UserIDResolver, dashboard.LoginCache) {
		if cfg.DevAuthHeader {
			debugf("%s auth resolver: development header X-Mochat-Go-User-ID", routeName)
			return dashboard.HeaderUserIDResolver{}, nil
		}
		if cfg.SkipJWTBlacklist {
			debugf("%s auth resolver: PHP simple-jwt compatible parser without Redis blacklist checks", routeName)
			return authjwt.Parser{
				Secret:        cfg.SimpleJWTSecret,
				Prefix:        cfg.SimpleJWTPrefix,
				SkipBlacklist: true,
				Sessions:      identitySessionChecker,
			}, nil
		}
		redisStore := getRedisStore()
		debugf("%s auth resolver: PHP simple-jwt compatible parser", routeName)
		return authjwt.Parser{
			Secret:        cfg.SimpleJWTSecret,
			Prefix:        cfg.SimpleJWTPrefix,
			Blacklist:     redisStore,
			SkipBlacklist: cfg.SkipJWTBlacklist,
			Sessions:      identitySessionChecker,
		}, redisStore
	}
}

func newSCRMModuleRouter(
	cfg config.Config,
	getMySQLStore func() *store.MySQLStore,
	principalResolver scrmhttp.PrincipalResolver,
) (*appmodules.Router, error) {
	router := appmodules.NewRouter()
	dependencies := appbootstrap.SCRMDependencies{}
	var mysqlStore *store.MySQLStore
	if cfg.EnablePhase22SCRMPilot {
		if getMySQLStore == nil || principalResolver == nil {
			return nil, errors.New("SCRM runtime dependencies are required")
		}
		mysqlStore = getMySQLStore()
		if mysqlStore == nil {
			return nil, errors.New("SCRM MySQL store is required")
		}
		dependencies.DB = mysqlStore.DB()
		dependencies.PrincipalResolver = principalResolver
		leadAuthorizer, err := appbootstrap.NewSCRMLeadAuthorizer(mysqlStore, dashboard.NewRBACResolver(mysqlStore))
		if err != nil {
			return nil, fmt.Errorf("build SCRM lead authorizer: %w", err)
		}
		dependencies.LeadAuthorizer = leadAuthorizer
		dependencies.EnableAcceptanceLifecycle = cfg.EnablePhase35AcceptanceLifecycle
		dependencies.AcceptanceEnvironmentID = cfg.Phase35AcceptanceEnvironmentID
	}
	if err := appbootstrap.RegisterSCRM(router, cfg.EnablePhase22SCRMPilot, dependencies); err != nil {
		return nil, fmt.Errorf("register SCRM pilot module: %w", err)
	}
	if cfg.EnablePhase22SCRMPilot {
		reportingService := reporting.NewSQLService(mysqlStore.DB())
		reportingHandler := reportinghttp.NewHandler(reportingService, reportingPrincipalResolver{}, reportingAuthorizer{delegate: dependencies.LeadAuthorizer})
		if err := router.Handle(http.MethodGet, reportinghttp.ReportsPath, reportingHandler); err != nil {
			return nil, fmt.Errorf("register reporting module: %w", err)
		}
	}
	return router, nil
}

type reportingPrincipalResolver struct{}

func (r reportingPrincipalResolver) Resolve(request *http.Request) (reportinghttp.Principal, error) {
	if request == nil {
		return reportinghttp.Principal{}, scrmhttp.ErrPrincipalUnauthorized
	}
	principal, err := dashboardprincipal.DashboardPrincipalFromContext(request.Context())
	if err != nil || principal.CorpStatus == dashboardprincipal.CorpBindingStatusSuspended {
		return reportinghttp.Principal{}, scrmhttp.ErrPrincipalUnauthorized
	}
	access, ok := dashboard.DashboardAccessFromContext(request.Context())
	if !ok || access.UserID != principal.UserID || access.TenantID != principal.TenantID || access.CorpID != principal.CorpID {
		return reportinghttp.Principal{}, scrmhttp.ErrPrincipalUnauthorized
	}
	allowed := make([]int64, 0, len(access.AllowedEmployeeIDs))
	for _, id := range access.AllowedEmployeeIDs {
		if id > 0 {
			allowed = append(allowed, int64(id))
		}
	}
	restricted := access.ScopeRequired && access.Scope != dashboard.DataScopeTenant
	return reportinghttp.Principal{UserID: int64(principal.UserID), TenantID: int64(principal.TenantID), CorpID: int64(principal.CorpID), WorkEmployeeID: int64(access.WorkEmployeeID), AllowedEmployeeIDs: allowed, EmployeeScopeRestricted: restricted}, nil
}

type reportingAuthorizer struct{ delegate scrmhttp.LeadAuthorizer }

func (a reportingAuthorizer) Authorize(ctx context.Context, principal reportinghttp.Principal, corpID int64, permission string) error {
	return a.delegate.Authorize(ctx, scrmhttp.Principal{UserID: principal.UserID, TenantID: principal.TenantID, CorpID: principal.CorpID, WorkEmployeeID: principal.WorkEmployeeID, AllowedEmployeeIDs: principal.AllowedEmployeeIDs, EmployeeScopeRestricted: principal.EmployeeScopeRestricted}, corpID, permission)
}

// dashboardModulePrincipalResolver is the only production bridge from the
// Dashboard request guard to module transport contracts. It consumes the
// server-created context facts and never parses request headers, JWTs, or
// legacy login/corp caches.
type dashboardModulePrincipalResolver struct{}

func (dashboardModulePrincipalResolver) Resolve(request *http.Request) (scrmhttp.Principal, error) {
	if request == nil {
		return scrmhttp.Principal{}, scrmhttp.ErrPrincipalUnauthorized
	}
	principal, err := dashboardprincipal.DashboardPrincipalFromContext(request.Context())
	if err != nil || principal.CorpStatus == dashboardprincipal.CorpBindingStatusSuspended {
		return scrmhttp.Principal{}, scrmhttp.ErrPrincipalUnauthorized
	}
	access, ok := dashboard.DashboardAccessFromContext(request.Context())
	if !ok || access.UserID != principal.UserID || access.TenantID != principal.TenantID || access.CorpID != principal.CorpID {
		return scrmhttp.Principal{}, scrmhttp.ErrPrincipalUnauthorized
	}
	allowed := make([]int64, 0, len(access.AllowedEmployeeIDs))
	for _, id := range access.AllowedEmployeeIDs {
		if id > 0 {
			allowed = append(allowed, int64(id))
		}
	}
	return scrmhttp.Principal{
		UserID: int64(principal.UserID), TenantID: int64(principal.TenantID), CorpID: int64(principal.CorpID),
		WorkEmployeeID: int64(access.WorkEmployeeID), AllowedEmployeeIDs: allowed,
		EmployeeScopeRestricted: access.ScopeRequired && access.Scope != dashboard.DataScopeTenant,
	}, nil
}

var _ scrmhttp.PrincipalResolver = dashboardModulePrincipalResolver{}
