package store

import (
	"os"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/wecomcapability"
)

func TestCapabilityLedgerStoreUsesLeaseExpiryActorAndTerminalFences(t *testing.T) {
	body, err := os.ReadFile("wecom_capability_ledger.go")
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(body))
	for _, required := range []string{
		"lease_expires_at > now(6)", "lease_expires_at is not null",
		"userid <= 0 && source != capabilityactorsystem", "operation.status",
		"leasetoken", "attempt",
	} {
		if !strings.Contains(lower, required) {
			t.Fatalf("ledger store missing fence contract %q", required)
		}
	}
	if strings.Count(lower, "lease_expires_at=date_add(now(6), interval ? microsecond)") < 2 {
		t.Fatalf("operation and dispatch claim leases must both use the database clock")
	}
	for _, forbidden := range []string{
		"time.now().utc().add(",
		"capabilityleaseisactive(operation.leaseexpiresat",
	} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("lease qualification must not use Go wall-clock logic: %s", forbidden)
		}
	}
}

func TestCapabilityOperationInputRejectsUnsafeAndCrossCapabilityContracts(t *testing.T) {
	valid := CapabilityOperationInput{
		Capability:     wecomcapability.ContactBatchSend,
		Action:         wecomcapability.ActionSend,
		IdempotencyKey: "operation-1",
		TargetTotal:    2,
	}
	if err := ValidateCapabilityOperationInput(valid); err != nil {
		t.Fatalf("valid operation rejected: %v", err)
	}
	for _, invalid := range []CapabilityOperationInput{
		{Capability: wecomcapability.ContactBatchSend, Action: wecomcapability.ActionSync, IdempotencyKey: "bad-action", TargetTotal: 1},
		{Capability: wecomcapability.ContactBatchSend, Action: wecomcapability.ActionSend, IdempotencyKey: "", TargetTotal: 1},
		{Capability: wecomcapability.ContactBatchSend, Action: wecomcapability.ActionSend, IdempotencyKey: "bad-target", TargetTotal: -1},
	} {
		if err := ValidateCapabilityOperationInput(invalid); err == nil {
			t.Fatalf("invalid operation accepted: %+v", invalid)
		}
	}
}

func TestCapabilityDispatchInputRequiresStringTargetAndScopedIdempotency(t *testing.T) {
	valid := CapabilityDispatchInput{
		OperationID:    9,
		DispatchKind:   "contact_batch_chunk",
		ChunkNo:        1,
		TargetID:       "external-user-α",
		IdempotencyKey: "dispatch-1",
	}
	if err := ValidateCapabilityDispatchInput(valid); err != nil {
		t.Fatalf("valid dispatch rejected: %v", err)
	}
	for _, invalid := range []CapabilityDispatchInput{
		{OperationID: 0, DispatchKind: "contact_batch_chunk", TargetID: "external-user", IdempotencyKey: "dispatch-2"},
		{OperationID: 9, DispatchKind: "contact_batch_chunk", TargetID: "", IdempotencyKey: "dispatch-3"},
		{OperationID: 9, DispatchKind: "contact_batch_chunk", TargetID: "external-user", IdempotencyKey: ""},
	} {
		if err := ValidateCapabilityDispatchInput(invalid); err == nil {
			t.Fatalf("invalid dispatch accepted: %+v", invalid)
		}
	}
}
