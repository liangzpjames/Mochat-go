package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTenantPackageUpdateApprovalStore(user User) *fakeSaaSAdminApprovalStore {
	base := &fakeSaaSAdminStore{
		users: map[int]User{user.ID: user},
		overview: SaaSAdminOverview{Tenants: []SaaSAdminTenantOverview{{
			TenantID: 12, TenantName: "华东客户", TenantStatus: 1,
			PackageCode: "scale", PackageName: "规模版", PackageStatus: 1, PackageVersion: 4,
			ExpiresAt: "2027-01-01 00:00:00", PackageLimits: SaaSAdminPackageLimits{MaxUsers: 300},
		}}},
		packages: []SaaSAdminPackage{
			{Code: "scale", Name: "规模版", Status: 1, Version: 5, Limits: SaaSAdminPackageLimits{MaxUsers: 300}},
			{Code: "growth", Name: "成长版", Status: 1, Version: 3, Limits: SaaSAdminPackageLimits{MaxUsers: 120}},
		},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {{Metric: SaaSMetricUsers, Current: 200, Limit: 300}},
		},
		updateResult: SaaSAdminTenantPackageUpdateResult{
			TenantID: 12, TenantName: "华东客户", PackageCode: "growth", PackageName: "成长版",
			ExpiresAt: "2027-06-30 00:00:00", Status: 1, Version: 5, OperationID: 85, MetricsRefreshed: 28,
		},
	}
	return &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{fakeSaaSAdminStore: base}}
}

func TestSaaSAdminApprovalRequestFreezesTenantPackageAssignmentAndImpact(t *testing.T) {
	store := newTenantPackageUpdateApprovalStore(User{ID: 7, Name: "权益申请人", TenantID: 1, IsSuperAdmin: 1})
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"tenant.package.update",
		"payload":{"tenantId":12,"packageCode":"growth","expiresAt":"2027-06-30","remark":"客户降配","expectedVersion":4},
		"reason":"客户合同调整","idempotencyKey":"approval-unit-tenant-package-update"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.ApprovalRequest(rec, req)

	if rec.Code != http.StatusOK || store.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
	input := store.createInput
	if input.ActionType != SaaSAdminApprovalActionTenantPackageUpdate || input.RequiredPermission != SaaSAdminPermissionTenantsManage ||
		input.RequiredApprovals != 2 || input.TargetType != "saas_tenant_package" || input.TargetID != "12" ||
		input.TargetName != "华东客户 / 成长版" {
		t.Fatalf("create input = %+v", input)
	}
	var plan SaaSAdminTenantPackageUpdatePlan
	if err := json.Unmarshal([]byte(input.RequestJSON), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Update.TenantID != 12 || plan.Update.PackageCode != "growth" || plan.Update.ExpiresAt != "2027-06-30 00:00:00" ||
		plan.Update.ExpectedVersion != 4 || plan.Update.ExpectedPackageVersion != 3 || plan.Update.ExpectedTenantStatus != 1 ||
		plan.Current == nil || plan.Current.Version != 4 || plan.Current.Limits.MaxUsers != 300 ||
		plan.TargetPackage.Version != 3 || plan.TargetPackage.Limits.MaxUsers != 120 {
		t.Fatalf("frozen plan = %+v", plan)
	}
	if !plan.Impact.Existing || plan.Impact.AssignedTenantCount != 1 || plan.Impact.CheckedTenantCount != 1 ||
		plan.Impact.ChangedLimitCount != 1 || plan.Impact.DecreasedLimitCount != 1 || plan.Impact.OverLimitTenantCount != 1 ||
		len(plan.Impact.OverLimitTenants) != 1 || plan.Impact.OverLimitTenants[0].Current != 200 {
		t.Fatalf("frozen impact = %+v", plan.Impact)
	}
}

func TestSaaSAdminApprovalRequestRejectsStaleTenantPackageVersion(t *testing.T) {
	store := newTenantPackageUpdateApprovalStore(User{ID: 7, Name: "权益申请人", TenantID: 1, IsSuperAdmin: 1})
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"tenant.package.update",
		"payload":{"tenantId":12,"packageCode":"growth","expectedVersion":3},
		"reason":"使用旧版本调整套餐"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.ApprovalRequest(rec, req)

	if rec.Code != http.StatusConflict || store.createCalls != 0 || !strings.Contains(rec.Body.String(), "版本已变化") {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
}

func TestSaaSAdminTenantPackageDirectUpdateRequiresApproval(t *testing.T) {
	store := newTenantPackageUpdateApprovalStore(User{ID: 1, Name: "平台超管", TenantID: 1, IsSuperAdmin: 1})
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantPackage", strings.NewReader(`{
		"tenantId":12,"packageCode":"growth","expiresAt":"2027-06-30","expectedVersion":4
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.UpdateTenantPackage(rec, req)

	base := store.fakeSaaSAdminAccessStore.fakeSaaSAdminStore
	if rec.Code != http.StatusPreconditionRequired || base.updateCalls != 0 || base.overviewCalls != 0 || base.packageCalls != 0 ||
		!strings.Contains(rec.Body.String(), SaaSAdminApprovalActionTenantPackageUpdate) {
		t.Fatalf("status=%d updates=%d overview=%d packages=%d body=%s", rec.Code, base.updateCalls, base.overviewCalls, base.packageCalls, rec.Body.String())
	}
}

func TestSaaSAdminApprovalExecuteUpdatesTenantPackageWithExecutionLease(t *testing.T) {
	store := newTenantPackageUpdateApprovalStore(User{ID: 9, Name: "权益执行人", TenantID: 1, IsSuperAdmin: 1})
	plan := SaaSAdminTenantPackageUpdatePlan{
		Update: SaaSAdminTenantPackageUpdate{
			TenantID: 12, PackageCode: "growth", ExpiresAt: "2027-06-30 00:00:00", Remark: "客户降配",
			ExpectedVersion: 4, ExpectedPackageVersion: 3, ExpectedTenantStatus: 1,
		},
		TenantName: "华东客户",
		Current: &SaaSAdminTenantPackageSnapshot{
			TenantID: 12, TenantName: "华东客户", TenantStatus: 1, PackageCode: "scale", PackageName: "规模版",
			ExpiresAt: "2027-01-01 00:00:00", Status: 1, Version: 4, Limits: SaaSAdminPackageLimits{MaxUsers: 300},
		},
		TargetPackage: SaaSAdminPackage{Code: "growth", Name: "成长版", Status: 1, Version: 3, Limits: SaaSAdminPackageLimits{MaxUsers: 120}},
		Impact:        SaaSAdminPackageImpact{Existing: true, ChangedLimitCount: 1, DecreasedLimitCount: 1, AssignedTenantCount: 1},
	}
	requestJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	store.beginResult = SaaSAdminApproval{
		ID: 85, RequestNo: "APR-TENANT-PACKAGE-85", ActionType: SaaSAdminApprovalActionTenantPackageUpdate,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 6,
		RequestJSON: string(requestJSON),
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":85,"expectedVersion":5}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()

	handler.ApprovalExecute(rec, req)

	base := store.fakeSaaSAdminAccessStore.fakeSaaSAdminStore
	if rec.Code != http.StatusOK || base.updateCalls != 1 || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d updates=%d finish=%+v body=%s", rec.Code, base.updateCalls, store.finishInput, rec.Body.String())
	}
	input := base.lastUpdate
	if input.ActorUserID != 9 || input.ActorTenantID != 1 || input.ApprovalExecutionID != 85 ||
		input.ApprovalExecutionVersion != 6 || input.ExpectedVersion != 4 || input.ExpectedPackageVersion != 3 ||
		input.ExpectedTenantStatus != 1 {
		t.Fatalf("tenant package input = %+v", input)
	}
	data := decodeSaaSAdminResponse(t, rec)
	result := data["result"].(map[string]any)
	if result["version"].(float64) != 5 || result["operationId"].(float64) != 85 ||
		result["impact"].(map[string]any)["changedLimitCount"].(float64) != 1 {
		t.Fatalf("data = %+v", data)
	}
}
