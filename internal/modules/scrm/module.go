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
	LeadAuthorizer    transporthttp.LeadAuthorizer
}

type Module struct {
	leads             *transporthttp.LeadHandler
	customerLifecycle *transporthttp.CustomerLifecycleHandler
	opportunities     application.OpportunityService
	opportunityHTTP   *transporthttp.OpportunityHandler
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
	if isNil(dependencies.LeadAuthorizer) {
		return nil, errors.New("SCRM lead authorizer is required")
	}

	repository, err := mysql.NewLeadRepository(dependencies.DB)
	if err != nil {
		return nil, fmt.Errorf("create SCRM lead repository: %w", err)
	}
	service, err := application.NewService(repository, dependencies.Clock, dependencies.IDGenerator)
	if err != nil {
		return nil, fmt.Errorf("create SCRM application service: %w", err)
	}
	handler := transporthttp.NewLeadHandler(service, dependencies.PrincipalResolver, dependencies.LeadAuthorizer)
	assignmentRepository, err := mysql.NewCustomerLifecycleRepository(dependencies.DB)
	if err != nil {
		return nil, fmt.Errorf("create SCRM assignment repository: %w", err)
	}
	assignmentService, err := application.NewCustomerLifecycleService(assignmentRepository)
	if err != nil {
		return nil, fmt.Errorf("create SCRM customer lifecycle service: %w", err)
	}
	assignmentHandler := transporthttp.NewCustomerLifecycleHandler(assignmentService, dependencies.PrincipalResolver)
	opportunityRepository, err := mysql.NewOpportunityRepository(dependencies.DB)
	if err != nil {
		return nil, fmt.Errorf("create SCRM opportunity repository: %w", err)
	}
	tagRepository, err := mysql.NewTagRepository(dependencies.DB)
	if err != nil {
		return nil, fmt.Errorf("create SCRM tag repository: %w", err)
	}
	opportunityService, err := application.NewOpportunityService(opportunityRepository, tagRepository)
	if err != nil {
		return nil, fmt.Errorf("create SCRM opportunity service: %w", err)
	}
	opportunityHTTP := transporthttp.NewOpportunityHandler(opportunityService, dependencies.PrincipalResolver)
	return &Module{leads: handler, customerLifecycle: assignmentHandler, opportunities: opportunityService, opportunityHTTP: opportunityHTTP}, nil
}

func (m *Module) RegisterRoutes(registrar appmodules.RouteRegistrar) error {
	if m == nil || m.leads == nil {
		return errors.New("SCRM module is not initialized")
	}
	if isNil(registrar) {
		return errors.New("route registrar is required")
	}
	if err := transporthttp.RegisterRoutes(registrar, m.leads); err != nil {
		return err
	}
	if err := transporthttp.RegisterCustomerLifecycleRoutes(registrar, m.customerLifecycle); err != nil {
		return err
	}
	return transporthttp.RegisterOpportunityRoutes(registrar, m.opportunityHTTP)
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
