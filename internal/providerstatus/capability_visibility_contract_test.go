package providerstatus

import (
	"context"
	"testing"

	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/modules/providers"
)

func TestServiceFiltersCapabilitiesByDashboardPermission(t *testing.T) {
	source := &statusTestSource{statuses: []providers.Status{{
		Kind: "wecom_standard", State: providers.StateLimited, Source: providers.SourceExternal,
		Capabilities: []string{"employee_sync", "contact_batch_send", "room_batch_send", "callback"},
		CapabilityStatuses: []providers.CapabilityStatus{
			{Capability: "contact_batch_send", State: providers.StateReady, Code: "wecom.capability_ready"},
			{Capability: "room_batch_send", State: providers.StateReady, Code: "wecom.capability_ready"},
		},
	}}}
	ctx := dashboardprincipal.WithCapabilityAccess(context.Background(), false, []string{"dashboard.acquisition.precise_group_send"})
	view, err := NewService(source).Resolve(ctx, dashboardprincipal.DashboardPrincipal{UserID: 1, TenantID: 2, CorpID: 3, AuthVersion: 1, CorpStatus: dashboardprincipal.CorpBindingStatusActive})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Providers[0].CapabilityStatuses) != 2 || view.Providers[0].CapabilityStatuses[0].Capability != "contact_batch_send" || view.Providers[0].CapabilityStatuses[1].Capability != "room_batch_send" {
		t.Fatalf("visible capability statuses=%#v", view.Providers[0].CapabilityStatuses)
	}
	want := []string{"contact_batch_send", "room_batch_send"}
	if len(view.Providers[0].Capabilities) != len(want) {
		t.Fatalf("visible parent capabilities=%#v, want %#v", view.Providers[0].Capabilities, want)
	}
	for index, capability := range want {
		if view.Providers[0].Capabilities[index] != capability {
			t.Fatalf("visible parent capabilities=%#v, want %#v", view.Providers[0].Capabilities, want)
		}
	}

	ctx = dashboardprincipal.WithCapabilityAccess(context.Background(), false, []string{"/dashboard/contact/batch"})
	view, err = NewService(source).Resolve(ctx, dashboardprincipal.DashboardPrincipal{UserID: 1, TenantID: 2, CorpID: 3, AuthVersion: 1, CorpStatus: dashboardprincipal.CorpBindingStatusActive})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Providers) != 0 {
		t.Fatalf("API path was treated as page permission: providers=%#v", view.Providers)
	}
}
