package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/saascompliance"
)

type fakeComplianceExportDeletionStore struct {
	saascompliance.Store
	item          saascompliance.DataExport
	scheduleInput saascompliance.DataExportDeletionSchedule
	scheduleCalls int
	holdActive    bool
}

func (s *fakeComplianceExportDeletionStore) ComplianceExport(context.Context, int64) (saascompliance.DataExport, error) {
	return s.item, nil
}

func (s *fakeComplianceExportDeletionStore) ActiveComplianceLegalHold(context.Context, int, time.Time) (saascompliance.LegalHold, bool, error) {
	return saascompliance.LegalHold{HoldNo: "HOLD-TEST"}, s.holdActive, nil
}

func (s *fakeComplianceExportDeletionStore) ComplianceErasures(context.Context, int, int) ([]saascompliance.ErasureRequest, error) {
	return nil, nil
}

func (s *fakeComplianceExportDeletionStore) ScheduleComplianceExportDeletion(_ context.Context, input saascompliance.DataExportDeletionSchedule) (saascompliance.DataExport, error) {
	s.scheduleCalls++
	s.scheduleInput = input
	s.item.DeletionStatus = saascompliance.ExportDeletionStatusPending
	s.item.DeletionArtifactStatus = saascompliance.ExportDeletionStepPending
	s.item.DeletionRecordStatus = saascompliance.ExportDeletionStepPending
	s.item.DeletionApprovalID = input.ApprovalExecutionID
	s.item.DeletionRequestedBy = input.Actor.UserID
	return s.item, nil
}

func (s *fakeComplianceExportDeletionStore) ClaimComplianceExportDeletion(_ context.Context, _ int64, retry bool, _ saascompliance.Actor) (saascompliance.DataExport, error) {
	if retry && s.item.DeletionStatus != saascompliance.ExportDeletionStatusFailed {
		return s.item, saascompliance.Conflict("删除任务尚未失败")
	}
	s.item.DeletionStatus = saascompliance.ExportDeletionStatusRunning
	s.item.DeletionAttempts++
	return s.item, nil
}

func (s *fakeComplianceExportDeletionStore) CheckpointComplianceExportDeletion(_ context.Context, input saascompliance.DataExportDeletionCheckpoint) (saascompliance.DataExport, error) {
	s.item.DeletionArtifactStatus = input.Status
	if input.Status == saascompliance.ExportDeletionStepFailed {
		s.item.DeletionStatus = saascompliance.ExportDeletionStatusFailed
		s.item.DeletionLastError = input.ErrorMessage
	} else {
		s.item.DeletionStatus = saascompliance.ExportDeletionStatusRunning
		s.item.DeletionLastError = ""
	}
	return s.item, nil
}

func (s *fakeComplianceExportDeletionStore) CompleteComplianceExportDeletion(context.Context, saascompliance.DataExportDeletionCompletion) (saascompliance.DataExport, error) {
	s.item.Status = saascompliance.ExportStatusDeleted
	s.item.DeletionStatus = saascompliance.ExportDeletionStatusSucceeded
	s.item.DeletionRecordStatus = saascompliance.ExportDeletionStepDeleted
	s.item.DeletionLastError = ""
	s.item.DeletedAt = "2026-07-15 12:00:00"
	return s.item, nil
}

func newComplianceDeletionApprovalHandler(t *testing.T, adminStore *fakeSaaSAdminApprovalStore, deletionStore *fakeComplianceExportDeletionStore, artifactRoot string) (*SaaSAdminHandler, *saascompliance.Manager) {
	t.Helper()
	manager, err := saascompliance.NewManager(deletionStore, saascompliance.Config{ArtifactRoot: artifactRoot})
	if err != nil {
		t.Fatal(err)
	}
	return NewSaaSAdminHandler(adminStore, HeaderUserIDResolver{}, 1).
		WithHighRiskApprovalRequired(true).
		WithComplianceManager(manager), manager
}

func TestSaaSAdminApprovalRequestFreezesComplianceExportDeletionPlan(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, Name: "合规发起人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	deletionStore := &fakeComplianceExportDeletionStore{item: saascompliance.DataExport{
		ID: 91, ExportNo: "EXP-91", TenantID: 8, TenantName: "示例租户", Status: saascompliance.ExportStatusSucceeded,
		ArtifactName: "exp-91.tar.gz.mgce", SHA256: strings.Repeat("a", 64), SizeBytes: 2048,
		FinishedAt: "2026-07-15 10:00:00", ExpiresAt: "2026-08-14 10:00:00",
	}}
	handler, _ := newComplianceDeletionApprovalHandler(t, adminStore, deletionStore, t.TempDir())
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"compliance.export.delete","payload":{"exportId":91},
		"reason":"客户要求提前销毁导出副本","idempotencyKey":"approval-unit-export-delete"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusOK || adminStore.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, adminStore.createCalls, rec.Body.String())
	}
	if adminStore.createInput.ActionType != SaaSAdminApprovalActionComplianceExportDelete ||
		adminStore.createInput.RequiredPermission != SaaSAdminPermissionComplianceManage || adminStore.createInput.RequiredApprovals != 2 ||
		adminStore.createInput.TargetID != "91" || adminStore.createInput.TargetName != "EXP-91" {
		t.Fatalf("create input = %+v", adminStore.createInput)
	}
	var plan saascompliance.DataExportDeletionPlan
	if err := json.Unmarshal([]byte(adminStore.createInput.RequestJSON), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.ExportID != 91 || plan.TenantID != 8 || plan.ArtifactName != "exp-91.tar.gz.mgce" ||
		plan.SHA256 != strings.Repeat("a", 64) || plan.SizeBytes != 2048 || plan.ExpiresAt != "2026-08-14 10:00:00" {
		t.Fatalf("frozen plan = %+v", plan)
	}
}

func TestSaaSAdminApprovalRequestMapsComplianceDeletionBlockerToConflict(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, Name: "合规发起人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	deletionStore := &fakeComplianceExportDeletionStore{holdActive: true, item: saascompliance.DataExport{
		ID: 94, ExportNo: "EXP-94", TenantID: 8, TenantName: "示例租户", Status: saascompliance.ExportStatusSucceeded,
		ArtifactName: "exp-94.tar.gz.mgce", ExpiresAt: "2026-08-14 10:00:00",
	}}
	handler, _ := newComplianceDeletionApprovalHandler(t, adminStore, deletionStore, t.TempDir())
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"compliance.export.delete","payload":{"exportId":94},"reason":"法律保留期间尝试删除"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusConflict || adminStore.createCalls != 0 || !strings.Contains(rec.Body.String(), "法律保留") {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, adminStore.createCalls, rec.Body.String())
	}
}

func TestSaaSAdminApprovalExecutePersistsFailedDeletionAndRetryCompletes(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{9: {ID: 9, Name: "合规执行人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	root := t.TempDir()
	artifactName := "exp-92.tar.gz.mgce"
	artifactPath := filepath.Join(root, artifactName)
	if err := os.Mkdir(artifactPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactPath, "blocker"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	deletionStore := &fakeComplianceExportDeletionStore{item: saascompliance.DataExport{
		ID: 92, ExportNo: "EXP-92", TenantID: 8, TenantName: "示例租户", Status: saascompliance.ExportStatusSucceeded,
		ArtifactName: artifactName, SHA256: strings.Repeat("b", 64), SizeBytes: 4096,
		FinishedAt: "2026-07-15 10:00:00", ExpiresAt: "2026-08-14 10:00:00",
	}}
	plan, _ := json.Marshal(saascompliance.DataExportDeletionPlan{
		ExportID: 92, ExportNo: "EXP-92", TenantID: 8, TenantName: "示例租户", Status: saascompliance.ExportStatusSucceeded,
		ArtifactName: artifactName, SHA256: strings.Repeat("b", 64), SizeBytes: 4096,
		FinishedAt: "2026-07-15 10:00:00", ExpiresAt: "2026-08-14 10:00:00",
	})
	adminStore.beginResult = SaaSAdminApproval{
		ID: 57, RequestNo: "APR-EXPORT-DELETE-57", ActionType: SaaSAdminApprovalActionComplianceExportDelete,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 6, RequestJSON: string(plan),
	}
	handler, manager := newComplianceDeletionApprovalHandler(t, adminStore, deletionStore, root)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":57,"expectedVersion":5}`))
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)
	if rec.Code != http.StatusOK || adminStore.finishCalls != 1 || !adminStore.finishInput.Success {
		t.Fatalf("status=%d finish=%+v body=%s", rec.Code, adminStore.finishInput, rec.Body.String())
	}
	if deletionStore.scheduleCalls != 1 || deletionStore.scheduleInput.ApprovalExecutionID != 57 ||
		deletionStore.scheduleInput.ApprovalExecutionVersion != 6 || deletionStore.scheduleInput.Actor.UserID != 9 ||
		deletionStore.item.DeletionStatus != saascompliance.ExportDeletionStatusFailed || deletionStore.item.DeletionAttempts != 1 ||
		deletionStore.item.DeletionApprovalID != 57 || deletionStore.item.DeletionLastError == "" {
		t.Fatalf("scheduled deletion = %+v item=%+v", deletionStore.scheduleInput, deletionStore.item)
	}
	if err := os.RemoveAll(artifactPath); err != nil {
		t.Fatal(err)
	}
	completed, err := manager.RetryExportDeletion(context.Background(), 92, saascompliance.Actor{UserID: 9, TenantID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != saascompliance.ExportStatusDeleted || completed.DeletionStatus != saascompliance.ExportDeletionStatusSucceeded ||
		completed.DeletionArtifactStatus != saascompliance.ExportDeletionStepMissing || completed.DeletionRecordStatus != saascompliance.ExportDeletionStepDeleted ||
		completed.DeletionAttempts != 2 || completed.DeletionApprovalID != 57 {
		t.Fatalf("completed deletion = %+v", completed)
	}
}

func TestSaaSAdminComplianceExportDirectDeleteRequiresApproval(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, Name: "平台超管", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	deletionStore := &fakeComplianceExportDeletionStore{item: saascompliance.DataExport{
		ID: 93, ExportNo: "EXP-93", TenantID: 8, Status: saascompliance.ExportStatusSucceeded,
		ArtifactName: "exp-93.tar.gz.mgce", ExpiresAt: "2026-08-14 10:00:00",
	}}
	handler, _ := newComplianceDeletionApprovalHandler(t, adminStore, deletionStore, t.TempDir())
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/complianceExport", strings.NewReader(`{"action":"delete","exportId":93}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ComplianceExport(rec, req)
	if rec.Code != http.StatusPreconditionRequired || deletionStore.scheduleCalls != 0 ||
		deletionStore.item.Status != saascompliance.ExportStatusSucceeded || deletionStore.item.DeletionStatus != "" {
		t.Fatalf("status=%d item=%+v body=%s", rec.Code, deletionStore.item, rec.Body.String())
	}
}

func TestSaaSAdminApprovalPolicyCannotDisableComplianceExportDeletionGate(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, Name: "治理发起人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	handler := NewSaaSAdminHandler(adminStore, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"approval.policy.update",
		"payload":{"actionType":"compliance.export.delete","enabled":false,"amountThresholdCents":0,"requiredApprovals":1,"slaMinutes":120,"reminderMinutes":30,"expiryHours":12,"expectedVersion":1},
		"reason":"尝试关闭合规删除门禁"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusBadRequest || adminStore.createCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, adminStore.createCalls, rec.Body.String())
	}
}

var _ saascompliance.Store = (*fakeComplianceExportDeletionStore)(nil)
