package wecomcapability

import (
	"reflect"
	"testing"
	"time"
)

func TestStableCapabilityNames(t *testing.T) {
	want := []string{
		"employee_sync", "department_sync", "external_contact_sync", "contact_tag_sync",
		"room_sync", "contact_way", "welcome_message", "contact_transfer", "agent_message",
		"contact_batch_send", "room_batch_send", "callback",
	}
	if !reflect.DeepEqual(All, want) {
		t.Fatalf("capabilities = %#v, want %#v", All, want)
	}
}

func TestOperationAndDispatchStatesAreSeparateAndFailClosed(t *testing.T) {
	for _, state := range []string{
		OperationPending, OperationClaimed, OperationSubmitting, OperationSubmitted,
		OperationPolling, OperationSucceeded, OperationPartialFailed, OperationFailed,
		OperationCancelled,
	} {
		if !IsValidOperationStatus(state) {
			t.Fatalf("operation state %q rejected", state)
		}
	}
	if IsValidOperationStatus(DispatchQueued) {
		t.Fatalf("dispatch-only state %q accepted as operation", DispatchQueued)
	}
	for _, state := range []string{
		DispatchQueued, DispatchClaimed, DispatchSubmitting, DispatchSubmitted,
		DispatchPolling, DispatchSucceeded, DispatchPartialFailed, DispatchFailed,
	} {
		if !IsValidDispatchStatus(state) {
			t.Fatalf("dispatch state %q rejected", state)
		}
	}
	for _, state := range []string{"", "running", "complete", "unknown"} {
		if IsValidOperationStatus(state) || IsValidDispatchStatus(state) {
			t.Fatalf("unknown or cross-layer state %q was accepted", state)
		}
	}
}

func TestOperationTransitionsFailClosed(t *testing.T) {
	valid := [][2]string{
		{OperationPending, OperationClaimed},
		{OperationClaimed, OperationSubmitting},
		{OperationSubmitting, OperationSubmitted},
		{OperationSubmitted, OperationPolling},
		{OperationPolling, OperationSucceeded},
		{OperationPolling, OperationPartialFailed},
		{OperationClaimed, OperationCancelled},
		{OperationSubmitting, OperationFailed},
	}
	for _, transition := range valid {
		if !CanTransitionOperation(transition[0], transition[1]) {
			t.Fatalf("valid transition %q -> %q rejected", transition[0], transition[1])
		}
	}
	for _, transition := range [][2]string{
		{OperationPending, OperationSucceeded},
		{OperationSucceeded, OperationSubmitting},
		{OperationFailed, OperationSucceeded},
		{OperationPolling, "unknown"},
		{"unknown", OperationClaimed},
	} {
		if CanTransitionOperation(transition[0], transition[1]) {
			t.Fatalf("invalid transition %q -> %q accepted", transition[0], transition[1])
		}
	}
}

func TestOperationEvidenceUsesCapabilitySpecificExternalContracts(t *testing.T) {
	finishedAt := time.Date(2026, 8, 15, 1, 0, 0, 0, time.UTC)
	base := Operation{ID: 1, TenantID: 7, CorpID: 11, Capability: ContactBatchSend, Action: "send", CredentialGroup: CredentialGroupContact, Status: OperationSucceeded, CredentialVersion: 3, ProviderRequestID: "msgid-1", TargetTotal: 2, SuccessTotal: 2, FinishedAt: &finishedAt}
	if !IsCurrentOperationEvidence(base, 7, 11, ContactBatchSend, 3) {
		t.Fatal("complete batch evidence should be current")
	}
	base.ProviderRequestID = ""
	if IsCurrentOperationEvidence(base, 7, 11, ContactBatchSend, 3) {
		t.Fatal("batch success without provider msgid passed")
	}
	sync := Operation{ID: 2, TenantID: 7, CorpID: 11, Capability: EmployeeSync, Action: "sync", CredentialGroup: CredentialGroupEmployee, Status: OperationSucceeded, CredentialVersion: 3, ExternalSuccess: true, FinishedAt: &finishedAt}
	if !IsCurrentOperationEvidence(sync, 7, 11, EmployeeSync, 3) {
		t.Fatal("empty employee sync success with errcode evidence should be current")
	}
	callback := Operation{ID: 3, TenantID: 7, CorpID: 11, Capability: Callback, Action: "receive", CredentialGroup: CredentialGroupCallback, Status: OperationSucceeded, CredentialVersion: 3, CallbackEvidence: true, TargetTotal: 1, SuccessTotal: 1, FinishedAt: &finishedAt}
	if !IsCurrentOperationEvidence(callback, 7, 11, Callback, 3) {
		t.Fatal("verified callback event should be current")
	}
	callback.CallbackEvidence = false
	if IsCurrentOperationEvidence(callback, 7, 11, Callback, 3) {
		t.Fatal("callback without event evidence passed")
	}
	partial := base
	partial.Status = OperationPartialFailed
	partial.ProviderRequestID = "msgid-1"
	partial.ErrorCode = "wecom.http_429"
	partial.SuccessTotal = 1
	partial.FailureTotal = 1
	if !IsCurrentOperationEvidence(partial, 7, 11, ContactBatchSend, 3) {
		t.Fatal("consistent partial batch evidence rejected")
	}
	partial.FailureTotal = 0
	if IsCurrentOperationEvidence(partial, 7, 11, ContactBatchSend, 3) {
		t.Fatal("inconsistent partial counts passed")
	}
}
