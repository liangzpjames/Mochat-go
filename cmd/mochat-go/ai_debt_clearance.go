package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	appbootstrap "jiyi/mochat-go/internal/app/bootstrap"
	appmodules "jiyi/mochat-go/internal/app/modules"
	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/dashboard"
	aiinsighthttp "jiyi/mochat-go/internal/modules/ai-insight/transport/http"
	aisettingshttp "jiyi/mochat-go/internal/modules/ai-settings/transport/http"
	"jiyi/mochat-go/internal/modules/providers"
	openai "jiyi/mochat-go/internal/modules/providers/ai/openai"
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
		PrincipalResolver: aiSettingsPrincipalResolver{delegate: principalResolver},
		Authorizer:        aiDebtAuthorizer{delegate: leadAuthorizer},
	}); err != nil {
		return err
	}
	var aiProvider providers.AIProvider
	if cfg.EnableAIInsight {
		aiProvider, err = buildAIProvider()
		if err != nil {
			return err
		}
	}
	return appbootstrap.RegisterAIInsight(router, true, appbootstrap.AIInsightDependencies{
		PrincipalResolver: aiInsightPrincipalResolver{delegate: principalResolver},
		Authorizer:        aiInsightAuthorizer{delegate: leadAuthorizer},
		DB:                mysqlStore.DB(),
		AIProvider:        aiProvider,
	})
}

func buildAIProvider() (providers.AIProvider, error) {
	provider, err := openai.New(openai.Config{
		BaseURL: os.Getenv("MOCHAT_GO_AI_PROVIDER_BASE_URL"),
		APIKey:  os.Getenv("MOCHAT_GO_AI_PROVIDER_KEY"),
		Model:   os.Getenv("MOCHAT_GO_AI_PROVIDER_MODEL"),
		Timeout: time.Duration(envInt("MOCHAT_GO_AI_PROVIDER_TIMEOUT_SECONDS", 30)) * time.Second,
	})
	if err != nil {
		return nil, err
	}
	return provider, nil
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
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

type aiSettingsPrincipalResolver struct {
	delegate scrmhttp.PrincipalResolver
}

func (r aiSettingsPrincipalResolver) Resolve(request *http.Request) (aisettingshttp.Principal, error) {
	principal, err := r.delegate.Resolve(request)
	if err != nil {
		return aisettingshttp.Principal{}, err
	}
	return aisettingshttp.Principal{UserID: principal.UserID, TenantID: principal.TenantID, CorpID: principal.CorpID}, nil
}

type aiInsightPrincipalResolver struct {
	delegate scrmhttp.PrincipalResolver
}

func (r aiInsightPrincipalResolver) Resolve(request *http.Request) (aiinsighthttp.Principal, error) {
	principal, err := r.delegate.Resolve(request)
	if err != nil {
		return aiinsighthttp.Principal{}, err
	}
	access, ok := dashboard.DashboardAccessFromContext(request.Context())
	if !ok || access.UserID != int(principal.UserID) || access.TenantID != int(principal.TenantID) {
		return aiinsighthttp.Principal{}, scrmhttp.ErrPrincipalUnauthorized
	}
	allowed := make([]int64, 0, len(access.AllowedEmployeeIDs))
	for _, id := range access.AllowedEmployeeIDs {
		if id > 0 {
			allowed = append(allowed, int64(id))
		}
	}
	return aiinsighthttp.Principal{UserID: principal.UserID, TenantID: principal.TenantID, CorpID: principal.CorpID, AllowedEmployeeIDs: allowed, EmployeeScopeRestricted: access.ScopeRequired && access.Scope != dashboard.DataScopeTenant}, nil
}
