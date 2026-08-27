package archive

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDurableBridgeRunnerUsesAuthoritativeCursorAndStableIdempotency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"messages":[{"seq":42,"msgid":"durable-42","action":"send","from":"employee","tolist":["contact"],"msgtype":"text","text":{"content":"durable"}}]}`))
	}))
	defer server.Close()
	client, err := NewBridgeArchiveClient(server.URL, "MOCHAT-LOCAL-ACCEPTANCE-BEARER-0123456789", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	store := &durableBridgeTestStore{syncTestStore: newSyncTestStore(), cursor: Cursor{Sequence: 41}, bindings: []DurableArchiveBinding{{Scope: Scope{TenantID: 11, CorpID: 27}, WXCorpID: "ww-local"}}}
	runner := NewDurableBridgeRunner(store, client, 10).WithClock(func() time.Time {
		return time.Date(2026, 8, 27, 12, 34, 10, 0, time.UTC)
	})
	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.upserts) != 1 || store.upserts[0].Seq != 42 {
		t.Fatalf("upserts=%#v", store.upserts)
	}
	wantKey := "archive:poll:11:27:wecom:ww-local:41:202608271234"
	if store.lastTemplate.IdempotencyKey != wantKey || store.lastTemplate.Cursor.Sequence != 41 {
		t.Fatalf("template=%#v want key=%q", store.lastTemplate, wantKey)
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
		Scope: Scope{TenantID: 11, CorpID: 27}, WXCorpID: "ww-local",
	}, "manual-abc-123")
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != SyncStatusQueued || serverCalls != 0 || store.lastTemplate.IdempotencyKey != "archive:manual:manual-abc-123" {
		t.Fatalf("run=%#v calls=%d template=%#v", run, serverCalls, store.lastTemplate)
	}
}

func TestDurableBridgeRunnerProcessesPendingManualRunBeforePolling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"messages":[{"seq":42,"msgid":"manual-42","action":"send","from":"employee","tolist":["contact"],"msgtype":"text","text":{"content":"manual"}}]}`))
	}))
	defer server.Close()
	client, err := NewBridgeArchiveClient(server.URL, "MOCHAT-LOCAL-ACCEPTANCE-BEARER-0123456789", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	store := &durableBridgeTestStore{
		syncTestStore: newSyncTestStore(), cursor: Cursor{Sequence: 41},
		bindings: []DurableArchiveBinding{{Scope: Scope{TenantID: 11, CorpID: 27}, WXCorpID: "ww-local"}},
		pending: []DurableArchivePendingRun{{
			Binding: DurableArchiveBinding{Scope: Scope{TenantID: 11, CorpID: 27}, WXCorpID: "ww-local"},
			Cursor:  Cursor{Sequence: 41}, IdempotencyKey: "archive:manual:manual-abc-123",
		}},
	}
	if err := NewDurableBridgeRunner(store, client, 10).RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.lastTemplate.IdempotencyKey != "archive:manual:manual-abc-123" || len(store.upserts) != 1 {
		t.Fatalf("template=%#v upserts=%#v", store.lastTemplate, store.upserts)
	}
}

type durableBridgeTestStore struct {
	*syncTestStore
	cursor       Cursor
	bindings     []DurableArchiveBinding
	lastTemplate SyncRun
	pending      []DurableArchivePendingRun
}

func (s *durableBridgeTestStore) DurableArchiveBindings(context.Context) ([]DurableArchiveBinding, error) {
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
func (s *durableBridgeTestStore) EnqueueArchiveSync(ctx context.Context, template SyncRun, retry bool) (SyncRun, error) {
	s.lastTemplate = template
	return s.syncTestStore.EnqueueArchiveSync(ctx, template, retry)
}

func TestArchivePipelineFlagsRejectLegacyAndDurableTogether(t *testing.T) {
	if err := ValidateArchivePipelineFlags(true, true); err == nil {
		t.Fatal("legacy and durable archive pipelines unexpectedly enabled together")
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
