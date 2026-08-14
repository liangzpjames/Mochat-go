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
)

type providerStatusTestProvider struct{ status providers.Status }

func (p providerStatusTestProvider) Status() providers.Status { return p.status }

func TestProviderStatusSourceKeepsEmployeeSyncReadyWhenArchiveProviderIsLimited(t *testing.T) {
	verifiedAt := time.Date(2026, 8, 14, 8, 0, 0, 0, time.UTC)
	syncFinishedAt := verifiedAt.Add(time.Hour)
	store := &companyProfileContractStore{profile: Profile{
		TenantID: 202, CorpID: 303, BindingStatus: "active", WXCorpID: "ww-authoritative", VerifiedAt: &verifiedAt,
		Credentials: CredentialStatuses{
			WeCom:   CredentialStatus{Configured: true},
			Archive: CredentialStatus{Configured: true},
		},
	}, syncStatus: SyncStatus{Status: "completed", FinishedAt: &syncFinishedAt}}
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
	view, err := providerstatus.NewService(source).Resolve(context.Background(), companyProfileTestPrincipal(false, dashboardprincipal.CorpBindingStatusActive))
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
	if byKind["wecom_standard"].State != providers.StateReady || byKind["wecom_standard"].Code != "wecom.runtime_verified" || byKind["wecom_standard"].LastSuccessAt == nil || !byKind["wecom_standard"].LastSuccessAt.Equal(syncFinishedAt) {
		t.Fatalf("standard status = %#v", byKind["wecom_standard"])
	}
	if byKind["wecom_archive"].State != providers.StateLimited || byKind["wecom_archive"].Code != "archive.getchatdata_unimplemented" {
		t.Fatalf("archive status = %#v", byKind["wecom_archive"])
	}
}

func TestProviderStatusSourceDoesNotReadSyncStatusForPendingBinding(t *testing.T) {
	store := &companyProfileContractStore{
		profile: Profile{
			TenantID: 202, CorpID: 303, BindingStatus: "pending", WXCorpID: "ww-candidate",
			Credentials: CredentialStatuses{WeCom: CredentialStatus{Configured: true}},
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
			Credentials: CredentialStatuses{WeCom: CredentialStatus{Configured: true}, Archive: CredentialStatus{Configured: true}}},
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
		Credentials: CredentialStatuses{WeCom: CredentialStatus{Configured: true}, Archive: CredentialStatus{Configured: true}},
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
		Credentials: CredentialStatuses{WeCom: CredentialStatus{Configured: true}},
	}, syncStatus: SyncStatus{Status: "failed", FinishedAt: &failedAt, ErrorCode: "wecom.sync_http_500"}}
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
	store.syncStatus = SyncStatus{Status: "succeeded", FinishedAt: &succeededAt}
	statuses, err = source.Statuses(context.Background(), companyProfileTestPrincipal(false, dashboardprincipal.CorpBindingStatusActive))
	if err != nil {
		t.Fatal(err)
	}
	succeeded := findStatus(statuses, "wecom_standard")
	if succeeded.State != providers.StateReady || succeeded.LastSuccessAt == nil || !succeeded.LastSuccessAt.Equal(succeededAt) {
		t.Fatalf("successful sync status = %#v", succeeded)
	}

	store.syncStatus = SyncStatus{Status: "failed", FinishedAt: &failedAt, ErrorCode: "SYNC_FAILED"}
	statuses, err = source.Statuses(context.Background(), companyProfileTestPrincipal(false, dashboardprincipal.CorpBindingStatusActive))
	if err != nil {
		t.Fatal(err)
	}
	if fallback := findStatus(statuses, "wecom_standard"); fallback.LastErrorCode != "wecom.sync_failed" {
		t.Fatalf("unclassified sync error code = %q, want stable wecom.sync_failed", fallback.LastErrorCode)
	}
}
