package saasbackup

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type cleanupSagaStore struct {
	*resilienceStore
	scheduleInput CleanupSchedule
	run           CleanupRun
	item          CleanupItem
}

func (s *cleanupSagaStore) ScheduleBackupCleanup(_ context.Context, input CleanupSchedule) (CleanupRun, error) {
	s.scheduleInput = input
	s.run = CleanupRun{ID: 41, CleanupNo: input.CleanupNo, Status: CleanupStatusPending, PolicyVersion: input.Plan.PolicyVersion,
		ScannedCount: input.Plan.Scanned, CandidateCount: len(input.Plan.Items), PreservedCount: input.Plan.Preserved,
		ApprovalID: input.ApprovalExecutionID, CreatedBy: input.Actor.UserID, ActorTenantID: input.Actor.TenantID, Version: 1}
	planned := input.Plan.Items[0]
	s.item = CleanupItem{ID: 51, CleanupRunID: s.run.ID, BackupRunID: planned.BackupRunID,
		BackupRunVersion: planned.BackupRunVersion + 1, BackupNo: planned.BackupNo, ArtifactName: planned.ArtifactName,
		ReplicaObjectKey: planned.ReplicaObjectKey, ReplicaVersionID: planned.ReplicaVersionID,
		Status: CleanupItemStatusPending, ReplicaStatus: CleanupStepPending, LocalStatus: CleanupStepPending, RecordStatus: CleanupStepPending}
	return s.run, nil
}

func (s *cleanupSagaStore) BackupCleanupRun(context.Context, int64) (CleanupRun, error) {
	return s.run, nil
}
func (s *cleanupSagaStore) BackupCleanupRuns(context.Context, int) ([]CleanupRun, error) {
	if s.run.ID == 0 {
		return nil, nil
	}
	return []CleanupRun{s.run}, nil
}
func (s *cleanupSagaStore) BackupCleanupItems(context.Context, int64) ([]CleanupItem, error) {
	return []CleanupItem{s.item}, nil
}
func (s *cleanupSagaStore) ClaimBackupCleanup(_ context.Context, _ int64, retry bool, _ Actor) (CleanupRun, []CleanupItem, error) {
	if retry && s.run.Status != CleanupStatusFailed && s.run.Status != CleanupStatusPartial {
		return CleanupRun{}, nil, Conflict("not retryable")
	}
	s.run.Status = CleanupStatusRunning
	s.run.Attempts++
	return s.run, []CleanupItem{s.item}, nil
}
func (s *cleanupSagaStore) ClaimNextBackupCleanup(context.Context, Actor) (CleanupRun, []CleanupItem, bool, error) {
	return CleanupRun{}, nil, false, nil
}
func (s *cleanupSagaStore) CheckpointBackupCleanupItem(_ context.Context, input CleanupCheckpoint) (CleanupItem, error) {
	if input.Step == CleanupStepReplica {
		s.item.ReplicaStatus = input.Status
	} else {
		s.item.LocalStatus = input.Status
	}
	s.item.Attempts++
	s.item.LastError = input.ErrorMessage
	if input.Status == CleanupStepFailed {
		s.item.Status = CleanupItemStatusFailed
	} else {
		s.item.Status = CleanupItemStatusRunning
	}
	return s.item, nil
}
func (s *cleanupSagaStore) CompleteBackupCleanupItem(context.Context, CleanupItemCompletion) (CleanupItem, error) {
	s.item.Status = CleanupItemStatusSucceeded
	s.item.RecordStatus = CleanupStepDeleted
	s.item.LastError = ""
	return s.item, nil
}
func (s *cleanupSagaStore) FinishBackupCleanup(context.Context, CleanupRunCompletion) (CleanupRun, error) {
	if s.item.Status == CleanupItemStatusSucceeded {
		s.run.Status, s.run.DeletedCount, s.run.FailedCount = CleanupStatusSucceeded, 1, 0
	} else {
		s.run.Status, s.run.DeletedCount, s.run.FailedCount = CleanupStatusFailed, 0, 1
		s.run.LastError = s.item.LastError
	}
	s.run.Version++
	return s.run, nil
}

type flakyCleanupReplica struct {
	fail        bool
	deleteCalls int
}

func (r *flakyCleanupReplica) Provider() string { return "s3" }
func (r *flakyCleanupReplica) Bucket() string   { return "cleanup-test" }
func (r *flakyCleanupReplica) ObjectKey(string, time.Time) (string, error) {
	return "backups/old.mgbk", nil
}
func (r *flakyCleanupReplica) Probe(context.Context) error { return nil }
func (r *flakyCleanupReplica) Put(context.Context, string, string, string, int64) (ReplicaObject, error) {
	return ReplicaObject{}, errors.New("unexpected put")
}
func (r *flakyCleanupReplica) Verify(context.Context, string, string, string, int64) (ReplicaObject, error) {
	return ReplicaObject{}, errors.New("unexpected verify")
}
func (r *flakyCleanupReplica) Open(context.Context, string, string) (ReplicaObject, io.ReadCloser, error) {
	return ReplicaObject{}, nil, errors.New("unexpected open")
}
func (r *flakyCleanupReplica) Delete(context.Context, string, string) error {
	r.deleteCalls++
	if r.fail {
		return errors.New("object lock denied")
	}
	return nil
}

func TestPlanCleanupFreezesEligibleRunsAndPreservesNewestSuccessful(t *testing.T) {
	now := time.Now()
	store := &resilienceStore{
		policy: Policy{Version: 7, RetentionDays: 1, MinSuccessfulBackups: 1},
		runs: []BackupRun{
			{ID: 5, BackupNo: "new", Status: RunStatusSucceeded, FinishedAt: now.Add(-time.Hour).Format("2006-01-02 15:04:05"), Version: 3},
			{ID: 4, BackupNo: "old-success", Status: RunStatusSucceeded, FinishedAt: now.Add(-48 * time.Hour).Format("2006-01-02 15:04:05"), ArtifactName: "old-success.mgbk", Version: 2},
			{ID: 3, BackupNo: "old-failed", Status: RunStatusFailed, FinishedAt: now.Add(-72 * time.Hour).Format("2006-01-02 15:04:05"), ArtifactName: "old-failed.mgbk", Version: 4},
			{ID: 2, BackupNo: "bound", Status: RunStatusSucceeded, FinishedAt: now.Add(-96 * time.Hour).Format("2006-01-02 15:04:05"), CleanupRunID: 9},
			{ID: 1, BackupNo: "uploading", Status: RunStatusSucceeded, FinishedAt: now.Add(-120 * time.Hour).Format("2006-01-02 15:04:05"), ReplicaStatus: ReplicaStatusUploading},
		},
	}
	manager, err := NewManager(store, Config{BackupRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := manager.PlanCleanup(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if plan.PolicyVersion != 7 || plan.Preserved != 1 || plan.Scanned != 3 || len(plan.Items) != 2 {
		t.Fatalf("plan = %+v", plan)
	}
	if plan.Items[0].BackupRunID != 4 || plan.Items[0].BackupRunVersion != 2 || plan.Items[1].BackupRunID != 3 {
		t.Fatalf("items = %+v", plan.Items)
	}
}

func TestCleanupSagaKeepsLocalArtifactUntilRemoteDeleteCanRetry(t *testing.T) {
	root := t.TempDir()
	artifactName := "old.sql.gz.mgbk"
	artifactPath := filepath.Join(root, artifactName)
	if err := os.WriteFile(artifactPath, []byte("backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	replica := &flakyCleanupReplica{fail: true}
	store := &cleanupSagaStore{resilienceStore: &resilienceStore{}}
	manager, err := NewManager(store, Config{BackupRoot: root, Replica: replica})
	if err != nil {
		t.Fatal(err)
	}
	plan := CleanupPlan{PolicyVersion: 3, CutoffAt: time.Now().Add(-24 * time.Hour), Scanned: 2, Preserved: 1, Items: []CleanupPlanItem{{
		BackupRunID: 7, BackupRunVersion: 5, BackupNo: "old", Status: RunStatusSucceeded,
		ArtifactName: artifactName, ReplicaStatus: ReplicaStatusSucceeded,
		ReplicaObjectKey: "backups/old.mgbk", ReplicaVersionID: "v1",
	}}}
	run, err := manager.ScheduleCleanup(context.Background(), plan, Actor{UserID: 9, TenantID: 1}, 37, 4)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != CleanupStatusFailed || run.FailedCount != 1 || store.scheduleInput.ApprovalExecutionID != 37 || store.scheduleInput.ApprovalExecutionVersion != 4 {
		t.Fatalf("first run = %+v schedule=%+v", run, store.scheduleInput)
	}
	if _, err := os.Stat(artifactPath); err != nil {
		t.Fatalf("local artifact was removed after remote failure: %v", err)
	}
	if store.item.LocalStatus != CleanupStepPending {
		t.Fatalf("local status = %s", store.item.LocalStatus)
	}

	replica.fail = false
	run, err = manager.RetryCleanup(context.Background(), run.ID, Actor{UserID: 10, TenantID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != CleanupStatusSucceeded || run.DeletedCount != 1 || replica.deleteCalls != 2 {
		t.Fatalf("retry run = %+v deleteCalls=%d", run, replica.deleteCalls)
	}
	if _, err := os.Stat(artifactPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("local artifact still exists after successful retry: %v", err)
	}
}

var _ Store = (*cleanupSagaStore)(nil)
var _ ReplicaStore = (*flakyCleanupReplica)(nil)
