package dashboard

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTenantProvisionApprovalStore(user User) *fakeSaaSAdminApprovalStore {
	base := &fakeSaaSAdminStore{
		users: map[int]User{user.ID: user},
		packages: []SaaSAdminPackage{{
			Code: "growth", Name: "成长版", Status: 1, Version: 3,
			Limits: SaaSAdminPackageLimits{MaxUsers: 120, MaxContacts: 5000},
		}},
		provisionResult: SaaSAdminTenantProvisionResult{
			TenantID: 91, TenantName: "审批开户客户", AdminUserID: 901, AdminPhone: "13800138191",
			AdminName: "租户管理员", RoleID: 902, RoleName: "超级管理员", PackageCode: "growth",
			PackageName: "成长版", ExpiresAt: "2028-07-09 00:00:00", MenuCount: 46,
			ConfigCopyCount: 3, MetricsRefreshed: 28, OperationID: 8601,
		},
	}
	return &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{fakeSaaSAdminStore: base}}
}

func TestSaaSAdminTenantProvisionDirectAndTaskApplyRequireApproval(t *testing.T) {
	for _, item := range []struct {
		name    string
		path    string
		handler func(*SaaSAdminHandler, http.ResponseWriter, *http.Request)
		body    string
	}{
		{
			name: "direct", path: "/dashboard/saasAdmin/tenantProvision",
			handler: func(handler *SaaSAdminHandler, w http.ResponseWriter, r *http.Request) { handler.ProvisionTenant(w, r) },
			body:    `{"tenantName":"审批开户客户","adminPhone":"13800138191","password":"abc123","packageCode":"growth"}`,
		},
		{
			name: "task", path: "/dashboard/saasAdmin/tenantProvisionTaskApply",
			handler: func(handler *SaaSAdminHandler, w http.ResponseWriter, r *http.Request) {
				handler.TenantProvisionTaskApply(w, r)
			},
			body: `{"taskId":86}`,
		},
		{
			name: "bulk", path: "/dashboard/saasAdmin/tenantProvisionTaskBulkApply",
			handler: func(handler *SaaSAdminHandler, w http.ResponseWriter, r *http.Request) {
				handler.TenantProvisionTaskBulkApply(w, r)
			},
			body: `{"taskType":"tenant_provision","status":"pending","limit":100}`,
		},
	} {
		t.Run(item.name, func(t *testing.T) {
			store := newTenantProvisionApprovalStore(User{ID: 1, Name: "平台超管", TenantID: 1, IsSuperAdmin: 1})
			handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1, "tenant-provision-secret").WithHighRiskApprovalRequired(true)
			req := httptest.NewRequest(http.MethodPost, item.path, strings.NewReader(item.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Mochat-Go-User-ID", "1")
			rec := httptest.NewRecorder()

			item.handler(handler, rec, req)

			base := store.fakeSaaSAdminAccessStore.fakeSaaSAdminStore
			if rec.Code != http.StatusPreconditionRequired || base.provisionCalls != 0 || base.taskCalls != 0 ||
				!strings.Contains(rec.Body.String(), SaaSAdminApprovalActionTenantProvision) {
				t.Fatalf("status=%d provisions=%d tasks=%d body=%s", rec.Code, base.provisionCalls, base.taskCalls, rec.Body.String())
			}
		})
	}
}

func TestSaaSAdminApprovalRequestFreezesDirectTenantProvisionWithoutCredentialLeak(t *testing.T) {
	store := newTenantProvisionApprovalStore(User{ID: 7, Name: "开户申请人", TenantID: 1, IsSuperAdmin: 1})
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1, "tenant-provision-secret").WithHighRiskApprovalRequired(true)
	const password = "abc123"
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"tenant.provision",
		"payload":{"tenantName":"审批开户客户","adminPhone":"13800138191","adminName":"租户管理员","password":"`+password+`","roleName":"超级管理员","packageCode":"growth","expiresAt":"2028-07-09","configCopyMode":"missing","remark":"合同已生效"},
		"reason":"新客户正式开户","idempotencyKey":"approval-unit-tenant-provision-direct"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.ApprovalRequest(rec, req)

	if rec.Code != http.StatusOK || store.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
	input := store.createInput
	if input.ActionType != SaaSAdminApprovalActionTenantProvision || input.RequiredPermission != SaaSAdminPermissionTenantsManage ||
		input.RequiredApprovals != 2 || input.TargetType != "tenant" || input.TargetID != "13800138191" ||
		input.TargetName != "审批开户客户 / 13800138191" {
		t.Fatalf("create input = %+v", input)
	}
	var plan SaaSAdminTenantProvisionApprovalPlan
	if err := json.Unmarshal([]byte(input.RequestJSON), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Source != "direct" || plan.Provision.AdminPasswordHash == "" || plan.Provision.AdminPasswordHash == password ||
		plan.CredentialFingerprint == "" || plan.Package.Code != "growth" || plan.Package.Version != 3 ||
		plan.Preview.PackageName != "成长版" || plan.Provision.ExpiresAt != "2028-07-09 00:00:00" {
		t.Fatalf("frozen plan = %+v", plan)
	}
	body := rec.Body.String()
	if strings.Contains(body, password) || strings.Contains(body, plan.Provision.AdminPasswordHash) ||
		strings.Contains(body, plan.CredentialFingerprint) || !strings.Contains(body, `"hasAdminPasswordHash":true`) {
		t.Fatalf("credential redaction failed: %s", body)
	}
}

func TestSaaSAdminApprovalRequestFreezesTenantProvisionTaskVersionAndDigest(t *testing.T) {
	store := newTenantProvisionApprovalStore(User{ID: 7, Name: "开户申请人", TenantID: 1, IsSuperAdmin: 1})
	requestJSON := saasAdminPayloadJSON(saasAdminTenantProvisionTaskRequestPayload(SaaSAdminTenantProvision{
		TenantName: "任务开户客户", AdminPhone: "13800138192", AdminName: "任务管理员",
		AdminPasswordHash: "$2y$10$task-password-hash", RoleName: "超级管理员",
		PackageCode: "growth", ExpiresAt: "2028-07-09 00:00:00", ConfigCopyMode: "missing", Remark: "任务开户",
	}))
	store.fakeSaaSAdminAccessStore.fakeSaaSAdminStore.tasks = []SaaSAdminTask{{
		ID: 86, TaskType: SaaSAdminTaskTypeTenantProvision, Status: SaaSAdminTaskStatusPending,
		Version: 4, PackageCode: "growth", RequestJSON: requestJSON, Remark: "任务开户",
	}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1, "tenant-provision-secret").WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"tenant.provision","payload":{"taskId":86,"expectedTaskVersion":4},
		"reason":"执行已复核的开户任务","idempotencyKey":"approval-unit-tenant-provision-task"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.ApprovalRequest(rec, req)

	if rec.Code != http.StatusOK || store.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
	var plan SaaSAdminTenantProvisionApprovalPlan
	if err := json.Unmarshal([]byte(store.createInput.RequestJSON), &plan); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(requestJSON))
	if plan.Source != "task" || plan.TaskID != 86 || plan.ExpectedTaskVersion != 4 ||
		plan.ExpectedTaskRequestSHA256 != hex.EncodeToString(digest[:]) ||
		plan.CredentialFingerprint != plan.ExpectedTaskRequestSHA256 || plan.Package.Version != 3 {
		t.Fatalf("frozen plan = %+v", plan)
	}
	if strings.Contains(rec.Body.String(), plan.Provision.AdminPasswordHash) ||
		strings.Contains(rec.Body.String(), `"credentialFingerprint"`) {
		t.Fatalf("credential redaction failed: %s", rec.Body.String())
	}
}

func TestSaaSAdminApprovalRequestRejectsStaleTenantProvisionTaskVersion(t *testing.T) {
	store := newTenantProvisionApprovalStore(User{ID: 7, Name: "开户申请人", TenantID: 1, IsSuperAdmin: 1})
	store.fakeSaaSAdminAccessStore.fakeSaaSAdminStore.tasks = []SaaSAdminTask{{
		ID: 86, TaskType: SaaSAdminTaskTypeTenantProvision, Status: SaaSAdminTaskStatusPending, Version: 5,
	}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1, "tenant-provision-secret").WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"tenant.provision","payload":{"taskId":86,"expectedTaskVersion":4},"reason":"使用旧版本开户"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.ApprovalRequest(rec, req)

	if rec.Code != http.StatusConflict || store.createCalls != 0 || !strings.Contains(rec.Body.String(), "任务版本已变化") {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
}

func TestSaaSAdminApprovalExecuteProvisionsTenantWithFrozenExecutionReferences(t *testing.T) {
	store := newTenantProvisionApprovalStore(User{ID: 9, Name: "开户执行人", TenantID: 1, IsSuperAdmin: 1})
	plan := SaaSAdminTenantProvisionApprovalPlan{
		Source:                    "task",
		TaskID:                    86,
		ExpectedTaskVersion:       4,
		ExpectedTaskRequestSHA256: strings.Repeat("a", 64),
		CredentialFingerprint:     strings.Repeat("b", 64),
		Provision: SaaSAdminTenantProvision{
			TenantName: "审批开户客户", AdminPhone: "13800138191", AdminName: "租户管理员",
			AdminPasswordHash: "$2y$10$frozen-password-hash", RoleName: "超级管理员",
			PackageCode: "growth", ExpiresAt: "2028-07-09 00:00:00", ConfigCopyMode: "missing", Remark: "合同已生效",
		},
		Preview: SaaSAdminTenantProvisionPreview{
			TenantName: "审批开户客户", AdminPhone: "13800138191", PackageCode: "growth", PackageName: "成长版",
		},
		Package: SaaSAdminPackage{Code: "growth", Name: "成长版", Status: 1, Version: 3},
	}
	requestJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	store.beginResult = SaaSAdminApproval{
		ID: 86, RequestNo: "APR-TENANT-PROVISION-86", ActionType: SaaSAdminApprovalActionTenantProvision,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 6,
		RequestJSON: string(requestJSON),
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1, "tenant-provision-secret").WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":86,"expectedVersion":5}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()

	handler.ApprovalExecute(rec, req)

	base := store.fakeSaaSAdminAccessStore.fakeSaaSAdminStore
	if rec.Code != http.StatusOK || base.provisionCalls != 1 || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d provisions=%d finish=%+v body=%s", rec.Code, base.provisionCalls, store.finishInput, rec.Body.String())
	}
	input := base.lastProvision
	if input.ActorUserID != 9 || input.ActorTenantID != 1 || input.ApprovalExecutionID != 86 ||
		input.ApprovalExecutionVersion != 6 || input.ExpectedPackageVersion != 3 || input.TaskID != 86 ||
		input.ExpectedTaskVersion != 4 || input.ExpectedTaskRequestSHA256 != strings.Repeat("a", 64) {
		t.Fatalf("provision input = %+v", input)
	}
	data := decodeSaaSAdminResponse(t, rec)
	result := data["result"].(map[string]any)
	if result["tenantId"].(float64) != 91 || result["operationId"].(float64) != 8601 ||
		result["source"] != "task" || result["taskId"].(float64) != 86 {
		t.Fatalf("data = %+v", data)
	}
}
