package wecomarchivedemo

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEvidenceStorePersistsStateAndJSONL(t *testing.T) {
	dir := t.TempDir()
	store, err := NewEvidenceStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := State{BoundReceiveID: "ww-test", Seq: 42, CallbackCount: 1, LastCallbackAt: time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)}
	if err := store.SaveState(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if got.BoundReceiveID != want.BoundReceiveID || got.Seq != want.Seq || got.CallbackCount != want.CallbackCount || !got.LastCallbackAt.Equal(want.LastCallbackAt) {
		t.Fatalf("state = %+v, want %+v", got, want)
	}
	if err := store.AppendCallback(CallbackEvidence{ReceivedAt: want.LastCallbackAt, ReceiveID: "ww-test", EventPath: "event.change_contact.create_user", ContentSHA256: "abc"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "callback-events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("callback evidence is empty")
	}
}
