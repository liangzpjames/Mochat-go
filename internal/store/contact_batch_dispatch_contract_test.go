package store

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcapability"
)

func TestContactBatchBusinessActorAllowsAuthorizedNonSuperadmin(t *testing.T) {
	facts := companyActorFacts{UserStatus: 1, IdentityStatus: 1, TenantID: 7, AuthVersion: 4, Activated: true}
	principal := dashboardprincipal.DashboardPrincipal{UserID: 21, TenantID: 7, CorpID: 9, AuthVersion: 4}
	if !businessActorFactsAllowed(facts, principal, true) {
		t.Fatal("an active authorized business actor must not require superadmin")
	}
	if businessActorFactsAllowed(companyActorFacts{UserStatus: 2, IdentityStatus: 1, TenantID: 7, AuthVersion: 4, Activated: true}, principal, true) {
		t.Fatal("inactive user must be rejected")
	}
	if businessActorFactsAllowed(facts, principal, false) {
		t.Fatal("missing active corp binding must be rejected")
	}
}

func TestContactBatchOwnedTargetMapDoesNotCrossSend(t *testing.T) {
	owned := buildContactBatchOwnedTargets([]int{101, 202}, []contactBatchOwnedTarget{
		{EmployeeID: 101, ExternalUserID: "external-a"},
		{EmployeeID: 202, ExternalUserID: "external-b"},
	})
	if got := owned[101]; len(got) != 1 || got[0] != "external-a" {
		t.Fatalf("employee A targets=%v, want only external-a", got)
	}
	if got := owned[202]; len(got) != 1 || got[0] != "external-b" {
		t.Fatalf("employee B targets=%v, want only external-b", got)
	}
	if totalContactBatchTargets(owned) != 2 {
		t.Fatalf("target total=%d, want 2", totalContactBatchTargets(owned))
	}
}

func TestContactBatchTargetPairsRetainEmployeeOwnershipBeforeExternalResolution(t *testing.T) {
	targets := buildContactBatchTargetPairs([]dashboard.ContactBatchTarget{
		{EmployeeID: 101, ContactID: 9001},
		{EmployeeID: 202, ContactID: 9002},
	})
	if got := targets[101]; len(got) != 1 || got[0].ContactID != 9001 {
		t.Fatalf("employee A targets=%v, want contact 9001 only", got)
	}
	if got := targets[202]; len(got) != 1 || got[0].ContactID != 9002 {
		t.Fatalf("employee B targets=%v, want contact 9002 only", got)
	}
	if targets[101][0].ExternalUserID != "" || targets[202][0].ExternalUserID != "" {
		t.Fatal("client target must not carry or infer an external user ID")
	}
}

func TestContactBatchResultStatusMapKeepsSharedContactPerEmployee(t *testing.T) {
	statuses := contactBatchResultStatusMap([]wecomcapability.OperationResult{
		{TargetKind: "employee_external_userid", TargetID: "101:external-shared", Status: wecomcapability.DispatchSucceeded},
		{TargetKind: "employee_external_userid", TargetID: "202:external-shared", Status: wecomcapability.DispatchFailed},
	})
	if statuses["101:external-shared"] != wecomcapability.DispatchSucceeded || statuses["202:external-shared"] != wecomcapability.DispatchFailed {
		t.Fatalf("shared contact results collapsed: %#v", statuses)
	}
}

func TestContactBatchAccessIdentityMustMatchAuthenticatedPrincipal(t *testing.T) {
	principal := dashboardprincipal.DashboardPrincipal{UserID: 21, TenantID: 7, CorpID: 9}
	valid := dashboard.DashboardAccessContext{UserID: 21, TenantID: 7, CorpID: 9}
	if !contactBatchAccessIdentityMatches(valid, principal) {
		t.Fatal("matching access identity should be accepted")
	}
	for name, access := range map[string]dashboard.DashboardAccessContext{
		"zero user":    {TenantID: 7, CorpID: 9},
		"wrong user":   {UserID: 22, TenantID: 7, CorpID: 9},
		"wrong tenant": {UserID: 21, TenantID: 8, CorpID: 9},
		"wrong corp":   {UserID: 21, TenantID: 7, CorpID: 10},
	} {
		if contactBatchAccessIdentityMatches(access, principal) {
			t.Fatalf("%s access identity must be rejected", name)
		}
	}
}

func TestContactBatchPermissionIsRecheckedInsideCreateTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}
	principal := dashboardprincipal.DashboardPrincipal{UserID: 21, TenantID: 7, CorpID: 9, AuthVersion: 4}
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s).*`+regexp.QuoteMeta("SELECT CASE WHEN EXISTS (")).
		WithArgs(principal.TenantID, principal.UserID, principal.TenantID, principal.UserID).
		WillReturnRows(sqlmock.NewRows([]string{"granted"}).AddRow(0))
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if store.contactBatchPagePermissionActiveTx(context.Background(), tx, principal) {
		t.Fatal("revoked page permission must fail closed")
	}
	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
