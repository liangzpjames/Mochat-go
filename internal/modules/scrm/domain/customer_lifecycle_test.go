package domain

import "testing"

func TestCustomerLifecycleTransitions(t *testing.T) {

	if !CanTransitionLead(LeadStatusNew, LeadStatusQualified) {
		t.Fatal("new lead should be qualified")
	}
	if CanTransitionLead(LeadStatusConverted, LeadStatusNew) {
		t.Fatal("converted lead should be terminal")
	}
	if !CanTransitionOpportunity(OpportunityStageProposal, OpportunityStatusLost) {
		t.Fatal("proposal should be losable")
	}
}

func TestLostOpportunityRequiresReason(t *testing.T) {
	if err := ValidateOpportunityTransition(OpportunityStatusWon, OpportunityStatusLost, ""); err == nil {
		t.Fatal("lost opportunity should require a reason")
	}
}

func TestAssignmentPublicPoolRules(t *testing.T) {
	if err := ValidateAssignmentStatus(AssignmentPublicPool); err != nil {
		t.Fatalf("public pool status rejected: %v", err)
	}
	if !CanClaimAssignment(AssignmentPublicPool) {
		t.Fatal("public pool assignment should be claimable")
	}
	if CanClaimAssignment(AssignmentOwned) || CanClaimAssignment(AssignmentCollaborating) {
		t.Fatal("owned/collaborating assignments must not be claimable")
	}
	if err := ValidateAssignmentStatus("unknown"); err == nil {
		t.Fatal("unknown assignment status should be rejected")
	}
}

func TestAssignmentVersionMustMatch(t *testing.T) {
	if err := ValidateAssignmentVersion(3, 3); err != nil {
		t.Fatalf("matching version rejected: %v", err)
	}
	if err := ValidateAssignmentVersion(3, 2); err == nil {
		t.Fatal("stale assignment version should be rejected")
	}
}
