package store

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

func TestNewRedisStoreEnablesContextTimeouts(t *testing.T) {
	store := NewRedisStore(RedisConfig{Addr: "127.0.0.1:1"})
	defer store.Close()
	if !store.client.Options().ContextTimeoutEnabled {
		t.Fatal("go-redis context timeouts are disabled")
	}
}

func TestWeWorkCallbackWakeupSliceNetworkTimeoutClassification(t *testing.T) {
	deadline := time.Unix(1_800_000_000, 0)
	timeoutErr := &net.OpError{Op: "read", Net: "tcp", Err: os.ErrDeadlineExceeded}
	tests := []struct {
		name             string
		err              error
		ownsDeadline     bool
		observedAt       time.Time
		wantSliceExpired bool
	}{
		{name: "own deadline reached", err: timeoutErr, ownsDeadline: true, observedAt: deadline, wantSliceExpired: true},
		{name: "own deadline close", err: timeoutErr, ownsDeadline: true, observedAt: deadline.Add(-5 * time.Millisecond), wantSliceExpired: true},
		{name: "network timeout too early", err: timeoutErr, ownsDeadline: true, observedAt: deadline.Add(-50 * time.Millisecond)},
		{name: "parent owns deadline", err: timeoutErr, observedAt: deadline},
		{name: "connection error", err: errors.New("connection reset by peer"), ownsDeadline: true, observedAt: deadline},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := weWorkCallbackWakeupSliceNetworkTimeout(tt.err, tt.ownsDeadline, tt.observedAt, deadline)
			if got != tt.wantSliceExpired {
				t.Fatalf("slice network timeout=%t, want %t", got, tt.wantSliceExpired)
			}
		})
	}
}

func TestWeWorkCallbackWakeupBRPopTimeoutUsesRedisMinimum(t *testing.T) {
	if got := weWorkCallbackWakeupBRPopTimeout(199 * time.Millisecond); got != time.Second {
		t.Fatalf("subsecond BRPOP timeout=%s, want 1s", got)
	}
	if got := weWorkCallbackWakeupBRPopTimeout(2 * time.Second); got != 2*time.Second {
		t.Fatalf("supported BRPOP timeout=%s, want 2s", got)
	}
}

func TestRedisStoreWakeWeWorkCallbackHonorsDeadlineAgainstUnresponsiveServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var connectionsMu sync.Mutex
	var connections []net.Conn
	done := make(chan struct{})
	go func() {
		for {
			connection, acceptErr := listener.Accept()
			if acceptErr != nil {
				close(done)
				return
			}
			connectionsMu.Lock()
			connections = append(connections, connection)
			connectionsMu.Unlock()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-done
		connectionsMu.Lock()
		defer connectionsMu.Unlock()
		for _, connection := range connections {
			_ = connection.Close()
		}
	})
	store := NewRedisStore(RedisConfig{Addr: listener.Addr().String()})
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := store.WakeWeWorkCallback(ctx); err == nil {
		t.Fatal("unresponsive Redis wakeup unexpectedly succeeded")
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("Redis wakeup ignored 50ms deadline: elapsed=%s", elapsed)
	}
}

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
