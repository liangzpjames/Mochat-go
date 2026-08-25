package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

type fakeSaaSAdminAccessStore struct {
	*fakeSaaSAdminStore
	profile           SaaSAdminAccessProfile
	roles             []SaaSAdminAccessRole
	assignments       []SaaSAdminAccessAssignment
	roleInput         SaaSAdminAccessRoleUpsert
	assignmentInput   SaaSAdminAccessAssignmentUpdate
	profileCalls      int
	roleUpdateCalls   int
	assignUpdateCalls int
}

func (s *fakeSaaSAdminAccessStore) SaaSAdminAccessProfile(_ context.Context, userID int, platformTenantID int) (SaaSAdminAccessProfile, error) {
	s.profileCalls++
	profile := s.profile
	profile.UserID = userID
	profile.TenantID = platformTenantID
	return profile, nil
}

func (s *fakeSaaSAdminAccessStore) SaaSAdminAccessRoles(context.Context) ([]SaaSAdminAccessRole, error) {
	return s.roles, nil
}

func (s *fakeSaaSAdminAccessStore) UpsertSaaSAdminAccessRole(_ context.Context, input SaaSAdminAccessRoleUpsert) (SaaSAdminAccessRoleUpsertResult, error) {
	s.roleInput = input
	s.roleUpdateCalls++
	return SaaSAdminAccessRoleUpsertResult{
		Role:        SaaSAdminAccessRole{ID: 9, Code: input.Code, Name: input.Name, Status: input.Status, Version: 1, Permissions: input.Permissions},
		OperationID: 99,
		Created:     input.ID == 0,
	}, nil
}

func (s *fakeSaaSAdminAccessStore) SaaSAdminAccessAssignments(context.Context, int, SaaSAdminAccessAssignmentOptions) ([]SaaSAdminAccessAssignment, error) {
	return s.assignments, nil
}

func (s *fakeSaaSAdminAccessStore) UpdateSaaSAdminAccessAssignment(_ context.Context, _ int, input SaaSAdminAccessAssignmentUpdate) (SaaSAdminAccessAssignmentUpdateResult, error) {
	s.assignmentInput = input
	s.assignUpdateCalls++
	return SaaSAdminAccessAssignmentUpdateResult{
		Assignment:  SaaSAdminAccessAssignment{UserID: input.UserID, Version: input.ExpectedVersion + 1},
		OperationID: 100,
	}, nil
}

func TestSaaSAdminRequiredPermission(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodGet, "/dashboard/saasAdmin/overview", SaaSAdminPermissionOverviewRead},
		{http.MethodGet, "/dashboard/saasAdmin/tenant", SaaSAdminPermissionTenantsRead},
		{http.MethodGet, "/dashboard/saasAdmin/tenantReadiness", SaaSAdminPermissionTenantsRead},
		{http.MethodPut, "/dashboard/saasAdmin/tenantStatus", SaaSAdminPermissionTenantsManage},
		{http.MethodPost, "/dashboard/saasAdmin/tenants/provision", SaaSAdminPermissionTenantsManage},
		{http.MethodPost, "/dashboard/saasAdmin/tenants/{tenantId}/activation/resend", SaaSAdminPermissionTenantsManage},
		{http.MethodPost, "/dashboard/saasAdmin/tenants/41/super-admin/replace", SaaSAdminPermissionTenantsManage},
		{http.MethodPost, "/dashboard/saasAdmin/tenants/{tenantId}/super-admin/status", SaaSAdminPermissionTenantsManage},
		{http.MethodGet, "/dashboard/saasAdmin/tenants/41/dashboard-admins", SaaSAdminPermissionTenantsManage},
		{http.MethodGet, "/dashboard/saasAdmin/operationQueue", SaaSAdminPermissionOperationsRead},
		{http.MethodPost, "/dashboard/saasAdmin/operationQueueAssign", SaaSAdminPermissionOperationsManage},
		{http.MethodGet, "/dashboard/saasAdmin/notificationSlo", SaaSAdminPermissionNotificationsRead},
		{http.MethodPost, "/dashboard/saasAdmin/notificationRetry", SaaSAdminPermissionNotificationsManage},
		{http.MethodPost, "/dashboard/saasAdmin/notificationCredentialRotation", SaaSAdminPermissionNotificationsManage},
		{http.MethodGet, "/dashboard/saasAdmin/paymentSettlementSyncRuns", SaaSAdminPermissionFinanceRead},
		{http.MethodPost, "/dashboard/saasAdmin/paymentSettlementSync", SaaSAdminPermissionFinanceManage},
		{http.MethodGet, "/dashboard/saasAdmin/operations", SaaSAdminPermissionAuditRead},
		{http.MethodGet, "/dashboard/saasAdmin/auditIntegrity", SaaSAdminPermissionAuditRead},
		{http.MethodPost, "/dashboard/saasAdmin/auditIntegrityVerify", SaaSAdminPermissionAuditManage},
		{http.MethodGet, "/dashboard/saasAdmin/auditAnchors", SaaSAdminPermissionAuditRead},
		{http.MethodPost, "/dashboard/saasAdmin/auditAnchor", SaaSAdminPermissionAuditManage},
		{http.MethodGet, "/dashboard/saasAdmin/accessRoles", SaaSAdminPermissionAccessManage},
		{http.MethodGet, "/dashboard/saasAdmin/approvalPolicies", SaaSAdminPermissionApprovalsRead},
		{http.MethodPut, "/dashboard/saasAdmin/approvalPolicy", SaaSAdminPermissionApprovalsManage},
		{http.MethodGet, "/dashboard/saasAdmin/approvalDecisions", SaaSAdminPermissionApprovalsRead},
		{http.MethodGet, "/dashboard/saasAdmin/approvalDelegations", SaaSAdminPermissionApprovalsRead},
		{http.MethodPut, "/dashboard/saasAdmin/approvalDelegation", SaaSAdminPermissionApprovalsManage},
		{http.MethodPost, "/dashboard/saasAdmin/approvalReminders", SaaSAdminPermissionApprovalsManage},
		{http.MethodPost, "/dashboard/saasAdmin/approvalRequest", SaaSAdminPermissionApprovalsRead},
		{http.MethodPost, "/dashboard/saasAdmin/approvalDecision", SaaSAdminPermissionApprovalsReview},
		{http.MethodPost, "/dashboard/saasAdmin/approvalExecute", SaaSAdminPermissionApprovalsExecute},
		{http.MethodGet, "/dashboard/saasAdmin/serviceAccounts", SaaSAdminPermissionIntegrationsRead},
		{http.MethodGet, "/dashboard/saasAdmin/serviceAccountUsage", SaaSAdminPermissionIntegrationsRead},
		{http.MethodGet, "/dashboard/saasAdmin/tenantAIProvider", SaaSAdminPermissionIntegrationsRead},
		{http.MethodPut, "/dashboard/saasAdmin/tenantAIProvider", SaaSAdminPermissionIntegrationsManage},
		{http.MethodPost, "/dashboard/saasAdmin/serviceAccountUsageAlertEvaluate", SaaSAdminPermissionIntegrationsManage},
		{http.MethodPost, "/dashboard/saasAdmin/serviceAccount", SaaSAdminPermissionIntegrationsManage},
		{http.MethodPut, "/dashboard/saasAdmin/serviceAccountKeyRotate", SaaSAdminPermissionIntegrationsManage},
		{http.MethodGet, "/dashboard/saasAdmin/wecomCredentialProtection", SaaSAdminPermissionIntegrationsRead},
		{http.MethodPost, "/dashboard/saasAdmin/wecomCredentialRotation", SaaSAdminPermissionIntegrationsManage},
		{http.MethodGet, "/dashboard/saasAdmin/backupOverview", SaaSAdminPermissionBackupsRead},
		{http.MethodPut, "/dashboard/saasAdmin/backupPolicy", SaaSAdminPermissionBackupsManage},
		{http.MethodPost, "/dashboard/saasAdmin/backupRun", SaaSAdminPermissionBackupsManage},
		{http.MethodPost, "/dashboard/saasAdmin/restoreDrill", SaaSAdminPermissionBackupsManage},
		{http.MethodGet, "/dashboard/saasAdmin/brandingProfiles", SaaSAdminPermissionBrandingRead},
		{http.MethodGet, "/dashboard/saasAdmin/brandingProfile", SaaSAdminPermissionBrandingRead},
		{http.MethodPut, "/dashboard/saasAdmin/brandingProfile", SaaSAdminPermissionBrandingManage},
		{http.MethodGet, "/dashboard/saasAdmin/tenantDomains", SaaSAdminPermissionDomainsRead},
		{http.MethodGet, "/dashboard/saasAdmin/tenantDomainDeliveryJobs", SaaSAdminPermissionDomainsRead},
		{http.MethodPut, "/dashboard/saasAdmin/tenantDomain", SaaSAdminPermissionDomainsManage},
		{http.MethodPost, "/dashboard/saasAdmin/tenantDomainDelivery", SaaSAdminPermissionDomainsManage},
		{http.MethodGet, "/dashboard/saasAdmin/export?type=paymentRefunds", SaaSAdminPermissionFinanceRead},
		{http.MethodGet, "/dashboard/saasAdmin/export?type=operations", SaaSAdminPermissionAuditRead},
		{http.MethodGet, "/dashboard/saasAdmin/unknown", ""},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.path, nil)
			if got := SaaSAdminRequiredPermission(req); got != test.want {
				t.Fatalf("permission = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSaaSAdminAccessWildcardPermissionAuthorizesWithoutLegacySuperAdminFlag(t *testing.T) {
	profile := SaaSAdminAccessProfile{Permissions: []string{"*"}}
	if !SaaSAdminAccessHasPermission(profile, SaaSAdminPermissionTenantsManage) {
		t.Fatal("SaaS RBAC wildcard permission was not recognized without legacy IsPlatformSuperAdmin")
	}
}

func TestEverySaaSAdminRuntimeRouteHasPermissionClassification(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	serverSource, err := os.ReadFile(filepath.Join(filepath.Dir(currentFile), "..", "server", "server.go"))
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`"(GET|POST|PUT|DELETE|PATCH|HEAD) (/dashboard/saasAdmin/[^" ]+)"`)
	matches := pattern.FindAllStringSubmatch(string(serverSource), -1)
	if len(matches) == 0 {
		t.Fatal("no SaaS admin runtime routes found")
	}
	checked := map[string]struct{}{}
	for _, match := range matches {
		method, path := match[1], match[2]
		if path == "/dashboard/saasAdmin/page" || path == "/dashboard/saasAdmin/export" {
			continue
		}
		key := method + " " + path
		if _, exists := checked[key]; exists {
			continue
		}
		checked[key] = struct{}{}
		req := httptest.NewRequest(method, path, nil)
		if permission := SaaSAdminRequiredPermission(req); permission == "" {
			t.Errorf("route %s has no platform permission classification", key)
		}
	}
}

func TestValidateSaaSAdminAccessRoleUpsertRequiresReadDependencies(t *testing.T) {
	input := SaaSAdminAccessRoleUpsert{
		Code: "finance_writer", Name: "财务写入", Status: 1,
		Permissions: []string{SaaSAdminPermissionFinanceManage},
	}
	if err := validateSaaSAdminAccessRoleUpsert(&input); err == nil || !strings.Contains(err.Error(), SaaSAdminPermissionFinanceRead) {
		t.Fatalf("err = %v", err)
	}
	input.Permissions = append(input.Permissions, SaaSAdminPermissionFinanceRead)
	if err := validateSaaSAdminAccessRoleUpsert(&input); err != nil {
		t.Fatalf("valid role: %v", err)
	}
	input.Permissions = []string{SaaSAdminPermissionAuditManage}
	if err := validateSaaSAdminAccessRoleUpsert(&input); err == nil || !strings.Contains(err.Error(), SaaSAdminPermissionAuditRead) {
		t.Fatalf("audit dependency err = %v", err)
	}
	input.Permissions = []string{SaaSAdminPermissionApprovalsReview}
	if err := validateSaaSAdminAccessRoleUpsert(&input); err == nil || !strings.Contains(err.Error(), SaaSAdminPermissionApprovalsRead) {
		t.Fatalf("approval dependency err = %v", err)
	}
	input.Permissions = []string{SaaSAdminPermissionBackupsManage, SaaSAdminPermissionAuditRead}
	if err := validateSaaSAdminAccessRoleUpsert(&input); err == nil || !strings.Contains(err.Error(), SaaSAdminPermissionBackupsRead) {
		t.Fatalf("err = %v", err)
	}
	input.Permissions = []string{SaaSAdminPermissionIntegrationsManage, SaaSAdminPermissionIntegrationsRead}
	if err := validateSaaSAdminAccessRoleUpsert(&input); err == nil || !strings.Contains(err.Error(), SaaSAdminPermissionAuditRead) {
		t.Fatalf("integration dependency err = %v", err)
	}
	input.Permissions = []string{SaaSAdminPermissionComplianceManage, SaaSAdminPermissionComplianceRead, SaaSAdminPermissionAuditRead}
	if err := validateSaaSAdminAccessRoleUpsert(&input); err == nil || !strings.Contains(err.Error(), SaaSAdminPermissionApprovalsRead) {
		t.Fatalf("compliance dependency err = %v", err)
	}
	input.Permissions = []string{SaaSAdminPermissionBrandingManage, SaaSAdminPermissionBrandingRead}
	if err := validateSaaSAdminAccessRoleUpsert(&input); err == nil || !strings.Contains(err.Error(), SaaSAdminPermissionAuditRead) {
		t.Fatalf("branding dependency err = %v", err)
	}
}

func TestSaaSAdminPlatformOperatorUsesRoutePermission(t *testing.T) {
	store := &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{8: {ID: 8, Name: "财务", TenantID: 1, IsSuperAdmin: 0}}},
		profile:            SaaSAdminAccessProfile{Permissions: []string{SaaSAdminPermissionFinanceRead}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	allowed := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/paymentOrders", nil)
	allowed.Header.Set("X-Mochat-Go-User-ID", "8")
	allowedRec := httptest.NewRecorder()
	if _, ok := handler.resolvePlatformSuperAdmin(allowedRec, allowed); !ok || allowedRec.Code != http.StatusOK {
		t.Fatalf("allowed status=%d body=%s", allowedRec.Code, allowedRec.Body.String())
	}

	denied := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/paymentOrder", nil)
	denied.Header.Set("X-Mochat-Go-User-ID", "8")
	deniedRec := httptest.NewRecorder()
	if _, ok := handler.resolvePlatformSuperAdmin(deniedRec, denied); ok || deniedRec.Code != http.StatusForbidden {
		t.Fatalf("denied status=%d body=%s", deniedRec.Code, deniedRec.Body.String())
	}

	unknown := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/unknown", nil)
	unknown.Header.Set("X-Mochat-Go-User-ID", "8")
	unknownRec := httptest.NewRecorder()
	if _, ok := handler.resolvePlatformSuperAdmin(unknownRec, unknown); ok || unknownRec.Code != http.StatusForbidden {
		t.Fatalf("unknown status=%d body=%s", unknownRec.Code, unknownRec.Body.String())
	}
}

func TestSaaSAdminOverviewOperatorWithoutTenantReadGetsAggregateOnly(t *testing.T) {
	store := &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{
			users: map[int]User{8: {ID: 8, Name: "经营", TenantID: 1, IsSuperAdmin: 0}},
			overview: SaaSAdminOverview{
				Summary: SaaSAdminSummary{TenantCount: 2},
				Tenants: []SaaSAdminTenantOverview{{TenantID: 10, TenantName: "不可见租户"}},
				Metrics: []SaaSAdminMetricOverview{{Metric: SaaSMetricContacts}},
			},
		},
		profile: SaaSAdminAccessProfile{Permissions: []string{SaaSAdminPermissionOverviewRead}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/overview", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "8")
	rec := httptest.NewRecorder()
	handler.Overview(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastOptions.Scope != SaaSAdminScopePlatform || store.lastOptions.TenantID != 0 {
		t.Fatalf("options=%+v", store.lastOptions)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if len(data["tenants"].([]any)) != 0 || len(data["metrics"].([]any)) != 0 {
		t.Fatalf("tenant details leaked: %+v", data)
	}

	filtered := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/overview?tenantId=10", nil)
	filtered.Header.Set("X-Mochat-Go-User-ID", "8")
	filteredRec := httptest.NewRecorder()
	handler.Overview(filteredRec, filtered)
	if filteredRec.Code != http.StatusForbidden {
		t.Fatalf("filtered status=%d body=%s", filteredRec.Code, filteredRec.Body.String())
	}
}

func TestSaaSAdminAccessRoleAndAssignmentHandlers(t *testing.T) {
	store := &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, Name: "平台管理员", TenantID: 1, IsSuperAdmin: 1}}},
		roles:              []SaaSAdminAccessRole{{ID: 1, Code: "platform_finance", Name: "平台财务", Status: 1, IsSystem: true, Version: 1, Permissions: []string{SaaSAdminPermissionFinanceRead}}},
		assignments:        []SaaSAdminAccessAssignment{{UserID: 8, UserName: "财务", Version: 1}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	roleReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/accessRole", strings.NewReader(`{"code":"custom_auditor","name":"自定义审计","status":1,"permissions":["platform.overview.read","platform.audit.read"]}`))
	roleReq.Header.Set("X-Mochat-Go-User-ID", "1")
	roleRec := httptest.NewRecorder()
	handler.AccessRole(roleRec, roleReq)
	if roleRec.Code != http.StatusOK || store.roleUpdateCalls != 1 || store.roleInput.ActorUserID != 1 {
		t.Fatalf("role status=%d calls=%d input=%+v body=%s", roleRec.Code, store.roleUpdateCalls, store.roleInput, roleRec.Body.String())
	}

	assignmentReq := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/accessAssignment", strings.NewReader(`{"userId":8,"roleIds":[1,1],"expectedVersion":0}`))
	assignmentReq.Header.Set("X-Mochat-Go-User-ID", "1")
	assignmentRec := httptest.NewRecorder()
	handler.AccessAssignment(assignmentRec, assignmentReq)
	if assignmentRec.Code != http.StatusOK || store.assignUpdateCalls != 1 || len(store.assignmentInput.RoleIDs) != 1 {
		t.Fatalf("assignment status=%d calls=%d input=%+v body=%s", assignmentRec.Code, store.assignUpdateCalls, store.assignmentInput, assignmentRec.Body.String())
	}
}

func TestSaaSAdminAccessManagerCannotChangeSelf(t *testing.T) {
	store := &fakeSaaSAdminAccessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{8: {ID: 8, Name: "安全", TenantID: 1, IsSuperAdmin: 0}}},
		profile:            SaaSAdminAccessProfile{Permissions: []string{SaaSAdminPermissionOverviewRead, SaaSAdminPermissionAuditRead, SaaSAdminPermissionAccessManage}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/accessAssignment", strings.NewReader(`{"userId":8,"roleIds":[],"expectedVersion":1}`))
	req.Header.Set("X-Mochat-Go-User-ID", "8")
	rec := httptest.NewRecorder()
	handler.AccessAssignment(rec, req)
	if rec.Code != http.StatusBadRequest || store.assignUpdateCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.assignUpdateCalls, rec.Body.String())
	}
}
