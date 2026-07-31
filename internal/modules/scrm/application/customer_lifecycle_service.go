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
	case errors.Is(err, ports.ErrAssignmentConflict), errors.Is(err, domain.ErrAssignmentVersionConflict):
		return fmt.Errorf("%w: %v", ports.ErrAssignmentConflict, err)
	default:
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
}
