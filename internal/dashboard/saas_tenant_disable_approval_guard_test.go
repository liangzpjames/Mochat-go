package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSaaSAdminTenantDisablePolicyRequiresTwoApprovers(t *testing.T) {
	policy, found := SaaSAdminApprovalPolicyByAction(SaaSAdminApprovalActionTenantDisable)
	if !found || !policy.Enabled || policy.RequiredApprovals != 2 || policy.RiskLevel != SaaSAdminApprovalRiskCritical {
		t.Fatalf("tenant disable policy = %+v found=%t", policy, found)
	}
}

func TestSaaSAdminTenantDisablePolicyCannotBeWeakened(t *testing.T) {
	for _, body := range []saasAdminApprovalPolicyBody{
		{ActionType: SaaSAdminApprovalActionTenantDisable, Enabled: false, RequiredApprovals: 2, SLAMinutes: 240, ReminderMinutes: 60, ExpiryHours: 24, ExpectedVersion: 1},
		{ActionType: SaaSAdminApprovalActionTenantDisable, Enabled: true, RequiredApprovals: 1, SLAMinutes: 240, ReminderMinutes: 60, ExpiryHours: 24, ExpectedVersion: 1},
	} {
		if err := validateSaaSAdminApprovalPolicyBody(&body); err == nil {
			t.Fatalf("expected tenant disable policy weakening to fail: %+v", body)
		}
	}
}

func TestSaaSAdminTenantDisableApprovalRequestUsesTwoApprovers(t *testing.T) {
	store := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, Name: "租户运营", TenantID: 1, IsSuperAdmin: 1}}},
	}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"tenant.disable","payload":{"tenantId":961,"status":2,"remark":"欠费停用"},
		"reason":"复核欠费停用","idempotencyKey":"approval-unit-tenant-disable"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusOK || store.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
	if store.createInput.ActionType != SaaSAdminApprovalActionTenantDisable ||
		store.createInput.RequiredPermission != SaaSAdminPermissionTenantsManage || store.createInput.RequiredApprovals != 2 ||
		store.createInput.TargetID != "961" {
		t.Fatalf("create input = %+v", store.createInput)
	}
}

func TestSaaSAdminTenantEnablePolicyRequiresTwoApprovers(t *testing.T) {
	policy, found := SaaSAdminApprovalPolicyByAction(SaaSAdminApprovalActionTenantEnable)
	if !found || !policy.Enabled || policy.RequiredApprovals != 2 || policy.RiskLevel != SaaSAdminApprovalRiskCritical {
		t.Fatalf("tenant enable policy = %+v found=%t", policy, found)
	}
}

func TestSaaSAdminTenantEnablePolicyCannotBeWeakened(t *testing.T) {
	for _, body := range []saasAdminApprovalPolicyBody{
		{ActionType: SaaSAdminApprovalActionTenantEnable, Enabled: false, RequiredApprovals: 2, SLAMinutes: 240, ReminderMinutes: 60, ExpiryHours: 24, ExpectedVersion: 1},
		{ActionType: SaaSAdminApprovalActionTenantEnable, Enabled: true, RequiredApprovals: 1, SLAMinutes: 240, ReminderMinutes: 60, ExpiryHours: 24, ExpectedVersion: 1},
	} {
		if err := validateSaaSAdminApprovalPolicyBody(&body); err == nil {
			t.Fatalf("expected tenant enable policy weakening to fail: %+v", body)
		}
	}
}

func TestSaaSAdminTenantEnableApprovalRequestFreezesTenantAndSubscription(t *testing.T) {
	baseStore := &fakeSaaSAdminStore{
		users: map[int]User{7: {ID: 7, Name: "租户运营", TenantID: 1, IsSuperAdmin: 1}},
		tenantStatusPlan: SaaSAdminTenantStatusApprovalPlan{
			SchemaVersion: SaaSAdminTenantStatusApprovalPlanSchemaVersion,
			Update: SaaSAdminTenantStatusUpdate{
				TenantID: 961, Status: 1, Remark: "恢复服务", ExpectedStatus: 2,
			},
			Snapshot: SaaSAdminTenantStatusApprovalSnapshot{
				TenantID: 961, TenantName: "北极星科技", TenantStatus: 2, SubscriptionPresent: true,
				SubscriptionID: 81, SubscriptionStatus: SaaSAdminSubscriptionStatusSuspended, SubscriptionVersion: 4,
			},
		},
	}
	store := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{fakeSaaSAdminStore: baseStore}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"tenant.enable","payload":{"tenantId":961,"status":1,"remark":"恢复服务"},
		"reason":"复核租户恢复条件","idempotencyKey":"approval-unit-tenant-enable"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)
	if rec.Code != http.StatusOK || store.createCalls != 1 || baseStore.tenantStatusPlanCalls != 1 {
		t.Fatalf("status=%d createCalls=%d planCalls=%d body=%s", rec.Code, store.createCalls, baseStore.tenantStatusPlanCalls, rec.Body.String())
	}
	if store.createInput.ActionType != SaaSAdminApprovalActionTenantEnable ||
		store.createInput.RequiredPermission != SaaSAdminPermissionTenantsManage || store.createInput.RequiredApprovals != 2 ||
		store.createInput.TargetID != "961" || store.createInput.TargetName != "北极星科技" {
		t.Fatalf("create input = %+v", store.createInput)
	}
	var plan SaaSAdminTenantStatusApprovalPlan
	if err := json.Unmarshal([]byte(store.createInput.RequestJSON), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.SchemaVersion != SaaSAdminTenantStatusApprovalPlanSchemaVersion || plan.Update.Status != 1 || plan.Update.ExpectedStatus != 2 ||
		plan.Snapshot.TenantStatus != 2 || !plan.Snapshot.SubscriptionPresent || plan.Snapshot.SubscriptionID != 81 ||
		plan.Snapshot.SubscriptionStatus != SaaSAdminSubscriptionStatusSuspended || plan.Snapshot.SubscriptionVersion != 4 {
		t.Fatalf("approval plan = %+v", plan)
	}
}

func TestSaaSAdminDirectTenantEnableRequiresApproval(t *testing.T) {
	store := &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, Name: "平台超管", TenantID: 1, IsSuperAdmin: 1}}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantStatus", strings.NewReader(`{"tenantId":961,"status":1,"remark":"恢复服务"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.UpdateTenantStatus(rec, req)
	if rec.Code != http.StatusPreconditionRequired || store.statusCalls != 0 ||
		!strings.Contains(rec.Body.String(), SaaSAdminApprovalActionTenantEnable) {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.statusCalls, rec.Body.String())
	}
}

func TestSaaSAdminTenantEnableApprovalExecutesFrozenPlan(t *testing.T) {
	plan := SaaSAdminTenantStatusApprovalPlan{
		SchemaVersion: SaaSAdminTenantStatusApprovalPlanSchemaVersion,
		Update:        SaaSAdminTenantStatusUpdate{TenantID: 961, Status: 1, Remark: "恢复服务", ExpectedStatus: 2},
		Snapshot: SaaSAdminTenantStatusApprovalSnapshot{
			TenantID: 961, TenantName: "北极星科技", TenantStatus: 2, SubscriptionPresent: true,
			SubscriptionID: 81, SubscriptionStatus: SaaSAdminSubscriptionStatusSuspended, SubscriptionVersion: 4,
		},
	}
	requestJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	baseStore := &fakeSaaSAdminStore{
		users:        map[int]User{9: {ID: 9, Name: "审批执行人", TenantID: 1, IsSuperAdmin: 1}},
		statusResult: SaaSAdminTenantStatusUpdateResult{TenantID: 961, TenantName: "北极星科技", PreviousStatus: 2, Status: 1, OperationID: 96},
	}
	store := &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{fakeSaaSAdminStore: baseStore}}
	store.beginResult = SaaSAdminApproval{
		ID: 96, RequestNo: "APR-TENANT-ENABLE-96", ActionType: SaaSAdminApprovalActionTenantEnable,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 6,
		RequestJSON: string(requestJSON),
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":96,"expectedVersion":5}`))
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)
	if rec.Code != http.StatusOK || baseStore.statusCalls != 1 || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d statusCalls=%d finish=%+v body=%s", rec.Code, baseStore.statusCalls, store.finishInput, rec.Body.String())
	}
	input := baseStore.lastStatusUpdate
	if input.TenantID != 961 || input.Status != 1 || input.ExpectedStatus != 2 || input.ApprovalPlan == nil ||
		input.ActorUserID != 9 || input.ActorTenantID != 1 || input.ApprovalExecutionID != 96 || input.ApprovalExecutionVersion != 6 {
		t.Fatalf("status update = %+v", input)
	}
}
