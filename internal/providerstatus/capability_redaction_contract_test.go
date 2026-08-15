package providerstatus

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/modules/providers"
)

func TestServiceRedactsCapabilitySecretSentinelForOrdinaryAndSuperadmin(t *testing.T) {
	for _, superadmin := range []bool{false, true} {
		source := &statusTestSource{statuses: []providers.Status{{
			Kind: "wecom_standard", State: providers.StateLimited, Source: providers.SourceExternal,
			CapabilityStatuses: []providers.CapabilityStatus{{
				Capability: "contact_batch_send", State: providers.StateLimited,
				Code: "wecom.capability_operation_failed", Reason: "secret=plaintext-secret-sentinel",
				Action: "token=plaintext-secret-sentinel", Missing: []string{"private_key=plaintext-secret-sentinel"},
				LastErrorCode: "wecom.capability_operation_failed",
			}},
		}}}
		ctx := context.Background()
		if !superadmin {
			ctx = dashboardprincipal.WithCapabilityAccess(ctx, false, []string{"dashboard.acquisition.precise_group_send"})
		}
		view, err := NewService(source).Resolve(ctx, dashboardprincipal.DashboardPrincipal{
			UserID: 1, TenantID: 7, CorpID: 11, CorpStatus: dashboardprincipal.CorpBindingStatusActive,
			IsSuperAdmin: superadmin, AuthVersion: 3,
		})
		if err != nil {
			t.Fatal(err)
		}
		payload, err := json.Marshal(view)
		if err != nil {
			t.Fatal(err)
		}
		encoded := string(payload)
		if strings.Contains(encoded, "plaintext-secret-sentinel") {
			t.Fatalf("superadmin=%v leaked sentinel: %s", superadmin, encoded)
		}
	}
}
