package store

import (
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

func TestCompanyActorAuthorizationRequiresCurrentAuthVersion(t *testing.T) {
	principal := dashboardprincipal.DashboardPrincipal{
		UserID: 101, TenantID: 202, CorpID: 303, AuthVersion: 7,
		IsSuperAdmin: true,
	}
	base := companyActorFacts{
		UserStatus: 1, IsSuperAdmin: 1, IdentityStatus: 1,
		TenantID: 202, AuthVersion: 7, Activated: true,
	}
	if !companyActorFactsAllowed(base, principal, false) {
		t.Fatal("matching active actor facts were rejected")
	}
	base.AuthVersion = 8
	if companyActorFactsAllowed(base, principal, false) {
		t.Fatal("stale auth_version was accepted")
	}
}

func TestCompanyActorAuthorizationAcceptsExactGrantForActiveOrdinaryActor(t *testing.T) {
	principal := dashboardprincipal.DashboardPrincipal{UserID: 101, TenantID: 202, CorpID: 303, AuthVersion: 7}
	facts := companyActorFacts{UserStatus: 1, IdentityStatus: 1, TenantID: 202, AuthVersion: 7, Activated: true}
	if !companyActorFactsAllowed(facts, principal, true) {
		t.Fatal("ordinary actor with exact company permission was rejected")
	}
	if companyActorFactsAllowed(facts, principal, false) {
		t.Fatal("ordinary actor without company permission was accepted")
	}
	facts.IdentityStatus = 2
	if companyActorFactsAllowed(facts, principal, true) {
		t.Fatal("disabled ordinary identity was accepted")
	}
}

func TestCompanyActorQueryLocksOnlyMutationTransactions(t *testing.T) {
	if !strings.Contains(companyActorQuery(true), "FOR UPDATE") {
		t.Fatal("mutation actor query must lock the identity row")
	}
	if strings.Contains(companyActorQuery(false), "FOR UPDATE") {
		t.Fatal("read actor query must not take a mutation lock")
	}
}
