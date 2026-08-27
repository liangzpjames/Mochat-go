package store

import (
	"errors"
	"testing"

	"jiyi/mochat-go/internal/dashboardadmin"
)

func TestValidateWeComBindingModeUsesBindingAsAuthority(t *testing.T) {
	for _, tc := range []struct {
		name    string
		binding weComBinding
		current *dashboardadmin.WeComIntegration
		ok      bool
	}{
		{"delegated match", weComBinding{Mode: dashboardadmin.WeComIntegrationModeThirdPartyDelegated}, &dashboardadmin.WeComIntegration{Mode: dashboardadmin.WeComIntegrationModeThirdPartyDelegated}, true},
		{"self built match", weComBinding{Mode: dashboardadmin.WeComIntegrationModeSelfBuilt}, &dashboardadmin.WeComIntegration{Mode: dashboardadmin.WeComIntegrationModeSelfBuilt}, true},
		{"row mismatch", weComBinding{Mode: dashboardadmin.WeComIntegrationModeSelfBuilt}, &dashboardadmin.WeComIntegration{Mode: dashboardadmin.WeComIntegrationModeThirdPartyDelegated}, false},
		{"blank binding", weComBinding{}, &dashboardadmin.WeComIntegration{Mode: dashboardadmin.WeComIntegrationModeSelfBuilt}, false},
		{"missing current", weComBinding{Mode: dashboardadmin.WeComIntegrationModeSelfBuilt}, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateWeComBindingMode(tc.binding, tc.current)
			if tc.ok && err != nil {
				t.Fatalf("err=%v", err)
			}
			if !tc.ok && !errors.Is(err, dashboardadmin.ErrWeComModeImmutable) {
				t.Fatalf("err=%v, want immutable", err)
			}
		})
	}
}
