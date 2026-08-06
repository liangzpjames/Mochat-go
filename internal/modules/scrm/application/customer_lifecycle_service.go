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
	TenantID         int64
	CorpID           int64
	Keyword          string
	Sources          []string
	BusinessTypes    []string
	TagIDs           []string
	Regions          []string
	Reasons          []string
	PreviousOwnerIDs []int64
	Cursor           string
	PageSize         int
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

func (s CustomerLifecycleService) CreateContact(ctx context.Context, command ports.CreateContactCommand) (ports.ContactSummary, error) {
	command.Name = strings.TrimSpace(command.Name)
	command.Phone = strings.TrimSpace(command.Phone)
	if command.TenantID <= 0 || command.CorpID <= 0 || command.ActorID <= 0 || command.Name == "" || len(command.Name) > 200 || len(command.Phone) > 64 {
		return ports.ContactSummary{}, fmt.Errorf("%w: invalid contact", ErrInvalidArgument)
	}
	creator, ok := s.assignments.(ports.ContactCreator)
	if !ok {
		return ports.ContactSummary{}, fmt.Errorf("%w: contact create unavailable", ErrUnavailable)
	}
	item, err := creator.CreateContact(ctx, command)
	if err != nil {
		return ports.ContactSummary{}, mapContactError(err)
	}
	return item, nil
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
	for _, ownerID := range query.PreviousOwnerIDs {
		if ownerID <= 0 {
			return ports.AssignmentPage{}, fmt.Errorf("%w: invalid previous owner", ErrInvalidArgument)
		}
	}
	page, err := s.assignments.ListPublicPool(ctx, ports.ListPublicPoolFilter{
		TenantID: query.TenantID, CorpID: query.CorpID, Keyword: strings.TrimSpace(query.Keyword),
		Sources: trimPublicPoolFilters(query.Sources), BusinessTypes: trimPublicPoolFilters(query.BusinessTypes),
		TagIDs: trimPublicPoolFilters(query.TagIDs), Regions: trimPublicPoolFilters(query.Regions),
		Reasons: trimPublicPoolFilters(query.Reasons), PreviousOwnerIDs: append([]int64(nil), query.PreviousOwnerIDs...),
		Cursor: query.Cursor, Limit: limit,
	})
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

func (s CustomerLifecycleService) MoveToPublicPool(ctx context.Context, command ports.MoveToPublicPoolCommand) (domain.CustomerAssignment, error) {
	command.ContactID = strings.TrimSpace(command.ContactID)
	command.Reason = strings.TrimSpace(command.Reason)
	command.IdempotencyKey = strings.TrimSpace(command.IdempotencyKey)
	if command.TenantID <= 0 || command.CorpID <= 0 || command.ContactID == "" || command.ActorID <= 0 || command.Version <= 0 || command.Reason == "" || command.IdempotencyKey == "" || domain.ValidatePublicPoolAction(command.Action) != nil {
		return domain.CustomerAssignment{}, fmt.Errorf("%w: invalid public pool move", ErrInvalidArgument)
	}
	assignment, err := s.assignments.MoveToPublicPool(ctx, command)
	if err != nil {
		return domain.CustomerAssignment{}, mapAssignmentError(err)
	}
	return assignment, nil
}

func (s CustomerLifecycleService) ClaimFromPublicPool(ctx context.Context, command ports.ClaimPublicPoolCommand) (domain.CustomerAssignment, error) {
	command.ContactID = strings.TrimSpace(command.ContactID)
	command.IdempotencyKey = strings.TrimSpace(command.IdempotencyKey)
	if command.TenantID <= 0 || command.CorpID <= 0 || command.ContactID == "" || command.UserID <= 0 || command.Version <= 0 || command.IdempotencyKey == "" {
		return domain.CustomerAssignment{}, fmt.Errorf("%w: invalid public pool claim", ErrInvalidArgument)
	}
	assignment, err := s.assignments.ClaimFromPublicPool(ctx, command)
	if err != nil {
		return domain.CustomerAssignment{}, mapAssignmentError(err)
	}
	return assignment, nil
}

type PublicPoolClaimTarget struct {
	ContactID      string `json:"contactId"`
	Version        int64  `json:"version"`
	IdempotencyKey string `json:"idempotencyKey"`
}

type BatchClaimPublicPoolCommand struct {
	TenantID int64
	CorpID   int64
	UserID   int64
	Targets  []PublicPoolClaimTarget
}

type PublicPoolMutationResult struct {
	ID         string                     `json:"id"`
	Status     string                     `json:"status"`
	ErrorCode  string                     `json:"errorCode"`
	Assignment *domain.CustomerAssignment `json:"assignment,omitempty"`
}

func (s CustomerLifecycleService) BatchClaimFromPublicPool(ctx context.Context, command BatchClaimPublicPoolCommand) ([]PublicPoolMutationResult, error) {
	if command.TenantID <= 0 || command.CorpID <= 0 || command.UserID <= 0 || len(command.Targets) == 0 || len(command.Targets) > 100 {
		return nil, fmt.Errorf("%w: invalid public pool batch claim", ErrInvalidArgument)
	}
	results := make([]PublicPoolMutationResult, 0, len(command.Targets))
	for _, target := range command.Targets {
		contactID := strings.TrimSpace(target.ContactID)
		result := PublicPoolMutationResult{ID: contactID, Status: "failed"}
		if contactID == "" || target.Version <= 0 || strings.TrimSpace(target.IdempotencyKey) == "" {
			result.ErrorCode = "VALIDATION_ERROR"
			results = append(results, result)
			continue
		}
		assignment, err := s.ClaimFromPublicPool(ctx, ports.ClaimPublicPoolCommand{TenantID: command.TenantID, CorpID: command.CorpID, ContactID: contactID, UserID: command.UserID, Version: target.Version, IdempotencyKey: target.IdempotencyKey})
		if err != nil {
			result.ErrorCode = publicPoolErrorCode(err)
			results = append(results, result)
			continue
		}
		result.Status, result.Assignment = "succeeded", &assignment
		results = append(results, result)
	}
	return results, nil
}

func publicPoolErrorCode(err error) string {
	switch {
	case errors.Is(err, ports.ErrAssignmentConflict):
		return "CONFLICT"
	case errors.Is(err, ports.ErrAssignmentForbidden):
		return "FORBIDDEN"
	case errors.Is(err, ErrNotFound):
		return "NOT_FOUND"
	case errors.Is(err, ErrInvalidArgument):
		return "VALIDATION_ERROR"
	default:
		return "UNAVAILABLE"
	}
}

func trimPublicPoolFilters(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
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
