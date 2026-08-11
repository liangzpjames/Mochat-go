package store

import (
	"testing"

	"jiyi/mochat-go/internal/saasauth"
)

func TestSaaSMFAStatusTransitionActivatesEnrollment(t *testing.T) {
	tests := []struct {
		name        string
		current     int
		challenge   string
		wantNext    int
		wantAllowed bool
	}{
		{name: "enrollment activates pending credential", current: saasauth.SaaSMFAStatusPending, challenge: saasauth.SaaSMFAChallengeEnrollment, wantNext: saasauth.SaaSMFAStatusActive, wantAllowed: true},
		{name: "login challenge keeps active credential", current: saasauth.SaaSMFAStatusActive, challenge: saasauth.SaaSMFAChallengeLogin, wantNext: saasauth.SaaSMFAStatusActive, wantAllowed: true},
		{name: "enrollment cannot replace active credential", current: saasauth.SaaSMFAStatusActive, challenge: saasauth.SaaSMFAChallengeEnrollment, wantNext: 0, wantAllowed: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			next, allowed := saasMFAStatusTransition(test.current, test.challenge)
			if next != test.wantNext || allowed != test.wantAllowed {
				t.Fatalf("transition(%d, %q) = (%d, %t), want (%d, %t)", test.current, test.challenge, next, allowed, test.wantNext, test.wantAllowed)
			}
		})
	}
}
