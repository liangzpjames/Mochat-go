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
	ErrConflict        = errors.New("version conflict")
	ErrNotFound        = errors.New("not found")
	ErrDuplicate       = errors.New("duplicate lead")
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
	CorpID      int64
	BusinessKey string
	Name        string
	Phone       string
	Source      domain.LeadSource
}

type LeadView struct {
	ID                 string
	TenantID           int64
	CorpID             int64
	BusinessKey        string
	Name               string
	Phone              string
	Source             domain.LeadSource
	Status             domain.LeadStatus
	OwnerID            *int64
	ConvertedContactID string
	DiscardReason      string
	Version            int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type CreateLeadResult struct {
	Lead    LeadView
	Created bool
}

func (s Service) CreateLead(ctx context.Context, command CreateLeadCommand) (CreateLeadResult, error) {
	if _, err := newCommandLead("", command, time.Time{}); err != nil {
		return CreateLeadResult{}, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	}

	id, err := s.idGenerator.NewID()
	if err != nil {
		return CreateLeadResult{}, fmt.Errorf("%w: generate lead ID: %v", ErrUnavailable, err)
	}
	lead, err := newCommandLead(id, command, s.clock.Now())
	if err != nil {
		return CreateLeadResult{}, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	}
	persisted, created, err := s.repository.CreateOrGet(ctx, lead)
	if err != nil {
		if errors.Is(err, ports.ErrDuplicateLead) {
			return CreateLeadResult{}, fmt.Errorf("%w: create lead", ErrDuplicate)
		}
		return CreateLeadResult{}, fmt.Errorf("%w: create lead: %v", ErrUnavailable, err)
	}
	return CreateLeadResult{Lead: leadView(persisted), Created: created}, nil
}

func newCommandLead(id string, command CreateLeadCommand, now time.Time) (domain.Lead, error) {
	if command.CorpID > 0 {
		return domain.NewCorpLead(id, command.TenantID, command.CorpID, command.BusinessKey, command.Name, command.Phone, command.Source, now)
	}
	return domain.NewLead(id, command.TenantID, command.BusinessKey, command.Name, command.Source, now)
}

type ListLeadsQuery struct {
	TenantID                int64
	CorpID                  int64
	Keyword                 string
	Statuses                []domain.LeadStatus
	Sources                 []domain.LeadSource
	OwnerIDs                []int64
	CreatedFrom             time.Time
	CreatedTo               time.Time
	Cursor                  string
	PageSize                int
	AllowedEmployeeIDs      []int64
	EmployeeScopeRestricted bool
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
	if query.CorpID < 0 || (!query.CreatedFrom.IsZero() && !query.CreatedTo.IsZero() && !query.CreatedFrom.Before(query.CreatedTo)) {
		return LeadPage{}, fmt.Errorf("%w: invalid lead filter", ErrInvalidArgument)
	}

	limit := query.PageSize
	if limit == 0 {
		limit = defaultPageSize
	}
	if limit > maximumPageSize {
		limit = maximumPageSize
	}

	page, err := s.repository.List(ctx, ports.ListLeadsFilter{
		TenantID:    query.TenantID,
		CorpID:      query.CorpID,
		Keyword:     strings.TrimSpace(query.Keyword),
		Statuses:    append([]domain.LeadStatus(nil), query.Statuses...),
		Sources:     append([]domain.LeadSource(nil), query.Sources...),
		OwnerIDs:    restrictOwnerIDs(query.OwnerIDs, query.AllowedEmployeeIDs, query.EmployeeScopeRestricted),
		CreatedFrom: query.CreatedFrom.UTC(), CreatedTo: query.CreatedTo.UTC(),
		Cursor: query.Cursor,
		Limit:  limit,
	})
	if err != nil {
		if errors.Is(err, ports.ErrInvalidCursor) {
			return LeadPage{}, fmt.Errorf("%w: list leads: %v", ErrInvalidArgument, err)
		}
		return LeadPage{}, fmt.Errorf("%w: list leads: %v", ErrUnavailable, err)
	}
	return page, nil
}

type LeadMutationTarget struct {
	ID      string `json:"id"`
	Version int64  `json:"version"`
}
type AssignLeadsCommand struct {
	TenantID, CorpID, OwnerID int64
	Targets                   []LeadMutationTarget
}
type LeadMutationResult struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	ErrorCode string    `json:"errorCode"`
	Lead      *LeadView `json:"lead,omitempty"`
}

func (s Service) AssignLeads(ctx context.Context, command AssignLeadsCommand) ([]LeadMutationResult, error) {
	if command.TenantID <= 0 || command.CorpID <= 0 || command.OwnerID <= 0 || len(command.Targets) == 0 || len(command.Targets) > 100 {
		return nil, fmt.Errorf("%w: invalid assignment", ErrInvalidArgument)
	}
	results := make([]LeadMutationResult, 0, len(command.Targets))
	for _, target := range command.Targets {
		result := LeadMutationResult{ID: target.ID, Status: "failed"}
		if strings.TrimSpace(target.ID) == "" || target.Version <= 0 {
			result.ErrorCode = "VALIDATION_ERROR"
			results = append(results, result)
			continue
		}
		lead, err := s.repository.Assign(ctx, ports.AssignLeadCommand{TenantID: command.TenantID, CorpID: command.CorpID, LeadID: target.ID, OwnerID: command.OwnerID, Version: target.Version, UpdatedAt: s.clock.Now().UTC()})
		if err != nil {
			result.ErrorCode = leadErrorCode(err)
			results = append(results, result)
			continue
		}
		view := leadView(lead)
		result.Status, result.Lead = "succeeded", &view
		results = append(results, result)
	}
	return results, nil
}

type TransitionLeadCommand struct {
	TenantID, CorpID int64
	LeadID           string
	ToStatus         domain.LeadStatus
	Version          int64
	DiscardReason    string
}

func (s Service) TransitionLead(ctx context.Context, command TransitionLeadCommand) (LeadView, error) {
	if command.TenantID <= 0 || command.CorpID <= 0 || strings.TrimSpace(command.LeadID) == "" || command.Version <= 0 ||
		(command.ToStatus != domain.LeadStatusQualified && command.ToStatus != domain.LeadStatusConverted && command.ToStatus != domain.LeadStatusDiscarded) ||
		(command.ToStatus == domain.LeadStatusDiscarded && strings.TrimSpace(command.DiscardReason) == "") {
		return LeadView{}, fmt.Errorf("%w: invalid transition", ErrInvalidArgument)
	}
	lead, err := s.repository.Transition(ctx, ports.TransitionLeadCommand{TenantID: command.TenantID, CorpID: command.CorpID, LeadID: command.LeadID, ToStatus: command.ToStatus, Version: command.Version, DiscardReason: strings.TrimSpace(command.DiscardReason), UpdatedAt: s.clock.Now().UTC()})
	if err != nil {
		return LeadView{}, mapLeadMutationError(err)
	}
	return leadView(lead), nil
}

func (s Service) FindDuplicateLeads(ctx context.Context, tenantID, corpID int64, businessKey, phone string) ([]LeadView, error) {
	if tenantID <= 0 || corpID <= 0 || (strings.TrimSpace(businessKey) == "" && strings.TrimSpace(phone) == "") {
		return nil, fmt.Errorf("%w: duplicate key required", ErrInvalidArgument)
	}
	items, err := s.repository.FindDuplicates(ctx, ports.DuplicateLeadFilter{TenantID: tenantID, CorpID: corpID, BusinessKey: strings.TrimSpace(businessKey), Phone: strings.TrimSpace(phone)})
	if err != nil {
		return nil, fmt.Errorf("%w: find duplicate leads: %v", ErrUnavailable, err)
	}
	views := make([]LeadView, len(items))
	for i := range items {
		views[i] = leadView(items[i])
	}
	return views, nil
}

func leadErrorCode(err error) string {
	switch {
	case errors.Is(err, ports.ErrLeadNotFound):
		return "NOT_FOUND"
	case errors.Is(err, ports.ErrLeadConflict):
		return "CONFLICT"
	case errors.Is(err, ports.ErrLeadOwnerOutOfScope):
		return "OWNER_OUT_OF_SCOPE"
	case errors.Is(err, domain.ErrInvalidLeadTransition):
		return "INVALID_TRANSITION"
	default:
		return "INTERNAL_ERROR"
	}
}
func mapLeadMutationError(err error) error {
	switch {
	case errors.Is(err, ports.ErrLeadNotFound):
		return fmt.Errorf("%w: lead", ErrNotFound)
	case errors.Is(err, ports.ErrLeadConflict):
		return fmt.Errorf("%w: lead", ErrConflict)
	case errors.Is(err, domain.ErrInvalidLeadTransition):
		return fmt.Errorf("%w: transition", ErrInvalidArgument)
	default:
		return fmt.Errorf("%w: mutate lead: %v", ErrUnavailable, err)
	}
}

func leadView(lead domain.Lead) LeadView {
	return LeadView{
		ID:          lead.ID,
		TenantID:    lead.TenantID,
		CorpID:      lead.CorpID,
		BusinessKey: lead.BusinessKey,
		Name:        lead.Name.String(),
		Phone:       lead.Phone,
		Source:      lead.Source,
		Status:      lead.Status,
		OwnerID:     lead.OwnerID, ConvertedContactID: lead.ConvertedContactID, DiscardReason: lead.DiscardReason,
		Version:   lead.Version,
		CreatedAt: lead.CreatedAt.UTC(),
		UpdatedAt: lead.UpdatedAt.UTC(),
	}
}
