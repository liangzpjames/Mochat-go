package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func newPackageUpsertApprovalStore(user User, current SaaSAdminPackage) *fakeSaaSAdminApprovalStore {
	base := &fakeSaaSAdminStore{
		users:    map[int]User{user.ID: user},
		packages: []SaaSAdminPackage{current},
		overview: SaaSAdminOverview{Tenants: []SaaSAdminTenantOverview{{
			TenantID: 12, TenantName: "使用规模版租户", PackageCode: current.Code,
		}}},
		usageByTenant: map[int][]SaaSAdminUsageMetric{
			12: {{Metric: SaaSMetricUsers, Current: 200, Limit: current.Limits.MaxUsers}},
		},
		upsertPackageResult: SaaSAdminPackage{
			Code: current.Code, Name: "规模版 2026", Description: "适用成长型团队",
			Status: 1, Version: current.Version + 1,
			Limits: SaaSAdminPackageLimits{MaxUsers: 120, ChannelCodes: 33, AsyncExecutions: 1000},
		},
	}
	return &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{fakeSaaSAdminStore: base}}
}

func testPackageUpsertCurrent() SaaSAdminPackage {
	return SaaSAdminPackage{
		Code: "scale", Name: "规模版", Description: "旧说明", Status: 1, Version: 4,
		Limits: SaaSAdminPackageLimits{MaxUsers: 300, ChannelCodes: 33, AsyncExecutions: 1000},
	}
}

func testPackageUpsertPayload(expectedVersion int) string {
	return `{"code":" scale ","name":" 规模版 2026 ","description":" 适用成长型团队 ","status":1,"expectedVersion":` +
		strconv.Itoa(expectedVersion) + `,"limits":{"maxUsers":120,"channelCodes":33,"asyncExecutions":1000}}`
}

func TestSaaSAdminApprovalRequestFreezesPackageDefinitionAndImpact(t *testing.T) {
	store := newPackageUpsertApprovalStore(User{ID: 7, Name: "套餐发起人", TenantID: 1, IsSuperAdmin: 1}, testPackageUpsertCurrent())
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"package.upsert",
		"payload":`+testPackageUpsertPayload(4)+`,
		"reason":"调整规模版权益","idempotencyKey":"approval-unit-package-upsert"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.ApprovalRequest(rec, req)

	if rec.Code != http.StatusOK || store.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
	input := store.createInput
	if input.ActionType != SaaSAdminApprovalActionPackageUpsert || input.RequiredPermission != SaaSAdminPermissionTenantsManage ||
		input.RequiredApprovals != 2 || input.TargetType != "saas_package" || input.TargetID != "scale" || input.TargetName != "规模版 2026" {
		t.Fatalf("create input = %+v", input)
	}
	var plan SaaSAdminPackageUpsertPlan
	if err := json.Unmarshal([]byte(input.RequestJSON), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Update.Code != "scale" || plan.Update.Name != "规模版 2026" || plan.Update.Description != "适用成长型团队" ||
		plan.Update.ExpectedVersion != 4 || plan.Current == nil || plan.Current.Version != 4 || plan.Current.Limits.MaxUsers != 300 {
		t.Fatalf("frozen plan = %+v", plan)
	}
	if !plan.Impact.Existing || plan.Impact.AssignedTenantCount != 1 || plan.Impact.CheckedTenantCount != 1 ||
		plan.Impact.ChangedLimitCount != 1 || plan.Impact.DecreasedLimitCount != 1 || plan.Impact.OverLimitTenantCount != 1 {
		t.Fatalf("frozen impact = %+v", plan.Impact)
	}
}

func TestSaaSAdminApprovalRequestRejectsStalePackageVersion(t *testing.T) {
	store := newPackageUpsertApprovalStore(User{ID: 7, Name: "套餐发起人", TenantID: 1, IsSuperAdmin: 1}, testPackageUpsertCurrent())
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"package.upsert","payload":`+testPackageUpsertPayload(3)+`,"reason":"使用旧版本调整套餐"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.ApprovalRequest(rec, req)

	if rec.Code != http.StatusConflict || store.createCalls != 0 || !strings.Contains(rec.Body.String(), "版本已变化") {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
}

func TestSaaSAdminPackageDirectUpdateRequiresApproval(t *testing.T) {
	store := newPackageUpsertApprovalStore(User{ID: 1, Name: "平台超管", TenantID: 1, IsSuperAdmin: 1}, testPackageUpsertCurrent())
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/package", strings.NewReader(testPackageUpsertPayload(4)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.UpsertPackage(rec, req)

	base := store.fakeSaaSAdminAccessStore.fakeSaaSAdminStore
	if rec.Code != http.StatusPreconditionRequired || base.upsertPackageCalls != 0 || base.packageCalls != 0 ||
		!strings.Contains(rec.Body.String(), SaaSAdminApprovalActionPackageUpsert) {
		t.Fatalf("status=%d upsert=%d packageReads=%d body=%s", rec.Code, base.upsertPackageCalls, base.packageCalls, rec.Body.String())
	}
}

func TestSaaSAdminApprovalExecuteUpdatesPackageWithExecutionLease(t *testing.T) {
	store := newPackageUpsertApprovalStore(User{ID: 9, Name: "套餐执行人", TenantID: 1, IsSuperAdmin: 1}, testPackageUpsertCurrent())
	plan := SaaSAdminPackageUpsertPlan{
		Update: SaaSAdminPackageUpsert{
			Code: "scale", Name: "规模版 2026", Description: "适用成长型团队", Status: 1, ExpectedVersion: 4,
			Limits: SaaSAdminPackageLimits{MaxUsers: 120, ChannelCodes: 33, AsyncExecutions: 1000},
		},
		Current: ptrSaaSAdminPackage(testPackageUpsertCurrent()),
		Impact:  SaaSAdminPackageImpact{Existing: true, ChangedLimitCount: 1, DecreasedLimitCount: 1, AssignedTenantCount: 1},
	}
	requestJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	store.beginResult = SaaSAdminApproval{
		ID: 84, RequestNo: "APR-PACKAGE-84", ActionType: SaaSAdminApprovalActionPackageUpsert,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 6,
		RequestJSON: string(requestJSON),
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":84,"expectedVersion":5}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()

	handler.ApprovalExecute(rec, req)

	base := store.fakeSaaSAdminAccessStore.fakeSaaSAdminStore
	if rec.Code != http.StatusOK || base.upsertPackageCalls != 1 || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d upsert=%d finish=%+v body=%s", rec.Code, base.upsertPackageCalls, store.finishInput, rec.Body.String())
	}
	input := base.lastPackageUpsert
	if input.ActorUserID != 9 || input.ActorTenantID != 1 || input.ApprovalExecutionID != 84 ||
		input.ApprovalExecutionVersion != 6 || input.ExpectedVersion != 4 {
		t.Fatalf("package input = %+v", input)
	}
	data := decodeSaaSAdminResponse(t, rec)
	result := data["result"].(map[string]any)
	if result["version"].(float64) != 5 || result["impact"].(map[string]any)["changedLimitCount"].(float64) != 1 {
		t.Fatalf("data = %+v", data)
	}
}

func ptrSaaSAdminPackage(item SaaSAdminPackage) *SaaSAdminPackage {
	return &item
}
