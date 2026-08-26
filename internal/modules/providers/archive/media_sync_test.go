package archive

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"jiyi/mochat-go/internal/testfixtures/archivesource"
	"jiyi/mochat-go/internal/wecomarchivedemo"
)

func TestArchiveFixtureMediaFailuresReachWorkerTerminalStates(t *testing.T) {
	for _, test := range []struct {
		name      string
		mode      archivesource.MediaMode
		wantState ArchiveMediaStatus
	}{
		{name: "missing", mode: archivesource.MediaMissing, wantState: ArchiveMediaMissing},
		{name: "corrupt", mode: archivesource.MediaCorrupt, wantState: ArchiveMediaCorrupt},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture, err := archivesource.NewArchiveFixture()
			if err != nil {
				t.Fatal(err)
			}
			defer fixture.Close()
			sdkFileID := fixture.MediaFileIDs()["image"]
			payload := fixture.ExpectedMedia(sdkFileID)
			md5Value := md5.Sum(payload)
			fixture.SetMediaMode(sdkFileID, test.mode)
			evidence, err := wecomarchivedemo.NewEvidenceStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			archiveService, err := wecomarchivedemo.NewArchiveService(fixture, fixture.PrivateKeyPEM(), evidence, 100, 5)
			if err != nil {
				t.Fatal(err)
			}
			const token = "MOCHAT-LOCAL-ACCEPTANCE-BEARER-0123456789"
			const wxCorpID = "ww-local-acceptance"
			server := httptest.NewServer(wecomarchivedemo.NewAdminHandler(wecomarchivedemo.Config{
				AdminToken: token, CorpID: wxCorpID, PullLimit: 100, TimeoutSeconds: 5,
			}, evidence, archiveService))
			defer server.Close()
			client, err := NewBridgeArchiveClient(server.URL, token, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			store := &fakeArchiveMediaStore{object: ArchiveMediaObject{
				ID: "f5e247fc-7c99-45de-b75d-85b3be77841e", Scope: Scope{TenantID: 11, CorpID: 27},
				WXCorpID: wxCorpID, SDKFileID: sdkFileID, ExpectedSize: int64(len(payload)), ExpectedMD5: hex.EncodeToString(md5Value[:]), Status: ArchiveMediaPending,
			}}
			root := t.TempDir()
			worked, runErr := NewMediaSyncService(store, client, root).RunOne(context.Background())
			if !worked || runErr == nil {
				t.Fatalf("worked=%v err=%v", worked, runErr)
			}
			if store.object.Status != test.wantState {
				t.Fatalf("status=%s want=%s err=%v", store.object.Status, test.wantState, runErr)
			}
			parts, err := filepath.Glob(filepath.Join(root, "archive-media", store.object.ID+".attempt-*.part"))
			if err != nil || len(parts) != 0 {
				t.Fatalf("terminal attempt files=%v err=%v", parts, err)
			}
		})
	}
}

func TestMediaAttemptCleanupPreservesOnlyDatabaseReferencedCheckpoint(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "archive-media")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	id := "301c5572-0cd6-457d-a750-9b26f94e3f9c"
	fetchingID := "dc941ae0-df92-4fe4-99f4-57ecefe9b785"
	otherID := "196e9abf-51cf-4f68-b092-a08243c02db5"
	for _, path := range []string{
		filepath.Join(dir, id+".attempt-1.part"),
		filepath.Join(dir, id+".attempt-2.part"),
		filepath.Join(dir, fetchingID+".attempt-2.part"),
		filepath.Join(dir, fetchingID+".attempt-3.part"),
		filepath.Join(dir, fetchingID+".attempt-4.part"),
		filepath.Join(dir, otherID+".attempt-9.part"),
		filepath.Join(dir, "not-a-uuid.attempt-1.part"),
	} {
		if err := os.WriteFile(path, []byte("local"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	store := &fakeArchiveMediaStore{
		object: ArchiveMediaObject{ID: id, Status: ArchiveMediaFailed, CheckpointAttempt: 1, Attempt: 2},
		references: []ArchiveMediaAttemptReference{
			{ID: id, Status: ArchiveMediaFailed, CheckpointAttempt: 1},
			{ID: fetchingID, Status: ArchiveMediaFetching, CheckpointAttempt: 3, ActiveAttempt: 4},
		},
	}
	service := NewMediaSyncService(store, &fakeArchiveMediaClient{}, root)
	removed, err := service.CleanupStaleAttempts(context.Background())
	if err != nil || removed != 2 {
		t.Fatalf("removed=%d err=%v", removed, err)
	}
	if _, err := os.Stat(filepath.Join(dir, id+".attempt-1.part")); err != nil {
		t.Fatalf("referenced failed checkpoint removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, id+".attempt-2.part")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("orphan attempt remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, fetchingID+".attempt-2.part")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("fetching orphan attempt remains: %v", err)
	}
	for _, attempt := range []int{3, 4} {
		if _, err := os.Stat(archiveMediaAttemptPath(dir, fetchingID, attempt)); err != nil {
			t.Fatalf("fetching referenced attempt %d removed: %v", attempt, err)
		}
	}
	for _, path := range []string{filepath.Join(dir, otherID+".attempt-9.part"), filepath.Join(dir, "not-a-uuid.attempt-1.part")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("cross-object or unsafe path was removed: %s: %v", path, err)
		}
	}
}

func TestMediaSyncCommitsFinishedCheckpointWithoutRefetchOrDuplicate(t *testing.T) {
	for _, test := range []struct {
		name         string
		withMetadata bool
	}{
		{name: "declared size and md5", withMetadata: true},
		{name: "no expected metadata", withMetadata: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			id := "aa77df33-47c9-4e9f-ab09-4b0b55d94af3"
			payload := []byte("complete-checkpoint-payload")
			dir := filepath.Join(root, "archive-media")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, id+".attempt-3.part"), payload, 0o600); err != nil {
				t.Fatal(err)
			}
			sha := sha256.Sum256(payload)
			object := ArchiveMediaObject{
				ID: id, Scope: Scope{TenantID: 11, CorpID: 27}, WXCorpID: "ww", SDKFileID: "internal",
				Status: ArchiveMediaFetching, BytesReceived: int64(len(payload)), Attempt: 3,
				CheckpointAttempt: 3, DownloadFinished: true, DownloadSHA256: hex.EncodeToString(sha[:]),
			}
			if test.withMetadata {
				md5Value := md5.Sum(payload)
				object.ExpectedSize = int64(len(payload))
				object.ExpectedMD5 = hex.EncodeToString(md5Value[:])
			}
			store := &fakeArchiveMediaStore{object: object}
			client := &fakeArchiveMediaClient{chunks: map[string]MediaChunk{}}
			worked, err := NewMediaSyncService(store, client, root).RunOne(context.Background())
			if err != nil || !worked {
				t.Fatalf("worked=%v err=%v", worked, err)
			}
			if len(client.indexes) != 0 {
				t.Fatalf("finished checkpoint was fetched again: %v", client.indexes)
			}
			got, err := os.ReadFile(filepath.Join(dir, id))
			if err != nil || string(got) != string(payload) {
				t.Fatalf("final=%q err=%v", got, err)
			}
		})
	}
}

func TestMediaSyncTakeoverIsolatesAttemptFilesAndFencesOldResponse(t *testing.T) {
	root := t.TempDir()
	store := newTakeoverMediaStore()
	client := newTakeoverMediaClient()
	service := NewMediaSyncService(store, client, root)
	oldResult := make(chan error, 1)
	go func() {
		_, err := service.RunOne(context.Background())
		oldResult <- err
	}()
	<-client.oldFetchStarted

	if worked, err := service.RunOne(context.Background()); err != nil || !worked {
		t.Fatalf("new claim worked=%v err=%v", worked, err)
	}
	close(client.releaseOldFetch)
	if err := <-oldResult; !errors.Is(err, errTestMediaFence) {
		t.Fatalf("old worker error=%v", err)
	}
	if _, err := service.CleanupStaleAttempts(context.Background()); err != nil {
		t.Fatalf("cleanup after old worker release: %v", err)
	}
	finalPath := filepath.Join(root, "archive-media", store.object.ID)
	got, err := os.ReadFile(finalPath)
	if err != nil || string(got) != "new-claim-bytes" {
		t.Fatalf("final=%q err=%v", got, err)
	}
	old := ArchiveMediaCheckpoint{ID: store.object.ID, Attempt: 1, LeaseToken: "lease-1", BytesReceived: 9}
	if err := store.CheckpointArchiveMedia(context.Background(), old, time.Now()); !errors.Is(err, errTestMediaFence) {
		t.Fatalf("old checkpoint error=%v", err)
	}
	if err := store.CompleteArchiveMedia(context.Background(), ArchiveMediaCompletion{ID: store.object.ID, Attempt: 1, LeaseToken: "lease-1", BytesReceived: 9, SHA256: strings.Repeat("a", 64), StoragePath: finalPath}, time.Now()); !errors.Is(err, errTestMediaFence) {
		t.Fatalf("old complete error=%v", err)
	}
	if parts, err := filepath.Glob(finalPath + ".attempt-*.part"); err != nil || len(parts) != 0 {
		t.Fatalf("takeover attempt files=%v err=%v", parts, err)
	}
}

func TestMediaSyncRunOneDownloadsChunksToUUIDPath(t *testing.T) {
	root := t.TempDir()
	payload := []byte("MOCHAT-LOCAL-ACCEPTANCE deterministic media bytes")
	sum := md5.Sum(payload)
	store := &fakeArchiveMediaStore{object: ArchiveMediaObject{
		ID: "014c1da7-1b2e-4aa1-90aa-a6a0d6f53380", Scope: Scope{TenantID: 11, CorpID: 27},
		WXCorpID: "ww-local-acceptance", SDKFileID: "../../sensitive-sdk-file-id", ExpectedSize: int64(len(payload)), ExpectedMD5: hex.EncodeToString(sum[:]),
		Status: ArchiveMediaPending,
	}}
	client := &fakeArchiveMediaClient{chunks: map[string]MediaChunk{
		"":        {Data: payload[:7], NextIndexBuf: "index-1"},
		"index-1": {Data: payload[7:14], NextIndexBuf: "index-2"},
		"index-2": {Data: payload[14:], Finished: true},
	}}
	service := NewMediaSyncService(store, client, root)
	if worked, err := service.RunOne(context.Background()); err != nil || !worked {
		t.Fatalf("worked=%v err=%v", worked, err)
	}
	wantPath := filepath.Join(root, "archive-media", store.object.ID)
	if store.completed.StoragePath != wantPath || strings.Contains(store.completed.StoragePath, "sensitive") {
		t.Fatalf("storage path = %q", store.completed.StoragePath)
	}
	got, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) || len(store.checkpoints) != 3 {
		t.Fatalf("payload=%q checkpoints=%#v", got, store.checkpoints)
	}
	if parts, err := filepath.Glob(wantPath + ".attempt-*.part"); err != nil || len(parts) != 0 {
		t.Fatalf("part files remain: %v err=%v", parts, err)
	}
}

func TestMediaSyncRestartsFromCheckpointAndRejectsEmptyStall(t *testing.T) {
	root := t.TempDir()
	id := "b12b56cd-4910-4630-a399-aa906894cc76"
	dir := filepath.Join(root, "archive-media")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".attempt-1.part"), []byte("1234567"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &fakeArchiveMediaStore{object: ArchiveMediaObject{ID: id, Scope: Scope{TenantID: 11, CorpID: 27}, WXCorpID: "ww", SDKFileID: "internal", IndexBuf: "index-1", BytesReceived: 7, CheckpointAttempt: 1, Attempt: 1, Status: ArchiveMediaFetching}}
	client := &fakeArchiveMediaClient{chunks: map[string]MediaChunk{"index-1": {Data: []byte("89"), Finished: true}}}
	if worked, err := NewMediaSyncService(store, client, root).RunOne(context.Background()); err != nil || !worked {
		t.Fatalf("restart worked=%v err=%v", worked, err)
	}
	if string(client.indexes[0]) != "index-1" || store.completed.BytesReceived != 9 {
		t.Fatalf("indexes=%v completed=%#v", client.indexes, store.completed)
	}

	stallStore := &fakeArchiveMediaStore{object: ArchiveMediaObject{ID: "d3ab32aa-dc97-4321-bd4c-8abc3445a82d", Scope: Scope{TenantID: 11, CorpID: 27}, WXCorpID: "ww", SDKFileID: "internal", Status: ArchiveMediaPending}}
	stallClient := &fakeArchiveMediaClient{chunks: map[string]MediaChunk{"": {Data: nil, NextIndexBuf: ""}}}
	if worked, err := NewMediaSyncService(stallStore, stallClient, root).RunOne(context.Background()); err == nil || !worked {
		t.Fatalf("stall worked=%v err=%v", worked, err)
	}
	if stallStore.failed.ErrorCode != "archive.media_cursor_stalled" {
		t.Fatalf("failed = %#v", stallStore.failed)
	}
}

func TestMediaSyncClassifiesMissingAndCorruptWithoutExposingIdentifier(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want ArchiveMediaStatus
	}{
		{name: "missing", err: &MediaFetchError{Code: "ARCHIVE_MEDIA_NOT_FOUND"}, want: ArchiveMediaMissing},
		{name: "corrupt", err: &MediaFetchError{Code: "ARCHIVE_MEDIA_CORRUPT"}, want: ArchiveMediaCorrupt},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeArchiveMediaStore{object: ArchiveMediaObject{ID: "86c21f45-245f-4358-b9b9-224235d67288", Scope: Scope{TenantID: 11, CorpID: 27}, WXCorpID: "ww", SDKFileID: "do-not-leak-this-id", Status: ArchiveMediaPending}}
			client := &fakeArchiveMediaClient{err: test.err}
			_, err := NewMediaSyncService(store, client, t.TempDir()).RunOne(context.Background())
			if err == nil || strings.Contains(err.Error(), store.object.SDKFileID) {
				t.Fatalf("error = %v", err)
			}
			if test.want == ArchiveMediaMissing && store.missing.ID == "" {
				t.Fatalf("missing not recorded: %#v", store)
			}
			if test.want == ArchiveMediaCorrupt && store.corrupt.ID == "" {
				t.Fatalf("corrupt not recorded: %#v", store)
			}
		})
	}
}

func TestMediaSyncMarksDeclaredSizeAndMD5MismatchCorruptAndRemovesPart(t *testing.T) {
	for _, test := range []struct {
		name         string
		expectedSize int64
		expectedMD5  string
	}{
		{name: "size", expectedSize: 99},
		{name: "md5", expectedSize: 7, expectedMD5: strings.Repeat("a", 32)},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			id := "ea14e040-2ac4-4edc-bb31-20bbab28d5d3"
			store := &fakeArchiveMediaStore{object: ArchiveMediaObject{ID: id, Scope: Scope{TenantID: 11, CorpID: 27}, WXCorpID: "ww", SDKFileID: "internal", ExpectedSize: test.expectedSize, ExpectedMD5: test.expectedMD5, Status: ArchiveMediaPending}}
			client := &fakeArchiveMediaClient{chunks: map[string]MediaChunk{"": {Data: []byte("1234567"), Finished: true}}}
			if worked, err := NewMediaSyncService(store, client, root).RunOne(context.Background()); err == nil || !worked {
				t.Fatalf("worked=%v err=%v", worked, err)
			}
			if store.corrupt.ErrorCode != "archive.media_integrity_mismatch" {
				t.Fatalf("corrupt=%#v", store.corrupt)
			}
			if _, err := os.Stat(filepath.Join(root, "archive-media", id+".part")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("terminal corrupt part remains: %v", err)
			}
		})
	}
}

func TestMediaSyncRecoversRenameCompletedBeforeLedgerCompletion(t *testing.T) {
	root := t.TempDir()
	id := "f8dcda15-9b44-4cea-ae1e-5cab7da34ea1"
	store := &fakeArchiveMediaStore{object: ArchiveMediaObject{ID: id, Scope: Scope{TenantID: 11, CorpID: 27}, WXCorpID: "ww", SDKFileID: "internal", ExpectedSize: 7, Status: ArchiveMediaPending}, completeErr: errors.New("ledger unavailable")}
	client := &fakeArchiveMediaClient{chunks: map[string]MediaChunk{"": {Data: []byte("1234567"), Finished: true}}}
	service := NewMediaSyncService(store, client, root)
	if worked, err := service.RunOne(context.Background()); err == nil || !worked {
		t.Fatalf("first worked=%v err=%v", worked, err)
	}
	store.claimed = false
	store.completeErr = nil
	if worked, err := service.RunOne(context.Background()); err != nil || !worked {
		t.Fatalf("recovery worked=%v err=%v", worked, err)
	}
	if store.completed.StoragePath != filepath.Join(root, "archive-media", id) {
		t.Fatalf("completion=%#v", store.completed)
	}
}

type fakeArchiveMediaStore struct {
	object      ArchiveMediaObject
	claimed     bool
	checkpoints []ArchiveMediaCheckpoint
	completed   ArchiveMediaCompletion
	failed      ArchiveMediaFailure
	missing     ArchiveMediaFailure
	corrupt     ArchiveMediaFailure
	completeErr error
	references  []ArchiveMediaAttemptReference
}

func (s *fakeArchiveMediaStore) ClaimArchiveMedia(context.Context, time.Time) (ArchiveMediaObject, bool, error) {
	if s.claimed {
		return ArchiveMediaObject{}, false, nil
	}
	s.claimed = true
	s.object.Attempt++
	s.object.LeaseToken = "lease-token"
	return s.object, true, nil
}
func (s *fakeArchiveMediaStore) RenewArchiveMedia(context.Context, string, int, string, time.Time) error {
	return nil
}
func (s *fakeArchiveMediaStore) CheckpointArchiveMedia(_ context.Context, checkpoint ArchiveMediaCheckpoint, _ time.Time) error {
	s.checkpoints = append(s.checkpoints, checkpoint)
	s.object.IndexBuf = checkpoint.NextIndexBuf
	s.object.BytesReceived = checkpoint.BytesReceived
	s.object.CheckpointAttempt = checkpoint.Attempt
	s.object.DownloadFinished = checkpoint.Finished
	s.object.DownloadSHA256 = checkpoint.SHA256
	return nil
}
func (s *fakeArchiveMediaStore) CompleteArchiveMedia(_ context.Context, value ArchiveMediaCompletion, _ time.Time) error {
	s.completed = value
	if s.completeErr == nil {
		s.object.Status = ArchiveMediaReady
	}
	return s.completeErr
}
func (s *fakeArchiveMediaStore) FailArchiveMedia(_ context.Context, value ArchiveMediaFailure, _ time.Time) error {
	s.failed = value
	s.object.Status = ArchiveMediaFailed
	return nil
}
func (s *fakeArchiveMediaStore) MarkArchiveMediaMissing(_ context.Context, value ArchiveMediaFailure, _ time.Time) error {
	s.missing = value
	s.object.Status = ArchiveMediaMissing
	return nil
}
func (s *fakeArchiveMediaStore) MarkArchiveMediaCorrupt(_ context.Context, value ArchiveMediaFailure, _ time.Time) error {
	s.corrupt = value
	s.object.Status = ArchiveMediaCorrupt
	return nil
}
func (s *fakeArchiveMediaStore) ArchiveMediaAttemptReferences(context.Context) ([]ArchiveMediaAttemptReference, error) {
	if s.references != nil {
		return append([]ArchiveMediaAttemptReference(nil), s.references...), nil
	}
	return []ArchiveMediaAttemptReference{{
		ID: s.object.ID, Status: s.object.Status, CheckpointAttempt: s.object.CheckpointAttempt, ActiveAttempt: s.object.Attempt,
	}}, nil
}

type fakeArchiveMediaClient struct {
	chunks  map[string]MediaChunk
	err     error
	indexes []string
}

func (c *fakeArchiveMediaClient) FetchMedia(_ context.Context, _ Scope, _ string, _ string, indexBuf string) (MediaChunk, error) {
	c.indexes = append(c.indexes, indexBuf)
	if c.err != nil {
		return MediaChunk{}, c.err
	}
	return c.chunks[indexBuf], nil
}

var errTestMediaFence = errors.New("test media fence rejected")

type takeoverMediaStore struct {
	mu            sync.Mutex
	object        ArchiveMediaObject
	claims        int
	activeAttempt int
	activeToken   string
}

func newTakeoverMediaStore() *takeoverMediaStore {
	return &takeoverMediaStore{object: ArchiveMediaObject{
		ID: "ce0c8c82-1231-4499-bba8-d0de7c4070ac", Scope: Scope{TenantID: 11, CorpID: 27},
		WXCorpID: "ww", SDKFileID: "internal", Status: ArchiveMediaPending,
	}}
}

func (s *takeoverMediaStore) ClaimArchiveMedia(context.Context, time.Time) (ArchiveMediaObject, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.claims >= 2 {
		return ArchiveMediaObject{}, false, nil
	}
	s.claims++
	s.activeAttempt = s.claims
	s.activeToken = "lease-" + string(rune('0'+s.claims))
	value := s.object
	value.Attempt = s.activeAttempt
	value.LeaseToken = s.activeToken
	value.Status = ArchiveMediaFetching
	return value, true, nil
}

func (s *takeoverMediaStore) fenced(attempt int, token string) error {
	if attempt != s.activeAttempt || token != s.activeToken {
		return errTestMediaFence
	}
	return nil
}
func (s *takeoverMediaStore) RenewArchiveMedia(_ context.Context, _ string, attempt int, token string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fenced(attempt, token)
}
func (s *takeoverMediaStore) CheckpointArchiveMedia(_ context.Context, value ArchiveMediaCheckpoint, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fenced(value.Attempt, value.LeaseToken); err != nil {
		return err
	}
	s.object.IndexBuf = value.NextIndexBuf
	s.object.BytesReceived = value.BytesReceived
	s.object.CheckpointAttempt = value.Attempt
	s.object.DownloadFinished = value.Finished
	s.object.DownloadSHA256 = value.SHA256
	return nil
}
func (s *takeoverMediaStore) CompleteArchiveMedia(_ context.Context, value ArchiveMediaCompletion, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fenced(value.Attempt, value.LeaseToken); err != nil {
		return err
	}
	s.object.Status = ArchiveMediaReady
	return nil
}
func (s *takeoverMediaStore) FailArchiveMedia(_ context.Context, value ArchiveMediaFailure, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fenced(value.Attempt, value.LeaseToken)
}
func (s *takeoverMediaStore) MarkArchiveMediaMissing(ctx context.Context, value ArchiveMediaFailure, at time.Time) error {
	return s.FailArchiveMedia(ctx, value, at)
}
func (s *takeoverMediaStore) MarkArchiveMediaCorrupt(ctx context.Context, value ArchiveMediaFailure, at time.Time) error {
	return s.FailArchiveMedia(ctx, value, at)
}
func (s *takeoverMediaStore) ArchiveMediaAttemptReferences(context.Context) ([]ArchiveMediaAttemptReference, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return []ArchiveMediaAttemptReference{{
		ID: s.object.ID, Status: s.object.Status, CheckpointAttempt: s.object.CheckpointAttempt, ActiveAttempt: s.activeAttempt,
	}}, nil
}

type takeoverMediaClient struct {
	mu              sync.Mutex
	calls           int
	oldFetchStarted chan struct{}
	releaseOldFetch chan struct{}
}

func newTakeoverMediaClient() *takeoverMediaClient {
	return &takeoverMediaClient{oldFetchStarted: make(chan struct{}), releaseOldFetch: make(chan struct{})}
}

func (c *takeoverMediaClient) FetchMedia(context.Context, Scope, string, string, string) (MediaChunk, error) {
	c.mu.Lock()
	c.calls++
	call := c.calls
	c.mu.Unlock()
	if call == 1 {
		close(c.oldFetchStarted)
		<-c.releaseOldFetch
		return MediaChunk{Data: []byte("old-bytes"), Finished: true}, nil
	}
	return MediaChunk{Data: []byte("new-claim-bytes"), Finished: true}, nil
}
