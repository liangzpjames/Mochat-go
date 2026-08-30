package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	appmodules "jiyi/mochat-go/internal/app/modules"
	"jiyi/mochat-go/internal/moduleprincipal"
	chatmedia "jiyi/mochat-go/internal/modules/chat-media"
	audiolocal "jiyi/mochat-go/internal/modules/providers/audio/local"
	scrmhttp "jiyi/mochat-go/internal/modules/scrm/transport/http"
)

type ChatMediaDependencies struct {
	DB                *sql.DB
	FileStorageRoot   string
	PrincipalResolver scrmhttp.PrincipalResolver
	Authorizer        scrmhttp.LeadAuthorizer
}

func RegisterChatMedia(router *appmodules.Router, enabled bool, dependencies ChatMediaDependencies) error {
	if !enabled {
		return nil
	}
	if router == nil {
		return errors.New("route registrar is required")
	}
	storage, err := audiolocal.New(audiolocal.Config{Root: dependencies.FileStorageRoot})
	if err != nil {
		return err
	}
	module, err := chatmedia.New(chatmedia.Dependencies{
		DB:                dependencies.DB,
		AudioProvider:     storage,
		PrincipalResolver: chatMediaPrincipalResolver{delegate: dependencies.PrincipalResolver},
		Authorizer:        chatMediaAuthorizer{delegate: dependencies.Authorizer},
	})
	if err != nil {
		return err
	}
	return module.RegisterRoutes(router)
}

type chatMediaPrincipalResolver struct{ delegate scrmhttp.PrincipalResolver }

func (a chatMediaPrincipalResolver) Resolve(r *http.Request) (moduleprincipal.Principal, error) {
	p, err := a.delegate.Resolve(r)
	return moduleprincipal.Principal{UserID: p.UserID, TenantID: p.TenantID, CorpID: p.CorpID}, err
}

type chatMediaAuthorizer struct{ delegate scrmhttp.LeadAuthorizer }

func (a chatMediaAuthorizer) Authorize(ctx context.Context, p moduleprincipal.Principal, corpID int64, permission string) error {
	return a.delegate.Authorize(ctx, scrmhttp.Principal{UserID: p.UserID, TenantID: p.TenantID, CorpID: p.CorpID}, corpID, permission)
}
