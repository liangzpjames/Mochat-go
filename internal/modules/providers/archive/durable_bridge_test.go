package archive

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
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
	runner := NewDurableBridgeRunner(store, client, 10)
	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.upserts) != 1 || store.upserts[0].Seq != 42 {
		t.Fatalf("upserts=%#v", store.upserts)
	}
	wantKey := "archive:11:27:wecom:ww-local:41"
	if store.lastTemplate.IdempotencyKey != wantKey || store.lastTemplate.Cursor.Sequence != 41 {
		t.Fatalf("template=%#v want key=%q", store.lastTemplate, wantKey)
	}
}

type durableBridgeTestStore struct {
	*syncTestStore
	cursor       Cursor
	bindings     []DurableArchiveBinding
	lastTemplate SyncRun
}

func (s *durableBridgeTestStore) DurableArchiveBindings(context.Context) ([]DurableArchiveBinding, error) {
	return s.bindings, nil
}
func (s *durableBridgeTestStore) LatestArchiveSyncCursor(context.Context, Scope, string) (Cursor, error) {
	return s.cursor, nil
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
}
