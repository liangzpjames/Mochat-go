package ports

import (
	"context"
	"errors"
	"time"

	"jiyi/mochat-go/internal/modules/scrm/domain"
)

var (
	ErrAssignmentNotFound  = errors.New("assignment not found")
	ErrAssignmentConflict  = errors.New("assignment conflict")
	ErrAssignmentForbidden = errors.New("assignment employee out of scope")
	ErrContactNotFound     = errors.New("contact not found")
)

type ContactSummary struct {
	ID, Name, Phone, AssignmentStatus string
	OwnerID                           *int64
	TagNames                          []string
	Version, AssignmentVersion        int64
	UpdatedAt                         time.Time
}

type ContactTagSummary struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Version    int64  `json:"version"`
	UsageCount int64  `json:"usageCount"`
}
type WeComFriendSummary struct {
	ExternalUserID, Name string
	EmployeeID           int64
	AddedAt              time.Time
}
type ContactOpportunitySummary struct {
	ID, Stage, Status string
	Amount            float64
	Version           int64
}
type ContactFollowUpSummary struct {
	ID, Content string
	CreatedBy   int64
	CreatedAt   time.Time
}
type ContactDetail struct {
	ContactSummary
	Assignment   domain.CustomerAssignment
	Tags         []ContactTagSummary
	WeComFriends []WeComFriendSummary
	// False until a durable contact_id, unionid, or external_userid relation exists.
	WeComFriendsAvailable bool
	Opportunities         []ContactOpportunitySummary
	FollowUps             []ContactFollowUpSummary
}
type ContactPage struct {
	Items      []ContactSummary
	NextCursor string
}
type CreateContactCommand struct {
	TenantID, CorpID, ActorID int64
	Name, Phone               string
}
type ContactCreator interface {
	CreateContact(context.Context, CreateContactCommand) (ContactSummary, error)
}
type ListContactsFilter struct {
	TenantID int64
	CorpID   int64
	Keyword  string
	OwnerIDs []int64
	TagIDs   []string
	Statuses []string
	Cursor   string
	Limit    int
}

type AssignmentPage struct {
	Items      []domain.CustomerAssignment
	NextCursor string
}

type ListPublicPoolFilter struct {
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
	Limit            int
}

type ClaimPublicPoolCommand struct {
	TenantID       int64
	CorpID         int64
	ContactID      string
	UserID         int64
	Version        int64
	IdempotencyKey string
}

type MoveToPublicPoolCommand struct {
	TenantID       int64
	CorpID         int64
	ContactID      string
	ActorID        int64
	Version        int64
	Action         string
	Reason         string
	IdempotencyKey string
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
	ListContacts(context.Context, ListContactsFilter) (ContactPage, error)
	GetContact(context.Context, int64, int64, string) (ContactDetail, error)
	ListPublicPool(context.Context, ListPublicPoolFilter) (AssignmentPage, error)
	UpdateAssignment(context.Context, UpdateAssignmentCommand) (domain.CustomerAssignment, error)
	MoveToPublicPool(context.Context, MoveToPublicPoolCommand) (domain.CustomerAssignment, error)
	ClaimFromPublicPool(context.Context, ClaimPublicPoolCommand) (domain.CustomerAssignment, error)
}
