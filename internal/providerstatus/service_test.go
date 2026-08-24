package providerstatus

import (
	"context"
	"errors"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/modules/providers"
)

type statusTestSource struct {
	statuses []providers.Status
	seen     dashboardprincipal.DashboardPrincipal
	err      error
}

func (s *statusTestSource) Statuses(_ context.Context, principal dashboardprincipal.DashboardPrincipal) ([]providers.Status, error) {
	s.seen = principal
	return s.statuses, s.err
}

func statusTestPrincipal(superadmin bool) dashboardprincipal.DashboardPrincipal {
	return dashboardprincipal.DashboardPrincipal{
		UserID: 1, TenantID: 7, CorpID: 11, CorpStatus: dashboardprincipal.CorpBindingStatusActive,
		IsSuperAdmin: superadmin, AuthVersion: 3,
	}
}

func TestServiceProjectsRuntimeStatusWithoutConfigurationNamesForOrdinaryUsers(t *testing.T) {
	source := &statusTestSource{statuses: []providers.Status{{
		Kind: "wecom_standard", State: providers.StateLimited, Code: "wecom.credentials_missing",
		Source: providers.SourceExternal, Reason: "MOCHAT_SECRET is missing", Action: "set MOCHAT_SECRET",
		Missing: []string{"MOCHAT_SECRET"}, Capabilities: []string{"contact_batch_send"}, LastErrorCode: "secret=do-not-return",
	}}}
	service := NewService(source)
	ctx := dashboardprincipal.WithCapabilityAccess(context.Background(), false, []string{"dashboard.acquisition.precise_group_send"})
	view, err := service.Resolve(ctx, statusTestPrincipal(false))
	if err != nil {
		t.Fatal(err)
	}
	if source.seen.TenantID != 7 || source.seen.CorpID != 11 || source.seen.UserID != 1 {
		t.Fatalf("source received wrong principal: %#v", source.seen)
	}
	if len(view.Providers) != 1 {
		t.Fatalf("providers = %#v", view.Providers)
	}
	status := view.Providers[0]
	if status.Code != "wecom.credentials_missing" || status.LastErrorCode != "" || status.Action != "请联系管理员配置或验证 Provider" || status.Reason != "" || len(status.Missing) != 0 {
		t.Fatalf("ordinary status leaked diagnostics: %#v", status)
	}
}

func TestServiceProjectsRedactedCompanyProviderStatusesForWebsitePermission(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 30, 0, 0, time.UTC)
	source := &statusTestSource{statuses: []providers.Status{
		{
			Kind: "wecom_standard", State: providers.StateLimited, Code: "wecom.credentials_missing",
			Source: providers.SourceExternal, Reason: "MOCHAT_WECOM_SECRET is missing", Missing: []string{"MOCHAT_WECOM_SECRET"},
			Capabilities: []string{"employee_sync"}, LastSuccessAt: &now,
		},
		{
			Kind: "wecom_archive", State: providers.StateLimited, Code: "archive.credentials_missing",
			Source: providers.SourceExternal, Reason: "MOCHAT_ARCHIVE_SECRET is missing", Missing: []string{"MOCHAT_ARCHIVE_SECRET"},
			Capabilities: []string{"archive_sync"}, LastFailureAt: &now,
		},
		{
			Kind: "ai", State: providers.StateLimited, Code: "ai.key_missing", Source: providers.SourceExternal,
			Capabilities: []string{"chat"},
		},
	}}
	ctx := dashboardprincipal.WithCapabilityAccess(context.Background(), false, []string{"dashboard.company_setting.website"})
	view, err := NewService(source).Resolve(ctx, statusTestPrincipal(false))
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Providers) != 2 {
		t.Fatalf("website provider statuses = %#v, want redacted WeCom and archive statuses only", view.Providers)
	}
	for _, status := range view.Providers {
		if status.Reason != "" || len(status.Missing) != 0 || status.Action != "请联系管理员配置或验证 Provider" {
			t.Fatalf("website status leaked diagnostics: %#v", status)
		}
	}
	if view.Providers[0].LastSuccessAt == nil || !view.Providers[0].LastSuccessAt.Equal(now) {
		t.Fatalf("standard provider timestamp = %#v, want %s", view.Providers[0].LastSuccessAt, now)
	}
	if view.Providers[1].LastFailureAt == nil || !view.Providers[1].LastFailureAt.Equal(now) {
		t.Fatalf("archive provider timestamp = %#v, want %s", view.Providers[1].LastFailureAt, now)
	}
}

func TestServiceRedactsUntrustedMachineCodesForOrdinaryUsers(t *testing.T) {
	source := &statusTestSource{statuses: []providers.Status{{
		Kind: "wecom_standard", State: providers.StateLimited, Code: "MOCHAT_SECRET", Source: providers.SourceExternal,
		Capabilities:  []string{"employee_sync"},
		LastErrorCode: "WECOM_API_ERROR_40001",
	}}}
	ctx := dashboardprincipal.WithCapabilityAccess(context.Background(), false, []string{"dashboard.index"})
	view, err := NewService(source).Resolve(ctx, statusTestPrincipal(false))
	if err != nil {
		t.Fatal(err)
	}
	status := view.Providers[0]
	if status.Code != "provider.status_unclassified" || status.LastErrorCode != "WECOM_API_ERROR_40001" {
		t.Fatalf("machine code projection = %#v", status)
	}
}

func TestServiceKeepsDiagnosticsForSuperadmin(t *testing.T) {
	source := &statusTestSource{statuses: []providers.Status{{
		Kind: "wecom_archive", State: providers.StateLimited, Code: "archive.credentials_missing",
		Source: providers.SourceExternal, Reason: "missing archive credential", Action: "configure archive",
		Missing: []string{"archive_secret"}, Capabilities: []string{"archive_sync"},
	}}}
	view, err := NewService(source).Resolve(context.Background(), statusTestPrincipal(true))
	if err != nil {
		t.Fatal(err)
	}
	status := view.Providers[0]
	if status.Reason != "会话存档凭据尚未配置" || status.Action != "配置会话存档凭据" || len(status.Missing) != 1 || status.Missing[0] != "archive_secret" {
		t.Fatalf("superadmin diagnostics were removed: %#v", status)
	}
}

func TestServiceMapsCancelledCapabilityCodeForSuperadmin(t *testing.T) {
	source := &statusTestSource{statuses: []providers.Status{{
		Kind: "wecom_standard", State: providers.StateLimited, Code: "wecom.capabilities_pending",
		Source: providers.SourceExternal, Capabilities: []string{"contact_batch_send"},
		CapabilityStatuses: []providers.CapabilityStatus{{
			Capability: "contact_batch_send", State: providers.StateLimited,
			Code: "wecom.capability_operation_cancelled", Source: providers.SourceExternal,
		}},
	}}}
	view, err := NewService(source).Resolve(context.Background(), statusTestPrincipal(true))
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Providers) != 1 || len(view.Providers[0].CapabilityStatuses) != 1 {
		t.Fatalf("cancelled capability projection=%#v", view.Providers)
	}
	item := view.Providers[0].CapabilityStatuses[0]
	if item.Code != "wecom.capability_operation_cancelled" || item.Reason == "" || item.Action == "" {
		t.Fatalf("cancelled capability diagnostics=%#v, want allowlisted reason/action", item)
	}
}

func TestServiceMapsCapabilityLifecycleDiagnostics(t *testing.T) {
	for _, test := range []struct {
		code  string
		state providers.State
	}{
		{code: "wecom.capability_ready", state: providers.StateReady},
		{code: "wecom.employee_sync_ready", state: providers.StateReady},
		{code: "wecom.capability_syncing", state: providers.StateLimited},
	} {
		source := &statusTestSource{statuses: []providers.Status{{
			Kind: "wecom_standard", State: test.state, Code: test.code,
			Source: providers.SourceExternal, Capabilities: []string{"employee_sync"},
		}}}
		view, err := NewService(source).Resolve(context.Background(), statusTestPrincipal(true))
		if err != nil {
			t.Fatal(err)
		}
		if len(view.Providers) != 1 || view.Providers[0].Reason == "" || view.Providers[0].Action == "" {
			t.Fatalf("code=%q projected=%#v, want lifecycle diagnostics", test.code, view.Providers)
		}
	}
}

func TestServiceRejectsSuspendedScopeAndPreservesSourceErrors(t *testing.T) {
	source := &statusTestSource{err: errors.New("runtime source unavailable")}
	service := NewService(source)
	principal := statusTestPrincipal(false)
	principal.CorpStatus = dashboardprincipal.CorpBindingStatusSuspended
	if _, err := service.Resolve(context.Background(), principal); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("suspended scope error = %v, want ErrScopeDenied", err)
	}
	if _, err := service.Resolve(context.Background(), statusTestPrincipal(false)); !errors.Is(err, ErrSourceUnavailable) {
		t.Fatalf("source error = %v, want ErrSourceUnavailable", err)
	}
}

func TestServiceUsesRuntimeFreshnessClock(t *testing.T) {
	now := time.Date(2026, 8, 14, 8, 0, 0, 0, time.UTC)
	view, err := NewService(&statusTestSource{}).WithClock(func() time.Time { return now }).Resolve(context.Background(), statusTestPrincipal(false))
	if err != nil {
		t.Fatal(err)
	}
	if !view.FreshAt.Equal(now) {
		t.Fatalf("freshAt = %s, want %s", view.FreshAt, now)
	}
}
