package ports

import (
	"context"

	"jiyi/mochat-go/internal/modules/example/domain"
)

type Repository interface {
	Save(context.Context, domain.Module) error
	Find(context.Context, string) (domain.Module, bool, error)
}
