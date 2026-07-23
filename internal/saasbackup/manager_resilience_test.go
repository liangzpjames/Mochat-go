package saasbackup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type resilienceStore struct {
	policy            Policy
	run               BackupRun
	runs              []BackupRun
	verificationCalls int
}

func (s *resilienceStore) BackupPolicy(context.Context) (Policy, error) { return s.policy, nil }
func (s *resilienceStore) UpdateBackupPolicy(context.Context, PolicyUpdate) (Policy, error) {
	return Policy{}, nil
}
func (s *resilienceStore) BackupSourceMetadata(context.Context) (SourceMetadata, error) {
	return SourceMetadata{}, nil
}
func (s *resilienceStore) StartBackupRun(context.Context, BackupStart) (BackupRun, error) {
	return BackupRun{}, nil
}
func (s *resilienceStore) CompleteBackupRun(context.Context, BackupCompletion) (BackupRun, error) {
	return BackupRun{}, nil
}
func (s *resilienceStore) RecordBackupVerification(_ context.Context, input BackupVerification) (BackupRun, error) {
	s.verificationCalls++
	s.run.VerificationStatus = input.Status
	return s.run, nil
}
func (s *resilienceStore) StartBackupReplica(context.Context, BackupReplicaStart) (BackupRun, error) {
	return BackupRun{}, nil
}
func (s *resilienceStore) CompleteBackupReplica(context.Context, BackupReplicaCompletion) (BackupRun, error) {
	return BackupRun{}, nil
}
func (s *resilienceStore) BackupRun(context.Context, int64) (BackupRun, error) {
	return s.run, nil
}

type failingReplica struct {
	deleteCalls int
}

func (r *failingReplica) Provider() string { return "s3" }
func (r *failingReplica) Bucket() string   { return "backup-test" }
func (r *failingReplica) ObjectKey(string, time.Time) (string, error) {
	return "backups/test.mgbk", nil
}
func (r *failingReplica) Probe(context.Context) error { return nil }
func (r *failingReplica) Put(context.Context, string, string, string, int64) (ReplicaObject, error) {
	return ReplicaObject{}, errors.New("unexpected upload")
}
func (r *failingReplica) Verify(context.Context, string, string, string, int64) (ReplicaObject, error) {
	return ReplicaObject{}, errors.New("remote checksum mismatch")
}
func (r *failingReplica) Open(context.Context, string, string) (ReplicaObject, io.ReadCloser, error) {
	data := []byte("corrupt!")
	return ReplicaObject{SizeBytes: int64(len(data))}, io.NopCloser(bytes.NewReader(data)), nil
}
func (r *failingReplica) Delete(context.Context, string, string) error {
	r.deleteCalls++
	return nil
}
func (s *resilienceStore) BackupRuns(context.Context, int) ([]BackupRun, error) { return s.runs, nil }
func (s *resilienceStore) StartRestoreDrill(context.Context, RestoreDrillStart) (RestoreDrill, error) {
	return RestoreDrill{}, nil
}
func (s *resilienceStore) CompleteRestoreDrill(context.Context, RestoreDrillCompletion) (RestoreDrill, error) {
	return RestoreDrill{}, nil
}
func (s *resilienceStore) RestoreDrills(context.Context, int) ([]RestoreDrill, error) {
	return nil, nil
}
func (s *resilienceStore) MarkBackupDeleted(context.Context, int64, Actor) (BackupRun, error) {
	return BackupRun{}, nil
}
func (s *resilienceStore) ScheduleBackupCleanup(context.Context, CleanupSchedule) (CleanupRun, error) {
	return CleanupRun{}, nil
}
func (s *resilienceStore) BackupCleanupRun(context.Context, int64) (CleanupRun, error) {
	return CleanupRun{}, nil
}
func (s *resilienceStore) BackupCleanupRuns(context.Context, int) ([]CleanupRun, error) {
	return nil, nil
}
func (s *resilienceStore) BackupCleanupItems(context.Context, int64) ([]CleanupItem, error) {
	return nil, nil
}
func (s *resilienceStore) ClaimBackupCleanup(context.Context, int64, bool, Actor) (CleanupRun, []CleanupItem, error) {
	return CleanupRun{}, nil, nil
}
func (s *resilienceStore) ClaimNextBackupCleanup(context.Context, Actor) (CleanupRun, []CleanupItem, bool, error) {
	return CleanupRun{}, nil, false, nil
}
func (s *resilienceStore) CheckpointBackupCleanupItem(context.Context, CleanupCheckpoint) (CleanupItem, error) {
	return CleanupItem{}, nil
}
func (s *resilienceStore) CompleteBackupCleanupItem(context.Context, CleanupItemCompletion) (CleanupItem, error) {
	return CleanupItem{}, nil
}
func (s *resilienceStore) FinishBackupCleanup(context.Context, CleanupRunCompletion) (CleanupRun, error) {
	return CleanupRun{}, nil
}

func TestManagerUsesActiveAndHistoricalEncryptionKeys(t *testing.T) {
	store := &resilienceStore{runs: []BackupRun{
		{ID: 1, Status: RunStatusSucceeded, Encrypted: true, EncryptionKeyID: "2026-q2"},
		{ID: 2, Status: RunStatusSucceeded, Encrypted: true, EncryptionKeyID: "2026-q3"},
	}}
	manager, err := NewManager(store, Config{
		EncryptionKeyID: "2026-q3",
		EncryptionKeys:  `{"2026-q2":"0707070707070707070707070707070707070707070707070707070707070707","2026-q3":"0808080808080808080808080808080808080808080808080808080808080808"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if manager.ConfigStatus().EncryptionKeyCount != 2 || !bytes.Equal(manager.activeEncryptionKey, bytes.Repeat([]byte{8}, 32)) ||
		!bytes.Equal(manager.encryptionKeyForID("2026-q2"), bytes.Repeat([]byte{7}, 32)) {
		t.Fatalf("manager key ring was not loaded: %+v", manager.ConfigStatus())
	}
	if err := manager.CheckEncryptionKeyRing(context.Background()); err != nil {
		t.Fatal(err)
	}
	overview, err := manager.Overview(context.Background(), 10)
	if err != nil || len(overview.Runs) != 2 || !overview.Runs[0].EncryptionKeyReady || !overview.Runs[1].EncryptionKeyReady {
		t.Fatalf("overview key readiness = %+v, %v", overview.Runs, err)
	}
	store.runs = append(store.runs, BackupRun{ID: 3, Status: RunStatusSucceeded, Encrypted: true, EncryptionKeyID: "retired"})
	if err := manager.CheckEncryptionKeyRing(context.Background()); err == nil || !strings.Contains(err.Error(), "retired") {
		t.Fatalf("missing historical key error = %v", err)
	}
}

func TestManagerRejectsMissingOrConflictingActiveKey(t *testing.T) {
	store := &resilienceStore{}
	manager, err := NewManager(store, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.CheckEncryptionKeyRing(context.Background()); err == nil || !strings.Contains(err.Error(), "active") {
		t.Fatalf("missing active key health error = %v", err)
	}
	_, err = NewManager(store, Config{
		EncryptionKeyID: "current",
		EncryptionKeys:  `{"historical":"0707070707070707070707070707070707070707070707070707070707070707"}`,
	})
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("missing active key error = %v", err)
	}
	_, err = NewManager(store, Config{
		EncryptionKeyID: "current",
		EncryptionKey:   strings.Repeat("07", 32),
		EncryptionKeys:  `{"current":"0808080808080808080808080808080808080808080808080808080808080808"}`,
	})
	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("conflicting active key error = %v", err)
	}
}

func TestManagerReportsAndChecksBackupAutomation(t *testing.T) {
	store := &resilienceStore{policy: Policy{Status: PolicyStatusActive}}
	manager, err := NewManager(store, Config{CronEnabled: true, CronInterval: 5 * time.Minute, CronRunOnStart: true})
	if err != nil {
		t.Fatal(err)
	}
	status := manager.ConfigStatus()
	if !status.CronEnabled || status.CronIntervalSeconds != 300 || !status.CronRunOnStart {
		t.Fatalf("automation status = %+v", status)
	}
	if err := manager.CheckAutomation(context.Background()); err != nil {
		t.Fatal(err)
	}

	disabled, err := NewManager(store, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := disabled.CheckAutomation(context.Background()); err == nil || !strings.Contains(err.Error(), "未启用") {
		t.Fatalf("disabled automation error = %v", err)
	}
	store.policy.Status = PolicyStatusDisabled
	if err := manager.CheckAutomation(context.Background()); err == nil || !strings.Contains(err.Error(), "策略已停用") {
		t.Fatalf("disabled policy automation error = %v", err)
	}
	if _, err := NewManager(store, Config{CronEnabled: true}); err == nil || !strings.Contains(err.Error(), "interval") {
		t.Fatalf("invalid cron interval error = %v", err)
	}
}

func TestReplicateKeepsInvalidRemoteWhenNoVerifiedLocalCopyExists(t *testing.T) {
	expected := []byte("expected")
	digest := sha256.Sum256(expected)
	store := &resilienceStore{run: BackupRun{
		ID: 7, Status: RunStatusSucceeded, ArtifactName: "missing.sql.gz", SHA256: hex.EncodeToString(digest[:]),
		SizeBytes: int64(len(expected)), ReplicaStatus: ReplicaStatusSucceeded,
		ReplicaObjectKey: "backups/missing.sql.gz", VerificationStatus: VerificationFailed,
	}}
	replica := &failingReplica{}
	manager, err := NewManager(store, Config{BackupRoot: t.TempDir(), Replica: replica})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Replicate(context.Background(), store.run.ID, Actor{}); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("replicate error = %v", err)
	}
	if replica.deleteCalls != 0 {
		t.Fatalf("invalid remote was deleted before a local copy was verified: calls=%d", replica.deleteCalls)
	}
}
