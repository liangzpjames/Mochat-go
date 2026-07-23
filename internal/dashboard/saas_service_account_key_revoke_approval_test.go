package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeServiceAccountKeyRevokeApprovalStore struct {
	*fakeSaaSAdminApprovalStore
	account            SaaSServiceAccount
	key                SaaSServiceAccountKey
	createTarget       SaaSServiceAccountCreateTarget
	serviceCreateInput SaaSServiceAccountCreate
	serviceCreateCalls int
	updateInput        SaaSServiceAccountUpdate
	updateCalls        int
	rotateInput        SaaSServiceAccountKeyRotate
	rotateCalls        int
	revokeInput        SaaSServiceAccountKeyRevoke
	revokeCalls        int
}

func (s *fakeServiceAccountKeyRevokeApprovalStore) SaaSServiceAccountKeyForApproval(_ context.Context, serviceAccountID int64, keyID int64) (SaaSServiceAccount, SaaSServiceAccountKey, error) {
	if s.account.ID != serviceAccountID || s.key.ID != keyID || s.key.ServiceAccountID != serviceAccountID {
		return SaaSServiceAccount{}, SaaSServiceAccountKey{}, NewSaaSAdminNotFound("API Key 不存在")
	}
	return s.account, s.key, nil
}

func (s *fakeServiceAccountKeyRevokeApprovalStore) SaaSServiceAccounts(context.Context, SaaSServiceAccountOptions) ([]SaaSServiceAccount, error) {
	return []SaaSServiceAccount{s.account}, nil
}

func (s *fakeServiceAccountKeyRevokeApprovalStore) SaaSServiceAccountUsage(context.Context, SaaSServiceAccountUsageOptions) (SaaSServiceAccountUsageReport, error) {
	return SaaSServiceAccountUsageReport{}, nil
}

func (s *fakeServiceAccountKeyRevokeApprovalStore) SaaSServiceAccountForApproval(_ context.Context, serviceAccountID int64) (SaaSServiceAccount, error) {
	if s.account.ID != serviceAccountID {
		return SaaSServiceAccount{}, NewSaaSAdminNotFound("服务账号不存在")
	}
	return s.account, nil
}

func (s *fakeServiceAccountKeyRevokeApprovalStore) SaaSServiceAccountCreateTarget(_ context.Context, tenantID int, code string) (SaaSServiceAccountCreateTarget, error) {
	if s.createTarget.TenantID != tenantID || s.createTarget.Code != code {
		return SaaSServiceAccountCreateTarget{}, NewSaaSAdminNotFound("租户不存在")
	}
	return s.createTarget, nil
}

func (s *fakeServiceAccountKeyRevokeApprovalStore) CreateSaaSServiceAccount(_ context.Context, input SaaSServiceAccountCreate) (SaaSServiceAccountCreateResult, error) {
	s.serviceCreateCalls++
	s.serviceCreateInput = input
	key := SaaSServiceAccountKey{
		ID: 93, ServiceAccountID: 82, Name: input.KeyName, Prefix: input.Key.Prefix,
		HashKeyID: input.Key.HashKeyID, LastFour: input.Key.LastFour, Status: SaaSServiceAccountKeyStatusActive,
		ExpiresAt: input.KeyExpiresAt, Version: 1,
	}
	account := SaaSServiceAccount{
		ID: 82, TenantID: input.TenantID, TenantName: s.createTarget.TenantName, TenantStatus: s.createTarget.TenantStatus,
		Code: input.Code, Name: input.Name, Description: input.Description, Status: input.Status,
		Scopes: append([]string(nil), input.Scopes...), AllowedCIDRs: append([]string(nil), input.AllowedCIDRs...),
		ExpiresAt: input.ExpiresAt, Version: 1, Keys: []SaaSServiceAccountKey{key},
	}
	return SaaSServiceAccountCreateResult{Account: account, Key: key, OperationID: 182}, nil
}

func (s *fakeServiceAccountKeyRevokeApprovalStore) UpdateSaaSServiceAccount(_ context.Context, input SaaSServiceAccountUpdate) (SaaSServiceAccountUpdateResult, error) {
	s.updateCalls++
	s.updateInput = input
	account := s.account
	account.Name = input.Name
	account.Description = input.Description
	account.Status = input.Status
	account.Scopes = append([]string(nil), input.Scopes...)
	account.AllowedCIDRs = append([]string(nil), input.AllowedCIDRs...)
	account.Version++
	return SaaSServiceAccountUpdateResult{Account: account, OperationID: 180}, nil
}

func (s *fakeServiceAccountKeyRevokeApprovalStore) RotateSaaSServiceAccountKey(_ context.Context, input SaaSServiceAccountKeyRotate) (SaaSServiceAccountKeyRotateResult, error) {
	s.rotateCalls++
	s.rotateInput = input
	key := SaaSServiceAccountKey{
		ID: 92, ServiceAccountID: input.ServiceAccountID, Name: input.Name, Prefix: input.Key.Prefix,
		HashKeyID: input.Key.HashKeyID, LastFour: input.Key.LastFour, Status: SaaSServiceAccountKeyStatusActive,
		ExpiresAt: input.ExpiresAt, Version: 1,
	}
	account := s.account
	account.Version++
	account.Keys = append(account.Keys, key)
	return SaaSServiceAccountKeyRotateResult{Account: account, Key: key, RetiringKeys: 1, OperationID: 181}, nil
}

func (s *fakeServiceAccountKeyRevokeApprovalStore) RevokeSaaSServiceAccountKey(_ context.Context, input SaaSServiceAccountKeyRevoke) (SaaSServiceAccountKeyRevokeResult, error) {
	s.revokeCalls++
	s.revokeInput = input
	key := s.key
	key.Status = SaaSServiceAccountKeyStatusRevoked
	key.Version++
	account := s.account
	account.Version++
	account.Keys = []SaaSServiceAccountKey{key}
	return SaaSServiceAccountKeyRevokeResult{Account: account, Key: key, OperationID: 179}, nil
}

func (s *fakeServiceAccountKeyRevokeApprovalStore) SaaSServiceAccountPrincipalByPrefix(context.Context, string) (SaaSServiceAccountPrincipal, bool, error) {
	return SaaSServiceAccountPrincipal{}, false, nil
}

func (s *fakeServiceAccountKeyRevokeApprovalStore) ConsumeSaaSServiceAccountRequest(context.Context, int64, int64, string, string) (SaaSServiceAccountRateLimitState, error) {
	return SaaSServiceAccountRateLimitState{}, nil
}

func newServiceAccountKeyRevokeApprovalStore(userID int) *fakeServiceAccountKeyRevokeApprovalStore {
	return &fakeServiceAccountKeyRevokeApprovalStore{
		fakeSaaSAdminApprovalStore: &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
			fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{userID: {ID: userID, Name: "平台集成管理员", TenantID: 1, IsSuperAdmin: 1}}},
		}},
		account:      SaaSServiceAccount{ID: 81, TenantID: 961, TenantStatus: 1, Name: "数据同步", Status: SaaSServiceAccountStatusActive, Scopes: []string{SaaSServiceAccountScopeProfileRead}, ExpiresAt: "2099-01-01 00:00:00", Version: 4},
		key:          SaaSServiceAccountKey{ID: 91, ServiceAccountID: 81, Name: "生产主密钥", Prefix: "0123456789ab", LastFour: "wxyz", Status: SaaSServiceAccountKeyStatusActive, Version: 3},
		createTarget: SaaSServiceAccountCreateTarget{TenantID: 962, TenantName: "审批目标租户", TenantStatus: 1, Code: "report_sync"},
	}
}

func TestSaaSAdminServiceAccountCreateApprovalRequestFreezesNormalizedPayloadWithoutKeyMaterial(t *testing.T) {
	store := newServiceAccountKeyRevokeApprovalStore(7)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"service_account.create",
		"payload":{"tenantId":962,"code":" Report_Sync ","name":" 报表同步 ","description":" 只读报表集成 ","status":"ACTIVE","scopes":["tenant.usage.read","tenant.profile.read","tenant.usage.read"],"allowedCidrs":["127.0.0.1"],"keyName":" 首个生产 Key ","keyExpiresAt":"2098-01-01 00:00:00"},
		"reason":"双人复核创建报表服务账号","idempotencyKey":"approval-unit-service-account-create"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusOK || store.fakeSaaSAdminApprovalStore.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.fakeSaaSAdminApprovalStore.createCalls, rec.Body.String())
	}
	input := store.fakeSaaSAdminApprovalStore.createInput
	if input.ActionType != SaaSAdminApprovalActionServiceAccountCreate || input.RequiredPermission != SaaSAdminPermissionIntegrationsManage ||
		input.RequiredApprovals != 2 || input.TargetType != SaaSAdminOperationTargetServiceAccount ||
		input.TargetID != "962:report_sync" || input.TargetName != "审批目标租户 / 报表同步" {
		t.Fatalf("create approval input = %+v", input)
	}
	if strings.Contains(input.RequestJSON, `"Key"`) || strings.Contains(input.RequestJSON, "mch_live_") ||
		strings.Contains(input.RequestJSON, `"Hash"`) || strings.Contains(input.RequestJSON, `"PlainText"`) {
		t.Fatalf("approval request leaked key material: %s", input.RequestJSON)
	}
	var payload SaaSServiceAccountCreate
	if err := json.Unmarshal([]byte(input.RequestJSON), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.TenantID != 962 || payload.Code != "report_sync" || payload.Name != "报表同步" ||
		payload.Description != "只读报表集成" || payload.Status != SaaSServiceAccountStatusActive ||
		payload.KeyName != "首个生产 Key" || payload.KeyExpiresAt != "2098-01-01 00:00:00" ||
		strings.Join(payload.Scopes, ",") != "tenant.profile.read,tenant.usage.read" ||
		strings.Join(payload.AllowedCIDRs, ",") != "127.0.0.1/32" {
		t.Fatalf("frozen payload = %+v", payload)
	}
}

func TestSaaSAdminServiceAccountDirectCreateRequiresApproval(t *testing.T) {
	store := newServiceAccountKeyRevokeApprovalStore(1)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/serviceAccount", strings.NewReader(`{
		"tenantId":962,"code":"report_sync","name":"报表同步","status":"active","scopes":["tenant.profile.read"],"keyExpiresAt":"2098-01-01 00:00:00"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ServiceAccount(rec, req)
	if rec.Code != http.StatusPreconditionRequired || store.serviceCreateCalls != 0 || !strings.Contains(rec.Body.String(), SaaSAdminApprovalActionServiceAccountCreate) {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.serviceCreateCalls, rec.Body.String())
	}
}

func TestSaaSAdminApprovalExecuteCreatesServiceAccountWithoutPersistingPlainText(t *testing.T) {
	store := newServiceAccountKeyRevokeApprovalStore(9)
	store.beginResult = SaaSAdminApproval{
		ID: 82, RequestNo: "APR-SERVICE-ACCOUNT-CREATE-82", ActionType: SaaSAdminApprovalActionServiceAccountCreate,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 6,
		RequestJSON: `{"tenantId":962,"code":"report_sync","name":"报表同步","description":"只读报表集成","status":"active","scopes":["tenant.profile.read","tenant.usage.read"],"allowedCidrs":["127.0.0.1/32"],"rateLimitPerMinute":60,"dailyRequestLimit":10000,"usageAlertEnabled":true,"usageWarningPercent":80,"rejectionWarningCount":1,"usageAlertCooldownMinutes":60,"expiresAt":"","keyName":"首个生产 Key","keyExpiresAt":"2098-01-01 00:00:00"}`,
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1, "test-service-account-pepper").WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":82,"expectedVersion":5}`))
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)
	if rec.Code != http.StatusOK || store.serviceCreateCalls != 1 || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d createCalls=%d finish=%+v body=%s", rec.Code, store.serviceCreateCalls, store.finishInput, rec.Body.String())
	}
	input := store.serviceCreateInput
	if input.ActorUserID != 9 || input.ActorTenantID != 1 || input.ApprovalExecutionID != 82 ||
		input.ApprovalExecutionVersion != 6 || input.Key.Hash == "" || input.Key.PlainText != "" {
		t.Fatalf("create input = %+v", input)
	}
	var response struct {
		Data struct {
			Result struct {
				PlainTextKey string `json:"plainTextKey"`
				InitialKey   bool   `json:"initialKey"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if rec.Header().Get("Cache-Control") != "no-store" || !response.Data.Result.InitialKey ||
		!strings.HasPrefix(response.Data.Result.PlainTextKey, "mch_live_") {
		t.Fatalf("one-time response headers=%v body=%s", rec.Header(), rec.Body.String())
	}
	if strings.Contains(store.finishInput.ResultJSON, response.Data.Result.PlainTextKey) ||
		strings.Contains(store.finishInput.ResultJSON, `"plainTextKey"`) ||
		!strings.Contains(store.finishInput.ResultJSON, `"plainTextKeyDelivered":true`) {
		t.Fatalf("persistent result leaked or omitted delivery marker: %s", store.finishInput.ResultJSON)
	}
}

func TestSaaSAdminServiceAccountUpdateApprovalRequestFreezesNormalizedVersionedPayload(t *testing.T) {
	store := newServiceAccountKeyRevokeApprovalStore(7)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"service_account.update",
		"payload":{"id":81,"name":" 数据同步受控账号 ","description":" 双人审批变更 ","status":"DISABLED","scopes":["tenant.usage.read","tenant.profile.read","tenant.usage.read"],"allowedCidrs":["127.0.0.1/32"],"expectedVersion":4},
		"reason":"停用并收紧服务账号","idempotencyKey":"approval-unit-service-account-update"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusOK || store.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
	input := store.createInput
	if input.ActionType != SaaSAdminApprovalActionServiceAccountUpdate || input.RequiredPermission != SaaSAdminPermissionIntegrationsManage ||
		input.RequiredApprovals != 2 || input.TargetType != SaaSAdminOperationTargetServiceAccount || input.TargetID != "81" || input.TargetName != "数据同步" {
		t.Fatalf("create input = %+v", input)
	}
	var payload SaaSServiceAccountUpdate
	if err := json.Unmarshal([]byte(input.RequestJSON), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ID != 81 || payload.ExpectedVersion != 4 || payload.Name != "数据同步受控账号" || payload.Description != "双人审批变更" ||
		payload.Status != SaaSServiceAccountStatusDisabled || strings.Join(payload.Scopes, ",") != "tenant.profile.read,tenant.usage.read" {
		t.Fatalf("frozen payload = %+v", payload)
	}
}

func TestSaaSAdminServiceAccountUpdateApprovalRequestRejectsStaleVersion(t *testing.T) {
	store := newServiceAccountKeyRevokeApprovalStore(7)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"service_account.update","payload":{"id":81,"name":"数据同步","status":"active","scopes":["tenant.profile.read"],"expectedVersion":3},"reason":"旧版本变更"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusConflict || store.createCalls != 0 || !strings.Contains(rec.Body.String(), "版本已变化") {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
}

func TestSaaSAdminServiceAccountDirectUpdateRequiresApproval(t *testing.T) {
	store := newServiceAccountKeyRevokeApprovalStore(1)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/serviceAccount", strings.NewReader(`{
		"id":81,"name":"数据同步","status":"disabled","scopes":["tenant.profile.read"],"expectedVersion":4
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ServiceAccount(rec, req)
	if rec.Code != http.StatusPreconditionRequired || store.updateCalls != 0 || !strings.Contains(rec.Body.String(), SaaSAdminApprovalActionServiceAccountUpdate) {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.updateCalls, rec.Body.String())
	}
}

func TestSaaSAdminApprovalExecuteUpdatesServiceAccountWithExecutionLease(t *testing.T) {
	store := newServiceAccountKeyRevokeApprovalStore(9)
	store.beginResult = SaaSAdminApproval{
		ID: 80, RequestNo: "APR-SERVICE-ACCOUNT-80", ActionType: SaaSAdminApprovalActionServiceAccountUpdate,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 6,
		RequestJSON: `{"id":81,"name":"数据同步受控账号","description":"双人审批变更","status":"disabled","scopes":["tenant.profile.read"],"allowedCidrs":["127.0.0.1/32"],"expectedVersion":4}`,
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":80,"expectedVersion":5}`))
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)
	if rec.Code != http.StatusOK || store.updateCalls != 1 || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d updateCalls=%d finish=%+v body=%s", rec.Code, store.updateCalls, store.finishInput, rec.Body.String())
	}
	input := store.updateInput
	if input.ActorUserID != 9 || input.ActorTenantID != 1 || input.ApprovalExecutionID != 80 ||
		input.ApprovalExecutionVersion != 6 || input.ExpectedVersion != 4 || input.Status != SaaSServiceAccountStatusDisabled {
		t.Fatalf("update input = %+v", input)
	}
}

func TestSaaSAdminServiceAccountKeyRotateApprovalRequestFreezesParametersWithoutKeyMaterial(t *testing.T) {
	store := newServiceAccountKeyRevokeApprovalStore(7)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"service_account.key.rotate",
		"payload":{"serviceAccountId":81,"expectedVersion":4,"name":" 2026-Q3 ","expiresAt":"2098-01-01 00:00:00","graceMinutes":60},
		"reason":"双人复核轮换生产密钥","idempotencyKey":"approval-unit-service-key-rotate"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusOK || store.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
	input := store.createInput
	if input.ActionType != SaaSAdminApprovalActionServiceAccountKeyRotate || input.RequiredPermission != SaaSAdminPermissionIntegrationsManage ||
		input.RequiredApprovals != 2 || input.TargetType != SaaSAdminOperationTargetServiceAccountKey || input.TargetID != "81" ||
		input.TargetName != "数据同步 / 2026-Q3" {
		t.Fatalf("create input = %+v", input)
	}
	if strings.Contains(input.RequestJSON, "mch_live_") || strings.Contains(input.RequestJSON, "Hash") || strings.Contains(input.RequestJSON, "Key") {
		t.Fatalf("approval request leaked key material: %s", input.RequestJSON)
	}
	var payload SaaSServiceAccountKeyRotate
	if err := json.Unmarshal([]byte(input.RequestJSON), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ServiceAccountID != 81 || payload.ExpectedVersion != 4 || payload.Name != "2026-Q3" || payload.GraceMinutes != 60 || payload.ExpiresAt != "2098-01-01 00:00:00" {
		t.Fatalf("frozen payload = %+v", payload)
	}
}

func TestSaaSAdminServiceAccountKeyRotateApprovalRequestRejectsStaleVersion(t *testing.T) {
	store := newServiceAccountKeyRevokeApprovalStore(7)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"service_account.key.rotate","payload":{"serviceAccountId":81,"expectedVersion":3,"name":"旧版本","expiresAt":"2098-01-01 00:00:00","graceMinutes":0},"reason":"旧版本轮换"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusConflict || store.createCalls != 0 || !strings.Contains(rec.Body.String(), "版本已变化") {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
}

func TestSaaSAdminServiceAccountKeyDirectRotateRequiresApproval(t *testing.T) {
	store := newServiceAccountKeyRevokeApprovalStore(1)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/serviceAccountKeyRotate", strings.NewReader(`{
		"serviceAccountId":81,"expectedVersion":4,"name":"2026-Q3","expiresAt":"2098-01-01 00:00:00","graceMinutes":60
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ServiceAccountKeyRotate(rec, req)
	if rec.Code != http.StatusPreconditionRequired || store.rotateCalls != 0 || !strings.Contains(rec.Body.String(), SaaSAdminApprovalActionServiceAccountKeyRotate) {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.rotateCalls, rec.Body.String())
	}
}

func TestSaaSAdminApprovalExecuteRotatesServiceAccountKeyWithoutPersistingPlainText(t *testing.T) {
	store := newServiceAccountKeyRevokeApprovalStore(9)
	store.beginResult = SaaSAdminApproval{
		ID: 81, RequestNo: "APR-SERVICE-KEY-ROTATE-81", ActionType: SaaSAdminApprovalActionServiceAccountKeyRotate,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 6,
		RequestJSON: `{"serviceAccountId":81,"expectedVersion":4,"name":"2026-Q3","expiresAt":"2098-01-01 00:00:00","graceMinutes":60}`,
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1, "test-service-account-pepper").WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":81,"expectedVersion":5}`))
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)
	if rec.Code != http.StatusOK || store.rotateCalls != 1 || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d rotateCalls=%d finish=%+v body=%s", rec.Code, store.rotateCalls, store.finishInput, rec.Body.String())
	}
	input := store.rotateInput
	if input.ActorUserID != 9 || input.ActorTenantID != 1 || input.ApprovalExecutionID != 81 ||
		input.ApprovalExecutionVersion != 6 || input.ExpectedVersion != 4 || input.Key.Hash == "" || input.Key.PlainText != "" {
		t.Fatalf("rotate input = %+v", input)
	}
	var response struct {
		Data struct {
			Result struct {
				PlainTextKey string `json:"plainTextKey"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if rec.Header().Get("Cache-Control") != "no-store" || !strings.HasPrefix(response.Data.Result.PlainTextKey, "mch_live_") {
		t.Fatalf("one-time response headers=%v body=%s", rec.Header(), rec.Body.String())
	}
	if strings.Contains(store.finishInput.ResultJSON, response.Data.Result.PlainTextKey) || strings.Contains(store.finishInput.ResultJSON, `"plainTextKey"`) ||
		!strings.Contains(store.finishInput.ResultJSON, `"plainTextKeyDelivered":true`) {
		t.Fatalf("persistent result leaked or omitted delivery marker: %s", store.finishInput.ResultJSON)
	}
}

func TestSaaSAdminServiceAccountKeyRevokeApprovalRequestFreezesCurrentVersion(t *testing.T) {
	store := newServiceAccountKeyRevokeApprovalStore(7)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"service_account.key.revoke",
		"payload":{"serviceAccountId":81,"keyId":91,"expectedVersion":3},
		"reason":"吊销泄露的生产密钥","idempotencyKey":"approval-unit-service-key-revoke"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusOK || store.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
	input := store.createInput
	if input.ActionType != SaaSAdminApprovalActionServiceAccountKeyRevoke || input.RequiredPermission != SaaSAdminPermissionIntegrationsManage ||
		input.RequiredApprovals != 2 || input.TargetType != SaaSAdminOperationTargetServiceAccountKey || input.TargetID != "91" ||
		input.TargetName != "数据同步 / 生产主密钥" {
		t.Fatalf("create input = %+v", input)
	}
	var payload SaaSServiceAccountKeyRevoke
	if err := json.Unmarshal([]byte(input.RequestJSON), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ServiceAccountID != 81 || payload.KeyID != 91 || payload.ExpectedVersion != 3 {
		t.Fatalf("frozen payload = %+v", payload)
	}
}

func TestSaaSAdminServiceAccountKeyRevokeApprovalRequestRejectsStaleVersion(t *testing.T) {
	store := newServiceAccountKeyRevokeApprovalStore(7)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"service_account.key.revoke","payload":{"serviceAccountId":81,"keyId":91,"expectedVersion":2},"reason":"旧版本吊销"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusConflict || store.createCalls != 0 || !strings.Contains(rec.Body.String(), "版本已变化") {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
}

func TestSaaSAdminServiceAccountKeyDirectRevokeRequiresApproval(t *testing.T) {
	store := newServiceAccountKeyRevokeApprovalStore(1)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/serviceAccountKeyRevoke", strings.NewReader(`{
		"serviceAccountId":81,"keyId":91,"expectedVersion":3
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ServiceAccountKeyRevoke(rec, req)
	if rec.Code != http.StatusPreconditionRequired || store.revokeCalls != 0 || !strings.Contains(rec.Body.String(), SaaSAdminApprovalActionServiceAccountKeyRevoke) {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.revokeCalls, rec.Body.String())
	}
}

func TestSaaSAdminApprovalExecuteRevokesServiceAccountKeyWithExecutionLease(t *testing.T) {
	store := newServiceAccountKeyRevokeApprovalStore(9)
	store.beginResult = SaaSAdminApproval{
		ID: 79, RequestNo: "APR-SERVICE-KEY-79", ActionType: SaaSAdminApprovalActionServiceAccountKeyRevoke,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 6,
		RequestJSON: `{"serviceAccountId":81,"keyId":91,"expectedVersion":3}`,
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":79,"expectedVersion":5}`))
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)
	if rec.Code != http.StatusOK || store.revokeCalls != 1 || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d revokeCalls=%d finish=%+v body=%s", rec.Code, store.revokeCalls, store.finishInput, rec.Body.String())
	}
	input := store.revokeInput
	if input.ActorUserID != 9 || input.ActorTenantID != 1 || input.ApprovalExecutionID != 79 ||
		input.ApprovalExecutionVersion != 6 || input.ExpectedVersion != 3 {
		t.Fatalf("revoke input = %+v", input)
	}
}

var _ SaaSServiceAccountStore = (*fakeServiceAccountKeyRevokeApprovalStore)(nil)
var _ SaaSServiceAccountApprovalStore = (*fakeServiceAccountKeyRevokeApprovalStore)(nil)
var _ SaaSServiceAccountCreateApprovalStore = (*fakeServiceAccountKeyRevokeApprovalStore)(nil)
var _ SaaSServiceAccountUpdateApprovalStore = (*fakeServiceAccountKeyRevokeApprovalStore)(nil)
