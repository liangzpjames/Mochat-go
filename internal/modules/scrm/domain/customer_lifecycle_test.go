package domain

import "testing"

func TestOpportunityValidationCoversAmountDateAndTerminalRules(t *testing.T) {
	if err := ValidateOpportunityInput(-1, "2026-08-02", "2026-08-01"); err == nil {
		t.Fatal("negative amount and reversed dates should be rejected")
	}
	if err := ValidateOpportunityInput(100, "2026-08-02", "2026-08-01"); err == nil {
		t.Fatal("end date before start date should be rejected")
	}
	if err := ValidateOpportunityTransition(OpportunityStatusWon, OpportunityStageProposal, ""); err == nil {
		t.Fatal("won opportunity must be terminal")
	}
}

func TestFollowUpContentAndChronology(t *testing.T) {
	if err := ValidateFollowUp("  "); err == nil {
		t.Fatal("blank follow-up should be rejected")
	}
	if err := ValidateFollowUpChronology("2026-08-02T10:00:00Z", "2026-08-01T10:00:00Z"); err == nil {
		t.Fatal("follow-up timestamps must not move backwards")
	}
}

func TestTagNameValidation(t *testing.T) {
	if err := ValidateTagName("  "); err == nil {
		t.Fatal("blank tag name should be rejected")
	}
}

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
	if err := ValidateOpportunityTransition(OpportunityStageProposal, OpportunityStatusLost, ""); err == nil {
		t.Fatal("lost opportunity should require a reason")
	}
	if err := ValidateOpportunityTransition(OpportunityStatusLost, OpportunityStageProposal, ""); err == nil {
		t.Fatal("lost opportunity should be terminal")
	}
	if err := ValidateOpportunityTransition(OpportunityStageProposal, "negotiation", ""); err != nil {
		t.Fatalf("open opportunity should advance to another stage: %v", err)
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
