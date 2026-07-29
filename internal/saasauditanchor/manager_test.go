package saasauditanchor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	chains          []Chain
	checkpoints     []Checkpoint
	chainError      error
	nextOperationID int64
}

type fakeRemoteStore struct {
	objects map[string][]byte
	fail    error
}

func newFakeRemoteStore() *fakeRemoteStore {
	return &fakeRemoteStore{objects: map[string][]byte{}}
}

func (s *fakeRemoteStore) Provider() string            { return "test-object-lock" }
func (s *fakeRemoteStore) Bucket() string              { return "audit-anchor-test" }
func (s *fakeRemoteStore) Prefix() string              { return "test/audit-anchors" }
func (s *fakeRemoteStore) RetentionMode() string       { return "compliance" }
func (s *fakeRemoteStore) RetentionDays() int          { return 3650 }
func (s *fakeRemoteStore) Probe(context.Context) error { return s.fail }

func (s *fakeRemoteStore) Ensure(_ context.Context, name string, content []byte, sha string, signedAt time.Time, existing RemoteArtifact) (RemoteArtifact, error) {
	if s.fail != nil {
		return RemoteArtifact{}, s.fail
	}
	key := existing.ObjectKey
	if key == "" {
		key = "test/audit-anchors/" + name
	}
	s.objects[key] = append([]byte(nil), content...)
	now := signedAt.UTC().Add(time.Hour)
	return RemoteArtifact{
		Provider: s.Provider(), Bucket: s.Bucket(), ObjectKey: key, ETag: "etag-test",
		VersionID: "version-test", SHA256: sha, SizeBytes: int64(len(content)),
		RetentionMode: s.RetentionMode(), RetainUntil: now.Add(3650 * 24 * time.Hour).Format(time.RFC3339),
		ExportedAt: now, VerifiedAt: now,
	}, nil
}

func (s *fakeRemoteStore) Verify(_ context.Context, artifact RemoteArtifact, content []byte, sha string) (RemoteArtifact, error) {
	if s.fail != nil {
		return RemoteArtifact{}, s.fail
	}
	actual, ok := s.objects[artifact.ObjectKey]
	if !ok {
		return RemoteArtifact{}, ErrRemoteArtifactNotFound
	}
	if string(actual) != string(content) || artifact.SHA256 != sha {
		return RemoteArtifact{}, errors.New("remote content mismatch")
	}
	artifact.VerifiedAt = time.Date(2026, 7, 13, 4, 0, 0, 0, time.UTC)
	return artifact, nil
}

func (s *fakeRemoteStore) ListObjectKeys(context.Context) ([]string, error) {
	if s.fail != nil {
		return nil, s.fail
	}
	items := make([]string, 0, len(s.objects))
	for key := range s.objects {
		items = append(items, key)
	}
	return items, nil
}

func (s *fakeStore) VerifyAuditAnchorChains(_ context.Context, _ CreateOptions) (AuditVerification, error) {
	result := AuditVerification{ScannedChains: len(s.chains), Chains: append([]Chain(nil), s.chains...)}
	for _, chain := range s.chains {
		if chain.Status == "failed" {
			result.FailedChains++
		} else {
			result.HealthyChains++
		}
	}
	return result, nil
}

func (s *fakeStore) AuditAnchorCheckpointSummary(_ context.Context, tenantID int) (CheckpointSummary, error) {
	var result CheckpointSummary
	for _, item := range s.checkpoints {
		if tenantID > 0 && tenantID != item.TenantID {
			continue
		}
		result.CheckpointCount++
		switch item.ArtifactStatus {
		case ArtifactStatusExported:
			result.ExportedCount++
		case ArtifactStatusFailed:
			result.ArtifactFailedCount++
		}
		switch item.VerificationStatus {
		case VerifyStatusPassed:
			result.VerificationPassedCount++
		case VerifyStatusFailed:
			result.VerificationFailedCount++
		default:
			result.PendingVerificationCount++
		}
	}
	return result, nil
}

func (s *fakeStore) AuditAnchorCheckpoints(_ context.Context, tenantID, limit int) ([]Checkpoint, error) {
	items := []Checkpoint{}
	for _, item := range s.checkpoints {
		if tenantID > 0 && tenantID != item.TenantID {
			continue
		}
		items = append(items, item)
		if len(items) == limit {
			break
		}
	}
	return items, nil
}

func (s *fakeStore) AuditAnchorCheckpointsPendingRemote(_ context.Context, tenantID, limit int) ([]Checkpoint, error) {
	items := []Checkpoint{}
	for _, item := range s.checkpoints {
		if tenantID > 0 && tenantID != item.TenantID {
			continue
		}
		if item.RemoteStatus == RemoteStatusExported && item.RemoteObjectKey != "" {
			continue
		}
		items = append(items, item)
		if len(items) == limit {
			break
		}
	}
	return items, nil
}

func (s *fakeStore) AuditAnchorCheckpointKeyIDs(context.Context) ([]string, error) {
	seen := map[string]struct{}{}
	result := []string{}
	for _, item := range s.checkpoints {
		if _, ok := seen[item.KeyID]; ok {
			continue
		}
		seen[item.KeyID] = struct{}{}
		result = append(result, item.KeyID)
	}
	return result, nil
}

func (s *fakeStore) AuditAnchorCheckpointNos(context.Context) ([]string, error) {
	result := make([]string, 0, len(s.checkpoints))
	for _, item := range s.checkpoints {
		result = append(result, item.CheckpointNo)
	}
	return result, nil
}

func (s *fakeStore) AuditAnchorRemoteObjectKeys(context.Context) ([]string, error) {
	result := []string{}
	for _, item := range s.checkpoints {
		if item.RemoteObjectKey != "" {
			result = append(result, item.RemoteObjectKey)
		}
	}
	return result, nil
}

func (s *fakeStore) AuditAnchorCheckpointByFingerprint(_ context.Context, fingerprint string) (Checkpoint, bool, error) {
	for _, item := range s.checkpoints {
		if item.Fingerprint == fingerprint {
			return item, true, nil
		}
	}
	return Checkpoint{}, false, nil
}

func (s *fakeStore) CreateAuditAnchorCheckpoint(_ context.Context, input CheckpointCreate) (Checkpoint, bool, error) {
	if item, ok, _ := s.AuditAnchorCheckpointByFingerprint(context.Background(), input.Checkpoint.Fingerprint); ok {
		return item, false, nil
	}
	item := input.Checkpoint
	item.ID = int64(len(s.checkpoints) + 1)
	item.CreatedAt = "2026-07-13 02:00:00"
	s.checkpoints = append(s.checkpoints, item)
	return item, true, nil
}

func (s *fakeStore) UpdateAuditAnchorArtifact(_ context.Context, update ArtifactUpdate) (Checkpoint, error) {
	for index := range s.checkpoints {
		if s.checkpoints[index].ID != update.ID {
			continue
		}
		s.checkpoints[index].ArtifactStatus = update.Status
		s.checkpoints[index].ArtifactName = update.ArtifactName
		s.checkpoints[index].ArtifactSHA256 = update.ArtifactSHA256
		s.checkpoints[index].ArtifactError = update.ErrorMessage
		if !update.ExportedAt.IsZero() {
			s.checkpoints[index].ExportedAt = update.ExportedAt.Format(time.RFC3339)
		}
		return s.checkpoints[index], nil
	}
	return Checkpoint{}, errors.New("not found")
}

func (s *fakeStore) UpdateAuditAnchorRemoteArtifact(_ context.Context, update RemoteArtifactUpdate) (Checkpoint, error) {
	for index := range s.checkpoints {
		if s.checkpoints[index].ID != update.ID {
			continue
		}
		item := &s.checkpoints[index]
		item.RemoteStatus = update.Status
		item.RemoteProvider = update.Artifact.Provider
		item.RemoteBucket = update.Artifact.Bucket
		item.RemoteObjectKey = update.Artifact.ObjectKey
		item.RemoteETag = update.Artifact.ETag
		item.RemoteVersionID = update.Artifact.VersionID
		item.RemoteSHA256 = update.Artifact.SHA256
		item.RemoteSizeBytes = update.Artifact.SizeBytes
		item.RemoteRetentionMode = update.Artifact.RetentionMode
		item.RemoteRetainUntil = update.Artifact.RetainUntil
		item.RemoteError = update.ErrorMessage
		if !update.Artifact.ExportedAt.IsZero() {
			item.RemoteExportedAt = update.Artifact.ExportedAt.Format(time.RFC3339)
		}
		if !update.Artifact.VerifiedAt.IsZero() {
			item.RemoteVerifiedAt = update.Artifact.VerifiedAt.Format(time.RFC3339)
		}
		return *item, nil
	}
	return Checkpoint{}, errors.New("not found")
}

func (s *fakeStore) UpdateAuditAnchorVerification(_ context.Context, update VerificationUpdate) (Checkpoint, error) {
	for index := range s.checkpoints {
		if s.checkpoints[index].ID != update.ID {
			continue
		}
		s.checkpoints[index].VerificationStatus = update.Status
		s.checkpoints[index].VerificationError = update.ErrorMessage
		s.checkpoints[index].LastVerifiedAt = update.VerifiedAt.Format(time.RFC3339)
		return s.checkpoints[index], nil
	}
	return Checkpoint{}, errors.New("not found")
}

func (s *fakeStore) CheckAuditAnchorCheckpointChain(context.Context, Checkpoint) error {
	return s.chainError
}

func (s *fakeStore) RecordAuditAnchorOperation(context.Context, OperationRecord) (int64, error) {
	s.nextOperationID++
	return s.nextOperationID, nil
}

func TestManagerCreateVerifyAndDetectArtifactTampering(t *testing.T) {
	root := t.TempDir()
	store := &fakeStore{chains: []Chain{{
		TenantID: 8, TenantName: "测试租户", ChainVersion: 1,
		AnchorLogID: 10, AnchorHash: strings.Repeat("a", 64), LegacyLogCount: 10,
		ChainHeadLogID: 12, ChainHeadHash: strings.Repeat("b", 64), SignedLogCount: 2,
		Status: "healthy",
	}}}
	manager, err := NewManager(store, Config{
		ArtifactRoot: root, HMACKey: strings.Repeat("07", 32), HMACKeyID: "2026-q3",
	})
	if err != nil {
		t.Fatal(err)
	}
	manager.clock = func() time.Time { return time.Date(2026, 7, 13, 2, 0, 0, 0, time.UTC) }
	created, err := manager.Create(context.Background(), CreateOptions{Limit: 10, Source: TriggerManual})
	if err != nil {
		t.Fatal(err)
	}
	if created.CreatedCheckpoints != 1 || created.ExportedArtifacts != 1 || len(store.checkpoints) != 1 {
		t.Fatalf("unexpected create result: %+v", created)
	}
	artifactPath := filepath.Join(root, store.checkpoints[0].ArtifactName)
	if info, err := os.Stat(artifactPath); err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0600) {
		t.Fatalf("artifact mode or stat: info=%v err=%v", info, err)
	}
	verified, err := manager.Verify(context.Background(), VerifyOptions{Limit: 10, Source: TriggerManual})
	if err != nil {
		t.Fatal(err)
	}
	if verified.PassedCheckpoints != 1 || verified.FailedCheckpoints != 0 {
		t.Fatalf("unexpected verify result: %+v", verified)
	}
	if err := os.WriteFile(artifactPath, []byte("tampered\n"), 0600); err != nil {
		t.Fatal(err)
	}
	verified, err = manager.Verify(context.Background(), VerifyOptions{Limit: 10, Source: TriggerManual})
	if err != nil {
		t.Fatal(err)
	}
	if verified.FailedCheckpoints != 1 || store.checkpoints[0].VerificationStatus != VerifyStatusFailed {
		t.Fatalf("artifact tampering was not detected: %+v", verified)
	}
}

func TestManagerCreateIsIdempotentAndDetectsDatabaseRollback(t *testing.T) {
	root := t.TempDir()
	store := &fakeStore{chains: []Chain{{
		TenantID: 1, ChainVersion: 1, AnchorHash: strings.Repeat("0", 64),
		ChainHeadHash: strings.Repeat("0", 64), Status: "healthy",
	}}}
	manager, err := NewManager(store, Config{ArtifactRoot: root, HMACKey: strings.Repeat("08", 32)})
	if err != nil {
		t.Fatal(err)
	}
	manager.clock = func() time.Time { return time.Date(2026, 7, 13, 3, 0, 0, 0, time.UTC) }
	if _, err := manager.Create(context.Background(), CreateOptions{Limit: 10}); err != nil {
		t.Fatal(err)
	}
	second, err := manager.Create(context.Background(), CreateOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if second.CreatedCheckpoints != 0 || second.ExistingCheckpoints != 1 || len(store.checkpoints) != 1 {
		t.Fatalf("create was not idempotent: %+v", second)
	}
	store.checkpoints = nil
	verified, err := manager.Verify(context.Background(), VerifyOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if verified.OrphanArtifactCount != 1 || verified.FailedCheckpoints != 1 {
		t.Fatalf("database rollback was not detected: %+v", verified)
	}
}

func TestManagerRejectsCreateWithoutHMACKey(t *testing.T) {
	manager, err := NewManager(&fakeStore{}, Config{ArtifactRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Create(context.Background(), CreateOptions{Limit: 10})
	var opErr *OperationError
	if !errors.As(err, &opErr) || opErr.Code != "unavailable" {
		t.Fatalf("expected unavailable error, got %v", err)
	}
}

func TestManagerCreatesAndVerifiesRequiredRemoteArtifact(t *testing.T) {
	remote := newFakeRemoteStore()
	store := &fakeStore{chains: []Chain{{
		TenantID: 9, ChainVersion: 1, AnchorHash: strings.Repeat("1", 64),
		ChainHeadHash: strings.Repeat("1", 64), Status: "healthy",
	}}}
	manager, err := NewManager(store, Config{
		ArtifactRoot: t.TempDir(), HMACKey: strings.Repeat("09", 32),
		Remote: remote, RequireRemote: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	manager.clock = func() time.Time { return time.Date(2026, 7, 13, 4, 0, 0, 0, time.UTC) }
	created, err := manager.Create(context.Background(), CreateOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if created.RemoteExportedArtifacts != 1 || store.checkpoints[0].RemoteStatus != RemoteStatusExported {
		t.Fatalf("remote artifact was not exported: result=%+v checkpoint=%+v", created, store.checkpoints[0])
	}
	verified, err := manager.Verify(context.Background(), VerifyOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if verified.FailedCheckpoints != 0 || verified.PassedCheckpoints != 1 {
		t.Fatalf("remote artifact verification failed: %+v", verified)
	}
	delete(remote.objects, store.checkpoints[0].RemoteObjectKey)
	verified, err = manager.Verify(context.Background(), VerifyOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if verified.FailedCheckpoints != 1 || verified.MissingRemoteCount != 1 {
		t.Fatalf("missing remote artifact was not detected: %+v", verified)
	}
}

func TestManagerBackfillsHistoricalCheckpointsToRemoteStore(t *testing.T) {
	root := t.TempDir()
	store := &fakeStore{chains: []Chain{{
		TenantID: 12, ChainVersion: 1, AnchorHash: strings.Repeat("2", 64),
		ChainHeadHash: strings.Repeat("2", 64), Status: "healthy",
	}}}
	localManager, err := NewManager(store, Config{
		ArtifactRoot: root, HMACKey: strings.Repeat("0b", 32), HMACKeyID: "preview-q3",
	})
	if err != nil {
		t.Fatal(err)
	}
	localManager.clock = func() time.Time { return time.Date(2026, 7, 13, 2, 0, 0, 0, time.UTC) }
	if _, err := localManager.Create(context.Background(), CreateOptions{Limit: 10}); err != nil {
		t.Fatal(err)
	}
	if store.checkpoints[0].RemoteStatus != RemoteStatusDisabled {
		t.Fatalf("historical checkpoint should start without remote evidence: %+v", store.checkpoints[0])
	}

	store.chains[0].ChainHeadLogID = 1
	store.chains[0].ChainHeadHash = strings.Repeat("3", 64)
	store.chains[0].SignedLogCount = 1
	remote := newFakeRemoteStore()
	remoteManager, err := NewManager(store, Config{
		ArtifactRoot: root, HMACKey: strings.Repeat("0b", 32), HMACKeyID: "preview-q3",
		Remote: remote, RequireRemote: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	remoteManager.clock = func() time.Time { return time.Date(2026, 7, 13, 3, 0, 0, 0, time.UTC) }
	created, err := remoteManager.Create(context.Background(), CreateOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if created.CreatedCheckpoints != 1 || created.BackfilledCheckpoints != 1 || created.RemoteExportedArtifacts != 2 {
		t.Fatalf("historical remote backfill result = %+v", created)
	}
	for _, item := range store.checkpoints {
		if item.RemoteStatus != RemoteStatusExported || item.RemoteObjectKey == "" {
			t.Fatalf("checkpoint was not backfilled: %+v", item)
		}
	}
	verified, err := remoteManager.Verify(context.Background(), VerifyOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if verified.PassedCheckpoints != 2 || verified.FailedCheckpoints != 0 {
		t.Fatalf("backfilled checkpoints did not verify: %+v", verified)
	}
}

func TestManagerDetectsOrphanRemoteArtifact(t *testing.T) {
	remote := newFakeRemoteStore()
	remote.objects["test/audit-anchors/AAN-AAAAAAAAAAAAAAAAAAAAAAAA.json"] = []byte("orphan")
	manager, err := NewManager(&fakeStore{}, Config{
		ArtifactRoot: t.TempDir(), HMACKey: strings.Repeat("0a", 32), Remote: remote,
	})
	if err != nil {
		t.Fatal(err)
	}
	verified, err := manager.Verify(context.Background(), VerifyOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if verified.OrphanRemoteCount != 1 || verified.FailedCheckpoints != 1 {
		t.Fatalf("orphan remote artifact was not detected: %+v", verified)
	}
}
