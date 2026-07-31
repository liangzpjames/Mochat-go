package domain

import "fmt"

const (
	LeadStatusQualified LeadStatus = "qualified"
	LeadStatusConverted LeadStatus = "converted"
	LeadStatusDiscarded LeadStatus = "discarded"

	AssignmentOwned      = "owned"
	AssignmentCollaborating = "collaborating"
	AssignmentPublicPool = "public_pool"

	OpportunityStageProposal = "proposal"
	OpportunityStatusWon     = "won"
	OpportunityStatusLost    = "lost"
	OpportunityWon           = OpportunityStatusWon
	OpportunityLost          = OpportunityStatusLost
)

type FollowUp struct { ID, TenantID, CorpID string; Content string }
type CustomerTag struct { ID, TenantID, CorpID string; Name string; Version int64 }

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
	if to == OpportunityStatusLost || to == OpportunityStatusWon { return from != OpportunityStatusWon && from != OpportunityStatusLost }
	return from == OpportunityStageProposal
}

func ValidateOpportunityTransition(from, to, reason string) error {
	if !CanTransitionOpportunity(from, to) { return fmt.Errorf("invalid opportunity transition: %s -> %s", from, to) }
	if to == OpportunityStatusLost && reason == "" { return fmt.Errorf("lost opportunity requires a reason") }
	return nil
}
