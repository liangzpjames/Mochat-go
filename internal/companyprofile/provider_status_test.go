package companyprofile

import (
	"context"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/modules/providers"
	"jiyi/mochat-go/internal/modules/providers/catalog"
	"jiyi/mochat-go/internal/providerstatus"
)

type providerStatusTestProvider struct{ status providers.Status }

func (p providerStatusTestProvider) Status() providers.Status { return p.status }

func TestProviderStatusSourceUsesTenantProfileAndRuntimeEvidence(t *testing.T) {
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
}
