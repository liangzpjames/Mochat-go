package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/saasbackup"
)

type fakeSaaSAdminApprovalStore struct {
	*fakeSaaSAdminAccessStore
	createInput            SaaSAdminApprovalCreate
	decisionInput          SaaSAdminApprovalDecision
	cancelInput            SaaSAdminApprovalCancel
	beginInput             SaaSAdminApprovalExecutionStart
	finishInput            SaaSAdminApprovalExecutionFinish
	beginResult            SaaSAdminApproval
	releaseEvidence        []SaaSReleaseEvidence
	releaseCandidateInput  SaaSReleaseCandidateCreate
	releaseCandidateResult SaaSReleaseCandidateCreateResult
	policyUpdateInput      SaaSAdminApprovalPolicyUpdate
	policyUpdateResult     SaaSAdminApprovalPolicyUpdateResult
	policies               []SaaSAdminApprovalPolicy
	createCalls            int
	finishCalls            int
	releaseCandidateCalls  int
	policyUpdateCalls      int
}

func (s *fakeSaaSAdminApprovalStore) SaaSAdminApprovalPolicies(context.Context) ([]SaaSAdminApprovalPolicy, error) {
	return s.policies, nil
}

func (s *fakeSaaSAdminApprovalStore) SaaSAdminApprovalPolicy(_ context.Context, actionType string) (SaaSAdminApprovalPolicy, bool, error) {
	for _, policy := range s.policies {
		if policy.ActionType == actionType {
			return policy, true, nil
		}
	}
	return SaaSAdminApprovalPolicy{}, false, nil
}

func (s *fakeSaaSAdminApprovalStore) UpdateSaaSAdminApprovalPolicy(_ context.Context, input SaaSAdminApprovalPolicyUpdate) (SaaSAdminApprovalPolicyUpdateResult, error) {
	s.policyUpdateCalls++
	s.policyUpdateInput = input
	if s.policyUpdateResult.Policy.ActionType == "" {
		s.policyUpdateResult = SaaSAdminApprovalPolicyUpdateResult{Policy: SaaSAdminApprovalPolicy{
			ActionType: input.ActionType, Enabled: input.Enabled, AmountThresholdCents: input.AmountThresholdCents,
			RequiredApprovals: input.RequiredApprovals, SLAMinutes: input.SLAMinutes, ReminderMinutes: input.ReminderMinutes,
			ExpiryHours: input.ExpiryHours, Version: input.ExpectedVersion + 1, UpdatedBy: input.ActorUserID,
		}, OperationID: 93}
	}
	return s.policyUpdateResult, nil
}

func (s *fakeSaaSAdminApprovalStore) SaaSAdminApprovalDecisions(context.Context, int64, int) ([]SaaSAdminApprovalDecisionRecord, error) {
	return nil, nil
}

func (s *fakeSaaSAdminApprovalStore) SaaSAdminApprovalDelegations(context.Context, SaaSAdminApprovalDelegationOptions) ([]SaaSAdminApprovalDelegation, error) {
	return nil, nil
}

func (s *fakeSaaSAdminApprovalStore) UpsertSaaSAdminApprovalDelegation(context.Context, SaaSAdminApprovalDelegationUpsert) (SaaSAdminApprovalDelegationUpsertResult, error) {
	return SaaSAdminApprovalDelegationUpsertResult{}, nil
}

func (s *fakeSaaSAdminApprovalStore) ResolveSaaSAdminApprovalDelegation(context.Context, int, int, time.Time) (SaaSAdminApprovalDelegation, bool, error) {
	return SaaSAdminApprovalDelegation{}, false, nil
}

func (s *fakeSaaSAdminApprovalStore) CreateSaaSAdminApprovalReminders(context.Context, SaaSAdminApprovalReminderCreate) (SaaSAdminApprovalReminderResult, error) {
	return SaaSAdminApprovalReminderResult{}, nil
}

func (s *fakeSaaSAdminApprovalStore) SaaSReleaseEvidence(context.Context) ([]SaaSReleaseEvidence, error) {
	return s.releaseEvidence, nil
}

func (s *fakeSaaSAdminApprovalStore) UpdateSaaSReleaseEvidence(context.Context, SaaSReleaseEvidenceUpdate) (SaaSReleaseEvidenceUpdateResult, error) {
	return SaaSReleaseEvidenceUpdateResult{}, nil
}

func (s *fakeSaaSAdminApprovalStore) SaaSReleaseCandidates(context.Context, int) ([]SaaSReleaseCandidate, error) {
	return nil, nil
}

func (s *fakeSaaSAdminApprovalStore) CreateSaaSReleaseCandidate(_ context.Context, input SaaSReleaseCandidateCreate) (SaaSReleaseCandidateCreateResult, error) {
	s.releaseCandidateCalls++
	s.releaseCandidateInput = input
	return s.releaseCandidateResult, nil
}

func approvalTestReleaseEvidence(fingerprint string) []SaaSReleaseEvidence {
	items := make([]SaaSReleaseEvidence, 0, SaaSReleaseEvidenceRequiredCount)
	for index, key := range []string{"mysql57_amd64", "real_wecom", "real_wechat_open", "real_saas_tenants", "production_frontend", "stability"} {
		items = append(items, SaaSReleaseEvidence{
			ID: int64(index + 1), Key: key, Required: true, Status: SaaSReleaseEvidenceStatusPassed,
			EvidenceURL: "https://evidence.company.cn/" + key, Environment: "production", SourceFingerprint: fingerprint,
			ArtifactSHA256: strings.Repeat(string(rune('a'+index)), 64), ArtifactSizeBytes: int64(1024 + index),
			CheckedAt: "2026-07-15 10:00:00", CheckedBy: 7, Version: 1,
		})
	}
	return items
}

func (s *fakeSaaSAdminApprovalStore) SaaSAdminApprovals(context.Context, SaaSAdminApprovalOptions) (SaaSAdminApprovalReport, error) {
	return SaaSAdminApprovalReport{Summary: SaaSAdminApprovalSummary{Total: 1}, Items: []SaaSAdminApproval{s.beginResult}}, nil
}

func (s *fakeSaaSAdminApprovalStore) SaaSAdminApprovalEvents(context.Context, int64, int) ([]SaaSAdminApprovalEvent, error) {
	return []SaaSAdminApprovalEvent{{ID: 1, ApprovalID: 7, EventType: "requested"}}, nil
}

func (s *fakeSaaSAdminApprovalStore) CreateSaaSAdminApproval(_ context.Context, input SaaSAdminApprovalCreate) (SaaSAdminApprovalCreateResult, error) {
	s.createInput = input
	s.createCalls++
	return SaaSAdminApprovalCreateResult{Approval: SaaSAdminApproval{
		ID: 7, RequestNo: input.RequestNo, ActionType: input.ActionType, RiskLevel: input.RiskLevel,
		Status: SaaSAdminApprovalStatusPending, RequesterUserID: input.RequesterUserID, RequestJSON: input.RequestJSON, Version: 1,
	}}, nil
}

func (s *fakeSaaSAdminApprovalStore) DecideSaaSAdminApproval(_ context.Context, input SaaSAdminApprovalDecision) (SaaSAdminApproval, error) {
	s.decisionInput = input
	return SaaSAdminApproval{ID: input.ApprovalID, Status: SaaSAdminApprovalStatusApproved, Version: input.ExpectedVersion + 1}, nil
}

func (s *fakeSaaSAdminApprovalStore) CancelSaaSAdminApproval(_ context.Context, input SaaSAdminApprovalCancel) (SaaSAdminApproval, error) {
	s.cancelInput = input
	return SaaSAdminApproval{ID: input.ApprovalID, Status: SaaSAdminApprovalStatusCanceled, Version: input.ExpectedVersion + 1}, nil
}

func (s *fakeSaaSAdminApprovalStore) BeginSaaSAdminApprovalExecution(_ context.Context, input SaaSAdminApprovalExecutionStart) (SaaSAdminApproval, error) {
	s.beginInput = input
	return s.beginResult, nil
}

func (s *fakeSaaSAdminApprovalStore) FinishSaaSAdminApprovalExecution(_ context.Context, input SaaSAdminApprovalExecutionFinish) (SaaSAdminApproval, error) {
	s.finishInput = input
	s.finishCalls++
	result := s.beginResult
	result.ID = input.ApprovalID
	result.Status = SaaSAdminApprovalStatusExecuted
	result.ResultJSON = input.ResultJSON
	result.Version = input.ExpectedVersion + 1
	return result, nil
}

func TestSaaSAdminApprovalRequestNormalizesProtectedRefund(t *testing.T) {
	store := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, Name: "平台超管", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"payment.refund.create",
		"payload":{"orderNo":"PAY-1","amountCents":1200,"currency":"CNY","reason":"客户退款","entitlementAction":"keep"},
		"reason":"复核客户退款凭证","idempotencyKey":"approval-unit-refund"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusOK || store.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
	if store.createInput.ActionType != SaaSAdminApprovalActionPaymentRefundCreate || store.createInput.RequiredPermission != SaaSAdminPermissionFinanceManage ||
		store.createInput.RequiredApprovals != 2 || store.createInput.TargetID == "" {
		t.Fatalf("create input = %+v", store.createInput)
	}
	var payload SaaSAdminPaymentRefundCreate
	if err := json.Unmarshal([]byte(store.createInput.RequestJSON), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.RefundNo == "" || payload.IdempotencyKey == "" || payload.OrderNo != "PAY-1" || payload.AmountCents != 1200 {
		t.Fatalf("normalized payload = %+v", payload)
	}
}

func TestSaaSAdminApprovalRequestRejectsSelfAccessAssignment(t *testing.T) {
	store := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, Name: "平台超管", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"access.assignment.save","payload":{"userId":1,"roleIds":[2],"expectedVersion":0},"reason":"给自己授权"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusBadRequest || store.createCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
}

func TestSaaSAdminApprovalRequestNormalizesReleaseCandidateGate(t *testing.T) {
	fingerprint := strings.Repeat("f", 64)
	store := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, Name: "发布运营", TenantID: 1, IsSuperAdmin: 1}}},
	}, releaseEvidence: approvalTestReleaseEvidence(fingerprint)}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).
		WithHighRiskApprovalRequired(true).
		WithReleaseSourceFingerprint(fingerprint, "build").
		WithReleaseEvidenceVerifier(&fakeSaaSReleaseArtifactVerifier{failures: map[string]string{}})
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"release.candidate.gate","payload":{"releaseVersion":"v1.2.3"},
		"reason":"生产发布复核","idempotencyKey":"approval-unit-release"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusOK || store.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
	if store.createInput.ActionType != SaaSAdminApprovalActionReleaseCandidateGate ||
		store.createInput.RequiredPermission != SaaSAdminPermissionReleaseManage || store.createInput.RequiredApprovals != 2 ||
		store.createInput.TargetID != fingerprint || store.createInput.TargetName != "v1.2.3" {
		t.Fatalf("create input = %+v", store.createInput)
	}
	var payload SaaSReleaseCandidateApprovalPayload
	if err := json.Unmarshal([]byte(store.createInput.RequestJSON), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ReleaseVersion != "v1.2.3" || payload.SourceFingerprint != fingerprint {
		t.Fatalf("normalized payload = %+v", payload)
	}
}

func TestSaaSAdminApprovalRequestNormalizesApprovalPolicyUpdate(t *testing.T) {
	store := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, Name: "治理发起人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"approval.policy.update",
		"payload":{"actionType":"payment.settlement.close","enabled":true,"amountThresholdCents":0,"requiredApprovals":2,"slaMinutes":180,"reminderMinutes":30,"expiryHours":24,"expectedVersion":1},
		"reason":"调整结算关账审批 SLA","idempotencyKey":"approval-unit-policy"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusOK || store.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
	if store.createInput.ActionType != SaaSAdminApprovalActionApprovalPolicyUpdate ||
		store.createInput.RequiredPermission != SaaSAdminPermissionApprovalsManage || store.createInput.RequiredApprovals != 2 ||
		store.createInput.TargetID != SaaSAdminApprovalActionPaymentSettlementClose || store.createInput.TargetName != "结算关账" {
		t.Fatalf("create input = %+v", store.createInput)
	}
	var payload saasAdminApprovalPolicyBody
	if err := json.Unmarshal([]byte(store.createInput.RequestJSON), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ActionType != SaaSAdminApprovalActionPaymentSettlementClose || payload.RequiredApprovals != 2 || payload.SLAMinutes != 180 || payload.ExpectedVersion != 1 {
		t.Fatalf("normalized payload = %+v", payload)
	}
}

func TestSaaSAdminApprovalRequestNormalizesBackupPolicyUpdate(t *testing.T) {
	store := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, Name: "灾备发起人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	backupStore := &fakeSaaSBackupStore{policy: saasbackup.Policy{ID: 1, Status: saasbackup.PolicyStatusActive, Version: 3}}
	manager, err := saasbackup.NewManager(backupStore, saasbackup.Config{})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).
		WithHighRiskApprovalRequired(true).
		WithBackupManager(manager)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"backup.policy.update",
		"payload":{"status":" ACTIVE ","intervalMinutes":60,"retentionDays":14,"minSuccessfulBackups":3,"maxBackupAgeMinutes":120,"restoreDrillIntervalDays":7,"requireEncryption":false,"requireOffsiteReplica":false,"expectedVersion":3},
		"reason":"调整平台灾备策略","idempotencyKey":"approval-unit-backup-policy"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusOK || store.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
	if store.createInput.ActionType != SaaSAdminApprovalActionBackupPolicyUpdate ||
		store.createInput.RequiredPermission != SaaSAdminPermissionBackupsManage || store.createInput.RequiredApprovals != 2 ||
		store.createInput.TargetID != "1" || store.createInput.TargetName != "平台数据库备份策略" {
		t.Fatalf("create input = %+v", store.createInput)
	}
	var payload saasbackup.PolicyUpdate
	if err := json.Unmarshal([]byte(store.createInput.RequestJSON), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Status != saasbackup.PolicyStatusActive || payload.ExpectedVersion != 3 || payload.IntervalMinutes != 60 || payload.RetentionDays != 14 {
		t.Fatalf("normalized payload = %+v", payload)
	}
}

func TestSaaSAdminApprovalRequestFreezesBackupRetentionCleanupPlan(t *testing.T) {
	store := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, Name: "灾备发起人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	now := time.Now()
	backupStore := &fakeSaaSBackupStore{
		policy: saasbackup.Policy{ID: 1, Status: saasbackup.PolicyStatusActive, RetentionDays: 1, MinSuccessfulBackups: 1, Version: 5},
		runs: []saasbackup.BackupRun{
			{ID: 9, BackupNo: "new", Status: saasbackup.RunStatusSucceeded, FinishedAt: now.Add(-time.Hour).Format("2006-01-02 15:04:05"), Version: 2},
			{ID: 8, BackupNo: "old-failed", Status: saasbackup.RunStatusFailed, FinishedAt: now.Add(-48 * time.Hour).Format("2006-01-02 15:04:05"), ArtifactName: "old-failed.mgbk", Version: 4},
		},
	}
	manager, err := saasbackup.NewManager(backupStore, saasbackup.Config{BackupRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).
		WithHighRiskApprovalRequired(true).
		WithBackupManager(manager)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"backup.retention.cleanup","payload":{},"reason":"清理过期平台数据库备份","idempotencyKey":"approval-unit-backup-cleanup"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusOK || store.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
	if store.createInput.ActionType != SaaSAdminApprovalActionBackupRetentionCleanup ||
		store.createInput.RequiredPermission != SaaSAdminPermissionBackupsManage || store.createInput.RequiredApprovals != 2 ||
		store.createInput.TargetName != "1 个过期备份" {
		t.Fatalf("create input = %+v", store.createInput)
	}
	var plan saasbackup.CleanupPlan
	if err := json.Unmarshal([]byte(store.createInput.RequestJSON), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.PolicyVersion != 5 || plan.Preserved != 1 || len(plan.Items) != 1 || plan.Items[0].BackupRunID != 8 || plan.Items[0].BackupRunVersion != 4 {
		t.Fatalf("normalized plan = %+v", plan)
	}
}

func TestSaaSAdminApprovalPolicyUpdateCannotDisableGovernanceGate(t *testing.T) {
	store := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, Name: "治理发起人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"approval.policy.update",
		"payload":{"actionType":"approval.policy.update","enabled":false,"amountThresholdCents":0,"requiredApprovals":1,"slaMinutes":120,"reminderMinutes":30,"expiryHours":12,"expectedVersion":1},
		"reason":"尝试降级治理门禁"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusBadRequest || store.createCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
}

func TestSaaSAdminDirectHighRiskActionRequiresApproval(t *testing.T) {
	store := &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, Name: "平台超管", TenantID: 1, IsSuperAdmin: 1}}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/accessRole", strings.NewReader(`{"code":"security_ops","name":"安全运营","status":1,"permissions":["platform.overview.read"]}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.AccessRole(rec, req)
	if rec.Code != http.StatusPreconditionRequired || store.roleUpdateCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.roleUpdateCalls, rec.Body.String())
	}
}

func TestSaaSAdminDirectApprovalPolicyUpdateRequiresApproval(t *testing.T) {
	store := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, Name: "平台超管", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/approvalPolicy", strings.NewReader(`{
		"actionType":"payment.settlement.close","enabled":true,"amountThresholdCents":0,
		"requiredApprovals":2,"slaMinutes":180,"reminderMinutes":30,"expiryHours":24,"expectedVersion":1
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ApprovalPolicy(rec, req)
	if rec.Code != http.StatusPreconditionRequired || store.policyUpdateCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.policyUpdateCalls, rec.Body.String())
	}
}

func TestSaaSAdminApprovalExecuteRunsNormalizedAccessAssignment(t *testing.T) {
	store := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{9: {ID: 9, Name: "审批人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	store.beginResult = SaaSAdminApproval{
		ID: 7, RequestNo: "APR-7", ActionType: SaaSAdminApprovalActionAccessAssignmentSave,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 8, ExecutionUserID: 9, Version: 3,
		RequestJSON: `{"userId":10,"roleIds":[2],"expectedVersion":0}`,
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":7,"expectedVersion":2}`))
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)
	if rec.Code != http.StatusOK || store.assignUpdateCalls != 1 || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d assignmentCalls=%d finish=%+v body=%s", rec.Code, store.assignUpdateCalls, store.finishInput, rec.Body.String())
	}
	if store.assignmentInput.UserID != 10 || store.assignmentInput.ActorUserID != 9 || store.assignmentInput.ApprovalExecutionID != 7 || store.assignmentInput.ApprovalExecutionVersion != 3 {
		t.Fatalf("assignment input = %+v", store.assignmentInput)
	}
}

func TestSaaSAdminApprovalExecuteRecoversCommittedEffectWithoutRepeatingAction(t *testing.T) {
	store := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{9: {ID: 9, Name: "审批人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	store.beginResult = SaaSAdminApproval{
		ID: 7, RequestNo: "APR-7", ActionType: SaaSAdminApprovalActionAccessAssignmentSave,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 8, ExecutionUserID: 9, Version: 5,
		RequestJSON:          `{"userId":10,"roleIds":[2],"expectedVersion":0}`,
		EffectAppliedAt:      "2026-07-11 01:00:00",
		EffectAppliedAtValue: time.Now(),
		EffectOperationID:    88,
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":7,"expectedVersion":4}`))
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)
	if rec.Code != http.StatusOK || store.assignUpdateCalls != 0 || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d assignmentCalls=%d finish=%+v body=%s", rec.Code, store.assignUpdateCalls, store.finishInput, rec.Body.String())
	}
	if store.finishInput.ExpectedVersion != 5 || !strings.Contains(store.finishInput.ResultJSON, `"recovered":true`) || !strings.Contains(store.finishInput.ResultJSON, `"effectOperationId":88`) {
		t.Fatalf("finish=%+v body=%s", store.finishInput, rec.Body.String())
	}
}

func TestSaaSAdminApprovalExecuteCreatesReleaseCandidateWithExecutionLease(t *testing.T) {
	fingerprint := strings.Repeat("f", 64)
	store := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{9: {ID: 9, Name: "发布审批人", TenantID: 1, IsSuperAdmin: 1}}},
	}, releaseEvidence: approvalTestReleaseEvidence(fingerprint)}
	store.beginResult = SaaSAdminApproval{
		ID: 17, RequestNo: "APR-REL-17", ActionType: SaaSAdminApprovalActionReleaseCandidateGate,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 4,
		RequestJSON: `{"releaseVersion":"v1.2.3","sourceFingerprint":"` + fingerprint + `"}`,
	}
	store.releaseCandidateResult = SaaSReleaseCandidateCreateResult{Candidate: SaaSReleaseCandidate{
		CandidateNo: "REL-APPROVED", ReleaseVersion: "v1.2.3", SourceFingerprint: fingerprint,
		Status: SaaSReleaseCandidateStatusReady, RequiredCount: 6, PassedCount: 6, MatchedCount: 6,
	}, OperationID: 91}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).
		WithHighRiskApprovalRequired(true).
		WithReleaseSourceFingerprint(fingerprint, "build").
		WithReleaseEvidenceVerifier(&fakeSaaSReleaseArtifactVerifier{failures: map[string]string{}})
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":17,"expectedVersion":3}`))
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)
	if rec.Code != http.StatusOK || store.releaseCandidateCalls != 1 || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d candidateCalls=%d finish=%+v body=%s", rec.Code, store.releaseCandidateCalls, store.finishInput, rec.Body.String())
	}
	input := store.releaseCandidateInput
	if input.ActorUserID != 9 || input.ActorTenantID != 1 || input.ApprovalExecutionID != 17 || input.ApprovalExecutionVersion != 4 ||
		input.SourceFingerprint != fingerprint || input.CandidateNo == "" || len(input.ArtifactVerifications) != SaaSReleaseEvidenceRequiredCount {
		t.Fatalf("candidate input = %+v", input)
	}
}

func TestSaaSAdminApprovalExecuteUpdatesPolicyWithExecutionLease(t *testing.T) {
	store := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{9: {ID: 9, Name: "治理执行人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	store.beginResult = SaaSAdminApproval{
		ID: 27, RequestNo: "APR-POLICY-27", ActionType: SaaSAdminApprovalActionApprovalPolicyUpdate,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 4,
		RequestJSON: `{"actionType":"payment.settlement.close","enabled":true,"amountThresholdCents":0,"requiredApprovals":2,"slaMinutes":180,"reminderMinutes":30,"expiryHours":24,"expectedVersion":1}`,
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":27,"expectedVersion":3}`))
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)
	if rec.Code != http.StatusOK || store.policyUpdateCalls != 1 || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d policyCalls=%d finish=%+v body=%s", rec.Code, store.policyUpdateCalls, store.finishInput, rec.Body.String())
	}
	input := store.policyUpdateInput
	if input.ActionType != SaaSAdminApprovalActionPaymentSettlementClose || input.ActorUserID != 9 || input.ActorTenantID != 1 ||
		input.ApprovalExecutionID != 27 || input.ApprovalExecutionVersion != 4 || input.RequiredApprovals != 2 || input.ExpectedVersion != 1 {
		t.Fatalf("policy input = %+v", input)
	}
}

func TestSaaSAdminApprovalExecuteUpdatesBackupPolicyWithExecutionLease(t *testing.T) {
	store := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{9: {ID: 9, Name: "灾备执行人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	store.beginResult = SaaSAdminApproval{
		ID: 37, RequestNo: "APR-BACKUP-37", ActionType: SaaSAdminApprovalActionBackupPolicyUpdate,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 4,
		RequestJSON: `{"status":"disabled","intervalMinutes":120,"retentionDays":21,"minSuccessfulBackups":5,"maxBackupAgeMinutes":240,"restoreDrillIntervalDays":14,"requireEncryption":false,"requireOffsiteReplica":false,"expectedVersion":3}`,
	}
	backupStore := &fakeSaaSBackupStore{policy: saasbackup.Policy{ID: 1, Status: saasbackup.PolicyStatusActive, Version: 3}}
	manager, err := saasbackup.NewManager(backupStore, saasbackup.Config{})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).
		WithHighRiskApprovalRequired(true).
		WithBackupManager(manager)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":37,"expectedVersion":3}`))
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)
	if rec.Code != http.StatusOK || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d finish=%+v body=%s", rec.Code, store.finishInput, rec.Body.String())
	}
	input := backupStore.policyUpdate
	if input.Status != saasbackup.PolicyStatusDisabled || input.Actor.UserID != 9 || input.Actor.TenantID != 1 ||
		input.ApprovalExecutionID != 37 || input.ApprovalExecutionVersion != 4 || input.ExpectedVersion != 3 || backupStore.policy.Version != 4 {
		t.Fatalf("backup policy input = %+v policy=%+v", input, backupStore.policy)
	}
}

func TestSaaSAdminApprovalExecuteSchedulesFrozenBackupCleanupWithExecutionLease(t *testing.T) {
	store := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{9: {ID: 9, Name: "灾备执行人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	store.beginResult = SaaSAdminApproval{
		ID: 47, RequestNo: "APR-BACKUP-CLEANUP-47", ActionType: SaaSAdminApprovalActionBackupRetentionCleanup,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 6,
		RequestJSON: `{"policyVersion":5,"cutoffAt":"2026-07-14T00:00:00+08:00","scanned":2,"preserved":1,"items":[{"backupRunId":8,"backupRunVersion":4,"backupNo":"old-failed","status":"failed","finishedAt":"2026-07-13 00:00:00","artifactName":"","sha256":"","sizeBytes":0,"replicaStatus":"disabled","replicaProvider":"","replicaBucket":"","replicaObjectKey":"","replicaVersionId":""}]}`,
	}
	backupStore := &fakeSaaSBackupStore{}
	manager, err := saasbackup.NewManager(backupStore, saasbackup.Config{BackupRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).
		WithHighRiskApprovalRequired(true).
		WithBackupManager(manager)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":47,"expectedVersion":5}`))
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)
	if rec.Code != http.StatusOK || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d finish=%+v body=%s", rec.Code, store.finishInput, rec.Body.String())
	}
	input := backupStore.cleanupSchedule
	if input.Actor.UserID != 9 || input.Actor.TenantID != 1 || input.ApprovalExecutionID != 47 ||
		input.ApprovalExecutionVersion != 6 || input.Plan.PolicyVersion != 5 || len(input.Plan.Items) != 1 {
		t.Fatalf("cleanup schedule = %+v", input)
	}
}
