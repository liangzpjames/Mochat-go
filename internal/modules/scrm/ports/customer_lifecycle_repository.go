package ports

import (
	"context"
	"errors"

	"jiyi/mochat-go/internal/modules/scrm/domain"
)

var (
	ErrAssignmentNotFound = errors.New("assignment not found")
	ErrAssignmentConflict = errors.New("assignment conflict")
)

type AssignmentPage struct {
	Items      []domain.CustomerAssignment
	NextCursor string
}

type ListPublicPoolFilter struct {
	TenantID int64
	CorpID   int64
	Cursor   string
	Limit    int
}

type UpdateAssignmentCommand struct {
	TenantID        int64
	CorpID          int64
	ContactID       string
	OwnerID         *int64
	CollaboratorIDs []int64
	Version         int64
	IdempotencyKey  string
}

type AssignmentRepository interface {
	ListPublicPool(context.Context, ListPublicPoolFilter) (AssignmentPage, error)
	UpdateAssignment(context.Context, UpdateAssignmentCommand) (domain.CustomerAssignment, error)
	ReleaseToPublicPool(context.Context, int64, int64, string, int64, string) (domain.CustomerAssignment, error)
	ClaimFromPublicPool(context.Context, int64, int64, string, int64, int64, string) (domain.CustomerAssignment, error)
}
