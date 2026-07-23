package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/saascompliance"
)

type fakeCompliancePolicyUpdateStore struct {
	saascompliance.Store
	policy      saascompliance.Policy
	updateInput saascompliance.PolicyUpdate
	updateCalls int
}

func (s *fakeCompliancePolicyUpdateStore) CompliancePolicy(context.Context) (saascompliance.Policy, error) {
	return s.policy, nil
}

func (s *fakeCompliancePolicyUpdateStore) UpdateCompliancePolicy(_ context.Context, input saascompliance.PolicyUpdate) (saascompliance.Policy, error) {
	s.updateCalls++
	s.updateInput = input
	if s.policy.Version != input.ExpectedVersion {
		return saascompliance.Policy{}, saascompliance.Conflict("合规策略版本已变化，请刷新后重试")
	}
	s.policy.Status = input.Status
	s.policy.ExportRetentionDays = input.ExportRetentionDays
	s.policy.ErasureGraceDays = input.ErasureGraceDays
	s.policy.RequireRecentExport = input.RequireRecentExport
	s.policy.RecentExportMaxAgeDays = input.RecentExportMaxAgeDays
	s.policy.BillingRetentionDays = input.BillingRetentionDays
	s.policy.AuditRetentionDays = input.AuditRetentionDays
	s.policy.ServiceAccountUsageRetentionDays = input.ServiceAccountUsageRetentionDays
	s.policy.Version++
	s.policy.UpdatedBy = input.Actor.UserID
	return s.policy, nil
}

func newCompliancePolicyApprovalHandler(t *testing.T, adminStore *fakeSaaSAdminApprovalStore, complianceStore *fakeCompliancePolicyUpdateStore) *SaaSAdminHandler {
	t.Helper()
	manager, err := saascompliance.NewManager(complianceStore, saascompliance.Config{PlatformTenantID: 1})
	if err != nil {
		t.Fatal(err)
	}
	return NewSaaSAdminHandler(adminStore, HeaderUserIDResolver{}, 1).
		WithHighRiskApprovalRequired(true).
		WithComplianceManager(manager)
}

func testCompliancePolicy() saascompliance.Policy {
	return saascompliance.Policy{
		ID: 1, Status: saascompliance.PolicyStatusActive, ExportRetentionDays: 30, ErasureGraceDays: 7,
		RequireRecentExport: true, RecentExportMaxAgeDays: 7, BillingRetentionDays: 365,
		AuditRetentionDays: 365, ServiceAccountUsageRetentionDays: 90, Version: 3,
	}
}

func TestSaaSAdminApprovalRequestFreezesCompliancePolicyUpdate(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, Name: "合规发起人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	complianceStore := &fakeCompliancePolicyUpdateStore{policy: testCompliancePolicy()}
	handler := newCompliancePolicyApprovalHandler(t, adminStore, complianceStore)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"compliance.policy.update",
		"payload":{"status":" ACTIVE ","exportRetentionDays":60,"erasureGraceDays":0,"requireRecentExport":true,"recentExportMaxAgeDays":14,"billingRetentionDays":180,"auditRetentionDays":730,"serviceAccountUsageRetentionDays":120,"expectedVersion":3},
		"reason":"调整租户数据生命周期边界","idempotencyKey":"approval-unit-compliance-policy"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusOK || adminStore.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, adminStore.createCalls, rec.Body.String())
	}
	input := adminStore.createInput
	if input.ActionType != SaaSAdminApprovalActionCompliancePolicyUpdate || input.RequiredPermission != SaaSAdminPermissionComplianceManage ||
		input.RequiredApprovals != 2 || input.TargetType != "saas_compliance_policy" || input.TargetID != "1" || input.TargetName != "租户数据合规策略" {
		t.Fatalf("create input = %+v", input)
	}
	var payload saascompliance.PolicyUpdate
	if err := json.Unmarshal([]byte(input.RequestJSON), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Status != saascompliance.PolicyStatusActive || payload.ExportRetentionDays != 60 || payload.BillingRetentionDays != 180 ||
		payload.AuditRetentionDays != 730 || payload.ServiceAccountUsageRetentionDays != 120 || payload.ExpectedVersion != 3 {
		t.Fatalf("frozen payload = %+v", payload)
	}
}

func TestSaaSAdminApprovalRequestRejectsStaleCompliancePolicyVersion(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, Name: "合规发起人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	handler := newCompliancePolicyApprovalHandler(t, adminStore, &fakeCompliancePolicyUpdateStore{policy: testCompliancePolicy()})
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"compliance.policy.update",
		"payload":{"status":"active","exportRetentionDays":60,"erasureGraceDays":0,"requireRecentExport":true,"recentExportMaxAgeDays":14,"billingRetentionDays":180,"auditRetentionDays":730,"serviceAccountUsageRetentionDays":120,"expectedVersion":2},
		"reason":"使用旧版本调整策略","idempotencyKey":"approval-unit-compliance-policy-stale"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusConflict || adminStore.createCalls != 0 || !strings.Contains(rec.Body.String(), "版本已变化") {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, adminStore.createCalls, rec.Body.String())
	}
}

func TestSaaSAdminCompliancePolicyDirectUpdateRequiresApproval(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, Name: "平台超管", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	complianceStore := &fakeCompliancePolicyUpdateStore{policy: testCompliancePolicy()}
	handler := newCompliancePolicyApprovalHandler(t, adminStore, complianceStore)
	req := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/compliancePolicy", strings.NewReader(`{
		"status":"active","exportRetentionDays":60,"erasureGraceDays":0,"requireRecentExport":true,"recentExportMaxAgeDays":14,"billingRetentionDays":180,"auditRetentionDays":730,"serviceAccountUsageRetentionDays":120,"expectedVersion":3
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.CompliancePolicy(rec, req)
	if rec.Code != http.StatusPreconditionRequired || complianceStore.updateCalls != 0 ||
		!strings.Contains(rec.Body.String(), SaaSAdminApprovalActionCompliancePolicyUpdate) {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, complianceStore.updateCalls, rec.Body.String())
	}
}

func TestSaaSAdminApprovalExecuteUpdatesCompliancePolicyWithExecutionLease(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{9: {ID: 9, Name: "合规执行人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	adminStore.beginResult = SaaSAdminApproval{
		ID: 57, RequestNo: "APR-COMPLIANCE-POLICY-57", ActionType: SaaSAdminApprovalActionCompliancePolicyUpdate,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 6,
		RequestJSON: `{"status":"disabled","exportRetentionDays":60,"erasureGraceDays":0,"requireRecentExport":false,"recentExportMaxAgeDays":14,"billingRetentionDays":180,"auditRetentionDays":730,"serviceAccountUsageRetentionDays":120,"expectedVersion":3}`,
	}
	complianceStore := &fakeCompliancePolicyUpdateStore{policy: testCompliancePolicy()}
	handler := newCompliancePolicyApprovalHandler(t, adminStore, complianceStore)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":57,"expectedVersion":5}`))
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)
	if rec.Code != http.StatusOK || complianceStore.updateCalls != 1 || adminStore.finishCalls != 1 || !adminStore.finishInput.Success {
		t.Fatalf("status=%d updateCalls=%d finish=%+v body=%s", rec.Code, complianceStore.updateCalls, adminStore.finishInput, rec.Body.String())
	}
	input := complianceStore.updateInput
	if input.Status != saascompliance.PolicyStatusDisabled || input.Actor.UserID != 9 || input.Actor.TenantID != 1 ||
		input.ApprovalExecutionID != 57 || input.ApprovalExecutionVersion != 6 || input.ExpectedVersion != 3 || complianceStore.policy.Version != 4 {
		t.Fatalf("policy input = %+v policy=%+v", input, complianceStore.policy)
	}
}

func TestSaaSAdminApprovalPolicyCannotDisableCompliancePolicyUpdateGate(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, Name: "治理发起人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	handler := NewSaaSAdminHandler(adminStore, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"approval.policy.update",
		"payload":{"actionType":"compliance.policy.update","enabled":false,"amountThresholdCents":0,"requiredApprovals":1,"slaMinutes":120,"reminderMinutes":30,"expiryHours":12,"expectedVersion":1},
		"reason":"尝试关闭合规策略变更门禁"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusBadRequest || adminStore.createCalls != 0 || !strings.Contains(rec.Body.String(), "严重风险门禁") {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, adminStore.createCalls, rec.Body.String())
	}
}

var _ saascompliance.Store = (*fakeCompliancePolicyUpdateStore)(nil)
