package ports

import (
	"context"
	"errors"
	"time"

	"jiyi/mochat-go/internal/modules/scrm/domain"
)

var ErrInvalidCursor = errors.New("invalid cursor")

var (
	ErrLeadNotFound        = errors.New("lead not found")
	ErrLeadConflict        = errors.New("lead version conflict")
	ErrDuplicateLead       = errors.New("duplicate lead")
	ErrLeadOwnerOutOfScope = errors.New("lead owner is not an active employee in tenant and corp scope")
)

type LeadRepository interface {
	// CreateOrGet resolves lead identity strictly by (TenantID, BusinessKey).
	// Duplicate or concurrent calls must create at most one lead and return the
	// existing lead. created is true only for the call that actually inserted it.
	CreateOrGet(context.Context, domain.Lead) (lead domain.Lead, created bool, err error)
	List(context.Context, ListLeadsFilter) (LeadPage, error)
	FindDuplicates(context.Context, DuplicateLeadFilter) ([]domain.Lead, error)
	Assign(context.Context, AssignLeadCommand) (domain.Lead, error)
	Transition(context.Context, TransitionLeadCommand) (domain.Lead, error)
}

type ListLeadsFilter struct {
	TenantID    int64
	CorpID      int64
	Keyword     string
	Statuses    []domain.LeadStatus
	Sources     []domain.LeadSource
	OwnerIDs    []int64
	CreatedFrom time.Time
	CreatedTo   time.Time
	Cursor      string
	Limit       int
}

type DuplicateLeadFilter struct {
	TenantID, CorpID   int64
	BusinessKey, Phone string
}
type AssignLeadCommand struct {
	TenantID, CorpID int64
	LeadID           string
	OwnerID, Version int64
	UpdatedAt        time.Time
}
type TransitionLeadCommand struct {
	TenantID, CorpID int64
	LeadID           string
	ToStatus         domain.LeadStatus
	Version          int64
	DiscardReason    string
	UpdatedAt        time.Time
}

type LeadPage struct {
	Items      []domain.Lead
	NextCursor string
}
