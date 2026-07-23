package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/saasbackup"
)

type fakeSaaSBackupStore struct {
	policy          saasbackup.Policy
	policyUpdate    saasbackup.PolicyUpdate
	runs            []saasbackup.BackupRun
	drills          []saasbackup.RestoreDrill
	cleanupRuns     []saasbackup.CleanupRun
	cleanupSchedule saasbackup.CleanupSchedule
}

func (s *fakeSaaSBackupStore) BackupPolicy(context.Context) (saasbackup.Policy, error) {
	return s.policy, nil
}

func (s *fakeSaaSBackupStore) UpdateBackupPolicy(_ context.Context, input saasbackup.PolicyUpdate) (saasbackup.Policy, error) {
	s.policyUpdate = input
	s.policy.Status = input.Status
	s.policy.IntervalMinutes = input.IntervalMinutes
	s.policy.RetentionDays = input.RetentionDays
	s.policy.MinSuccessfulBackups = input.MinSuccessfulBackups
	s.policy.MaxBackupAgeMinutes = input.MaxBackupAgeMinutes
	s.policy.RestoreDrillIntervalDays = input.RestoreDrillIntervalDays
	s.policy.RequireEncryption = input.RequireEncryption
	s.policy.RequireOffsiteReplica = input.RequireOffsiteReplica
	s.policy.Version++
	return s.policy, nil
}

func (s *fakeSaaSBackupStore) BackupSourceMetadata(context.Context) (saasbackup.SourceMetadata, error) {
	return saasbackup.SourceMetadata{}, nil
}

func (s *fakeSaaSBackupStore) StartBackupRun(context.Context, saasbackup.BackupStart) (saasbackup.BackupRun, error) {
	return saasbackup.BackupRun{}, nil
}

func (s *fakeSaaSBackupStore) CompleteBackupRun(context.Context, saasbackup.BackupCompletion) (saasbackup.BackupRun, error) {
	return saasbackup.BackupRun{}, nil
}

func (s *fakeSaaSBackupStore) RecordBackupVerification(context.Context, saasbackup.BackupVerification) (saasbackup.BackupRun, error) {
	return saasbackup.BackupRun{}, nil
}

func (s *fakeSaaSBackupStore) StartBackupReplica(context.Context, saasbackup.BackupReplicaStart) (saasbackup.BackupRun, error) {
	return saasbackup.BackupRun{}, nil
}

func (s *fakeSaaSBackupStore) CompleteBackupReplica(context.Context, saasbackup.BackupReplicaCompletion) (saasbackup.BackupRun, error) {
	return saasbackup.BackupRun{}, nil
}

func (s *fakeSaaSBackupStore) BackupRun(_ context.Context, id int64) (saasbackup.BackupRun, error) {
	for _, run := range s.runs {
		if run.ID == id {
			return run, nil
		}
	}
	return saasbackup.BackupRun{}, saasbackup.NotFound("备份运行不存在")
}

func (s *fakeSaaSBackupStore) BackupRuns(context.Context, int) ([]saasbackup.BackupRun, error) {
	return s.runs, nil
}

func (s *fakeSaaSBackupStore) StartRestoreDrill(context.Context, saasbackup.RestoreDrillStart) (saasbackup.RestoreDrill, error) {
	return saasbackup.RestoreDrill{}, nil
}

func (s *fakeSaaSBackupStore) CompleteRestoreDrill(context.Context, saasbackup.RestoreDrillCompletion) (saasbackup.RestoreDrill, error) {
	return saasbackup.RestoreDrill{}, nil
}

func (s *fakeSaaSBackupStore) RestoreDrills(context.Context, int) ([]saasbackup.RestoreDrill, error) {
	return s.drills, nil
}

func (s *fakeSaaSBackupStore) MarkBackupDeleted(context.Context, int64, saasbackup.Actor) (saasbackup.BackupRun, error) {
	return saasbackup.BackupRun{}, nil
}

func (s *fakeSaaSBackupStore) ScheduleBackupCleanup(_ context.Context, input saasbackup.CleanupSchedule) (saasbackup.CleanupRun, error) {
	s.cleanupSchedule = input
	run := saasbackup.CleanupRun{ID: 41, CleanupNo: input.CleanupNo, Status: saasbackup.CleanupStatusPending,
		PolicyVersion: input.Plan.PolicyVersion, CandidateCount: len(input.Plan.Items), ApprovalID: input.ApprovalExecutionID, Version: 1}
	s.cleanupRuns = []saasbackup.CleanupRun{run}
	return run, nil
}
func (s *fakeSaaSBackupStore) BackupCleanupRun(context.Context, int64) (saasbackup.CleanupRun, error) {
	return saasbackup.CleanupRun{}, nil
}
func (s *fakeSaaSBackupStore) BackupCleanupRuns(context.Context, int) ([]saasbackup.CleanupRun, error) {
	return s.cleanupRuns, nil
}
func (s *fakeSaaSBackupStore) BackupCleanupItems(context.Context, int64) ([]saasbackup.CleanupItem, error) {
	return nil, nil
}
func (s *fakeSaaSBackupStore) ClaimBackupCleanup(context.Context, int64, bool, saasbackup.Actor) (saasbackup.CleanupRun, []saasbackup.CleanupItem, error) {
	if len(s.cleanupRuns) == 0 {
		return saasbackup.CleanupRun{}, nil, saasbackup.NotFound("备份清理任务不存在")
	}
	run := s.cleanupRuns[0]
	run.Status, run.Attempts = saasbackup.CleanupStatusRunning, run.Attempts+1
	s.cleanupRuns[0] = run
	return run, nil, nil
}
func (s *fakeSaaSBackupStore) ClaimNextBackupCleanup(context.Context, saasbackup.Actor) (saasbackup.CleanupRun, []saasbackup.CleanupItem, bool, error) {
	return saasbackup.CleanupRun{}, nil, false, nil
}
func (s *fakeSaaSBackupStore) CheckpointBackupCleanupItem(context.Context, saasbackup.CleanupCheckpoint) (saasbackup.CleanupItem, error) {
	return saasbackup.CleanupItem{}, nil
}
func (s *fakeSaaSBackupStore) CompleteBackupCleanupItem(context.Context, saasbackup.CleanupItemCompletion) (saasbackup.CleanupItem, error) {
	return saasbackup.CleanupItem{}, nil
}
func (s *fakeSaaSBackupStore) FinishBackupCleanup(context.Context, saasbackup.CleanupRunCompletion) (saasbackup.CleanupRun, error) {
	if len(s.cleanupRuns) == 0 {
		return saasbackup.CleanupRun{}, saasbackup.NotFound("备份清理任务不存在")
	}
	run := s.cleanupRuns[0]
	run.Status, run.DeletedCount, run.Version = saasbackup.CleanupStatusSucceeded, run.CandidateCount, run.Version+1
	s.cleanupRuns[0] = run
	return run, nil
}

func TestSaaSBackupOverviewAndPolicyHandlers(t *testing.T) {
	backupStore := &fakeSaaSBackupStore{
		policy: saasbackup.Policy{ID: 1, Status: saasbackup.PolicyStatusActive, IntervalMinutes: 1440, RetentionDays: 30,
			MinSuccessfulBackups: 7, MaxBackupAgeMinutes: 1800, RestoreDrillIntervalDays: 30, RequireEncryption: true, Version: 1},
		runs: []saasbackup.BackupRun{
			{ID: 9, BackupNo: "bkp_test", Status: saasbackup.RunStatusSucceeded,
				VerificationStatus: saasbackup.VerificationPassed, Encrypted: true, ReplicaStatus: saasbackup.ReplicaStatusSucceeded},
			{ID: 10, BackupNo: "bkp_deleted", Status: saasbackup.RunStatusDeleted,
				VerificationStatus: saasbackup.VerificationPassed, Encrypted: true},
		},
		drills: []saasbackup.RestoreDrill{{ID: 4, DrillNo: "rdr_test", Status: saasbackup.DrillStatusSucceeded}},
	}
	manager, err := saasbackup.NewManager(backupStore, saasbackup.Config{
		CronEnabled: true, CronInterval: 5 * time.Minute, CronRunOnStart: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	adminStore := &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, TenantID: 1, IsSuperAdmin: 1}}}
	handler := NewSaaSAdminHandler(adminStore, HeaderUserIDResolver{}, 1).WithBackupManager(manager)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/backupOverview", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.BackupOverview(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"verifiedCount":1`) ||
		!strings.Contains(rec.Body.String(), `"replicaSucceededCount":1`) || !strings.Contains(rec.Body.String(), `"successfulDrillCount":1`) ||
		!strings.Contains(rec.Body.String(), `"cronEnabled":true`) || !strings.Contains(rec.Body.String(), `"cronIntervalSeconds":300`) ||
		!strings.Contains(rec.Body.String(), `"cronRunOnStart":true`) {
		t.Fatalf("overview status=%d body=%s", rec.Code, rec.Body.String())
	}

	body := `{"status":"active","intervalMinutes":60,"retentionDays":14,"minSuccessfulBackups":3,"maxBackupAgeMinutes":120,"restoreDrillIntervalDays":7,"requireEncryption":false,"requireOffsiteReplica":false,"expectedVersion":1}`
	req = httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/backupPolicy", strings.NewReader(body))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec = httptest.NewRecorder()
	handler.BackupPolicy(rec, req)
	if rec.Code != http.StatusOK || backupStore.policyUpdate.Actor.UserID != 7 || backupStore.policyUpdate.Actor.TenantID != 1 ||
		backupStore.policyUpdate.RequireOffsiteReplica || backupStore.policy.Version != 2 {
		t.Fatalf("policy status=%d body=%s input=%+v", rec.Code, rec.Body.String(), backupStore.policyUpdate)
	}
}

func TestSaaSBackupHandlerRejectsMissingManagerAndInvalidAction(t *testing.T) {
	adminStore := &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, TenantID: 1, IsSuperAdmin: 1}}}
	handler := NewSaaSAdminHandler(adminStore, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/backupOverview", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.BackupOverview(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing manager status=%d body=%s", rec.Code, rec.Body.String())
	}

	backupStore := &fakeSaaSBackupStore{policy: saasbackup.Policy{Status: saasbackup.PolicyStatusActive, Version: 1}}
	manager, _ := saasbackup.NewManager(backupStore, saasbackup.Config{})
	handler.WithBackupManager(manager)
	req = httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/backupRun", strings.NewReader(`{"action":"drop"}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec = httptest.NewRecorder()
	handler.BackupRun(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid action status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSaaSBackupPolicyRequiresApprovalBeforeMutation(t *testing.T) {
	backupStore := &fakeSaaSBackupStore{policy: saasbackup.Policy{ID: 1, Status: saasbackup.PolicyStatusActive, Version: 1}}
	manager, err := saasbackup.NewManager(backupStore, saasbackup.Config{})
	if err != nil {
		t.Fatal(err)
	}
	adminStore := &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, TenantID: 1, IsSuperAdmin: 1}}}
	handler := NewSaaSAdminHandler(adminStore, HeaderUserIDResolver{}, 1).
		WithBackupManager(manager).
		WithHighRiskApprovalRequired(true)
	body := `{"status":"disabled","intervalMinutes":60,"retentionDays":14,"minSuccessfulBackups":3,"maxBackupAgeMinutes":120,"restoreDrillIntervalDays":7,"requireEncryption":false,"requireOffsiteReplica":false,"expectedVersion":1}`
	req := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/backupPolicy", strings.NewReader(body))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.BackupPolicy(rec, req)
	if rec.Code != http.StatusPreconditionRequired || backupStore.policyUpdate.ExpectedVersion != 0 || backupStore.policy.Version != 1 {
		t.Fatalf("status=%d body=%s input=%+v policy=%+v", rec.Code, rec.Body.String(), backupStore.policyUpdate, backupStore.policy)
	}
}

func TestSaaSBackupCleanupRequiresApprovalBeforePlanningOrMutation(t *testing.T) {
	backupStore := &fakeSaaSBackupStore{policy: saasbackup.Policy{ID: 1, Status: saasbackup.PolicyStatusActive, RetentionDays: 1, MinSuccessfulBackups: 1, Version: 1}}
	manager, err := saasbackup.NewManager(backupStore, saasbackup.Config{BackupRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	adminStore := &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, TenantID: 1, IsSuperAdmin: 1}}}
	handler := NewSaaSAdminHandler(adminStore, HeaderUserIDResolver{}, 1).
		WithBackupManager(manager).
		WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/backupRun", strings.NewReader(`{"action":"cleanup"}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.BackupRun(rec, req)
	if rec.Code != http.StatusPreconditionRequired || backupStore.cleanupSchedule.CleanupNo != "" ||
		!strings.Contains(rec.Body.String(), `"actionType":"backup.retention.cleanup"`) {
		t.Fatalf("status=%d body=%s schedule=%+v", rec.Code, rec.Body.String(), backupStore.cleanupSchedule)
	}
}

var _ saasbackup.Store = (*fakeSaaSBackupStore)(nil)
