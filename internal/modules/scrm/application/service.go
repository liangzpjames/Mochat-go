package application

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"jiyi/mochat-go/internal/modules/scrm/domain"
	"jiyi/mochat-go/internal/modules/scrm/ports"
)

var (
	ErrInvalidArgument = errors.New("invalid argument")
	ErrUnavailable     = errors.New("repository unavailable")
)

const (
	defaultPageSize = 20
	maximumPageSize = 100
)

type Service struct {
	repository  ports.LeadRepository
	clock       ports.Clock
	idGenerator ports.IDGenerator
}

func NewService(repository ports.LeadRepository, clock ports.Clock, idGenerator ports.IDGenerator) (Service, error) {
	if isNilDependency(repository) || isNilDependency(clock) || isNilDependency(idGenerator) {
		return Service{}, fmt.Errorf("%w: service dependencies are required", ErrInvalidArgument)
	}
	return Service{repository: repository, clock: clock, idGenerator: idGenerator}, nil
}

func isNilDependency(dependency any) bool {
	if dependency == nil {
		return true
	}
	value := reflect.ValueOf(dependency)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

type CreateLeadCommand struct {
	TenantID    int64
	BusinessKey string
	Name        string
	Source      domain.LeadSource
}

type LeadView struct {
	ID          string
	TenantID    int64
	BusinessKey string
	Name        string
	Source      domain.LeadSource
	Status      domain.LeadStatus
	Version     int64
}

func (s Service) CreateLead(ctx context.Context, command CreateLeadCommand) (LeadView, error) {
	if _, err := domain.NewLead("", command.TenantID, command.BusinessKey, command.Name, command.Source, time.Time{}); err != nil {
		return LeadView{}, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	}

	id, err := s.idGenerator.NewID()
	if err != nil {
		return LeadView{}, fmt.Errorf("%w: generate lead ID: %v", ErrUnavailable, err)
	}
	lead, err := domain.NewLead(id, command.TenantID, command.BusinessKey, command.Name, command.Source, s.clock.Now())
	if err != nil {
		return LeadView{}, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	}
	persisted, _, err := s.repository.CreateOrGet(ctx, lead)
	if err != nil {
		return LeadView{}, fmt.Errorf("%w: create lead: %v", ErrUnavailable, err)
	}
	return leadView(persisted), nil
}

type ListLeadsQuery struct {
	TenantID int64
	Cursor   string
	PageSize int
}

type LeadPage = ports.LeadPage

func (s Service) ListLeads(ctx context.Context, query ListLeadsQuery) (LeadPage, error) {
	if query.TenantID <= 0 {
		return LeadPage{}, fmt.Errorf("%w: tenant ID is required", ErrInvalidArgument)
	}
	if strings.HasPrefix(strings.TrimSpace(query.Cursor), "-") {
		return LeadPage{}, fmt.Errorf("%w: cursor must not be negative", ErrInvalidArgument)
	}
	if query.PageSize < 0 {
		return LeadPage{}, fmt.Errorf("%w: page size must not be negative", ErrInvalidArgument)
	}

	limit := query.PageSize
	if limit == 0 {
		limit = defaultPageSize
	}
	if limit > maximumPageSize {
		limit = maximumPageSize
	}

	page, err := s.repository.List(ctx, ports.ListLeadsFilter{
		TenantID: query.TenantID,
		Cursor:   query.Cursor,
		Limit:    limit,
	})
	if err != nil {
		return LeadPage{}, fmt.Errorf("%w: list leads: %v", ErrUnavailable, err)
	}
	return page, nil
}

func leadView(lead domain.Lead) LeadView {
	return LeadView{
		ID:          lead.ID,
		TenantID:    lead.TenantID,
		BusinessKey: lead.BusinessKey,
		Name:        lead.Name.String(),
		Source:      lead.Source,
		Status:      lead.Status,
		Version:     lead.Version,
	}
}
