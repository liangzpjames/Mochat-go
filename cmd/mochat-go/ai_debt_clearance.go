package main

import (
	"context"
	"errors"
	"net/http"

	appbootstrap "jiyi/mochat-go/internal/app/bootstrap"
	appmodules "jiyi/mochat-go/internal/app/modules"
	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/dashboard"
	aiinsighthttp "jiyi/mochat-go/internal/modules/ai-insight/transport/http"
	aisettingshttp "jiyi/mochat-go/internal/modules/ai-settings/transport/http"
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
	buildUserResolver func(string) (dashboard.UserIDResolver, dashboard.LoginCache),
) error {
	if !cfg.EnableAIDebtClearance {
		return nil
	}
	if router == nil {
		return errors.New("AI debt clearance route registrar is required")
	}
	if getMySQLStore == nil || buildUserResolver == nil {
		return errors.New("AI debt clearance runtime dependencies are required")
	}
	mysqlStore := getMySQLStore()
	if mysqlStore == nil {
		return errors.New("AI debt clearance MySQL store is required")
	}
	userIDs, _ := buildUserResolver("AI debt clearance")
	principalResolver, err := appbootstrap.NewSCRMPrincipalResolver(userIDs, mysqlStore)
	if err != nil {
		return err
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
	return appbootstrap.RegisterAIInsight(router, true, appbootstrap.AIInsightDependencies{
		PrincipalResolver: aiInsightPrincipalResolver{delegate: principalResolver},
		Authorizer:        aiInsightAuthorizer{delegate: leadAuthorizer},
	})
}

type aiDebtAuthorizer struct {
	delegate scrmhttp.LeadAuthorizer
}

func (a aiDebtAuthorizer) Authorize(ctx context.Context, principal aisettingshttp.Principal, corpID int64, permission string) error {
	return a.delegate.Authorize(ctx, scrmhttp.Principal{UserID: principal.UserID, TenantID: principal.TenantID}, corpID, permission)
}

type aiInsightAuthorizer struct {
	delegate scrmhttp.LeadAuthorizer
}

func (a aiInsightAuthorizer) Authorize(ctx context.Context, principal aiinsighthttp.Principal, corpID int64, permission string) error {
	return a.delegate.Authorize(ctx, scrmhttp.Principal{UserID: principal.UserID, TenantID: principal.TenantID}, corpID, permission)
}

type aiSettingsPrincipalResolver struct {
	delegate scrmhttp.PrincipalResolver
}

func (r aiSettingsPrincipalResolver) Resolve(request *http.Request) (aisettingshttp.Principal, error) {
	principal, err := r.delegate.Resolve(request)
	if err != nil {
		return aisettingshttp.Principal{}, err
	}
	return aisettingshttp.Principal{UserID: principal.UserID, TenantID: principal.TenantID}, nil
}

type aiInsightPrincipalResolver struct {
	delegate scrmhttp.PrincipalResolver
}

func (r aiInsightPrincipalResolver) Resolve(request *http.Request) (aiinsighthttp.Principal, error) {
	principal, err := r.delegate.Resolve(request)
	if err != nil {
		return aiinsighthttp.Principal{}, err
	}
	return aiinsighthttp.Principal{UserID: principal.UserID, TenantID: principal.TenantID}, nil
}
