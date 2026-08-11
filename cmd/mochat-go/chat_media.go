package main

import (
	"context"
	"errors"
	"net/http"

	appbootstrap "jiyi/mochat-go/internal/app/bootstrap"
	appmodules "jiyi/mochat-go/internal/app/modules"
	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/dashboard"
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
		PrincipalResolver: chatMediaPrincipalResolver{delegate: principalResolver},
		Authorizer:        chatMediaAuthorizer{delegate: leadAuthorizer},
	})
}

type chatMediaPrincipalResolver struct {
	delegate scrmhttp.PrincipalResolver
}

func (r chatMediaPrincipalResolver) Resolve(request *http.Request) (scrmhttp.Principal, error) {
	return r.delegate.Resolve(request)
}

type chatMediaAuthorizer struct {
	delegate scrmhttp.LeadAuthorizer
}

func (a chatMediaAuthorizer) Authorize(ctx context.Context, principal scrmhttp.Principal, corpID int64, permission string) error {
	return a.delegate.Authorize(ctx, principal, corpID, permission)
}
