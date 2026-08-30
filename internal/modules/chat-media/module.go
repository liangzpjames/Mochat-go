package chatmedia

import (
	"database/sql"
	"errors"

	appmodules "jiyi/mochat-go/internal/app/modules"
	"jiyi/mochat-go/internal/moduleprincipal"
	chatmysql "jiyi/mochat-go/internal/modules/chat-media/adapters/mysql"
	transporthttp "jiyi/mochat-go/internal/modules/chat-media/transport/http"
	"jiyi/mochat-go/internal/modules/providers"
)

type Dependencies struct {
	DB                *sql.DB
	AudioProvider     providers.AudioProvider
	PrincipalResolver moduleprincipal.Resolver
	Authorizer        moduleprincipal.Authorizer
}

type Module struct {
	handler *transporthttp.MediaHandler
}

func New(dependencies Dependencies) (*Module, error) {
	if dependencies.DB == nil {
		return nil, errors.New("chat media database is required")
	}
	if dependencies.PrincipalResolver == nil {
		return nil, errors.New("chat media principal resolver is required")
	}
	if dependencies.Authorizer == nil {
		return nil, errors.New("chat media authorizer is required")
	}
	store := chatmysql.NewSQLMediaStore(dependencies.DB)
	handler, err := transporthttp.NewMediaHandler(store, dependencies.AudioProvider, dependencies.PrincipalResolver, dependencies.Authorizer)
	if err != nil {
		return nil, err
	}
	return &Module{handler: handler}, nil
}

func (m *Module) RegisterRoutes(registrar appmodules.RouteRegistrar) error {
	if m == nil || m.handler == nil {
		return errors.New("chat media module is not initialized")
	}
	return transporthttp.RegisterRoutes(registrar, m.handler)
}
