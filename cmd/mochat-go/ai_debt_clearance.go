package main

import (
	"context"
	"errors"
	"log"
	"net/http"

	appbootstrap "jiyi/mochat-go/internal/app/bootstrap"
	appmodules "jiyi/mochat-go/internal/app/modules"
	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardprincipal"
	aiinsight "jiyi/mochat-go/internal/modules/ai-insight"
	aiinsighthttp "jiyi/mochat-go/internal/modules/ai-insight/transport/http"
	aisettingshttp "jiyi/mochat-go/internal/modules/ai-settings/transport/http"
	"jiyi/mochat-go/internal/modules/providers"
	providercatalog "jiyi/mochat-go/internal/modules/providers/catalog"
	scrmhttp "jiyi/mochat-go/internal/modules/scrm/transport/http"
	"jiyi/mochat-go/internal/store"
)

// registerAIDebtClearanceModules wires the AI settings and AI insight modules
// into the dashboard module router. Both modules are gated by
// MOCHAT_GO_ENABLE_AI_DEBT_CLEARANCE and reuse the SCRM principal resolver and
// RBAC authorizer adapters so tenant/corp scope checks stay consistent.
func registerAIDebtClearanceModules(
	router *appmodules.Router,
	cfg config.Config,
	getMySQLStore func() *store.MySQLStore,
	principalResolver scrmhttp.PrincipalResolver,
) error {
	if !cfg.EnableAIDebtClearance {
		return nil
	}
	if router == nil {
		return errors.New("AI debt clearance route registrar is required")
	}
	if getMySQLStore == nil || principalResolver == nil {
		return errors.New("AI debt clearance runtime dependencies are required")
	}
	mysqlStore := getMySQLStore()
	if mysqlStore == nil {
		return errors.New("AI debt clearance MySQL store is required")
	}
	leadAuthorizer, err := appbootstrap.NewSCRMLeadAuthorizer(mysqlStore, dashboard.NewRBACResolver(mysqlStore))
	if err != nil {
		return err
	}
	if err := appbootstrap.RegisterAISettings(router, true, appbootstrap.AISettingsDependencies{
		DB:                mysqlStore.DB(),
		FileStorageRoot:   cfg.FileStorageRoot,
		PrincipalResolver: aiSettingsPrincipalResolver{},
		Authorizer:        aiDebtAuthorizer{delegate: leadAuthorizer},
	}); err != nil {
		return err
	}
	var aiResolver providers.AIProviderResolver
	if cfg.EnableAIInsight {
		aiResolver = mysqlStore.TenantAIProviderResolver()
	}
	if err := appbootstrap.RegisterAIInsight(router, true, appbootstrap.AIInsightDependencies{
		PrincipalResolver:  aiInsightPrincipalResolver{},
		Authorizer:         aiInsightAuthorizer{delegate: leadAuthorizer},
		DB:                 mysqlStore.DB(),
		AIProviderResolver: aiResolver,
	}); err != nil {
		return err
	}
	startAIInsightDailyAnalysis(cfg, mysqlStore, aiResolver)
	return nil
}

// startAIInsightDailyAnalysis starts the once-per-day analysis loop. It is the
// only component allowed to call the AI model; page reads are read-only.
func startAIInsightDailyAnalysis(cfg config.Config, mysqlStore *store.MySQLStore, resolver providers.AIProviderResolver) {
	if !cfg.EnableAIInsight || !cfg.AIInsightDailyAnalysisEnabled || mysqlStore == nil || resolver == nil {
		return
	}
	go aiinsight.RunDailyLoop(context.Background(), aiinsight.DailyConfig{
		DB:         mysqlStore.DB(),
		Resolver:   resolver,
		Hour:       cfg.AIInsightAnalysisHour,
		RunOnStart: cfg.AIInsightAnalysisRunOnStart,
		Logger:     log.Default(),
	})
	log.Printf("go cron enabled: AI insight daily analysis hour=%02d run_on_start=%v", cfg.AIInsightAnalysisHour, cfg.AIInsightAnalysisRunOnStart)
}

func buildDashboardAIStatusProvider(cfg config.Config) (providers.StatusProvider, error) {
	if !cfg.EnableAIDebtClearance || !cfg.EnableAIInsight {
		return providercatalog.DisabledAIProvider{}, nil
	}
	return tenantScopedAIStatusProvider{}, nil
}

// tenantScopedAIStatusProvider prevents the process-wide registry from
// claiming readiness: only a resolver can determine a tenant/corp's state.
type tenantScopedAIStatusProvider struct{}

func (tenantScopedAIStatusProvider) Status() providers.Status {
	return providers.Status{Kind: "ai", State: providers.StateLimited, Source: providers.SourceExternal, Code: "ai.tenant_scoped", Reason: "AI 模型状态需在当前租户企业范围内解析", Action: "在 AI 洞察工作区查看当前租户配置"}
}

type aiDebtAuthorizer struct {
	delegate scrmhttp.LeadAuthorizer
}

func (a aiDebtAuthorizer) Authorize(ctx context.Context, principal aisettingshttp.Principal, corpID int64, permission string) error {
	return a.delegate.Authorize(ctx, scrmhttp.Principal{UserID: principal.UserID, TenantID: principal.TenantID, CorpID: principal.CorpID}, corpID, permission)
}

type aiInsightAuthorizer struct {
	delegate scrmhttp.LeadAuthorizer
}

func (a aiInsightAuthorizer) Authorize(ctx context.Context, principal aiinsighthttp.Principal, corpID int64, permission string) error {
	return a.delegate.Authorize(ctx, scrmhttp.Principal{UserID: principal.UserID, TenantID: principal.TenantID, CorpID: principal.CorpID, AllowedEmployeeIDs: principal.AllowedEmployeeIDs, EmployeeScopeRestricted: principal.EmployeeScopeRestricted}, corpID, permission)
}

type aiSettingsPrincipalResolver struct{}

func (r aiSettingsPrincipalResolver) Resolve(request *http.Request) (aisettingshttp.Principal, error) {
	if request == nil {
		return aisettingshttp.Principal{}, scrmhttp.ErrPrincipalUnauthorized
	}
	principal, err := dashboardprincipal.DashboardPrincipalFromContext(request.Context())
	if err != nil || principal.CorpStatus == dashboardprincipal.CorpBindingStatusSuspended {
		return aisettingshttp.Principal{}, scrmhttp.ErrPrincipalUnauthorized
	}
	access, ok := dashboard.DashboardAccessFromContext(request.Context())
	if !ok || access.UserID != principal.UserID || access.TenantID != principal.TenantID || access.CorpID != principal.CorpID {
		return aisettingshttp.Principal{}, scrmhttp.ErrPrincipalUnauthorized
	}
	return aisettingshttp.Principal{UserID: int64(principal.UserID), TenantID: int64(principal.TenantID), CorpID: int64(principal.CorpID)}, nil
}

type aiInsightPrincipalResolver struct{}

func (r aiInsightPrincipalResolver) Resolve(request *http.Request) (aiinsighthttp.Principal, error) {
	if request == nil {
		return aiinsighthttp.Principal{}, scrmhttp.ErrPrincipalUnauthorized
	}
	principal, err := dashboardprincipal.DashboardPrincipalFromContext(request.Context())
	if err != nil || principal.CorpStatus == dashboardprincipal.CorpBindingStatusSuspended {
		return aiinsighthttp.Principal{}, scrmhttp.ErrPrincipalUnauthorized
	}
	access, ok := dashboard.DashboardAccessFromContext(request.Context())
	if !ok || access.UserID != principal.UserID || access.TenantID != principal.TenantID || access.CorpID != principal.CorpID {
		return aiinsighthttp.Principal{}, scrmhttp.ErrPrincipalUnauthorized
	}
	allowed := make([]int64, 0, len(access.AllowedEmployeeIDs))
	for _, id := range access.AllowedEmployeeIDs {
		if id > 0 {
			allowed = append(allowed, int64(id))
		}
	}
	return aiinsighthttp.Principal{UserID: int64(principal.UserID), TenantID: int64(principal.TenantID), CorpID: int64(principal.CorpID), AllowedEmployeeIDs: allowed, EmployeeScopeRestricted: access.ScopeRequired && access.Scope != dashboard.DataScopeTenant, IsSuperAdmin: principal.IsSuperAdmin}, nil
}
