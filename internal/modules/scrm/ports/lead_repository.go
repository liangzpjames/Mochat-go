package ports

import (
	"context"

	"jiyi/mochat-go/internal/modules/scrm/domain"
)

type LeadRepository interface {
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
