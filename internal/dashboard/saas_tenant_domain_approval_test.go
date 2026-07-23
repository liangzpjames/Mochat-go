package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeSaaSTenantDomainApprovalCombinedStore struct {
	*fakeSaaSAdminApprovalStore
	domainStore       *fakeSaaSAdminTenantDomainStore
	plan              SaaSTenantDomainCommandApprovalPlan
	planRequest       SaaSAdminTenantDomainRequest
	planCalls         int
	createTarget      SaaSAdminTenantDomainCreateTarget
	createTargetCalls int
}

func newFakeSaaSTenantDomainApprovalStore(user User, action string) *fakeSaaSTenantDomainApprovalCombinedStore {
	domain := SaaSTenantDomain{
		ID: 18, TenantID: 9, TenantName: "客户租户", Hostname: "login.customer.example.com",
		Status: SaaSTenantDomainStatusActive, IsPrimary: false, VerifiedAt: "2026-07-11 20:00:00", Version: 4,
	}
	snapshot := SaaSTenantDomainApprovalSnapshot{
		ID: domain.ID, TenantID: domain.TenantID, Hostname: domain.Hostname, Status: domain.Status,
		IsPrimary: domain.IsPrimary, VerifiedAt: domain.VerifiedAt, Version: domain.Version,
	}
	base := &fakeSaaSAdminStore{users: map[int]User{user.ID: user}}
	domainStore := &fakeSaaSAdminTenantDomainStore{
		fakeSaaSAdminStore: base,
		domain:             domain,
		commandResult: SaaSAdminTenantDomainResult{
			Domain: SaaSTenantDomain{
				ID: domain.ID, TenantID: domain.TenantID, TenantName: domain.TenantName, Hostname: domain.Hostname,
				Status: SaaSTenantDomainStatusActive, IsPrimary: action == SaaSTenantDomainActionSetPrimary,
				VerifiedAt: domain.VerifiedAt, Version: domain.Version + 1,
			},
			OperationID: 9018,
		},
	}
	return &fakeSaaSTenantDomainApprovalCombinedStore{
		fakeSaaSAdminApprovalStore: &fakeSaaSAdminApprovalStore{
			fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{fakeSaaSAdminStore: base},
		},
		domainStore: domainStore,
		createTarget: SaaSAdminTenantDomainCreateTarget{
			TenantID: domain.TenantID, TenantName: domain.TenantName, TenantStatus: 1,
			Hostname: "new.customer.example.com", DomainCount: 1,
		},
		plan: SaaSTenantDomainCommandApprovalPlan{
			SchemaVersion: SaaSTenantDomainApprovalPlanSchemaVersion,
			Command: SaaSAdminTenantDomainCommand{
				ID: domain.ID, Action: action, ExpectedVersion: domain.Version,
			},
			Domain:         snapshot,
			RoutingDomains: []SaaSTenantDomainApprovalSnapshot{snapshot},
			RoutingSHA256:  strings.Repeat("a", 64),
		},
	}
}

func (s *fakeSaaSTenantDomainApprovalCombinedStore) SaaSTenantDomainByHostname(ctx context.Context, hostname string) (SaaSTenantDomain, bool, error) {
	return s.domainStore.SaaSTenantDomainByHostname(ctx, hostname)
}

func (s *fakeSaaSTenantDomainApprovalCombinedStore) SaaSAdminTenantDomains(ctx context.Context, options SaaSAdminTenantDomainOptions) ([]SaaSTenantDomain, error) {
	return s.domainStore.SaaSAdminTenantDomains(ctx, options)
}

func (s *fakeSaaSTenantDomainApprovalCombinedStore) SaaSAdminTenantDomain(ctx context.Context, id int64) (SaaSTenantDomain, error) {
	return s.domainStore.SaaSAdminTenantDomain(ctx, id)
}

func (s *fakeSaaSTenantDomainApprovalCombinedStore) CreateSaaSAdminTenantDomain(ctx context.Context, input SaaSAdminTenantDomainCreate) (SaaSAdminTenantDomainResult, error) {
	return s.domainStore.CreateSaaSAdminTenantDomain(ctx, input)
}

func (s *fakeSaaSTenantDomainApprovalCombinedStore) CompleteSaaSAdminTenantDomainVerification(ctx context.Context, input SaaSAdminTenantDomainVerification) (SaaSAdminTenantDomainResult, error) {
	return s.domainStore.CompleteSaaSAdminTenantDomainVerification(ctx, input)
}

func (s *fakeSaaSTenantDomainApprovalCombinedStore) ApplySaaSAdminTenantDomainCommand(ctx context.Context, input SaaSAdminTenantDomainCommand) (SaaSAdminTenantDomainResult, error) {
	return s.domainStore.ApplySaaSAdminTenantDomainCommand(ctx, input)
}

func (s *fakeSaaSTenantDomainApprovalCombinedStore) PlanSaaSAdminTenantDomainCommand(_ context.Context, request SaaSAdminTenantDomainRequest) (SaaSTenantDomainCommandApprovalPlan, error) {
	s.planCalls++
	s.planRequest = request
	return s.plan, nil
}

func (s *fakeSaaSTenantDomainApprovalCombinedStore) SaaSAdminTenantDomainCreateTarget(_ context.Context, tenantID int, hostname string) (SaaSAdminTenantDomainCreateTarget, error) {
	s.createTargetCalls++
	target := s.createTarget
	target.TenantID = tenantID
	target.Hostname = hostname
	return target, nil
}

func TestSaaSTenantDomainDirectCreateRequiresApprovalBeforeTokenGeneration(t *testing.T) {
	store := newFakeSaaSTenantDomainApprovalStore(User{ID: 1, Name: "平台超管", TenantID: 1, IsSuperAdmin: 1}, SaaSTenantDomainActionSetPrimary)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenantDomain", strings.NewReader(`{"action":"create","tenantId":9,"hostname":"New.Customer.Example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.TenantDomain(rec, req)

	if rec.Code != http.StatusPreconditionRequired || store.domainStore.createCalls != 0 ||
		!strings.Contains(rec.Body.String(), SaaSAdminApprovalActionTenantDomainCreate) {
		t.Fatalf("status=%d createCalls=%d body=%s", rec.Code, store.domainStore.createCalls, rec.Body.String())
	}
}

func TestSaaSTenantDomainCreateApprovalRequestFreezesNormalizedTarget(t *testing.T) {
	store := newFakeSaaSTenantDomainApprovalStore(User{ID: 7, Name: "域名申请人", TenantID: 1, IsSuperAdmin: 1}, SaaSTenantDomainActionSetPrimary)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"tenant.domain.create",
		"payload":{"action":"create","tenantId":9,"hostname":"New.Customer.Example.com"},
		"reason":"已复核客户域名归属","idempotencyKey":"tenant-domain-create-unit"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)

	if rec.Code != http.StatusOK || store.createCalls != 1 || store.createTargetCalls != 1 {
		t.Fatalf("status=%d createCalls=%d targetCalls=%d body=%s", rec.Code, store.createCalls, store.createTargetCalls, rec.Body.String())
	}
	input := store.createInput
	if input.ActionType != SaaSAdminApprovalActionTenantDomainCreate || input.RequiredPermission != SaaSAdminPermissionDomainsManage ||
		input.RequiredApprovals != 2 || input.TargetType != SaaSAdminOperationTargetTenantDomain ||
		input.TargetID != "9:new.customer.example.com" || !strings.Contains(input.TargetName, "new.customer.example.com") {
		t.Fatalf("approval input = %+v", input)
	}
	var request SaaSAdminTenantDomainRequest
	if err := json.Unmarshal([]byte(input.RequestJSON), &request); err != nil {
		t.Fatal(err)
	}
	if request.Action != SaaSTenantDomainActionCreate || request.TenantID != 9 || request.Hostname != "new.customer.example.com" ||
		strings.Contains(input.RequestJSON, "verificationToken") || strings.Contains(input.RequestJSON, `"token"`) {
		t.Fatalf("normalized request = %+v json=%s", request, input.RequestJSON)
	}
}

func TestSaaSTenantDomainCreateApprovalGeneratesTokenOnlyAtExecution(t *testing.T) {
	store := newFakeSaaSTenantDomainApprovalStore(User{ID: 9, Name: "域名执行人", TenantID: 1, IsSuperAdmin: 1}, SaaSTenantDomainActionSetPrimary)
	requestJSON := `{"action":"create","tenantId":9,"hostname":"new.customer.example.com"}`
	store.domainStore.createResult = SaaSAdminTenantDomainResult{Domain: SaaSTenantDomain{
		ID: 19, TenantID: 9, TenantName: "客户租户", Hostname: "new.customer.example.com",
		Status: SaaSTenantDomainStatusPending, VerificationToken: "generated-token", Version: 1,
	}, OperationID: 9019}
	store.beginResult = SaaSAdminApproval{
		ID: 92, RequestNo: "APR-DOMAIN-92", ActionType: SaaSAdminApprovalActionTenantDomainCreate,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 4,
		RequestJSON: requestJSON,
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":92,"expectedVersion":3}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)

	if rec.Code != http.StatusOK || store.domainStore.createCalls != 1 || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d createCalls=%d finish=%+v body=%s", rec.Code, store.domainStore.createCalls, store.finishInput, rec.Body.String())
	}
	input := store.domainStore.lastCreate
	if input.TenantID != 9 || input.Hostname != "new.customer.example.com" || len(input.Token) != 43 ||
		input.ActorUserID != 9 || input.ActorTenantID != 1 || input.ApprovalExecutionID != 92 || input.ApprovalExecutionVersion != 4 {
		t.Fatalf("tenant domain create input = %+v", input)
	}
	if strings.Contains(requestJSON, "generated-token") || strings.Contains(store.finishInput.ResultJSON, "generated-token") ||
		strings.Contains(store.finishInput.ResultJSON, "verificationToken\"") || !strings.Contains(store.finishInput.ResultJSON, "verificationTokenDelivered") {
		t.Fatalf("persistent result leaked token: request=%s result=%s", requestJSON, store.finishInput.ResultJSON)
	}
}

func TestSaaSTenantDomainDirectLifecycleCommandRequiresApproval(t *testing.T) {
	store := newFakeSaaSTenantDomainApprovalStore(User{ID: 1, Name: "平台超管", TenantID: 1, IsSuperAdmin: 1}, SaaSTenantDomainActionSetPrimary)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/tenantDomain", strings.NewReader(`{"action":"set_primary","id":18,"expectedVersion":4}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.TenantDomain(rec, req)

	if rec.Code != http.StatusPreconditionRequired || store.domainStore.commandCalls != 0 ||
		!strings.Contains(rec.Body.String(), SaaSAdminApprovalActionTenantDomainCommand) {
		t.Fatalf("status=%d commandCalls=%d body=%s", rec.Code, store.domainStore.commandCalls, rec.Body.String())
	}
}

func TestSaaSTenantDomainApprovalRequestFreezesRoutingSnapshot(t *testing.T) {
	store := newFakeSaaSTenantDomainApprovalStore(User{ID: 7, Name: "域名申请人", TenantID: 1, IsSuperAdmin: 1}, SaaSTenantDomainActionSetPrimary)
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{
		"actionType":"tenant.domain.command",
		"payload":{"action":"set_primary","id":18,"expectedVersion":4},
		"reason":"已复核客户域名切换窗口","idempotencyKey":"tenant-domain-unit"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ApprovalRequest(rec, req)

	if rec.Code != http.StatusOK || store.createCalls != 1 || store.planCalls != 1 {
		t.Fatalf("status=%d createCalls=%d planCalls=%d body=%s", rec.Code, store.createCalls, store.planCalls, rec.Body.String())
	}
	input := store.createInput
	if input.ActionType != SaaSAdminApprovalActionTenantDomainCommand || input.RequiredPermission != SaaSAdminPermissionDomainsManage ||
		input.RequiredApprovals != 2 || input.TargetType != SaaSAdminOperationTargetTenantDomain ||
		input.TargetID != "18" || !strings.Contains(input.TargetName, "login.customer.example.com") {
		t.Fatalf("approval input = %+v", input)
	}
	var plan SaaSTenantDomainCommandApprovalPlan
	if err := json.Unmarshal([]byte(input.RequestJSON), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.SchemaVersion != SaaSTenantDomainApprovalPlanSchemaVersion || plan.Command.Action != SaaSTenantDomainActionSetPrimary ||
		plan.Domain.ID != 18 || plan.Domain.Version != 4 || len(plan.RoutingDomains) != 1 || len(plan.RoutingSHA256) != 64 ||
		strings.Contains(input.RequestJSON, "verificationToken") || strings.Contains(input.RequestJSON, "token") {
		t.Fatalf("frozen plan = %+v json=%s", plan, input.RequestJSON)
	}
}

func TestSaaSTenantDomainApprovalExecuteUsesFrozenPlan(t *testing.T) {
	store := newFakeSaaSTenantDomainApprovalStore(User{ID: 9, Name: "域名执行人", TenantID: 1, IsSuperAdmin: 1}, SaaSTenantDomainActionSetPrimary)
	requestJSON, err := json.Marshal(store.plan)
	if err != nil {
		t.Fatal(err)
	}
	store.beginResult = SaaSAdminApproval{
		ID: 90, RequestNo: "APR-DOMAIN-90", ActionType: SaaSAdminApprovalActionTenantDomainCommand,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 6,
		RequestJSON: string(requestJSON),
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":90,"expectedVersion":5}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)

	if rec.Code != http.StatusOK || store.domainStore.commandCalls != 1 || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("status=%d commandCalls=%d finish=%+v body=%s", rec.Code, store.domainStore.commandCalls, store.finishInput, rec.Body.String())
	}
	input := store.domainStore.lastCommand
	if input.ID != 18 || input.Action != SaaSTenantDomainActionSetPrimary || input.ExpectedVersion != 4 || input.Token != "" ||
		input.ApprovalPlan == nil || input.ApprovalPlan.RoutingSHA256 != store.plan.RoutingSHA256 ||
		input.ActorUserID != 9 || input.ActorTenantID != 1 || input.ApprovalExecutionID != 90 || input.ApprovalExecutionVersion != 6 {
		t.Fatalf("tenant domain input = %+v", input)
	}
}

func TestSaaSTenantDomainRotateTokenGeneratedOnlyAtApprovalExecution(t *testing.T) {
	store := newFakeSaaSTenantDomainApprovalStore(User{ID: 9, Name: "域名执行人", TenantID: 1, IsSuperAdmin: 1}, SaaSTenantDomainActionRotateToken)
	requestJSON, err := json.Marshal(store.plan)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(requestJSON), `"token":`) || strings.Contains(string(requestJSON), "verificationToken") {
		t.Fatalf("approval payload leaked token: %s", requestJSON)
	}
	store.beginResult = SaaSAdminApproval{
		ID: 91, RequestNo: "APR-DOMAIN-91", ActionType: SaaSAdminApprovalActionTenantDomainCommand,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9, Version: 3,
		RequestJSON: string(requestJSON),
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithHighRiskApprovalRequired(true)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":91,"expectedVersion":2}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)

	if rec.Code != http.StatusOK || store.domainStore.commandCalls != 1 || len(store.domainStore.lastCommand.Token) != 43 {
		t.Fatalf("status=%d command=%+v body=%s", rec.Code, store.domainStore.lastCommand, rec.Body.String())
	}
}
