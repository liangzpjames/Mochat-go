package store

import (
	"encoding/json"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
)

func TestDecodeReliableQueuePayloadSupportsEnvelopeMetadata(t *testing.T) {
	payload, err := json.Marshal(dashboard.EmployeeApplyEvent{BindingID: 7, Source: "test"})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(reliableQueueEnvelope{
		Queue:          dashboard.QueueNameEmployeeApply,
		PayloadType:    dashboard.QueuePayloadTypeEmployeeApply,
		IdempotencyKey: "mochat-go:queue-idempotency:employee-apply:test",
		EnqueuedAt:     "2026-07-04T12:00:00+08:00",
		Payload:        payload,
		Attempts:       2,
	})
	if err != nil {
		t.Fatal(err)
	}

	var event dashboard.EmployeeApplyEvent
	attempts, err := decodeReliableQueuePayload(string(raw), &event)
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d", attempts)
	}
	if event.BindingID != 7 || event.Source != "test" {
		t.Fatalf("event = %+v", event)
	}
}

func TestReliableQueueEnvelopeForEventPreservesMetadata(t *testing.T) {
	payload, err := json.Marshal(dashboard.ContactWelcomeEvent{CorpID: 7, ContactID: 8, EmployeeID: 9})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(reliableQueueEnvelope{
		Queue:          dashboard.QueueNameContactWelcome,
		PayloadType:    dashboard.QueuePayloadTypeContactWelcome,
		IdempotencyKey: "mochat-go:queue-idempotency:contact-welcome:test",
		EnqueuedAt:     "2026-07-04T12:00:00+08:00",
		Payload:        payload,
		Attempts:       1,
		LastError:      "temporary",
	})
	if err != nil {
		t.Fatal(err)
	}

	envelope, err := reliableQueueEnvelopeForEvent(string(raw), dashboard.ContactWelcomeEvent{})
	if err != nil {
		t.Fatal(err)
	}
	if envelope.Queue != dashboard.QueueNameContactWelcome || envelope.PayloadType != dashboard.QueuePayloadTypeContactWelcome {
		t.Fatalf("envelope metadata = %+v", envelope)
	}
	if envelope.IdempotencyKey == "" || envelope.EnqueuedAt == "" || envelope.LastError != "temporary" {
		t.Fatalf("envelope = %+v", envelope)
	}
}

func TestReliableQueueEnvelopeForEventWrapsLegacyPayload(t *testing.T) {
	legacy := `{"corpIds":[9],"source":"legacy"}`
	envelope, err := reliableQueueEnvelopeForEvent(legacy, dashboard.EmployeeApplyEvent{})
	if err != nil {
		t.Fatal(err)
	}
	if envelope.Queue != "" || envelope.PayloadType != "" || string(envelope.Payload) != legacy {
		t.Fatalf("envelope = %+v", envelope)
	}
}
