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
			BindingVersion:        1,
			CredentialGenerations: CredentialGenerationSet{Employee: 1, Contact: 1, Agent: 1, Callback: 1},
			Credentials:           CredentialStatuses{WeCom: CredentialStatus{Configured: true, EmployeeConfigured: true, ContactConfigured: true}, Agent: CredentialStatus{Configured: true, AgentIDConfigured: true, AgentSecretConfigured: true}},
		}, syncStatus: SyncStatus{Status: "succeeded", CredentialVersion: 1, FinishedAt: &successAt}},
		latest: map[string]wecomcapability.Operation{
			"employee_sync":      {Capability: "employee_sync", Status: wecomcapability.OperationSucceeded, CredentialVersion: 1, FinishedAt: &successAt},
			"contact_batch_send": {ID: 2, TenantID: 202, CorpID: 303, Capability: "contact_batch_send", Action: "send", CredentialGroup: wecomcapability.CredentialGroupContact, Status: wecomcapability.OperationFailed, CredentialVersion: 1, ErrorCode: "wecom.http_429", FinishedAt: &successAt},
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
		CredentialGenerations: CredentialGenerationSet{Employee: 2, Contact: 2, Agent: 2, Callback: 2},
		Credentials:           CredentialStatuses{WeCom: CredentialStatus{Configured: true, EmployeeConfigured: true, ContactConfigured: true, KeyID: "same-encryption-key"}},
	}
	store := &capabilityOperationStore{
		companyProfileContractStore: &companyProfileContractStore{profile: profile, syncStatus: SyncStatus{Status: "succeeded", FinishedAt: &finishedAt}},
		latest: map[string]wecomcapability.Operation{
			"contact_batch_send": {ID: 2, TenantID: 202, CorpID: 303, Capability: "contact_batch_send", Action: "send", CredentialGroup: wecomcapability.CredentialGroupContact, Status: wecomcapability.OperationSucceeded, CredentialVersion: 1, ProviderRequestID: "request-1", TargetTotal: 1, SuccessTotal: 1, FinishedAt: &finishedAt},
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
		ID: 2, TenantID: 202, CorpID: 303, Capability: "contact_batch_send", Action: "send", CredentialGroup: wecomcapability.CredentialGroupContact, Status: wecomcapability.OperationSucceeded, CredentialVersion: 2, ProviderRequestID: "request-2", TargetTotal: 1, SuccessTotal: 1, FinishedAt: &finishedAt,
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

func TestProviderStatusUsesCapabilitySpecificCredentialFacts(t *testing.T) {
	verifiedAt := time.Date(2026, 8, 14, 8, 0, 0, 0, time.UTC)
	finishedAt := verifiedAt.Add(time.Hour)
	baseProfile := Profile{
		TenantID: 202, CorpID: 303, BindingStatus: "active", WXCorpID: "ww-authoritative",
		BindingVersion: 1, VerifiedAt: &verifiedAt,
		CredentialGenerations: CredentialGenerationSet{Employee: 1, Contact: 1, Agent: 1, Callback: 1},
	}
	succeeded := func(capability string) wecomcapability.Operation {
		action := "send"
		if capability == wecomcapability.Callback {
			action = "receive"
		}
		operation := wecomcapability.Operation{ID: 10, TenantID: 202, CorpID: 303, Capability: capability, Action: action, CredentialGroup: wecomcapability.CredentialGroupForCapability(capability), Status: wecomcapability.OperationSucceeded, CredentialVersion: 1, ProviderRequestID: "request-1", TargetTotal: 1, SuccessTotal: 1, FinishedAt: &finishedAt}
		if capability == wecomcapability.Callback {
			operation.CallbackEvidence = true
		}
		return operation
	}
	statusFor := func(t *testing.T, profile Profile, sync SyncStatus, operations map[string]wecomcapability.Operation, runtime providers.Status) map[string]providerstatus.CapabilityStatus {
		t.Helper()
		store := &capabilityOperationStore{
			companyProfileContractStore: &companyProfileContractStore{profile: profile, syncStatus: sync},
			latest:                      operations,
		}
		registry, err := catalog.NewRegistry(catalog.Dependencies{
			AI:            providerStatusTestProvider{status: providers.Status{State: providers.StateLimited}},
			Archive:       providerStatusTestProvider{status: providers.Status{State: providers.StateLimited}},
			AudioStorage:  providerStatusTestProvider{status: providers.Status{State: providers.StateLimited}},
			WeComStandard: providerStatusTestProvider{status: runtime},
			AIEnabled:     true,
		})
		if err != nil {
			t.Fatal(err)
		}
		view, err := providerstatus.NewService(NewProviderStatusSource(store, registry)).Resolve(context.Background(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive))
		if err != nil {
			t.Fatal(err)
		}
		standard := findProviderStatusForContract(view, "wecom_standard")
		result := make(map[string]providerstatus.CapabilityStatus, len(standard.CapabilityStatuses))
		for _, item := range standard.CapabilityStatuses {
			result[item.Capability] = item
		}
		return result
	}

	employeeOnly := baseProfile
	employeeOnly.Credentials.WeCom = CredentialStatus{Configured: true, EmployeeConfigured: true}
	byCapability := statusFor(t, employeeOnly,
		SyncStatus{Status: "succeeded", CredentialVersion: 1, FinishedAt: &finishedAt},
		map[string]wecomcapability.Operation{
			wecomcapability.ContactBatchSend: succeeded(wecomcapability.ContactBatchSend),
			wecomcapability.Callback:         succeeded(wecomcapability.Callback),
		},
		providers.Status{Kind: "wecom_standard", State: providers.StateReady, Source: providers.SourceExternal, Capabilities: wecomcapability.All, CallbackRouteConfigured: true, CallbackWorkerConfigured: true})
	if byCapability[wecomcapability.EmployeeSync].State != providers.StateReady {
		t.Fatalf("employee-only employee status=%#v, want ready", byCapability[wecomcapability.EmployeeSync])
	}
	if byCapability[wecomcapability.ContactBatchSend].State == providers.StateReady || byCapability[wecomcapability.ContactBatchSend].Code != "wecom.contact_credentials_missing" {
		t.Fatalf("employee-only contact status=%#v, must not use generic configured evidence", byCapability[wecomcapability.ContactBatchSend])
	}
	if byCapability[wecomcapability.Callback].State == providers.StateReady || byCapability[wecomcapability.Callback].Code != "wecom.callback_credentials_missing" {
		t.Fatalf("employee-only callback status=%#v, must require callback credentials", byCapability[wecomcapability.Callback])
	}
	if byCapability[wecomcapability.DepartmentSync].State == providers.StateReady {
		t.Fatalf("department sync inferred readiness from employee sync: %#v", byCapability[wecomcapability.DepartmentSync])
	}

	contactOnly := baseProfile
	contactOnly.Credentials.WeCom = CredentialStatus{Configured: true, ContactConfigured: true}
	byCapability = statusFor(t, contactOnly,
		SyncStatus{Status: "succeeded", CredentialVersion: 1, FinishedAt: &finishedAt},
		map[string]wecomcapability.Operation{wecomcapability.ContactBatchSend: succeeded(wecomcapability.ContactBatchSend)},
		providers.Status{Kind: "wecom_standard", State: providers.StateReady, Source: providers.SourceExternal, Capabilities: wecomcapability.All})
	if byCapability[wecomcapability.ContactBatchSend].State != providers.StateReady {
		t.Fatalf("contact-only contact status=%#v, want ready", byCapability[wecomcapability.ContactBatchSend])
	}
	missingBatchID := succeeded(wecomcapability.ContactBatchSend)
	missingBatchID.ProviderRequestID = ""
	byCapability = statusFor(t, contactOnly, SyncStatus{Status: "idle"},
		map[string]wecomcapability.Operation{wecomcapability.ContactBatchSend: missingBatchID},
		providers.Status{Kind: "wecom_standard", State: providers.StateReady, Source: providers.SourceExternal, Capabilities: wecomcapability.All})
	if byCapability[wecomcapability.ContactBatchSend].State == providers.StateReady || byCapability[wecomcapability.ContactBatchSend].Code != "wecom.capability_evidence_invalid" {
		t.Fatalf("batch operation without provider msgid=%#v, want limited evidence_invalid", byCapability[wecomcapability.ContactBatchSend])
	}
	emptyContactSync := wecomcapability.Operation{
		ID: 12, TenantID: 202, CorpID: 303, Capability: wecomcapability.ExternalContactSync,
		Action: "sync", CredentialGroup: wecomcapability.CredentialGroupContact,
		Status: wecomcapability.OperationSucceeded, CredentialVersion: 1,
		ExternalSuccess: true, FinishedAt: &finishedAt,
	}
	byCapability = statusFor(t, contactOnly, SyncStatus{Status: "idle"},
		map[string]wecomcapability.Operation{wecomcapability.ExternalContactSync: emptyContactSync},
		providers.Status{Kind: "wecom_standard", State: providers.StateReady, Source: providers.SourceExternal, Capabilities: wecomcapability.All})
	if byCapability[wecomcapability.ExternalContactSync].State != providers.StateReady {
		t.Fatalf("empty contact sync success=%#v, want ready without fabricated provider id", byCapability[wecomcapability.ExternalContactSync])
	}
	if byCapability[wecomcapability.EmployeeSync].State == providers.StateReady || byCapability[wecomcapability.EmployeeSync].Code != "wecom.employee_credentials_missing" {
		t.Fatalf("contact-only employee status=%#v, must require employee credential", byCapability[wecomcapability.EmployeeSync])
	}

	callbackMissingAES := baseProfile
	callbackMissingAES.Credentials.WeCom = CredentialStatus{Configured: true, CallbackTokenConfigured: true}
	byCapability = statusFor(t, callbackMissingAES, SyncStatus{Status: "idle"},
		map[string]wecomcapability.Operation{wecomcapability.Callback: succeeded(wecomcapability.Callback)},
		providers.Status{Kind: "wecom_standard", State: providers.StateReady, Source: providers.SourceExternal, Capabilities: wecomcapability.All, CallbackRouteConfigured: true, CallbackWorkerConfigured: true})
	if byCapability[wecomcapability.Callback].State == providers.StateReady || byCapability[wecomcapability.Callback].Code != "wecom.callback_credentials_missing" {
		t.Fatalf("callback missing AES status=%#v, must fail closed", byCapability[wecomcapability.Callback])
	}

	callbackReady := baseProfile
	callbackReady.Credentials.WeCom = CredentialStatus{Configured: true, CallbackTokenConfigured: true, CallbackAESConfigured: true}
	byCapability = statusFor(t, callbackReady, SyncStatus{Status: "idle"},
		map[string]wecomcapability.Operation{wecomcapability.Callback: succeeded(wecomcapability.Callback)},
		providers.Status{Kind: "wecom_standard", State: providers.StateReady, Source: providers.SourceExternal, Capabilities: wecomcapability.All, CallbackRouteConfigured: true, CallbackWorkerConfigured: true})
	if byCapability[wecomcapability.Callback].State != providers.StateReady {
		t.Fatalf("callback complete evidence status=%#v, want ready", byCapability[wecomcapability.Callback])
	}

	byCapability = statusFor(t, callbackReady, SyncStatus{Status: "idle"}, nil,
		providers.Status{Kind: "wecom_standard", State: providers.StateReady, Source: providers.SourceExternal, Capabilities: wecomcapability.All, CallbackRouteConfigured: true, CallbackWorkerConfigured: true})
	if byCapability[wecomcapability.Callback].State == providers.StateReady || byCapability[wecomcapability.Callback].Code != "wecom.callback_evidence_pending" {
		t.Fatalf("callback runtime flags without operation=%#v, must remain pending", byCapability[wecomcapability.Callback])
	}
	byCapability = statusFor(t, callbackReady, SyncStatus{Status: "idle"},
		map[string]wecomcapability.Operation{wecomcapability.Callback: {Capability: wecomcapability.Callback, Status: wecomcapability.OperationSucceeded, CredentialVersion: 0}},
		providers.Status{Kind: "wecom_standard", State: providers.StateReady, Source: providers.SourceExternal, Capabilities: wecomcapability.All, CallbackRouteConfigured: true, CallbackWorkerConfigured: true})
	if byCapability[wecomcapability.Callback].State == providers.StateReady {
		t.Fatalf("callback zero-version operation became ready: %#v", byCapability[wecomcapability.Callback])
	}
	invalidCallback := succeeded(wecomcapability.Callback)
	invalidCallback.CallbackEvidence = false
	byCapability = statusFor(t, callbackReady, SyncStatus{Status: "idle"},
		map[string]wecomcapability.Operation{wecomcapability.Callback: invalidCallback},
		providers.Status{Kind: "wecom_standard", State: providers.StateReady, Source: providers.SourceExternal, Capabilities: wecomcapability.All, CallbackRouteConfigured: true, CallbackWorkerConfigured: true})
	if byCapability[wecomcapability.Callback].State == providers.StateReady || byCapability[wecomcapability.Callback].Code != "wecom.capability_evidence_invalid" {
		t.Fatalf("callback operation without verified event=%#v, want limited evidence_invalid", byCapability[wecomcapability.Callback])
	}
}

func TestProviderStatusRequiresIndependentAgentIDAndSecret(t *testing.T) {
	verifiedAt := time.Date(2026, 8, 14, 8, 0, 0, 0, time.UTC)
	profile := Profile{TenantID: 202, CorpID: 303, BindingStatus: "active", WXCorpID: "ww-authoritative", BindingVersion: 1, VerifiedAt: &verifiedAt,
		CredentialGenerations: CredentialGenerationSet{Agent: 1},
		Credentials:           CredentialStatuses{WeCom: CredentialStatus{Configured: true}, Agent: CredentialStatus{Configured: true, AgentIDConfigured: true}}}
	store := &capabilityOperationStore{companyProfileContractStore: &companyProfileContractStore{profile: profile}, latest: map[string]wecomcapability.Operation{
		wecomcapability.AgentMessage: {ID: 11, TenantID: 202, CorpID: 303, Capability: wecomcapability.AgentMessage, Action: "send", CredentialGroup: wecomcapability.CredentialGroupAgent, Status: wecomcapability.OperationSucceeded, CredentialVersion: 1, ProviderRequestID: "request-1", TargetTotal: 1, SuccessTotal: 1},
	}}
	registry, err := catalog.NewRegistry(catalog.Dependencies{WeComStandard: providerStatusTestProvider{status: providers.Status{Kind: "wecom_standard", State: providers.StateReady, Source: providers.SourceExternal, Capabilities: wecomcapability.All}}})
	if err != nil {
		t.Fatal(err)
	}
	view, err := providerstatus.NewService(NewProviderStatusSource(store, registry)).Resolve(context.Background(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive))
	if err != nil {
		t.Fatal(err)
	}
	agent := findCapabilityForContract(findProviderStatusForContract(view, "wecom_standard"), wecomcapability.AgentMessage)
	if agent.State == providers.StateReady || agent.Code != "wecom.agent_credentials_missing" {
		t.Fatalf("agent without secret status=%#v, want limited", agent)
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
