package aiinsight

import (
	"errors"

	appmodules "jiyi/mochat-go/internal/app/modules"
	transporthttp "jiyi/mochat-go/internal/modules/ai-insight/transport/http"
)

type Dependencies struct {
	PrincipalResolver transporthttp.PrincipalResolver
	Authorizer        transporthttp.Authorizer
}

type Module struct {
	handler *transporthttp.InsightHandler
}

func New(dependencies Dependencies) (*Module, error) {
	if dependencies.PrincipalResolver == nil {
		return nil, errors.New("AI insight principal resolver is required")
	}
	return &Module{handler: transporthttp.NewInsightHandler(dependencies.PrincipalResolver, dependencies.Authorizer)}, nil
}

func (m *Module) RegisterRoutes(registrar appmodules.RouteRegistrar) error {
	if m == nil || m.handler == nil {
		return errors.New("AI insight module is not initialized")
	}
	return transporthttp.RegisterRoutes(registrar, m.handler)
}
