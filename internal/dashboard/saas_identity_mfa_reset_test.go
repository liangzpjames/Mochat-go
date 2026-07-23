package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/identitysecurity"
)

type fakeIdentityMFAResetStore struct {
	identitysecurity.Store
	state      identitysecurity.UserState
	credential identitysecurity.MFACredential
	resetInput identitysecurity.MFAReset
	resetCalls int
}

func (s *fakeIdentityMFAResetStore) IdentityUserState(context.Context, int) (identitysecurity.UserState, bool, error) {
	return s.state, s.state.UserID > 0, nil
}

func (s *fakeIdentityMFAResetStore) IdentityMFACredential(context.Context, int) (identitysecurity.MFACredential, bool, error) {
	return s.credential, s.credential.UserID > 0, nil
}

func (s *fakeIdentityMFAResetStore) ResetIdentityMFA(_ context.Context, input identitysecurity.MFAReset, _ time.Time) (identitysecurity.MFAResetResult, error) {
	s.resetCalls++
	s.resetInput = input
	if input.UserID != s.credential.UserID || input.TenantID != s.credential.TenantID ||
		input.ExpectedVersion != s.credential.Version {
		return identitysecurity.MFAResetResult{}, identitysecurity.Conflict("MFA 凭据状态或版本已变化")
	}
	s.credential.Status = identitysecurity.MFAStatusDisabled
	s.credential.RecoveryCodesRemaining = 0
	s.credential.Version++
	return identitysecurity.MFAResetResult{
		Credential: s.credential, RevokedSessions: 3, OperationID: 190,
	}, nil
}

func newIdentityMFAResetApprovalHandler(t *testing.T, adminStore *fakeSaaSAdminApprovalStore, identityStore *fakeIdentityMFAResetStore) *SaaSAdminHandler {
	t.Helper()
	manager, err := identitysecurity.NewManager(identityStore, identitysecurity.Config{
		EncryptionKey: strings.Repeat("83", 32), EncryptionKeyID: "identity-mfa-reset-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewSaaSAdminHandler(adminStore, HeaderUserIDResolver{}, 1).
		WithHighRiskApprovalRequired(true).
		WithIdentitySecurityManager(manager)
}

func testIdentityMFAResetStore() *fakeIdentityMFAResetStore {
	return &fakeIdentityMFAResetStore{
		state: identitysecurity.UserState{
			UserID: 981, TenantID: 23, UserName: "租户安全管理员", UserStatus: 1,
			MFAStatus: identitysecurity.MFAStatusActive, MFAVersion: 7, ActiveSessions: 3, Version: 5,
		},
		credential: identitysecurity.MFACredential{
			ID: 8, UserID: 981, TenantID: 23, Status: identitysecurity.MFAStatusActive,
			EncryptionKeyID: "identity-q3", RecoveryCodesRemaining: 6, Version: 7,
		},
	}
}

func TestSaaSAdminApprovalRequestFreezesIdentityMFAReset(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, Name: "身份发起人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	handler := newIdentityMFAResetApprovalHandler(t, adminStore, testIdentityMFAResetStore())
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"identity.mfa.reset",
		"payload":{"userId":981,"expectedVersion":7,"reason":"  用户更换认证设备  "},
		"reason":"双人复核 MFA 重置","idempotencyKey":"approval-unit-identity-mfa-reset"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusOK || adminStore.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, adminStore.createCalls, rec.Body.String())
	}
	input := adminStore.createInput
	if input.ActionType != SaaSAdminApprovalActionIdentityMFAReset || input.RequiredPermission != SaaSAdminPermissionIdentityManage ||
		input.RequiredApprovals != 2 || input.TargetType != "saas_identity_mfa" || input.TargetID != "981" ||
		input.TargetName != "租户安全管理员 / 租户 23" {
		t.Fatalf("create input = %+v", input)
	}
	var payload identitysecurity.MFAReset
	if err := json.Unmarshal([]byte(input.RequestJSON), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.UserID != 981 || payload.TenantID != 23 || payload.ExpectedVersion != 7 || payload.Reason != "用户更换认证设备" {
		t.Fatalf("frozen payload = %+v", payload)
	}
}

func TestSaaSAdminApprovalRequestRejectsStaleIdentityMFAVersion(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, Name: "身份发起人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	handler := newIdentityMFAResetApprovalHandler(t, adminStore, testIdentityMFAResetStore())
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"identity.mfa.reset",
		"payload":{"userId":981,"expectedVersion":6,"reason":"旧版本重置"},
		"reason":"旧版本申请"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusConflict || adminStore.createCalls != 0 || !strings.Contains(rec.Body.String(), "版本已变化") {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, adminStore.createCalls, rec.Body.String())
	}
}

func TestSaaSAdminIdentityMFADirectResetRequiresApproval(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, Name: "平台超管", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	identityStore := testIdentityMFAResetStore()
	handler := newIdentityMFAResetApprovalHandler(t, adminStore, identityStore)
	req := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/identityMFA", strings.NewReader(`{
		"userId":981,"expectedVersion":7,"reason":"用户更换认证设备"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.IdentityMFA(rec, req)
	if rec.Code != http.StatusPreconditionRequired || identityStore.resetCalls != 0 ||
		!strings.Contains(rec.Body.String(), SaaSAdminApprovalActionIdentityMFAReset) {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, identityStore.resetCalls, rec.Body.String())
	}
}

func TestSaaSAdminApprovalExecuteResetsIdentityMFAWithExecutionLease(t *testing.T) {
	adminStore := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{9: {ID: 9, Name: "身份执行人", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	adminStore.beginResult = SaaSAdminApproval{
		ID: 83, RequestNo: "APR-IDENTITY-MFA-83", ActionType: SaaSAdminApprovalActionIdentityMFAReset,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 6,
		RequestJSON: `{"userId":981,"tenantId":23,"expectedVersion":7,"reason":"用户更换认证设备"}`,
	}
	identityStore := testIdentityMFAResetStore()
	handler := newIdentityMFAResetApprovalHandler(t, adminStore, identityStore)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":83,"expectedVersion":5}`))
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)
	if rec.Code != http.StatusOK || identityStore.resetCalls != 1 || adminStore.finishCalls != 1 || !adminStore.finishInput.Success {
		t.Fatalf("status=%d resetCalls=%d finish=%+v body=%s", rec.Code, identityStore.resetCalls, adminStore.finishInput, rec.Body.String())
	}
	input := identityStore.resetInput
	if input.Actor.UserID != 9 || input.Actor.TenantID != 1 || input.ApprovalExecutionID != 83 ||
		input.ApprovalExecutionVersion != 6 || input.TenantID != 23 || input.ExpectedVersion != 7 {
		t.Fatalf("reset input = %+v", input)
	}
	if !strings.Contains(adminStore.finishInput.ResultJSON, `"revokedSessions":3`) ||
		!strings.Contains(adminStore.finishInput.ResultJSON, `"operationId":190`) {
		t.Fatalf("result JSON = %s", adminStore.finishInput.ResultJSON)
	}
}

var _ identitysecurity.Store = (*fakeIdentityMFAResetStore)(nil)
