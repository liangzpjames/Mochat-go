package scrm

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	appmodules "jiyi/mochat-go/internal/app/modules"
	"jiyi/mochat-go/internal/modules/scrm/adapters/mysql"
	"jiyi/mochat-go/internal/modules/scrm/application"
	"jiyi/mochat-go/internal/modules/scrm/ports"
	transporthttp "jiyi/mochat-go/internal/modules/scrm/transport/http"
)

type Dependencies struct {
	DB                *sql.DB
	Clock             ports.Clock
	IDGenerator       ports.IDGenerator
	PrincipalResolver transporthttp.PrincipalResolver
}

type Module struct {
	leads *transporthttp.LeadHandler
}

func New(dependencies Dependencies) (*Module, error) {
	if dependencies.DB == nil {
		return nil, errors.New("SCRM database is required")
	}
	if isNil(dependencies.Clock) {
		return nil, errors.New("SCRM clock is required")
	}
	if isNil(dependencies.IDGenerator) {
		return nil, errors.New("SCRM ID generator is required")
	}
	if isNil(dependencies.PrincipalResolver) {
		return nil, errors.New("SCRM principal resolver is required")
	}

	repository, err := mysql.NewLeadRepository(dependencies.DB)
	if err != nil {
		return nil, fmt.Errorf("create SCRM lead repository: %w", err)
	}
	service, err := application.NewService(repository, dependencies.Clock, dependencies.IDGenerator)
	if err != nil {
		return nil, fmt.Errorf("create SCRM application service: %w", err)
	}
	handler := transporthttp.NewLeadHandler(service, dependencies.PrincipalResolver)
	return &Module{leads: handler}, nil
}

func (m *Module) RegisterRoutes(registrar appmodules.RouteRegistrar) error {
	if m == nil || m.leads == nil {
		return errors.New("SCRM module is not initialized")
	}
	if isNil(registrar) {
		return errors.New("route registrar is required")
	}
	return transporthttp.RegisterRoutes(registrar, m.leads)
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
