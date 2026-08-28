package archive

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDurableBridgeRunnerScheduledEnqueueUsesAuthoritativeCursorAndStableIdempotency(t *testing.T) {
	serverCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"messages":[{"source_mode":"self_built","seq":42,"msgid":"durable-42","action":"send","from":"employee","tolist":["contact"],"msgtype":"text","text":{"content":"durable"}}]}`))
	}))
	defer server.Close()
	client, err := NewBridgeArchiveClient(server.URL, "MOCHAT-LOCAL-ACCEPTANCE-BEARER-0123456789", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	store := &durableBridgeTestStore{syncTestStore: newSyncTestStore(), cursor: Cursor{Sequence: 41}, bindings: []DurableArchiveBinding{{Scope: Scope{TenantID: 11, CorpID: 27}, WXCorpID: "ww-local", IntegrationMode: IntegrationModeSelfBuilt}}}
	runner := NewDurableBridgeRunner(store, client, 10).WithClock(func() time.Time {
		return time.Date(2026, 8, 27, 12, 34, 10, 0, time.UTC)
	})
	if err := runner.EnqueueScheduledOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.upserts) != 0 || serverCalls != 1 {
		t.Fatalf("scheduled enqueue performed worker writes: upserts=%#v bridge_calls=%d", store.upserts, serverCalls)
	}
	wantKey := "archive:poll:11:27:wecom:self_built:ww-local:41:202608271234"
	if store.lastTemplate.IdempotencyKey != wantKey || store.lastTemplate.Cursor.Sequence != 41 {
		t.Fatalf("template=%#v want key=%q", store.lastTemplate, wantKey)
	}
	if err := runner.EnqueueScheduledOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.runs) != 1 || len(store.lifecycle) != 1 {
		t.Fatalf("same-window enqueue runs=%d lifecycle=%v", len(store.runs), store.lifecycle)
	}
}

func TestDurableBridgeRunnerPendingWorkerDoesNotPollBindingsAcrossCycles(t *testing.T) {
	serverCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"messages":[]}`))
	}))
	defer server.Close()
	client, err := NewBridgeArchiveClient(server.URL, "MOCHAT-LOCAL-ACCEPTANCE-BEARER-0123456789", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	store := &durableBridgeTestStore{
		syncTestStore: newSyncTestStore(),
		bindings: []DurableArchiveBinding{{
			Scope: Scope{TenantID: 11, CorpID: 27}, WXCorpID: "ww-local", IntegrationMode: IntegrationModeSelfBuilt,
		}},
	}
	runner := NewDurableBridgeRunner(store, client, 10)
	if err := runner.RunPendingOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := runner.RunPendingOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.bindingCalls != 0 || store.busyCalls != 0 || serverCalls != 0 || len(store.runs) != 0 {
		t.Fatalf("empty pending worker mutated state: binding_calls=%d busy_calls=%d bridge_calls=%d runs=%d", store.bindingCalls, store.busyCalls, serverCalls, len(store.runs))
	}
}

func TestDurableBridgeRunnerScheduledProbeSkipsEmptyAndBusyScopes(t *testing.T) {
	serverCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"messages":[]}`))
	}))
	defer server.Close()
	client, err := NewBridgeArchiveClient(server.URL, "MOCHAT-LOCAL-ACCEPTANCE-BEARER-0123456789", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	bindings := []DurableArchiveBinding{
		{Scope: Scope{TenantID: 11, CorpID: 27}, WXCorpID: "ww-empty", IntegrationMode: IntegrationModeSelfBuilt},
		{Scope: Scope{TenantID: 22, CorpID: 38}, WXCorpID: "ww-busy", IntegrationMode: IntegrationModeSelfBuilt},
	}
	store := &durableBridgeTestStore{
		syncTestStore: newSyncTestStore(), bindings: bindings,
		busy: []Scope{bindings[1].Scope},
	}
	if err := NewDurableBridgeRunner(store, client, 10).EnqueueScheduledOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if serverCalls != 1 || len(store.runs) != 0 || store.bindingCalls != 1 || store.busyCalls != 1 {
		t.Fatalf("scheduled empty probe calls=%d runs=%d binding_calls=%d busy_calls=%d", serverCalls, len(store.runs), store.bindingCalls, store.busyCalls)
	}
}

func TestDurableBridgeRunnerEnqueuesManualRunWithoutCallingBridge(t *testing.T) {
	serverCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"messages":[]}`))
	}))
	defer server.Close()
	client, err := NewBridgeArchiveClient(server.URL, "MOCHAT-LOCAL-ACCEPTANCE-BEARER-0123456789", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	store := &durableBridgeTestStore{syncTestStore: newSyncTestStore(), cursor: Cursor{Sequence: 41}}
	run, err := NewDurableBridgeRunner(store, client, 10).Enqueue(context.Background(), DurableArchiveBinding{
		Scope: Scope{TenantID: 11, CorpID: 27}, WXCorpID: "ww-local", IntegrationMode: IntegrationModeSelfBuilt,
	}, "manual-abc-123")
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != SyncStatusQueued || serverCalls != 0 || store.lastTemplate.IdempotencyKey != "archive:manual:manual-abc-123" {
		t.Fatalf("run=%#v calls=%d template=%#v", run, serverCalls, store.lastTemplate)
	}
}

func TestDurableBridgeRunnerProcessesPendingManualRunWithoutPolling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"messages":[{"source_mode":"self_built","seq":42,"msgid":"manual-42","action":"send","from":"employee","tolist":["contact"],"msgtype":"text","text":{"content":"manual"}}]}`))
	}))
	defer server.Close()
	client, err := NewBridgeArchiveClient(server.URL, "MOCHAT-LOCAL-ACCEPTANCE-BEARER-0123456789", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	store := &durableBridgeTestStore{
		syncTestStore: newSyncTestStore(), cursor: Cursor{Sequence: 41},
		bindings: []DurableArchiveBinding{{Scope: Scope{TenantID: 11, CorpID: 27}, WXCorpID: "ww-local", IntegrationMode: IntegrationModeSelfBuilt}},
		pending: []DurableArchivePendingRun{{
			Binding: DurableArchiveBinding{Scope: Scope{TenantID: 11, CorpID: 27}, WXCorpID: "ww-local", IntegrationMode: IntegrationModeSelfBuilt},
			Cursor:  Cursor{Sequence: 41}, IdempotencyKey: "archive:manual:manual-abc-123",
		}},
	}
	if err := NewDurableBridgeRunner(store, client, 10).RunPendingOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.lastTemplate.IdempotencyKey != "archive:manual:manual-abc-123" || len(store.upserts) != 1 || store.bindingCalls != 0 {
		t.Fatalf("template=%#v upserts=%#v binding_calls=%d", store.lastTemplate, store.upserts, store.bindingCalls)
	}
}

type durableBridgeTestStore struct {
	*syncTestStore
	cursor       Cursor
	bindings     []DurableArchiveBinding
	lastTemplate SyncRun
	pending      []DurableArchivePendingRun
	busy         []Scope
	bindingCalls int
	busyCalls    int
}

func (s *durableBridgeTestStore) DurableArchiveBindings(context.Context) ([]DurableArchiveBinding, error) {
	s.bindingCalls++
	return s.bindings, nil
}
func (s *durableBridgeTestStore) DurableArchiveBindingForScope(_ context.Context, scope Scope) (DurableArchiveBinding, bool, error) {
	for _, binding := range s.bindings {
		if binding.Scope == scope {
			return binding, true, nil
		}
	}
	return DurableArchiveBinding{}, false, nil
}
func (s *durableBridgeTestStore) LatestArchiveSyncCursor(context.Context, Scope, string) (Cursor, error) {
	return s.cursor, nil
}
func (s *durableBridgeTestStore) PendingDurableArchiveRuns(context.Context, int) ([]DurableArchivePendingRun, error) {
	return s.pending, nil
}
func (s *durableBridgeTestStore) BusyDurableArchiveScopes(context.Context) ([]Scope, error) {
	s.busyCalls++
	return s.busy, nil
}
func (s *durableBridgeTestStore) EnqueueArchiveSync(ctx context.Context, template SyncRun, retry bool) (SyncRun, error) {
	s.lastTemplate = template
	return s.syncTestStore.EnqueueArchiveSync(ctx, template, retry)
}

func TestArchivePipelineFlagsAllowDurableScheduledMode(t *testing.T) {
	if err := ValidateArchivePipelineFlags(true, true); err != nil {
		t.Fatalf("durable scheduled mode rejected: %v", err)
	}
	if err := ValidateArchivePipelineFlags(false, false); err != nil {
		t.Fatal(err)
	}
	if got := DurableArchiveIdempotencyKey(Scope{TenantID: 11, CorpID: 27}, "wecom:ww-local", Cursor{Sequence: 41}); got != fmt.Sprintf("archive:%d:%d:%s:%d", 11, 27, "wecom:ww-local", 41) {
		t.Fatalf("idempotency key=%q", got)
	}
	if got := DurableArchivePollIdempotencyKey(Scope{TenantID: 11, CorpID: 27}, "wecom:ww-local", Cursor{Sequence: 41}, time.Date(2026, 8, 27, 12, 34, 59, 0, time.UTC)); got != "archive:poll:11:27:wecom:ww-local:41:202608271234" {
		t.Fatalf("poll idempotency key=%q", got)
	}
}
