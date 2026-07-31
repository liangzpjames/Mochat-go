package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	LeadStatusQualified LeadStatus = "qualified"
	LeadStatusConverted LeadStatus = "converted"
	LeadStatusDiscarded LeadStatus = "discarded"

	AssignmentOwned         = "owned"
	AssignmentCollaborating = "collaborating"
	AssignmentPublicPool    = "public_pool"

	OpportunityStageProposal = "proposal"
	OpportunityStatusWon     = "won"
	OpportunityStatusLost    = "lost"
	OpportunityWon           = OpportunityStatusWon
	OpportunityLost          = OpportunityStatusLost
)

type FollowUp struct {
	ID, TenantID, CorpID string
	Content              string
}
type CustomerTag struct {
	ID, TenantID, CorpID string
	Name                 string
	Version              int64
}

type CustomerAssignment struct {
	ID              string
	TenantID        int64
	CorpID          int64
	ContactID       string
	OwnerID         *int64
	CollaboratorIDs []int64
	Status          string
	Version         int64
	UpdatedAt       time.Time
}

func ValidateAssignmentStatus(status string) error {
	switch status {
	case AssignmentOwned, AssignmentCollaborating, AssignmentPublicPool:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrInvalidAssignmentStatus, status)
	}
}

func CanClaimAssignment(status string) bool {
	return status == AssignmentPublicPool
}

func ValidateAssignmentVersion(actual, expected int64) error {
	if actual != expected {
		return fmt.Errorf("%w: actual=%d expected=%d", ErrAssignmentVersionConflict, actual, expected)
	}
	return nil
}

func CanTransitionLead(from, to LeadStatus) bool {
	switch from {
	case LeadStatusNew:
		return to == LeadStatusQualified || to == LeadStatusDiscarded
	case LeadStatusQualified:
		return to == LeadStatusConverted || to == LeadStatusDiscarded
	default:
		return false
	}
}

func CanTransitionOpportunity(from, to string) bool {
	if to == OpportunityStatusLost || to == OpportunityStatusWon {
		return from != OpportunityStatusWon && from != OpportunityStatusLost
	}
	return from == OpportunityStageProposal
}

func ValidateOpportunityTransition(from, to, reason string) error {
	if !CanTransitionOpportunity(from, to) {
		return fmt.Errorf("invalid opportunity transition: %s -> %s", from, to)
	}
	if to == OpportunityStatusLost && reason == "" {
		return fmt.Errorf("lost opportunity requires a reason")
	}
	return nil
}

func ValidateOpportunityInput(amount float64, startDate, endDate string) error {
	if amount < 0 {
		return fmt.Errorf("opportunity amount must be non-negative")
	}
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return fmt.Errorf("invalid opportunity start date: %w", err)
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return fmt.Errorf("invalid opportunity end date: %w", err)
	}
	if end.Before(start) {
		return fmt.Errorf("opportunity end date must not precede start date")
	}
	return nil
}

func ValidateFollowUp(content string) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("follow-up content is required")
	}
	return nil
}

func ValidateFollowUpChronology(previous, next string) error {
	previousAt, err := time.Parse(time.RFC3339, previous)
	if err != nil {
		return fmt.Errorf("invalid previous follow-up time: %w", err)
	}
	nextAt, err := time.Parse(time.RFC3339, next)
	if err != nil {
		return fmt.Errorf("invalid next follow-up time: %w", err)
	}
	if nextAt.Before(previousAt) {
		return fmt.Errorf("follow-up timestamps must be chronological")
	}
	return nil
}

func ValidateTagName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("tag name is required")
	}
	return nil
}
