package store

import (
	"context"
	"errors"
	"testing"

	"jiyi/mochat-go/internal/dashboardadmin"
	"jiyi/mochat-go/internal/wecomcredentials"
)

func TestLegacyWeComIntegrationStoreLifecycleFailsClosedWithoutDatabaseAccess(t *testing.T) {
	store := &MySQLStore{}
	actor := dashboardadmin.Actor{UserID: 700, Active: true}
	assertImmutable := func(name string, err error) {
		t.Helper()
		if !errors.Is(err, dashboardadmin.ErrWeComModeImmutable) {
			t.Fatalf("%s err=%v", name, err)
		}
	}
	_, err := store.SaveWeComIntegrationCandidate(context.Background(), actor, 41, dashboardadmin.WeComIntegrationCandidateInput{})
	assertImmutable("save candidate", err)
	_, err = store.WeComIntegrationVerificationCandidate(context.Background(), actor, 41, 1)
	assertImmutable("load verification candidate", err)
	_, err = store.CompleteWeComIntegrationVerification(context.Background(), actor, 41, 1, dashboardadmin.WeComVerificationResult{}, "")
	assertImmutable("complete verification", err)
	_, err = store.SwitchWeComIntegration(context.Background(), actor, 41, 1)
	assertImmutable("switch", err)
	_, err = store.RollbackWeComIntegration(context.Background(), actor, 41, 1)
	assertImmutable("rollback", err)
}

func TestWeComIntegrationCredentialHintContainsNamesNotValues(t *testing.T) {
	hint := weComCredentialHint(wecomcredentials.AuthorizationCredential{EmployeeSecret: "employee-secret-value", PermanentCode: "permanent-secret-value"})
	if hint != "employee,permanent_code" {
		t.Fatalf("hint=%q", hint)
	}
}
