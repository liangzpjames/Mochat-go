package main

import (
	"context"
	"errors"
	"net/http"

	appbootstrap "jiyi/mochat-go/internal/app/bootstrap"
	appmodules "jiyi/mochat-go/internal/app/modules"
	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardprincipal"
	scrmhttp "jiyi/mochat-go/internal/modules/scrm/transport/http"
	"jiyi/mochat-go/internal/store"
)

func registerChatMediaModule(
	router *appmodules.Router,
	cfg config.Config,
	getMySQLStore func() *store.MySQLStore,
	principalResolver scrmhttp.PrincipalResolver,
) error {
	if router == nil {
		return errors.New("chat media route registrar is required")
	}
	if getMySQLStore == nil || principalResolver == nil {
		return errors.New("chat media runtime dependencies are required")
	}
	mysqlStore := getMySQLStore()
	if mysqlStore == nil {
		return errors.New("chat media MySQL store is required")
	}
	leadAuthorizer, err := appbootstrap.NewSCRMLeadAuthorizer(mysqlStore, dashboard.NewRBACResolver(mysqlStore))
	if err != nil {
		return err
	}
	return appbootstrap.RegisterChatMedia(router, true, appbootstrap.ChatMediaDependencies{
		DB:                mysqlStore.DB(),
		FileStorageRoot:   cfg.FileStorageRoot,
		PrincipalResolver: chatMediaPrincipalResolver{},
		Authorizer:        chatMediaAuthorizer{delegate: leadAuthorizer},
	})
}

type chatMediaPrincipalResolver struct{}

func (r chatMediaPrincipalResolver) Resolve(request *http.Request) (scrmhttp.Principal, error) {
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

type chatMediaAuthorizer struct {
	delegate scrmhttp.LeadAuthorizer
}

func (a chatMediaAuthorizer) Authorize(ctx context.Context, principal scrmhttp.Principal, corpID int64, permission string) error {
	return a.delegate.Authorize(ctx, principal, corpID, permission)
}
