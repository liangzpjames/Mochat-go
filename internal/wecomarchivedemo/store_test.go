package wecomarchivedemo

import (
	"bufio"
	"os"
	"path/filepath"
	"sync"
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

func TestEvidenceStoreRejectsConcurrentCrossCorpBinding(t *testing.T) {
	store, err := NewEvidenceStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, receiveID := range []string{"ww-a", "ww-b"} {
		go func(value string) {
			<-start
			results <- store.BindReceiveID(value)
		}(receiveID)
	}
	close(start)
	first, second := <-results, <-results
	if (first == nil) == (second == nil) {
		t.Fatalf("binding results = %v, %v; want exactly one success", first, second)
	}
	state, err := store.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if state.BoundReceiveID != "ww-a" && state.BoundReceiveID != "ww-b" {
		t.Fatalf("bound receive ID = %q", state.BoundReceiveID)
	}
}

func TestEvidenceStoreCountsConcurrentCallbacksWithoutLostUpdates(t *testing.T) {
	dir := t.TempDir()
	store, err := NewEvidenceStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	const total = 40
	var wait sync.WaitGroup
	errorsChannel := make(chan error, total)
	for index := 0; index < total; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			errorsChannel <- store.RecordCallback(CallbackEvidence{ReceivedAt: time.Now().UTC(), ReceiveID: "ww-test", EventPath: "event.test", ContentSHA256: "abc"})
		}()
	}
	wait.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		if err != nil {
			t.Fatal(err)
		}
	}
	state, err := store.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if state.CallbackCount != total {
		t.Fatalf("callback count = %d, want %d", state.CallbackCount, total)
	}
	file, err := os.Open(filepath.Join(dir, "callback-events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	lines := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if lines != total {
		t.Fatalf("evidence lines = %d, want %d", lines, total)
	}
}
