package ports

import (
	"context"

	"jiyi/mochat-go/internal/modules/scrm/domain"
)

type LeadRepository interface {
	// CreateOrGet resolves lead identity strictly by (TenantID, BusinessKey).
	// Duplicate or concurrent calls must create at most one lead and return the
	// existing lead. created is true only for the call that actually inserted it.
	CreateOrGet(context.Context, domain.Lead) (lead domain.Lead, created bool, err error)
	List(context.Context, ListLeadsFilter) (LeadPage, error)
}

type ListLeadsFilter struct {
	TenantID int64
	Cursor   string
	Limit    int
}

type LeadPage struct {
	Items      []domain.Lead
	NextCursor string
}
