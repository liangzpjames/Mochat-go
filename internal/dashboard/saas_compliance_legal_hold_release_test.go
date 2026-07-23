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

type fakeComplianceLegalHoldReleaseStore struct {
	saascompliance.Store
	item         saascompliance.LegalHold
	releaseInput saascompliance.LegalHoldRelease
	releaseCalls int
}

func (s *fakeComplianceLegalHoldReleaseStore) ComplianceLegalHold(context.Context, int64) (saascompliance.LegalHold, error) {
	return s.item, nil
}

func (s *fakeComplianceLegalHoldReleaseStore) ReleaseComplianceLegalHold(_ context.Context, input saascompliance.LegalHoldRelease) (saascompliance.LegalHold, error) {
	s.releaseCalls++
	s.releaseInput = input
	if s.item.Status != saascompliance.HoldStatusActive || s.item.Version != input.ExpectedVersion {
		return saascompliance.LegalHold{}, saascompliance.Conflict("法律保留已变化，请刷新后重试")
	}
	s.item.Status = saascompliance.HoldStatusReleased
	s.item.ReleaseReason = input.Reason
	s.item.ReleasedBy = input.Actor.UserID
	s.item.Version++
	s.item.OperationID = 901
	return s.item, nil
}

func newComplianceLegalHoldReleaseApprovalHandler(t *testing.T, adminStore *fakeSaaSAdminApprovalStore, complianceStore *fakeComplianceLegalHoldReleaseStore) *SaaSAdminHandler {
	t.Helper()
	manager, err := saascompliance.NewManager(complianceStore, saascompliance.Config{PlatformTenantID: 1})
	if err != nil {
		t.Fatal(err)
	}
	return NewSaaSAdminHandler(adminStore, HeaderUserIDResolver{}, 1).
		WithHighRiskApprovalRequired(true).
		WithComplianceManager(manager)
}

func testActiveComplianceLegalHold() saascompliance.LegalHold {
	return saascompliance.LegalHold{
		ID: 81, HoldNo: "HOLD-81", TenantID: 8, TenantName: "示例租户", Status: saascompliance.HoldStatusActive,
		Reason: "监管调查保全", StartsAt: "2026-07-15 10:00:00", ExpiresAt: "2026-08-15 10:00:00", Version: 3,
	}
}

func TestSaaSAdminApprovalRequestFreezesComplianceLegalHoldReleasePlan(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, Name: "合规发起人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	complianceStore := &fakeComplianceLegalHoldReleaseStore{item: testActiveComplianceLegalHold()}
	handler := newComplianceLegalHoldReleaseApprovalHandler(t, adminStore, complianceStore)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"compliance.legal_hold.release",
		"payload":{"holdId":81,"reason":"监管事项已结案","expectedVersion":3},
		"reason":"复核法律保留解除依据","idempotencyKey":"approval-unit-hold-release"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusOK || adminStore.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, adminStore.createCalls, rec.Body.String())
	}
	input := adminStore.createInput
	if input.ActionType != SaaSAdminApprovalActionComplianceLegalHoldRelease || input.RequiredPermission != SaaSAdminPermissionComplianceManage ||
		input.RequiredApprovals != 2 || input.TargetID != "81" || input.TargetName != "HOLD-81" {
		t.Fatalf("create input = %+v", input)
	}
	var plan saascompliance.LegalHoldReleasePlan
	if err := json.Unmarshal([]byte(input.RequestJSON), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.HoldID != 81 || plan.HoldNo != "HOLD-81" || plan.TenantID != 8 || plan.TenantName != "示例租户" ||
		plan.Status != saascompliance.HoldStatusActive || plan.HoldReason != "监管调查保全" ||
		plan.StartsAt != "2026-07-15 10:00:00" || plan.ExpiresAt != "2026-08-15 10:00:00" ||
		plan.ExpectedVersion != 3 || plan.ReleaseReason != "监管事项已结案" {
		t.Fatalf("frozen plan = %+v", plan)
	}
}

func TestSaaSAdminComplianceLegalHoldDirectReleaseRequiresApproval(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, Name: "平台超管", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	complianceStore := &fakeComplianceLegalHoldReleaseStore{item: testActiveComplianceLegalHold()}
	handler := newComplianceLegalHoldReleaseApprovalHandler(t, adminStore, complianceStore)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/complianceLegalHold", strings.NewReader(`{
		"action":"release","holdId":81,"reason":"监管事项已结案","expectedVersion":3
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ComplianceLegalHold(rec, req)
	if rec.Code != http.StatusPreconditionRequired || complianceStore.releaseCalls != 0 ||
		!strings.Contains(rec.Body.String(), SaaSAdminApprovalActionComplianceLegalHoldRelease) {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, complianceStore.releaseCalls, rec.Body.String())
	}
}

func TestSaaSAdminApprovalExecuteReleasesFrozenComplianceLegalHold(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{9: {ID: 9, Name: "合规执行人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	hold := testActiveComplianceLegalHold()
	plan, err := json.Marshal(saascompliance.LegalHoldReleasePlan{
		HoldID: hold.ID, HoldNo: hold.HoldNo, TenantID: hold.TenantID, TenantName: hold.TenantName,
		Status: hold.Status, HoldReason: hold.Reason, StartsAt: hold.StartsAt, ExpiresAt: hold.ExpiresAt,
		ExpectedVersion: hold.Version, ReleaseReason: "监管事项已结案",
	})
	if err != nil {
		t.Fatal(err)
	}
	adminStore.beginResult = SaaSAdminApproval{
		ID: 67, RequestNo: "APR-HOLD-RELEASE-67", ActionType: SaaSAdminApprovalActionComplianceLegalHoldRelease,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 6, RequestJSON: string(plan),
	}
	complianceStore := &fakeComplianceLegalHoldReleaseStore{item: hold}
	handler := newComplianceLegalHoldReleaseApprovalHandler(t, adminStore, complianceStore)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":67,"expectedVersion":5}`))
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)
	if rec.Code != http.StatusOK || complianceStore.releaseCalls != 1 || adminStore.finishCalls != 1 || !adminStore.finishInput.Success {
		t.Fatalf("status=%d releaseCalls=%d finish=%+v body=%s", rec.Code, complianceStore.releaseCalls, adminStore.finishInput, rec.Body.String())
	}
	input := complianceStore.releaseInput
	if input.HoldID != 81 || input.Reason != "监管事项已结案" || input.ExpectedVersion != 3 ||
		input.Actor.UserID != 9 || input.Actor.TenantID != 1 || input.ApprovalExecutionID != 67 ||
		input.ApprovalExecutionVersion != 6 || input.ApprovalPlan == nil || input.ApprovalPlan.HoldNo != "HOLD-81" ||
		complianceStore.item.Status != saascompliance.HoldStatusReleased || complianceStore.item.Version != 4 {
		t.Fatalf("release input = %+v item=%+v", input, complianceStore.item)
	}
}

func TestSaaSAdminApprovalPolicyCannotDisableComplianceLegalHoldReleaseGate(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, Name: "治理发起人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	handler := NewSaaSAdminHandler(adminStore, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"approval.policy.update",
		"payload":{"actionType":"compliance.legal_hold.release","enabled":false,"amountThresholdCents":0,"requiredApprovals":1,"slaMinutes":120,"reminderMinutes":30,"expiryHours":12,"expectedVersion":1},
		"reason":"尝试关闭法律保留解除门禁"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusBadRequest || adminStore.createCalls != 0 || !strings.Contains(rec.Body.String(), "严重风险门禁") {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, adminStore.createCalls, rec.Body.String())
	}
}

var _ saascompliance.Store = (*fakeComplianceLegalHoldReleaseStore)(nil)
