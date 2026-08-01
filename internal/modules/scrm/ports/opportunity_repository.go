package ports

import (
	"context"
	"errors"
	"time"
)

var (
	ErrOpportunityNotFound = errors.New("opportunity not found")
	ErrTagNotFound         = errors.New("tag not found")
)

type Opportunity struct {
	ID, ContactID, Stage, Status, LostReason string
	TenantID, CorpID, OwnerID                int64
	Amount                                   float64
	StartDate, EndDate                       time.Time
	Version                                  int64
}

type OpportunityFilter struct {
	TenantID, CorpID int64
	Stage            string
	OwnerID          *int64
}

type CreateOpportunityCommand struct {
	TenantID, CorpID, OwnerID int64
	ContactID, Stage          string
	Amount                    float64
	StartDate, EndDate        string
	IdempotencyKey            string
}

type ChangeOpportunityStageCommand struct {
	TenantID, CorpID int64
	OpportunityID    string
	ToStage, Reason  string
	Version          int64
	IdempotencyKey   string
}

type FollowUpRecord struct {
	ID, ContactID, Content string
	TenantID, CorpID       int64
	CreatedBy              int64
	CreatedAt              time.Time
}

type AppendFollowUpCommand struct {
	TenantID, CorpID, CreatedBy int64
	ContactID, Content          string
	IdempotencyKey              string
}

type Tag struct {
	ID, Name         string
	TenantID, CorpID int64
	Version          int64
}

type TagRepository interface {
	ListTags(context.Context, int64, int64) ([]Tag, error)
	CreateTag(context.Context, int64, int64, string, string) (Tag, error)
	RenameTag(context.Context, int64, int64, string, string, int64, string) (Tag, error)
	BindTags(context.Context, int64, int64, string, []string, string) error
}

type OpportunityRepository interface {
	ListOpportunities(context.Context, OpportunityFilter) ([]Opportunity, error)
	CreateOpportunity(context.Context, CreateOpportunityCommand) (Opportunity, error)
	ChangeOpportunityStage(context.Context, ChangeOpportunityStageCommand) (Opportunity, error)
	ListFollowUps(context.Context, int64, int64, string) ([]FollowUpRecord, error)
	AppendFollowUp(context.Context, AppendFollowUpCommand) (FollowUpRecord, error)
}
