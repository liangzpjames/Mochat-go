package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newSubscriptionTransitionApprovalStore(user User) *fakeSaaSAdminApprovalStore {
	base := &fakeSaaSAdminStore{
		users: map[int]User{user.ID: user},
		subscriptionReport: SaaSAdminSubscriptionReport{Subscriptions: []SaaSAdminSubscription{{
			ID: 21, TenantID: 12, TenantName: "订阅客户", TenantStatus: 1,
			PackageCode: "growth", PackageName: "成长版",
			Status: SaaSAdminSubscriptionStatusActive, EffectiveStatus: SaaSAdminSubscriptionStatusActive,
			BillingCycle: "custom", CurrentPeriodStartsAt: "2036-01-01 00:00:00",
			CurrentPeriodEndsAt: "2037-01-01 00:00:00", GraceEndsAt: "2037-01-08 00:00:00",
			LatestBillingEventID: 88, Version: 6, StateReason: "续费已生效",
		}}},
		subscriptionTransitionResult: SaaSAdminSubscriptionTransitionResult{
			Subscription: SaaSAdminSubscription{
				ID: 21, TenantID: 12, TenantName: "订阅客户", TenantStatus: 1,
				Status: SaaSAdminSubscriptionStatusSuspended, EffectiveStatus: SaaSAdminSubscriptionStatusSuspended,
				Version: 7,
			},
			PreviousStatus: SaaSAdminSubscriptionStatusActive,
			Changed:        true,
			EventID:        8801,
			OperationID:    8802,
		},
	}
	return &fakeSaaSAdminApprovalStore{
		fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{fakeSaaSAdminStore: base},
	}
}

func TestSaaSAdminSubscriptionTransitionDirectRequiresApproval(t *testing.T) {
	store := newSubscriptionTransitionApprovalStore(User{ID: 1, Name: "平台超管", TenantID: 1, IsSuperAdmin: 1})
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/subscriptionTransition", strings.NewReader(`{
		"tenantId":12,"status":"suspended","expectedVersion":6,"reason":"合同暂停"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TransitionSubscription(rec, req)

	base := store.fakeSaaSAdminAccessStore.fakeSaaSAdminStore
	if rec.Code != http.StatusPreconditionRequired || base.subscriptionTransitionCalls != 0 ||
		!strings.Contains(rec.Body.String(), SaaSAdminApprovalActionSubscriptionTransition) {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, base.subscriptionTransitionCalls, rec.Body.String())
	}
}

func TestSaaSAdminApprovalRequestFreezesSubscriptionTransitionSnapshot(t *testing.T) {
	store := newSubscriptionTransitionApprovalStore(User{ID: 7, Name: "订阅申请人", TenantID: 1, IsSuperAdmin: 1})
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"tenant.subscription.transition",
		"payload":{"tenantId":12,"status":"suspended","expectedVersion":6,"reason":"合同暂停"},
		"reason":"合同与客户通知已复核","idempotencyKey":"approval-unit-subscription-transition"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.ApprovalRequest(rec, req)

	if rec.Code != http.StatusOK || store.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
	input := store.createInput
	if input.ActionType != SaaSAdminApprovalActionSubscriptionTransition ||
		input.RequiredPermission != SaaSAdminPermissionFinanceManage ||
		input.RequiredApprovals != 2 ||
		input.TargetType != SaaSAdminOperationTargetSubscription ||
		input.TargetID != "21" ||
		input.TargetName != "订阅客户 / active -> suspended" {
		t.Fatalf("create input = %+v", input)
	}
	var plan SaaSAdminSubscriptionTransitionApprovalPlan
	if err := json.Unmarshal([]byte(input.RequestJSON), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Subscription.ID != 21 ||
		plan.Subscription.TenantID != 12 ||
		plan.Subscription.TenantStatus != 1 ||
		plan.Subscription.Status != SaaSAdminSubscriptionStatusActive ||
		plan.Subscription.PackageCode != "growth" ||
		plan.Subscription.CurrentPeriodEndsAt != "2037-01-01 00:00:00" ||
		plan.Subscription.LatestBillingEventID != 88 ||
		plan.Subscription.Version != 6 ||
		plan.Transition.ExpectedVersion != 6 ||
		plan.Transition.Status != SaaSAdminSubscriptionStatusSuspended ||
		plan.Transition.Source != "approval" ||
		!strings.HasPrefix(plan.Transition.IdempotencyKey, "approval:") {
		t.Fatalf("frozen plan = %+v", plan)
	}
}

func TestSaaSAdminApprovalRequestRejectsStaleSubscriptionTransitionVersion(t *testing.T) {
	store := newSubscriptionTransitionApprovalStore(User{ID: 7, Name: "订阅申请人", TenantID: 1, IsSuperAdmin: 1})
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"tenant.subscription.transition",
		"payload":{"tenantId":12,"status":"suspended","expectedVersion":5,"reason":"合同暂停"},
		"reason":"合同与客户通知已复核","idempotencyKey":"approval-unit-subscription-transition-stale"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.ApprovalRequest(rec, req)

	if rec.Code != http.StatusConflict || store.createCalls != 0 || !strings.Contains(rec.Body.String(), "订阅版本已变化") {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
}

func TestSaaSAdminApprovalExecuteTransitionsSubscriptionWithFrozenReferences(t *testing.T) {
	store := newSubscriptionTransitionApprovalStore(User{ID: 9, Name: "订阅执行人", TenantID: 1, IsSuperAdmin: 1})
	plan := SaaSAdminSubscriptionTransitionApprovalPlan{
		Transition: SaaSAdminSubscriptionTransition{
			TenantID: 12, Status: SaaSAdminSubscriptionStatusSuspended, ExpectedVersion: 6,
			IdempotencyKey: "approval:transition-88", Reason: "合同暂停", Source: "approval",
		},
		Subscription: SaaSAdminSubscriptionTransitionSnapshot{
			ID: 21, TenantID: 12, TenantName: "订阅客户", TenantStatus: 1,
			PackageCode: "growth", PackageName: "成长版",
			Status: SaaSAdminSubscriptionStatusActive, BillingCycle: "custom",
			CurrentPeriodEndsAt: "2037-01-01 00:00:00", Version: 6,
		},
	}
	requestJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	store.beginResult = SaaSAdminApproval{
		ID: 88, RequestNo: "APR-SUBSCRIPTION-88", ActionType: SaaSAdminApprovalActionSubscriptionTransition,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 6,
		RequestJSON: string(requestJSON),
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":88,"expectedVersion":5}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()

	handler.ApprovalExecute(rec, req)

	base := store.fakeSaaSAdminAccessStore.fakeSaaSAdminStore
	if rec.Code != http.StatusOK || base.subscriptionTransitionCalls != 1 || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d calls=%d finish=%+v body=%s", rec.Code, base.subscriptionTransitionCalls, store.finishInput, rec.Body.String())
	}
	input := base.lastSubscriptionTransition
	if input.ExpectedSubscriptionID != 21 ||
		input.ExpectedTenantStatus != 1 ||
		input.ExpectedVersion != 6 ||
		input.ActorUserID != 9 ||
		input.ActorTenantID != 1 ||
		input.ApprovalExecutionID != 88 ||
		input.ApprovalExecutionVersion != 6 ||
		input.Source != "approval" {
		t.Fatalf("transition input = %+v", input)
	}
	data := decodeSaaSAdminResponse(t, rec)
	result := data["result"].(map[string]any)
	if result["eventId"].(float64) != 8801 || result["operationId"].(float64) != 8802 {
		t.Fatalf("data = %+v", data)
	}
}
