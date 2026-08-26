package store

import (
	"errors"
	"testing"

	"jiyi/mochat-go/internal/dashboardadmin"
	"jiyi/mochat-go/internal/wecomcredentials"
)

func TestWeComIntegrationGenerationIsStrictlyMonotonic(t *testing.T) {
	if got := nextWeComIntegrationGeneration(9, 12); got != 13 {
		t.Fatalf("generation=%d, want 13", got)
	}
	if got := nextWeComIntegrationGeneration(15, 2); got != 16 {
		t.Fatalf("generation=%d, want 16", got)
	}
}

func TestWeComIntegrationSwapFailsClosedBeforeGenerationChange(t *testing.T) {
	binding := weComBinding{CorpID: 63, WXCorpID: "ww-authoritative"}
	current := dashboardadmin.WeComIntegration{ID: "current", Slot: "current", Status: "active", Generation: 9, Version: 5}
	verified := dashboardadmin.WeComIntegration{ID: "candidate", Slot: "candidate", Status: "active", VerifiedWXCorpID: "ww-authoritative", VerifiedAt: "2026-08-27T00:00:00Z", Generation: 12, Version: 3}
	tests := []struct {
		name       string
		candidate  dashboardadmin.WeComIntegration
		version    uint64
		leases     int
		decryptErr error
		want       error
	}{
		{"version conflict", verified, 4, 0, nil, dashboardadmin.ErrVersionConflict},
		{"not verified", func() dashboardadmin.WeComIntegration { v := verified; v.Status = "failed"; return v }(), 5, 0, nil, dashboardadmin.ErrWeComCandidateNotVerified},
		{"corp mismatch", func() dashboardadmin.WeComIntegration { v := verified; v.VerifiedWXCorpID = "ww-other"; return v }(), 5, 0, nil, dashboardadmin.ErrWeComCorpMismatch},
		{"missing scope", func() dashboardadmin.WeComIntegration {
			v := verified
			v.MissingCapabilities = []string{"archive.read"}
			return v
		}(), 5, 0, nil, dashboardadmin.ErrWeComMissingCapabilities},
		{"decrypt failure", verified, 5, 0, errors.New("cipher detail must not escape"), dashboardadmin.ErrWeComCredentialDecrypt},
		{"active lease", verified, 5, 1, nil, dashboardadmin.ErrWeComActiveMediaLease},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			generation, err := validateWeComIntegrationSwap(binding, current, tc.candidate, tc.version, tc.leases, tc.decryptErr)
			if !errors.Is(err, tc.want) || generation != 0 {
				t.Fatalf("generation=%d err=%v want=%v", generation, err, tc.want)
			}
		})
	}
	generation, err := validateWeComIntegrationSwap(binding, current, verified, 5, 0, nil)
	if err != nil || generation != 13 {
		t.Fatalf("generation=%d err=%v", generation, err)
	}
}

func TestWeComIntegrationCredentialHintContainsNamesNotValues(t *testing.T) {
	hint := weComCredentialHint(wecomcredentials.AuthorizationCredential{EmployeeSecret: "employee-secret-value", PermanentCode: "permanent-secret-value"})
	if hint != "employee,permanent_code" {
		t.Fatalf("hint=%q", hint)
	}
}
