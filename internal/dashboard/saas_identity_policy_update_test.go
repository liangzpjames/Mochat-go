package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/identitysecurity"
)

type fakeIdentityPolicyUpdateStore struct {
	identitysecurity.Store
	policy      identitysecurity.Policy
	updateInput identitysecurity.PolicyUpdate
	updateCalls int
	activeUsers int
	enrolled    int
}

func (s *fakeIdentityPolicyUpdateStore) IdentityPolicy(context.Context, int) (identitysecurity.Policy, bool, error) {
	return s.policy, s.policy.Version > 0, nil
}

func (s *fakeIdentityPolicyUpdateStore) IdentityMFAEnrollmentCoverage(context.Context, int) (int, int, error) {
	return s.activeUsers, s.enrolled, nil
}

func (s *fakeIdentityPolicyUpdateStore) UpdateIdentityPolicy(_ context.Context, input identitysecurity.PolicyUpdate) (identitysecurity.Policy, error) {
	s.updateCalls++
	s.updateInput = input
	if s.policy.Version != input.ExpectedVersion {
		return identitysecurity.Policy{}, identitysecurity.Conflict("身份安全策略版本已变化，请刷新后重试")
	}
	s.policy.Status = input.Status
	s.policy.MaxFailedAttempts = input.MaxFailedAttempts
	s.policy.LockoutMinutes = input.LockoutMinutes
	s.policy.SessionTTLMinutes = input.SessionTTLMinutes
	s.policy.IdleTimeoutMinutes = input.IdleTimeoutMinutes
	s.policy.MaxConcurrentSessions = input.MaxConcurrentSessions
	s.policy.RequireMFA = input.RequireMFA
	s.policy.AllowedIPCIDRs = input.AllowedIPCIDRs
	s.policy.LoginEventRetentionDays = input.LoginEventRetentionDays
	s.policy.SessionRetentionDays = input.SessionRetentionDays
	s.policy.Version++
	s.policy.UpdatedBy = input.Actor.UserID
	return s.policy, nil
}

func newIdentityPolicyApprovalHandler(t *testing.T, adminStore *fakeSaaSAdminApprovalStore, identityStore *fakeIdentityPolicyUpdateStore) *SaaSAdminHandler {
	t.Helper()
	manager, err := identitysecurity.NewManager(identityStore, identitysecurity.Config{
		EncryptionKey: strings.Repeat("69", 32), EncryptionKeyID: "identity-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewSaaSAdminHandler(adminStore, HeaderUserIDResolver{}, 1).
		WithHighRiskApprovalRequired(true).
		WithIdentitySecurityManager(manager)
}

func testIdentityPolicy() identitysecurity.Policy {
	return identitysecurity.Policy{
		TenantID: 23, TenantName: "身份安全租户", Status: identitysecurity.PolicyStatusActive,
		MaxFailedAttempts: 5, LockoutMinutes: 30, SessionTTLMinutes: 10080,
		IdleTimeoutMinutes: 1440, MaxConcurrentSessions: 5, AllowedIPCIDRs: []string{},
		LoginEventRetentionDays: 180, SessionRetentionDays: 90, Version: 3,
	}
}

func identityPolicyPayload(expectedVersion int) string {
	return `{"tenantId":23,"status":" ACTIVE ","maxFailedAttempts":6,"lockoutMinutes":45,"sessionTtlMinutes":20160,"idleTimeoutMinutes":720,"maxConcurrentSessions":8,"requireMfa":false,"allowedIpCidrs":[" 10.2.3.4/8 ","10.0.0.0/8"],"loginEventRetentionDays":365,"sessionRetentionDays":180,"expectedVersion":` +
		strconv.Itoa(expectedVersion) + `}`
}

func TestSaaSAdminApprovalRequestFreezesIdentityPolicyUpdate(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, Name: "身份发起人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	identityStore := &fakeIdentityPolicyUpdateStore{policy: testIdentityPolicy(), activeUsers: 4, enrolled: 4}
	handler := newIdentityPolicyApprovalHandler(t, adminStore, identityStore)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"identity.policy.update",
		"payload":`+identityPolicyPayload(3)+`,
		"reason":"调整租户身份安全边界","idempotencyKey":"approval-unit-identity-policy"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusOK || adminStore.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, adminStore.createCalls, rec.Body.String())
	}
	input := adminStore.createInput
	if input.ActionType != SaaSAdminApprovalActionIdentityPolicyUpdate || input.RequiredPermission != SaaSAdminPermissionIdentityManage ||
		input.RequiredApprovals != 2 || input.TargetType != "saas_identity_policy" || input.TargetID != "23" || input.TargetName != "身份安全租户" {
		t.Fatalf("create input = %+v", input)
	}
	var payload identitysecurity.PolicyUpdate
	if err := json.Unmarshal([]byte(input.RequestJSON), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Status != identitysecurity.PolicyStatusActive || payload.MaxFailedAttempts != 6 || payload.ExpectedVersion != 3 ||
		len(payload.AllowedIPCIDRs) != 1 || payload.AllowedIPCIDRs[0] != "10.0.0.0/8" {
		t.Fatalf("frozen payload = %+v", payload)
	}
}

func TestSaaSAdminApprovalRequestRejectsStaleIdentityPolicyVersion(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, Name: "身份发起人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	handler := newIdentityPolicyApprovalHandler(t, adminStore, &fakeIdentityPolicyUpdateStore{policy: testIdentityPolicy()})
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"identity.policy.update","payload":`+identityPolicyPayload(2)+`,"reason":"使用旧版本调整策略"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusConflict || adminStore.createCalls != 0 || !strings.Contains(rec.Body.String(), "版本已变化") {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, adminStore.createCalls, rec.Body.String())
	}
}

func TestSaaSAdminIdentityPolicyDirectUpdateRequiresApproval(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, Name: "平台超管", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	identityStore := &fakeIdentityPolicyUpdateStore{policy: testIdentityPolicy()}
	handler := newIdentityPolicyApprovalHandler(t, adminStore, identityStore)
	req := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/identityPolicy", strings.NewReader(identityPolicyPayload(3)))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.IdentityPolicy(rec, req)
	if rec.Code != http.StatusPreconditionRequired || identityStore.updateCalls != 0 ||
		!strings.Contains(rec.Body.String(), SaaSAdminApprovalActionIdentityPolicyUpdate) {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, identityStore.updateCalls, rec.Body.String())
	}
}

func TestSaaSAdminApprovalExecuteUpdatesIdentityPolicyWithExecutionLease(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{9: {ID: 9, Name: "身份执行人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	adminStore.beginResult = SaaSAdminApproval{
		ID: 76, RequestNo: "APR-IDENTITY-POLICY-76", ActionType: SaaSAdminApprovalActionIdentityPolicyUpdate,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 6,
		RequestJSON: identityPolicyPayload(3),
	}
	identityStore := &fakeIdentityPolicyUpdateStore{policy: testIdentityPolicy()}
	handler := newIdentityPolicyApprovalHandler(t, adminStore, identityStore)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":76,"expectedVersion":5}`))
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)
	if rec.Code != http.StatusOK || identityStore.updateCalls != 1 || adminStore.finishCalls != 1 || !adminStore.finishInput.Success {
		t.Fatalf("status=%d updateCalls=%d finish=%+v body=%s", rec.Code, identityStore.updateCalls, adminStore.finishInput, rec.Body.String())
	}
	input := identityStore.updateInput
	if input.Actor.UserID != 9 || input.Actor.TenantID != 1 || input.ApprovalExecutionID != 76 ||
		input.ApprovalExecutionVersion != 6 || input.ExpectedVersion != 3 || identityStore.policy.Version != 4 {
		t.Fatalf("policy input = %+v policy=%+v", input, identityStore.policy)
	}
}

func TestSaaSAdminApprovalPolicyCannotDisableIdentityPolicyUpdateGate(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, Name: "治理发起人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	handler := NewSaaSAdminHandler(adminStore, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"approval.policy.update",
		"payload":{"actionType":"identity.policy.update","enabled":false,"amountThresholdCents":0,"requiredApprovals":1,"slaMinutes":120,"reminderMinutes":30,"expiryHours":12,"expectedVersion":1},
		"reason":"尝试关闭身份安全策略变更门禁"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusBadRequest || adminStore.createCalls != 0 || !strings.Contains(rec.Body.String(), "严重风险门禁") {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, adminStore.createCalls, rec.Body.String())
	}
}

var _ identitysecurity.Store = (*fakeIdentityPolicyUpdateStore)(nil)
