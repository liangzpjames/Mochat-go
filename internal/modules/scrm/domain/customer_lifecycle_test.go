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
