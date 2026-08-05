package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	appbootstrap "jiyi/mochat-go/internal/app/bootstrap"
	appmodules "jiyi/mochat-go/internal/app/modules"
	"jiyi/mochat-go/internal/authjwt"
	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/dashboard"
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
			log.Printf("%s auth resolver: development header X-Mochat-Go-User-ID", routeName)
			return dashboard.HeaderUserIDResolver{}, nil
		}
		if cfg.SkipJWTBlacklist {
			log.Printf("%s auth resolver: PHP simple-jwt compatible parser without Redis blacklist checks", routeName)
			return authjwt.Parser{
				Secret:        cfg.SimpleJWTSecret,
				Prefix:        cfg.SimpleJWTPrefix,
				SkipBlacklist: true,
				Sessions:      identitySessionChecker,
			}, nil
		}
		redisStore := getRedisStore()
		log.Printf("%s auth resolver: PHP simple-jwt compatible parser", routeName)
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
	buildUserResolver func(string) (dashboard.UserIDResolver, dashboard.LoginCache),
) (*appmodules.Router, error) {
	router := appmodules.NewRouter()
	dependencies := appbootstrap.SCRMDependencies{}
	if cfg.EnablePhase22SCRMPilot {
		if getMySQLStore == nil || buildUserResolver == nil {
			return nil, errors.New("SCRM runtime dependencies are required")
		}
		mysqlStore := getMySQLStore()
		if mysqlStore == nil {
			return nil, errors.New("SCRM MySQL store is required")
		}
		userIDs, _ := buildUserResolver("SCRM pilot")
		principalResolver, err := appbootstrap.NewSCRMPrincipalResolver(userIDs, mysqlStore)
		if err != nil {
			return nil, fmt.Errorf("build SCRM principal resolver: %w", err)
		}
		dependencies.DB = mysqlStore.DB()
		dependencies.PrincipalResolver = principalResolver
		leadAuthorizer, err := appbootstrap.NewSCRMLeadAuthorizer(mysqlStore, dashboard.NewRBACResolver(mysqlStore))
		if err != nil {
			return nil, fmt.Errorf("build SCRM lead authorizer: %w", err)
		}
		dependencies.LeadAuthorizer = leadAuthorizer
	}
	if err := appbootstrap.RegisterSCRM(router, cfg.EnablePhase22SCRMPilot, dependencies); err != nil {
		return nil, fmt.Errorf("register SCRM pilot module: %w", err)
	}
	if cfg.EnablePhase22SCRMPilot {
		reportingService := reporting.NewService(map[reporting.ReportKind]reporting.Source{
			reporting.CustomerReport:   reporting.UnavailableSource("customer_reporting", "客户报表数据源尚未装配"),
			reporting.EmployeeReport:   reporting.UnavailableSource("conversation_archive", "会话存档 Provider 不可用"),
			reporting.ConversionReport: reporting.UnavailableSource("conversion_reporting", "转化报表数据源尚未装配"),
			reporting.BehaviorReport:   reporting.UnavailableSource("behavior_events", "行为事件数据源尚未装配"),
			reporting.DetailReport:     reporting.UnavailableSource("reporting", "综合报表数据源尚未装配"),
		})
		reportingHandler := reportinghttp.NewHandler(reportingService, reportingPrincipalResolver{delegate: dependencies.PrincipalResolver}, reportingAuthorizer{delegate: dependencies.LeadAuthorizer})
		if err := router.Handle(http.MethodGet, reportinghttp.ReportsPath, reportingHandler); err != nil {
			return nil, fmt.Errorf("register reporting module: %w", err)
		}
	}
	return router, nil
}

type reportingPrincipalResolver struct{ delegate scrmhttp.PrincipalResolver }

func (r reportingPrincipalResolver) Resolve(request *http.Request) (reportinghttp.Principal, error) {
	principal, err := r.delegate.Resolve(request)
	return reportinghttp.Principal{UserID: principal.UserID, TenantID: principal.TenantID}, err
}

type reportingAuthorizer struct{ delegate scrmhttp.LeadAuthorizer }

func (a reportingAuthorizer) Authorize(ctx context.Context, principal reportinghttp.Principal, corpID int64, permission string) error {
	return a.delegate.Authorize(ctx, scrmhttp.Principal{UserID: principal.UserID, TenantID: principal.TenantID}, corpID, permission)
}
