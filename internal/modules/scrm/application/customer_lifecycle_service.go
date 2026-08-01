package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"jiyi/mochat-go/internal/modules/scrm/domain"
	"jiyi/mochat-go/internal/modules/scrm/ports"
)

type CustomerLifecycleService struct {
	assignments ports.AssignmentRepository
}

func NewCustomerLifecycleService(assignments ports.AssignmentRepository) (CustomerLifecycleService, error) {
	if isNilDependency(assignments) {
		return CustomerLifecycleService{}, fmt.Errorf("%w: assignment repository is required", ErrInvalidArgument)
	}
	return CustomerLifecycleService{assignments: assignments}, nil
}

type ListPublicPoolQuery struct {
	TenantID int64
	CorpID   int64
	Cursor   string
	PageSize int
}

type ListContactsQuery struct {
	TenantID int64
	CorpID   int64
	Keyword  string
	OwnerIDs []int64
	TagIDs   []string
	Statuses []string
	Cursor   string
	PageSize int
}

func (s CustomerLifecycleService) ListContacts(ctx context.Context, query ListContactsQuery) (ports.ContactPage, error) {
	if query.TenantID <= 0 || query.CorpID <= 0 || strings.HasPrefix(strings.TrimSpace(query.Cursor), "-") || query.PageSize < 0 {
		return ports.ContactPage{}, fmt.Errorf("%w: invalid contact query", ErrInvalidArgument)
	}
	for _, id := range query.OwnerIDs {
		if id <= 0 {
			return ports.ContactPage{}, fmt.Errorf("%w: invalid contact owner", ErrInvalidArgument)
		}
	}
	for _, status := range query.Statuses {
		if err := domain.ValidateAssignmentStatus(status); err != nil {
			return ports.ContactPage{}, fmt.Errorf("%w: %v", ErrInvalidArgument, err)
		}
	}
	limit := query.PageSize
	if limit == 0 {
		limit = defaultPageSize
	}
	if limit > maximumPageSize {
		limit = maximumPageSize
	}
	page, err := s.assignments.ListContacts(ctx, ports.ListContactsFilter{TenantID: query.TenantID, CorpID: query.CorpID, Keyword: strings.TrimSpace(query.Keyword), OwnerIDs: query.OwnerIDs, TagIDs: query.TagIDs, Statuses: query.Statuses, Cursor: query.Cursor, Limit: limit})
	if err != nil {
		return ports.ContactPage{}, mapContactError(err)
	}
	return page, nil
}

func (s CustomerLifecycleService) GetContact(ctx context.Context, tenantID, corpID int64, contactID string) (ports.ContactDetail, error) {
	if tenantID <= 0 || corpID <= 0 || strings.TrimSpace(contactID) == "" {
		return ports.ContactDetail{}, fmt.Errorf("%w: invalid contact detail", ErrInvalidArgument)
	}
	detail, err := s.assignments.GetContact(ctx, tenantID, corpID, strings.TrimSpace(contactID))
	if err != nil {
		return ports.ContactDetail{}, mapContactError(err)
	}
	return detail, nil
}

func mapContactError(err error) error {
	switch {
	case errors.Is(err, ports.ErrContactNotFound), errors.Is(err, ports.ErrAssignmentNotFound):
		return fmt.Errorf("%w: %v", ErrNotFound, err)
	case errors.Is(err, ports.ErrAssignmentConflict):
		return fmt.Errorf("%w: %v", ports.ErrAssignmentConflict, err)
	default:
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
}

func (s CustomerLifecycleService) ListPublicPool(ctx context.Context, query ListPublicPoolQuery) (ports.AssignmentPage, error) {
	if query.TenantID <= 0 || query.CorpID <= 0 || strings.HasPrefix(strings.TrimSpace(query.Cursor), "-") || query.PageSize < 0 {
		return ports.AssignmentPage{}, fmt.Errorf("%w: invalid public pool query", ErrInvalidArgument)
	}
	limit := query.PageSize
	if limit == 0 {
		limit = defaultPageSize
	}
	if limit > maximumPageSize {
		limit = maximumPageSize
	}
	page, err := s.assignments.ListPublicPool(ctx, ports.ListPublicPoolFilter{TenantID: query.TenantID, CorpID: query.CorpID, Cursor: query.Cursor, Limit: limit})
	if err != nil {
		return ports.AssignmentPage{}, mapAssignmentError(err)
	}
	return page, nil
}

func (s CustomerLifecycleService) UpdateAssignment(ctx context.Context, command ports.UpdateAssignmentCommand) (domain.CustomerAssignment, error) {
	if command.TenantID <= 0 || command.CorpID <= 0 || strings.TrimSpace(command.ContactID) == "" || command.Version <= 0 || strings.TrimSpace(command.IdempotencyKey) == "" {
		return domain.CustomerAssignment{}, fmt.Errorf("%w: invalid assignment update", ErrInvalidArgument)
	}
	assignment, err := s.assignments.UpdateAssignment(ctx, command)
	if err != nil {
		return domain.CustomerAssignment{}, mapAssignmentError(err)
	}
	return assignment, nil
}

func (s CustomerLifecycleService) ReleaseToPublicPool(ctx context.Context, tenantID, corpID int64, contactID string, version int64, idempotencyKey string) (domain.CustomerAssignment, error) {
	if tenantID <= 0 || corpID <= 0 || strings.TrimSpace(contactID) == "" || version <= 0 || strings.TrimSpace(idempotencyKey) == "" {
		return domain.CustomerAssignment{}, fmt.Errorf("%w: invalid public pool release", ErrInvalidArgument)
	}
	assignment, err := s.assignments.ReleaseToPublicPool(ctx, tenantID, corpID, contactID, version, idempotencyKey)
	if err != nil {
		return domain.CustomerAssignment{}, mapAssignmentError(err)
	}
	return assignment, nil
}

func (s CustomerLifecycleService) ClaimFromPublicPool(ctx context.Context, tenantID, corpID int64, contactID string, userID, version int64, idempotencyKey string) (domain.CustomerAssignment, error) {
	if tenantID <= 0 || corpID <= 0 || strings.TrimSpace(contactID) == "" || userID <= 0 || version <= 0 || strings.TrimSpace(idempotencyKey) == "" {
		return domain.CustomerAssignment{}, fmt.Errorf("%w: invalid public pool claim", ErrInvalidArgument)
	}
	assignment, err := s.assignments.ClaimFromPublicPool(ctx, tenantID, corpID, contactID, userID, version, idempotencyKey)
	if err != nil {
		return domain.CustomerAssignment{}, mapAssignmentError(err)
	}
	return assignment, nil
}

func mapAssignmentError(err error) error {
	switch {
	case errors.Is(err, ports.ErrAssignmentForbidden):
		return fmt.Errorf("%w: %v", ports.ErrAssignmentForbidden, err)
	case errors.Is(err, ports.ErrAssignmentNotFound), errors.Is(err, ports.ErrContactNotFound):
		return fmt.Errorf("%w: %v", ErrNotFound, err)
	case errors.Is(err, ports.ErrAssignmentConflict), errors.Is(err, domain.ErrAssignmentVersionConflict):
		return fmt.Errorf("%w: %v", ports.ErrAssignmentConflict, err)
	default:
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
}
