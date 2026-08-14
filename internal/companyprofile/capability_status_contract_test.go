package companyprofile

import (
	"context"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/modules/providers"
	"jiyi/mochat-go/internal/modules/providers/catalog"
	"jiyi/mochat-go/internal/providerstatus"
	"jiyi/mochat-go/internal/wecomcapability"
)

type capabilityOperationStore struct {
	*companyProfileContractStore
	latest map[string]wecomcapability.Operation
}

func (s *capabilityOperationStore) LatestCapabilityOperations(_ context.Context, _ dashboardprincipal.DashboardPrincipal, _ []string) (map[string]wecomcapability.Operation, error) {
	return s.latest, nil
}

func TestProviderStatusProjectsIndependentCapabilityEvidence(t *testing.T) {
	verifiedAt := time.Date(2026, 8, 14, 8, 0, 0, 0, time.UTC)
	successAt := verifiedAt.Add(time.Hour)
	store := &capabilityOperationStore{
		companyProfileContractStore: &companyProfileContractStore{profile: Profile{
			TenantID: 202, CorpID: 303, BindingStatus: "active", WXCorpID: "ww-authoritative", VerifiedAt: &verifiedAt,
			BindingVersion: 1,
			Credentials:    CredentialStatuses{WeCom: CredentialStatus{Configured: true}, Agent: CredentialStatus{Configured: true}},
		}, syncStatus: SyncStatus{Status: "succeeded", CredentialVersion: 1, FinishedAt: &successAt}},
		latest: map[string]wecomcapability.Operation{
			"employee_sync":      {Capability: "employee_sync", Status: wecomcapability.OperationSucceeded, CredentialVersion: 1, FinishedAt: &successAt},
			"contact_batch_send": {Capability: "contact_batch_send", Status: wecomcapability.OperationFailed, CredentialVersion: 1, ErrorCode: "wecom.http_429", FinishedAt: &successAt},
		},
	}
	registry, err := catalog.NewRegistry(catalog.Dependencies{
		AI:            providerStatusTestProvider{status: providers.Status{State: providers.StateLimited}},
		Archive:       providerStatusTestProvider{status: providers.Status{State: providers.StateLimited}},
		AudioStorage:  providerStatusTestProvider{status: providers.Status{State: providers.StateLimited}},
		WeComStandard: providerStatusTestProvider{status: providers.Status{State: providers.StateLimited}},
		AIEnabled:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err := providerstatus.NewService(NewProviderStatusSource(store, registry)).Resolve(context.Background(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive))
	if err != nil {
		t.Fatal(err)
	}
	var standard providerstatus.ProviderStatus
	for _, item := range view.Providers {
		if item.Kind == "wecom_standard" {
			standard = item
		}
	}
	if len(standard.CapabilityStatuses) != len(wecomcapability.All) {
		t.Fatalf("capability statuses=%#v, want %d entries", standard.CapabilityStatuses, len(wecomcapability.All))
	}
	byCapability := make(map[string]providerstatus.CapabilityStatus, len(standard.CapabilityStatuses))
	for _, item := range standard.CapabilityStatuses {
		byCapability[item.Capability] = item
	}
	if byCapability["employee_sync"].State != providers.StateReady {
		t.Fatalf("employee sync=%#v, want ready", byCapability["employee_sync"])
	}
	if byCapability["contact_batch_send"].State != providers.StateLimited || byCapability["contact_batch_send"].Code != "wecom.capability_operation_failed" {
		t.Fatalf("contact batch send=%#v, want failed limited", byCapability["contact_batch_send"])
	}
	if byCapability["room_batch_send"].State == providers.StateReady {
		t.Fatal("room batch send became ready without its own operation evidence")
	}
}

func TestProviderStatusRejectsStaleOperationAfterBindingVersionRotation(t *testing.T) {
	verifiedAt := time.Date(2026, 8, 14, 8, 0, 0, 0, time.UTC)
	finishedAt := verifiedAt.Add(time.Hour)
	profile := Profile{
		TenantID: 202, CorpID: 303, BindingStatus: "active", WXCorpID: "ww-authoritative",
		BindingVersion: 2, VerifiedAt: &verifiedAt,
		Credentials: CredentialStatuses{WeCom: CredentialStatus{Configured: true, KeyID: "same-encryption-key"}},
	}
	store := &capabilityOperationStore{
		companyProfileContractStore: &companyProfileContractStore{profile: profile, syncStatus: SyncStatus{Status: "succeeded", FinishedAt: &finishedAt}},
		latest: map[string]wecomcapability.Operation{
			"contact_batch_send": {Capability: "contact_batch_send", Status: wecomcapability.OperationSucceeded, CredentialVersion: 1, FinishedAt: &finishedAt},
		},
	}
	registry, err := catalog.NewRegistry(catalog.Dependencies{
		AI:            providerStatusTestProvider{status: providers.Status{State: providers.StateLimited}},
		Archive:       providerStatusTestProvider{status: providers.Status{State: providers.StateLimited}},
		AudioStorage:  providerStatusTestProvider{status: providers.Status{State: providers.StateLimited}},
		WeComStandard: providerStatusTestProvider{status: providers.Status{State: providers.StateLimited}},
		AIEnabled:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	principal := companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive)
	view, err := providerstatus.NewService(NewProviderStatusSource(store, registry)).Resolve(context.Background(), principal)
	if err != nil {
		t.Fatal(err)
	}
	standard := findProviderStatusForContract(view, "wecom_standard")
	contact := findCapabilityForContract(standard, "contact_batch_send")
	employee := findCapabilityForContract(standard, wecomcapability.EmployeeSync)
	if contact.State == providers.StateReady {
		t.Fatalf("stale operation became ready after binding rotation: %#v", contact)
	}
	if employee.State == providers.StateReady {
		t.Fatalf("legacy sync marker became ready after binding rotation: %#v", employee)
	}

	store.latest["contact_batch_send"] = wecomcapability.Operation{
		Capability: "contact_batch_send", Status: wecomcapability.OperationSucceeded, CredentialVersion: 2, FinishedAt: &finishedAt,
	}
	store.syncStatus.CredentialVersion = 2
	view, err = providerstatus.NewService(NewProviderStatusSource(store, registry)).Resolve(context.Background(), principal)
	if err != nil {
		t.Fatal(err)
	}
	contact = findCapabilityForContract(findProviderStatusForContract(view, "wecom_standard"), "contact_batch_send")
	if contact.State != providers.StateReady {
		t.Fatalf("current-version operation did not become ready: %#v", contact)
	}
	employee = findCapabilityForContract(findProviderStatusForContract(view, "wecom_standard"), wecomcapability.EmployeeSync)
	if employee.State != providers.StateReady {
		t.Fatalf("current-version sync did not become ready: %#v", employee)
	}
}

func findProviderStatusForContract(view providerstatus.View, kind string) providerstatus.ProviderStatus {
	for _, status := range view.Providers {
		if status.Kind == kind {
			return status
		}
	}
	return providerstatus.ProviderStatus{}
}

func findCapabilityForContract(status providerstatus.ProviderStatus, capability string) providerstatus.CapabilityStatus {
	for _, item := range status.CapabilityStatuses {
		if item.Capability == capability {
			return item
		}
	}
	return providerstatus.CapabilityStatus{}
}
