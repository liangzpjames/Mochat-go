package companyprofile

import (
	"context"
	"errors"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/modules/providers"
	"jiyi/mochat-go/internal/modules/providers/catalog"
	"jiyi/mochat-go/internal/providerstatus"
	"jiyi/mochat-go/internal/wecomcapability"
)

type providerStatusTestProvider struct{ status providers.Status }

func (p providerStatusTestProvider) Status() providers.Status { return p.status }

func TestApplyCapabilitySyncStatusRejectsStaleLifecycleMarkers(t *testing.T) {
	finishedAt := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name   string
		status string
	}{
		{name: "queued", status: "queued"},
		{name: "running", status: "running"},
		{name: "failed", status: "failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, version := range []uint64{0, 2} {
				status := providers.CapabilityStatus{State: providers.StateLimited}
				applyCapabilitySyncStatus(&status, SyncStatus{
					Status:            test.status,
					CredentialVersion: version,
					StartedAt:         &finishedAt,
					FinishedAt:        &finishedAt,
					ErrorCode:         "wecom.sync_http_500",
				}, 1)
				if status.State != providers.StateLimited || status.Code != "wecom.sync_stale" || status.LastErrorCode != "wecom.credential_version_stale" {
					t.Fatalf("version=%d status=%#v, want limited stale", version, status)
				}
				if status.Reason == "" || status.Action == "" {
					t.Fatalf("version=%d status=%#v, want actionable stale state", version, status)
				}
			}
		})
	}

	current := providers.CapabilityStatus{State: providers.StateLimited}
	applyCapabilitySyncStatus(&current, SyncStatus{Status: "syncing", CredentialVersion: 1, StartedAt: &finishedAt}, 1)
	if current.Code != "wecom.capability_syncing" || current.State != providers.StateLimited {
		t.Fatalf("current running status=%#v, want syncing", current)
	}
}

func TestApplyCapabilityOperationProjectsCancelledAsLimited(t *testing.T) {
	finishedAt := time.Date(2026, 8, 15, 11, 0, 0, 0, time.UTC)
	status := providers.CapabilityStatus{State: providers.StateLimited}
	applyCapabilityOperation(&status, wecomcapability.Operation{
		Status:     wecomcapability.OperationCancelled,
		FinishedAt: &finishedAt,
		ErrorCode:  "wecom.capability_operation_cancelled",
	})
	if status.State != providers.StateLimited || status.Code != "wecom.capability_operation_cancelled" {
		t.Fatalf("cancelled status=%#v, want limited cancelled", status)
	}
	if status.LastFailureAt == nil || !status.LastFailureAt.Equal(finishedAt) || status.LastErrorCode != "wecom.capability_operation_cancelled" || status.Action == "" {
		t.Fatalf("cancelled status=%#v, want stable failure evidence and action", status)
	}
}

func TestProviderStatusSourceKeepsEmployeeSyncReadyWhenArchiveProviderIsLimited(t *testing.T) {
	verifiedAt := time.Date(2026, 8, 14, 8, 0, 0, 0, time.UTC)
	syncFinishedAt := verifiedAt.Add(time.Hour)
	store := &companyProfileContractStore{profile: Profile{
		TenantID: 202, CorpID: 303, BindingStatus: "active", WXCorpID: "ww-authoritative", VerifiedAt: &verifiedAt,
		BindingVersion:        1,
		CredentialGenerations: CredentialGenerationSet{Employee: 1, Contact: 1, Agent: 1, Callback: 1},
		Credentials: CredentialStatuses{
			WeCom:   CredentialStatus{Configured: true, EmployeeConfigured: true},
			Archive: CredentialStatus{Configured: true},
		},
	}, syncStatus: SyncStatus{Status: "completed", CredentialVersion: 1, FinishedAt: &syncFinishedAt}}
	registry, err := catalog.NewRegistry(catalog.Dependencies{
		Archive:       providerStatusTestProvider{status: providers.Status{Kind: "wecom_archive", State: providers.StateReady, Code: "archive.runtime"}},
		AudioStorage:  providerStatusTestProvider{status: providers.Status{Kind: "audio_storage", State: providers.StateReady, Code: "audio.runtime"}},
		AI:            providerStatusTestProvider{status: providers.Status{Kind: "ai", State: providers.StateLimited, Code: "ai.key_missing"}},
		WeComStandard: providerStatusTestProvider{status: providers.Status{Kind: "wecom_standard", State: providers.StateReady, Code: "wecom.runtime"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	source := NewProviderStatusSource(store, registry)
	view, err := providerstatus.NewService(source).Resolve(context.Background(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive))
	if err != nil {
		t.Fatal(err)
	}
	if store.getCalls != 1 {
		t.Fatalf("profile reads = %d, want 1", store.getCalls)
	}
	if store.syncStatusCalls != 1 {
		t.Fatalf("active verified profile sync reads = %d, want 1", store.syncStatusCalls)
	}
	byKind := make(map[string]providerstatus.ProviderStatus, len(view.Providers))
	for _, status := range view.Providers {
		byKind[status.Kind] = status
	}
	if byKind["wecom_standard"].State != providers.StateReady || byKind["wecom_standard"].Code != "wecom.capability_ready" || byKind["wecom_standard"].LastSuccessAt == nil || !byKind["wecom_standard"].LastSuccessAt.Equal(syncFinishedAt) {
		t.Fatalf("standard status = %#v", byKind["wecom_standard"])
	}
	if byKind["wecom_archive"].State != providers.StateLimited || byKind["wecom_archive"].Code != "archive.getchatdata_unimplemented" {
		t.Fatalf("archive status = %#v", byKind["wecom_archive"])
	}
}

func TestProviderStatusSourceRejectsExternalArchiveReadyFromOptionalStore(t *testing.T) {
	verifiedAt := time.Date(2026, 8, 14, 8, 0, 0, 0, time.UTC)
	store := &companyProfileContractStore{
		profile: Profile{TenantID: 202, CorpID: 303, BindingStatus: "active", WXCorpID: "ww-authoritative", VerifiedAt: &verifiedAt,
			Credentials: CredentialStatuses{WeCom: CredentialStatus{Configured: true, EmployeeConfigured: true}, Archive: CredentialStatus{Configured: true}}},
		archiveSourceStatus: providers.Status{Kind: "wecom_archive", Source: providers.SourceExternal, State: providers.StateReady, Code: "archive.runtime_verified"},
	}
	registry, err := catalog.NewRegistry(catalog.Dependencies{
		Archive: providerStatusTestProvider{status: providers.Status{Kind: "wecom_archive", State: providers.StateReady, Source: providers.SourceExternal}},
	})
	if err != nil {
		t.Fatal(err)
	}
	statuses, err := NewProviderStatusSource(store, registry).Statuses(context.Background(), companyProfileTestPrincipal(false, dashboardprincipal.CorpBindingStatusActive))
	if err != nil {
		t.Fatal(err)
	}
	archiveStatus := findStatus(statuses, "wecom_archive")
	if archiveStatus.Source != providers.SourceExternal || archiveStatus.State != providers.StateLimited || archiveStatus.Code != "archive.getchatdata_unimplemented" {
		t.Fatalf("archive status=%#v", archiveStatus)
	}
}

func TestProviderStatusSourceRejectsArchiveRuntimeWithWrongKindOrEmptySource(t *testing.T) {
	fallback := providers.Status{
		Kind: "wecom_archive", Source: providers.SourceExternal, State: providers.StateLimited,
		Code: "archive.getchatdata_unimplemented",
	}
	for _, runtime := range []providers.Status{
		{Kind: "other_archive", Source: providers.SourceSimulated, State: providers.StateReady, Code: "archive.simulation_ready"},
		{Kind: "wecom_archive", State: providers.StateReady, Code: "archive.runtime_verified"},
	} {
		got := mergeArchiveRuntimeStatus(fallback, runtime)
		if got.Kind != "wecom_archive" || got.Source != providers.SourceExternal || got.State != providers.StateLimited || got.Code != "archive.getchatdata_unimplemented" {
			t.Fatalf("runtime=%#v merged=%#v", runtime, got)
		}
	}
}

func TestProviderStatusSourceDoesNotReadSyncStatusForPendingBinding(t *testing.T) {
	store := &companyProfileContractStore{
		profile: Profile{
			TenantID: 202, CorpID: 303, BindingStatus: "pending", WXCorpID: "ww-candidate",
			Credentials: CredentialStatuses{WeCom: CredentialStatus{Configured: true, EmployeeConfigured: true}},
		},
		syncStatusErr: errors.New("pending binding must not read sync status"),
	}
	registry, err := catalog.NewRegistry(catalog.Dependencies{
		WeComStandard: providerStatusTestProvider{status: providers.Status{Kind: "wecom_standard", State: providers.StateReady, Code: "wecom.runtime"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	statuses, err := NewProviderStatusSource(store, registry).Statuses(context.Background(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusPending))
	if err != nil {
		t.Fatalf("pending provider status error=%v, want limited status without sync read", err)
	}
	if store.syncStatusCalls != 0 {
		t.Fatalf("pending binding sync reads=%d, want 0", store.syncStatusCalls)
	}
	if store.archiveStatusCalls != 0 {
		t.Fatalf("pending binding archive source reads=%d, want 0", store.archiveStatusCalls)
	}
	standard := findStatus(statuses, "wecom_standard")
	if standard.State != providers.StateLimited || standard.Code != "wecom.runtime_unverified" {
		t.Fatalf("pending standard status=%#v, want limited runtime_unverified", standard)
	}
}

func TestProviderStatusSourceUsesTenantScopedSimulationSourceStatus(t *testing.T) {
	verifiedAt := time.Date(2026, 8, 14, 8, 0, 0, 0, time.UTC)
	lastSuccess := verifiedAt.Add(time.Hour)
	lastFailure := lastSuccess.Add(time.Hour)
	store := &companyProfileContractStore{
		profile: Profile{TenantID: 202, CorpID: 303, BindingStatus: "active", WXCorpID: "ww-authoritative", VerifiedAt: &verifiedAt,
			Credentials: CredentialStatuses{WeCom: CredentialStatus{Configured: true, EmployeeConfigured: true}, Archive: CredentialStatus{Configured: true}}},
		archiveSourceStatus: providers.Status{Kind: "wecom_archive", Source: providers.SourceSimulated, State: providers.StateLimited,
			Code: "archive.simulation_ready", LastSyncAt: &lastFailure, LastSuccessAt: &lastSuccess, LastFailureAt: &lastFailure,
			LastErrorCode: "archive.sync_failed", Action: "仅用于验收"},
	}
	registry, err := catalog.NewRegistry(catalog.Dependencies{
		Archive:       providerStatusTestProvider{status: providers.Status{Kind: "wecom_archive", State: providers.StateReady, Source: providers.SourceExternal}},
		WeComStandard: providerStatusTestProvider{status: providers.Status{Kind: "wecom_standard", State: providers.StateLimited}},
	})
	if err != nil {
		t.Fatal(err)
	}
	statuses, err := NewProviderStatusSource(store, registry).Statuses(context.Background(), companyProfileTestPrincipal(false, dashboardprincipal.CorpBindingStatusActive))
	if err != nil {
		t.Fatal(err)
	}
	archiveStatus := findStatus(statuses, "wecom_archive")
	if archiveStatus.Source != providers.SourceSimulated || archiveStatus.Code != "archive.simulation_ready" || archiveStatus.LastSuccessAt == nil || archiveStatus.LastFailureAt == nil || archiveStatus.LastErrorCode != "archive.sync_failed" {
		t.Fatalf("archive status = %#v", archiveStatus)
	}
	if archiveStatus.Source == providers.SourceExternal && archiveStatus.State == providers.StateReady {
		t.Fatal("simulation source was reported as external ready")
	}
	if store.archiveStatusCalls != 1 {
		t.Fatalf("active archive source reads=%d, want 1", store.archiveStatusCalls)
	}
}

func TestProviderStatusSourceKeepsStandardWeComLimitedWhenTenantCredentialIsMissing(t *testing.T) {
	store := &companyProfileContractStore{profile: Profile{TenantID: 202, CorpID: 303, BindingStatus: "active"}}
	registry, err := catalog.NewRegistry(catalog.Dependencies{WeComStandard: providerStatusTestProvider{status: providers.Status{State: providers.StateReady}}})
	if err != nil {
		t.Fatal(err)
	}
	source := NewProviderStatusSource(store, registry)
	statuses, err := source.Statuses(context.Background(), companyProfileTestPrincipal(false, dashboardprincipal.CorpBindingStatusActive))
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range statuses {
		if status.Kind == "wecom_standard" {
			if status.State != providers.StateLimited || status.Code != "wecom.credentials_missing" {
				t.Fatalf("standard status = %#v", status)
			}
			return
		}
	}
	t.Fatal("wecom_standard status missing")
}

func TestProviderStatusSourceFailsClosedWhenRuntimeAdapterIsUnavailable(t *testing.T) {
	verifiedAt := time.Date(2026, 8, 14, 8, 0, 0, 0, time.UTC)
	store := &companyProfileContractStore{profile: Profile{
		TenantID: 202, CorpID: 303, BindingStatus: "active", WXCorpID: "ww-authoritative", VerifiedAt: &verifiedAt,
		Credentials: CredentialStatuses{WeCom: CredentialStatus{Configured: true, EmployeeConfigured: true}, Archive: CredentialStatus{Configured: true}},
	}}
	registry, err := catalog.NewRegistry(catalog.Dependencies{
		WeComStandard: providerStatusTestProvider{status: providers.Status{State: providers.StateUnavailable, Code: "provider.runtime_component_missing"}},
		Archive:       providerStatusTestProvider{status: providers.Status{State: providers.StateUnavailable, Code: "provider.runtime_component_missing"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	statuses, err := NewProviderStatusSource(store, registry).Statuses(context.Background(), companyProfileTestPrincipal(false, dashboardprincipal.CorpBindingStatusActive))
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range statuses {
		if status.Kind == "wecom_standard" || status.Kind == "wecom_archive" {
			if status.State != providers.StateUnavailable || status.Code != "provider.runtime_component_missing" {
				t.Fatalf("%s status = %#v, want unavailable runtime status", status.Kind, status)
			}
		}
	}
}

func TestProviderStatusSourceDegradesWhenStandardSyncFailsAndRecoversOnSuccess(t *testing.T) {
	verifiedAt := time.Date(2026, 8, 14, 8, 0, 0, 0, time.UTC)
	failedAt := verifiedAt.Add(time.Hour)
	store := &companyProfileContractStore{profile: Profile{
		TenantID: 202, CorpID: 303, BindingStatus: "active", WXCorpID: "ww-authoritative", VerifiedAt: &verifiedAt,
		BindingVersion:        1,
		CredentialGenerations: CredentialGenerationSet{Employee: 1},
		Credentials:           CredentialStatuses{WeCom: CredentialStatus{Configured: true, EmployeeConfigured: true}},
	}, syncStatus: SyncStatus{Status: "failed", CredentialVersion: 1, FinishedAt: &failedAt, ErrorCode: "wecom.sync_http_500"}}
	registry, err := catalog.NewRegistry(catalog.Dependencies{
		WeComStandard: providerStatusTestProvider{status: providers.Status{Kind: "wecom_standard", State: providers.StateReady, Code: "wecom.runtime"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	source := NewProviderStatusSource(store, registry)
	statuses, err := source.Statuses(context.Background(), companyProfileTestPrincipal(false, dashboardprincipal.CorpBindingStatusActive))
	if err != nil {
		t.Fatal(err)
	}
	failed := findStatus(statuses, "wecom_standard")
	if failed.State != providers.StateLimited || failed.Code != "wecom.sync_failed" || failed.LastFailureAt == nil || failed.LastErrorCode != "wecom.sync_http_500" {
		t.Fatalf("failed sync status = %#v", failed)
	}

	succeededAt := failedAt.Add(time.Hour)
	store.syncStatus = SyncStatus{Status: "succeeded", CredentialVersion: 1, FinishedAt: &succeededAt}
	statuses, err = source.Statuses(context.Background(), companyProfileTestPrincipal(false, dashboardprincipal.CorpBindingStatusActive))
	if err != nil {
		t.Fatal(err)
	}
	succeeded := findStatus(statuses, "wecom_standard")
	if succeeded.State != providers.StateReady || succeeded.LastSuccessAt == nil || !succeeded.LastSuccessAt.Equal(succeededAt) {
		t.Fatalf("successful sync status = %#v", succeeded)
	}

	store.syncStatus = SyncStatus{Status: "failed", CredentialVersion: 0, FinishedAt: &failedAt, ErrorCode: "SYNC_FAILED"}
	statuses, err = source.Statuses(context.Background(), companyProfileTestPrincipal(false, dashboardprincipal.CorpBindingStatusActive))
	if err != nil {
		t.Fatal(err)
	}
	if fallback := findStatus(statuses, "wecom_standard"); fallback.Code != "wecom.sync_stale" || fallback.LastErrorCode != "wecom.credential_version_stale" {
		t.Fatalf("zero-version sync failure = %#v, want stale fail-closed status", fallback)
	}
}
