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

func TestCapabilityActionMatrixRejectsCrossCapabilityActions(t *testing.T) {
	valid := map[string][]OperationAction{
		EmployeeSync:        {ActionSync, ActionPull},
		DepartmentSync:      {ActionSync, ActionPull},
		ExternalContactSync: {ActionSync, ActionPull},
		ContactTagSync:      {ActionSync, ActionPull},
		RoomSync:            {ActionSync, ActionPull},
		ContactWay:          {ActionCreate, ActionUpdate},
		WelcomeMessage:      {ActionCreate, ActionUpdate, ActionSend},
		ContactTransfer:     {ActionTransfer},
		AgentMessage:        {ActionSend},
		ContactBatchSend:    {ActionSend, ActionPoll, ActionRetry},
		RoomBatchSend:       {ActionSend, ActionPoll, ActionRetry},
		Callback:            {ActionReceive, ActionVerify},
	}
	for capability, actions := range valid {
		allowed := make(map[OperationAction]struct{}, len(actions))
		for _, action := range actions {
			allowed[action] = struct{}{}
			if !IsValidOperationActionForCapability(capability, action) {
				t.Errorf("valid action %q rejected for %q", action, capability)
			}
		}
		for _, action := range []OperationAction{ActionSync, ActionPull, ActionCreate, ActionUpdate, ActionSend, ActionTransfer, ActionReceive, ActionVerify, ActionPoll, ActionRetry} {
			if _, ok := allowed[action]; ok {
				continue
			}
			if IsValidOperationActionForCapability(capability, action) {
				t.Errorf("invalid action %q accepted for %q", action, capability)
			}
		}
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
	agent := Operation{
		ID: 4, TenantID: 7, CorpID: 11, Capability: AgentMessage, Action: "send",
		CredentialGroup: CredentialGroupAgent, Status: OperationSucceeded, CredentialVersion: 3,
		ProviderObjectID: "100001", ExternalSuccess: true, TargetTotal: 1, SuccessTotal: 1,
		FinishedAt: &finishedAt,
	}
	if !IsCurrentOperationEvidence(agent, 7, 11, AgentMessage, 3) {
		t.Fatal("agent message with external success, target count, and agent id should be current")
	}
	agent.ExternalSuccess = false
	if IsCurrentOperationEvidence(agent, 7, 11, AgentMessage, 3) {
		t.Fatal("agent message without external success passed")
	}
	agent.ExternalSuccess = true
	agent.TargetTotal = 0
	if IsCurrentOperationEvidence(agent, 7, 11, AgentMessage, 3) {
		t.Fatal("agent message with zero target count passed")
	}
	agent.TargetTotal = 1
	agent.ProviderObjectID = ""
	if IsCurrentOperationEvidence(agent, 7, 11, AgentMessage, 3) {
		t.Fatal("agent message without actual agent id passed")
	}
	for _, capability := range []string{ContactWay, WelcomeMessage, ContactTransfer} {
		operation := Operation{
			ID: 10, TenantID: 7, CorpID: 11, Capability: capability, Action: "create",
			CredentialGroup: CredentialGroupContact, Status: OperationSucceeded, CredentialVersion: 3,
			ExternalSuccess: true,
			TargetTotal:     1, SuccessTotal: 1, FinishedAt: &finishedAt,
		}
		if capability == ContactTransfer {
			operation.Action = "transfer"
		}
		if capability == ContactTransfer {
			operation.ProviderObjectID = "transfer-1"
		}
		if !IsCurrentOperationEvidence(operation, 7, 11, capability, 3) {
			t.Fatalf("%s errcode-success evidence without object id should be current", capability)
		}
		operation.ProviderObjectID = "response-1"
		operation.ExternalSuccess = false
		if IsCurrentOperationEvidence(operation, 7, 11, capability, 3) {
			t.Fatalf("%s without external success passed", capability)
		}
	}
}
