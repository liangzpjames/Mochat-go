package domain

import (
	"fmt"
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
