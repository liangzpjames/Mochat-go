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

func newTenantRenewalApprovalStore(user User) *fakeSaaSAdminApprovalStore {
	base := &fakeSaaSAdminStore{
		users: map[int]User{user.ID: user},
		overview: SaaSAdminOverview{Tenants: []SaaSAdminTenantOverview{{
			TenantID: 12, TenantName: "续费客户", TenantStatus: 1,
			PackageCode: "growth", PackageName: "成长版", PackageStatus: 1, PackageVersion: 4,
			ExpiresAt: "2027-01-01 00:00:00", PackageLimits: SaaSAdminPackageLimits{MaxUsers: 120},
		}}},
		packages: []SaaSAdminPackage{{
			Code: "growth", Name: "成长版", Status: 1, Version: 3,
			Limits: SaaSAdminPackageLimits{MaxUsers: 120},
		}},
		subscriptionReport: SaaSAdminSubscriptionReport{Subscriptions: []SaaSAdminSubscription{{
			ID: 21, TenantID: 12, TenantName: "续费客户", TenantStatus: 1,
			PackageCode: "growth", PackageName: "成长版", Status: SaaSAdminSubscriptionStatusActive,
			CurrentPeriodEndsAt: "2027-01-01 00:00:00", LatestBillingEventID: 88, Version: 6,
		}}},
		renewalResult: SaaSAdminTenantRenewalResult{
			TenantID: 12, TenantName: "续费客户", PackageCode: "growth", PackageName: "成长版",
			PreviousExpiresAt: "2027-01-01 00:00:00", ExpiresAt: "2028-01-01 00:00:00",
			AmountCents: 1280000, Currency: "CNY", BillingEventID: 8701, OperationID: 8702,
			MetricsRefreshed: 28,
		},
	}
	return &fakeSaaSAdminApprovalStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{fakeSaaSAdminStore: base}}
}

func TestSaaSAdminTenantRenewalDirectAndTaskApplyRequireApproval(t *testing.T) {
	for _, item := range []struct {
		name    string
		path    string
		handler func(*SaaSAdminHandler, http.ResponseWriter, *http.Request)
		body    string
	}{
		{
			name: "direct", path: "/dashboard/saasAdmin/tenantRenewal",
			handler: func(handler *SaaSAdminHandler, w http.ResponseWriter, r *http.Request) { handler.RenewTenant(w, r) },
			body:    `{"tenantId":12,"packageCode":"growth","expiresAt":"2028-01-01","amount":"12800"}`,
		},
		{
			name: "task", path: "/dashboard/saasAdmin/tenantRenewalTaskApply",
			handler: func(handler *SaaSAdminHandler, w http.ResponseWriter, r *http.Request) {
				handler.TenantRenewalTaskApply(w, r)
			},
			body: `{"taskId":87}`,
		},
		{
			name: "bulk", path: "/dashboard/saasAdmin/tenantRenewalTaskBulkApply",
			handler: func(handler *SaaSAdminHandler, w http.ResponseWriter, r *http.Request) {
				handler.TenantRenewalTaskBulkApply(w, r)
			},
			body: `{"taskType":"tenant_renewal","status":"pending","limit":100}`,
		},
	} {
		t.Run(item.name, func(t *testing.T) {
			store := newTenantRenewalApprovalStore(User{ID: 1, Name: "平台超管", TenantID: 1, IsSuperAdmin: 1})
			handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
			req := httptest.NewRequest(http.MethodPost, item.path, strings.NewReader(item.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Mochat-Go-User-ID", "1")
			rec := httptest.NewRecorder()

			item.handler(handler, rec, req)

			base := store.fakeSaaSAdminAccessStore.fakeSaaSAdminStore
			if rec.Code != http.StatusPreconditionRequired || base.renewalCalls != 0 || base.taskCalls != 0 ||
				!strings.Contains(rec.Body.String(), SaaSAdminApprovalActionTenantRenewal) {
				t.Fatalf("status=%d renewals=%d tasks=%d body=%s", rec.Code, base.renewalCalls, base.taskCalls, rec.Body.String())
			}
		})
	}
}

func TestSaaSAdminApprovalRequestFreezesDirectTenantRenewalState(t *testing.T) {
	store := newTenantRenewalApprovalStore(User{ID: 7, Name: "续费申请人", TenantID: 1, IsSuperAdmin: 1})
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"tenant.renewal",
		"payload":{"tenantId":12,"packageCode":"growth","expiresAt":"2028-01-01","amount":"12800","currency":"cny","externalOrderNo":"ORDER-0087","remark":"续费一年"},
		"reason":"客户合同与回款已复核","idempotencyKey":"approval-unit-tenant-renewal-direct"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.ApprovalRequest(rec, req)

	if rec.Code != http.StatusOK || store.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
	input := store.createInput
	if input.ActionType != SaaSAdminApprovalActionTenantRenewal ||
		input.RequiredPermission != SaaSAdminPermissionTenantsManage ||
		input.RequiredApprovals != 2 ||
		input.TargetType != "saas_tenant_renewal" ||
		input.TargetID != "12" ||
		input.TargetName != "续费客户 / 成长版 / 2028-01-01 00:00:00" {
		t.Fatalf("create input = %+v", input)
	}
	var plan SaaSAdminTenantRenewalApprovalPlan
	if err := json.Unmarshal([]byte(input.RequestJSON), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Source != "direct" ||
		plan.ExpectedTenantStatus != 1 ||
		plan.Renewal.TenantID != 12 ||
		plan.Renewal.PackageCode != "growth" ||
		plan.Renewal.ExpiresAt != "2028-01-01 00:00:00" ||
		plan.Renewal.AmountCents != 1280000 ||
		plan.CurrentPackage == nil ||
		plan.CurrentPackage.Version != 4 ||
		plan.TargetPackage.Version != 3 ||
		!plan.Subscription.Exists ||
		plan.Subscription.Version != 6 ||
		plan.Subscription.LatestBillingEventID != 88 ||
		plan.Preview.SubscriptionVersion != 6 {
		t.Fatalf("frozen plan = %+v", plan)
	}
}

func TestSaaSAdminApprovalRequestFreezesTenantRenewalTaskVersionAndDigest(t *testing.T) {
	store := newTenantRenewalApprovalStore(User{ID: 7, Name: "续费申请人", TenantID: 1, IsSuperAdmin: 1})
	requestJSON := saasAdminPayloadJSON(saasAdminTenantRenewalTaskRequestPayload(SaaSAdminTenantRenewal{
		TenantID: 12, PackageCode: "growth", ExpiresAt: "2028-01-01 00:00:00",
		AmountCents: 1280000, Currency: "CNY", ExternalOrderNo: "TASK-0087", Remark: "任务续费",
	}))
	store.fakeSaaSAdminAccessStore.fakeSaaSAdminStore.tasks = []SaaSAdminTask{{
		ID: 87, TaskType: SaaSAdminTaskTypeTenantRenewal, Status: SaaSAdminTaskStatusPending,
		Version: 5, TenantID: 12, PackageCode: "growth", RequestJSON: requestJSON, Remark: "任务续费",
	}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"tenant.renewal","payload":{"taskId":87,"expectedTaskVersion":5},
		"reason":"执行已复核的续费任务","idempotencyKey":"approval-unit-tenant-renewal-task"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()

	handler.ApprovalRequest(rec, req)

	if rec.Code != http.StatusOK || store.createCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
	var plan SaaSAdminTenantRenewalApprovalPlan
	if err := json.Unmarshal([]byte(store.createInput.RequestJSON), &plan); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(requestJSON))
	if plan.Source != "task" ||
		plan.TaskID != 87 ||
		plan.ExpectedTaskVersion != 5 ||
		plan.ExpectedTaskRequestSHA256 != hex.EncodeToString(digest[:]) ||
		plan.CurrentPackage == nil ||
		plan.CurrentPackage.Version != 4 ||
		plan.TargetPackage.Version != 3 ||
		plan.Subscription.Version != 6 {
		t.Fatalf("frozen plan = %+v", plan)
	}
}

func TestSaaSAdminApprovalExecuteRenewsTenantWithFrozenExecutionReferences(t *testing.T) {
	store := newTenantRenewalApprovalStore(User{ID: 9, Name: "续费执行人", TenantID: 1, IsSuperAdmin: 1})
	plan := SaaSAdminTenantRenewalApprovalPlan{
		Source:                    "task",
		TaskID:                    87,
		ExpectedTaskVersion:       5,
		ExpectedTaskRequestSHA256: strings.Repeat("a", 64),
		ExpectedTenantStatus:      1,
		Renewal: SaaSAdminTenantRenewal{
			TenantID: 12, PackageCode: "growth", ExpiresAt: "2028-01-01 00:00:00",
			AmountCents: 1280000, Currency: "CNY", ExternalOrderNo: "TASK-0087", Remark: "任务续费",
		},
		Preview: SaaSAdminTenantRenewalPreview{
			TenantID: 12, TenantName: "续费客户", TenantStatus: 1,
			PreviousPackageCode: "growth", PreviousPackageName: "成长版", PreviousPackageVersion: 4,
			PackageCode: "growth", PackageName: "成长版", PackageVersion: 3,
			ExpiresAt: "2028-01-01 00:00:00", SubscriptionExists: true, SubscriptionVersion: 6,
		},
		CurrentPackage: &SaaSAdminTenantPackageSnapshot{
			TenantID: 12, TenantName: "续费客户", TenantStatus: 1,
			PackageCode: "growth", PackageName: "成长版", Status: 1, Version: 4,
		},
		TargetPackage: SaaSAdminPackage{Code: "growth", Name: "成长版", Status: 1, Version: 3},
		Subscription: SaaSAdminTenantRenewalSubscriptionSnapshot{
			Exists: true, Status: SaaSAdminSubscriptionStatusActive, Version: 6,
			PackageCode: "growth", PackageName: "成长版", CurrentPeriodEndsAt: "2027-01-01 00:00:00",
		},
	}
	requestJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	store.beginResult = SaaSAdminApproval{
		ID: 87, RequestNo: "APR-TENANT-RENEWAL-87", ActionType: SaaSAdminApprovalActionTenantRenewal,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 6,
		RequestJSON: string(requestJSON),
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":87,"expectedVersion":5}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()

	handler.ApprovalExecute(rec, req)

	base := store.fakeSaaSAdminAccessStore.fakeSaaSAdminStore
	if rec.Code != http.StatusOK || base.renewalCalls != 1 || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d renewals=%d finish=%+v body=%s", rec.Code, base.renewalCalls, store.finishInput, rec.Body.String())
	}
	input := base.lastRenewal
	if input.ActorUserID != 9 ||
		input.ActorTenantID != 1 ||
		input.ApprovalExecutionID != 87 ||
		input.ApprovalExecutionVersion != 6 ||
		input.ExpectedTenantStatus != 1 ||
		input.ExpectedPackageAssignmentVersion != 4 ||
		input.ExpectedPackageVersion != 3 ||
		!input.ExpectedSubscriptionExists ||
		input.ExpectedSubscriptionVersion != 6 ||
		input.TaskID != 87 ||
		input.ExpectedTaskVersion != 5 ||
		input.ExpectedTaskRequestSHA256 != strings.Repeat("a", 64) {
		t.Fatalf("renewal input = %+v", input)
	}
	data := decodeSaaSAdminResponse(t, rec)
	result := data["result"].(map[string]any)
	if result["billingEventId"].(float64) != 8701 ||
		result["operationId"].(float64) != 8702 ||
		result["source"] != "task" ||
		result["taskId"].(float64) != 87 {
		t.Fatalf("data = %+v", data)
	}
}
