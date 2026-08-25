package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

type fakeDashboardAccessGuardStore struct {
	identity       DashboardAccessIdentity
	identityFound  bool
	tenantAccess   DashboardTenantAccess
	resources      []DashboardPermissionResource
	catalog        []DashboardPermissionDefinition
	grants         []DashboardPermissionGrantFact
	employee       DashboardEmployeeScope
	employeeFound  bool
	allowedCorpIDs []int
	resourceCalls  int
	grantCalls     int
	tenantCalls    int
}

func (store *fakeDashboardAccessGuardStore) DashboardAccessIdentity(context.Context, int) (DashboardAccessIdentity, bool, error) {
	return store.identity, store.identityFound, nil
}

func (store *fakeDashboardAccessGuardStore) DashboardTenantAccess(_ context.Context, tenantID int, _ time.Time) (DashboardTenantAccess, error) {
	store.tenantCalls++
	access := store.tenantAccess
	if access.TenantID == 0 {
		access.TenantID = tenantID
	}
	return access, nil
}

func TestDashboardAccessGuardProfileRequiresAuthenticationAndTenantGate(t *testing.T) {
	guard, store := newDashboardAccessGuardFixture(false)
	request := dashboardAccessGuardRequest(guard, http.MethodGet, "/dashboard/access/profile", nil)
	response := httptest.NewRecorder()
	if !guard.Authorize(response, request) {
		t.Fatalf("profile rejected: status=%d body=%s", response.Code, response.Body.String())
	}
	access, ok := DashboardAccessFromContext(request.Context())
	if !ok || access.UserID != 7 || access.TenantID != 9 || access.CorpID != 12 || access.IsSuperAdmin {
		t.Fatalf("access=%+v ok=%v", access, ok)
	}
	if store.tenantCalls != 0 || store.resourceCalls != 0 || store.grantCalls != 0 {
		t.Fatalf("tenant=%d resource=%d grants=%d", store.tenantCalls, store.resourceCalls, store.grantCalls)
	}

	guard, store = newDashboardAccessGuardFixture(false)
	response = httptest.NewRecorder()
	request = dashboardAccessGuardRequest(guard, http.MethodGet, "/dashboard/access/profile", nil)
	principal, _ := dashboardprincipal.DashboardPrincipalFromContext(request.Context())
	principal.CorpStatus = dashboardprincipal.CorpBindingStatusSuspended
	request = request.WithContext(dashboardprincipal.WithPrincipal(request.Context(), principal))
	if guard.Authorize(response, request) {
		t.Fatal("profile accepted for suspended binding")
	}
	if response.Code != http.StatusForbidden || machineCode(t, response) != DashboardTenantAccessDeniedCode {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestDashboardAccessGuardManualAIInsightRunIsSuperadminDenyOnly(t *testing.T) {
	for _, test := range []struct {
		name       string
		superadmin bool
		want       bool
	}{
		{name: "ordinary denied", superadmin: false, want: false},
		{name: "superadmin allowed", superadmin: true, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			guard, store := newDashboardAccessGuardFixture(test.superadmin)
			request := dashboardAccessGuardRequest(guard, http.MethodPost, "/dashboard/ai-insight/run", nil)
			response := httptest.NewRecorder()
			if got := guard.Authorize(response, request); got != test.want {
				t.Fatalf("Authorize=%v status=%d body=%s", got, response.Code, response.Body.String())
			}
			if !test.want {
				if response.Code != http.StatusForbidden || machineCode(t, response) != DashboardPermissionDeniedCode {
					t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
				}
				return
			}
			access, ok := DashboardAccessFromContext(request.Context())
			if !ok || !access.IsSuperAdmin || access.TenantID != 9 || access.CorpID != 12 {
				t.Fatalf("access=%+v ok=%v", access, ok)
			}
			if store.resourceCalls != 0 || store.grantCalls != 0 {
				t.Fatalf("deny-only route queried resources=%d grants=%d", store.resourceCalls, store.grantCalls)
			}
		})
	}
}

func TestDashboardAccessGuardPendingBindingAllowsOnlyConfigurationContracts(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		superadmin  bool
		wantAllowed bool
		wantCode    string
	}{
		{name: "business page", method: http.MethodGet, path: "/dashboard/workContact/123", wantCode: "CORP_CONFIGURATION_REQUIRED"},
		{name: "profile", method: http.MethodGet, path: "/dashboard/access/profile", superadmin: true, wantAllowed: true},
		{name: "company profile for superadmin", method: http.MethodGet, path: "/dashboard/company/profile", superadmin: true, wantAllowed: true},
		{name: "company profile for ordinary user", method: http.MethodGet, path: "/dashboard/company/profile", wantCode: DashboardPermissionDeniedCode},
		{name: "application credentials for superadmin", method: http.MethodPut, path: "/dashboard/company/application-credentials", superadmin: true, wantAllowed: true},
		{name: "callback configuration for superadmin", method: http.MethodGet, path: "/dashboard/company/callback-configuration", superadmin: true, wantAllowed: true},
		{name: "provider status for pending superadmin", method: http.MethodGet, path: "/dashboard/providers/status", superadmin: true, wantAllowed: true},
		{name: "provider status for pending ordinary user", method: http.MethodGet, path: "/dashboard/providers/status", wantCode: DashboardPermissionDeniedCode},
		{name: "callback rotation for superadmin", method: http.MethodPost, path: "/dashboard/company/callback-configuration/regenerate", superadmin: true, wantAllowed: true},
		{name: "employee sync remains blocked", method: http.MethodPost, path: "/dashboard/company/employee-sync", superadmin: true, wantCode: "CORP_CONFIGURATION_REQUIRED"},
		{name: "sync status remains blocked", method: http.MethodGet, path: "/dashboard/company/sync-status", superadmin: true, wantCode: "CORP_CONFIGURATION_REQUIRED"},
		{name: "unknown api remains blocked", method: http.MethodGet, path: "/dashboard/not-classified", superadmin: true, wantCode: "CORP_CONFIGURATION_REQUIRED"},
		{name: "session", method: http.MethodGet, path: "/dashboard/auth/session", superadmin: true, wantAllowed: true},
		{name: "security MFA is not a configuration contract", method: http.MethodGet, path: "/dashboard/user/securityMFA", superadmin: true, wantCode: "CORP_CONFIGURATION_REQUIRED"},
		{name: "logout", method: http.MethodPost, path: "/dashboard/auth/logout", superadmin: true, wantAllowed: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			guard, _ := newDashboardAccessGuardFixture(test.superadmin)
			request := dashboardAccessGuardRequest(guard, test.method, test.path, nil)
			principal, err := dashboardprincipal.DashboardPrincipalFromContext(request.Context())
			if err != nil {
				t.Fatal(err)
			}
			principal.CorpStatus = dashboardprincipal.CorpBindingStatusPending
			request = request.WithContext(dashboardprincipal.WithPrincipal(request.Context(), principal))
			response := httptest.NewRecorder()
			got := guard.Authorize(response, request)
			if got != test.wantAllowed {
				t.Fatalf("Authorize=%v status=%d body=%s", got, response.Code, response.Body.String())
			}
			if !test.wantAllowed && (response.Code != http.StatusForbidden || machineCode(t, response) != test.wantCode) {
				t.Fatalf("status=%d body=%s wantCode=%s", response.Code, response.Body.String(), test.wantCode)
			}
		})
	}
}

func TestDashboardAccessGuardManagementRoutesAreSuperadminOnly(t *testing.T) {
	for _, contract := range []struct{ method, path string }{
		{http.MethodGet, "/dashboard/access/catalog"},
		{http.MethodGet, "/dashboard/access/users/7"},
		{http.MethodPut, "/dashboard/access/users/7"},
		{http.MethodPost, "/dashboard/access/roles"},
		{http.MethodPut, "/dashboard/access/roles/8/status"},
		{http.MethodDelete, "/dashboard/access/roles/8"},
		{http.MethodGet, "/dashboard/access/audits"},
	} {
		t.Run(contract.method+" "+contract.path, func(t *testing.T) {
			ordinary, _ := newDashboardAccessGuardFixture(false)
			ordinaryResponse := httptest.NewRecorder()
			if ordinary.Authorize(ordinaryResponse, dashboardAccessGuardRequest(ordinary, contract.method, contract.path, nil)) {
				t.Fatal("ordinary user accepted")
			}
			if ordinaryResponse.Code != http.StatusForbidden || machineCode(t, ordinaryResponse) != DashboardPermissionDeniedCode {
				t.Fatalf("ordinary status=%d body=%s", ordinaryResponse.Code, ordinaryResponse.Body.String())
			}

			superadmin, _ := newDashboardAccessGuardFixture(true)
			superadminRequest := dashboardAccessGuardRequest(superadmin, contract.method, contract.path, nil)
			if !superadmin.Authorize(httptest.NewRecorder(), superadminRequest) {
				t.Fatal("superadmin rejected")
			}
			access, ok := DashboardAccessFromContext(superadminRequest.Context())
			if !ok || !access.IsSuperAdmin || access.TenantID != 9 {
				t.Fatalf("access=%+v ok=%v", access, ok)
			}
		})
	}
}

func (store *fakeDashboardAccessGuardStore) DashboardPermissionResources(context.Context, string) ([]DashboardPermissionResource, error) {
	store.resourceCalls++
	return append([]DashboardPermissionResource(nil), store.resources...), nil
}

func (store *fakeDashboardAccessGuardStore) DashboardPermissionCatalog(context.Context) ([]DashboardPermissionDefinition, error) {
	return append([]DashboardPermissionDefinition(nil), store.catalog...), nil
}

func (store *fakeDashboardAccessGuardStore) DashboardPermissionGrants(context.Context, int, int) ([]DashboardPermissionGrantFact, error) {
	store.grantCalls++
	return append([]DashboardPermissionGrantFact(nil), store.grants...), nil
}

func (store *fakeDashboardAccessGuardStore) DashboardEmployeeScope(context.Context, int, int, int) (DashboardEmployeeScope, bool, error) {
	return store.employee, store.employeeFound, nil
}

func (store *fakeDashboardAccessGuardStore) EmployeeIDByUserCorp(_ context.Context, userID, corpID int) (int, error) {
	if userID == store.identity.UserID && containsInt(store.allowedCorpIDs, corpID) {
		return store.employee.EmployeeID, nil
	}
	return 0, nil
}

func (store *fakeDashboardAccessGuardStore) FirstEmployeeByUser(context.Context, int) (int, int, bool, error) {
	if len(store.allowedCorpIDs) == 0 {
		return 0, 0, false, nil
	}
	return store.allowedCorpIDs[0], store.employee.EmployeeID, true, nil
}

func (store *fakeDashboardAccessGuardStore) CorpIDsByTenant(context.Context, int) ([]int, error) {
	return append([]int(nil), store.allowedCorpIDs...), nil
}

func (store *fakeDashboardAccessGuardStore) CorpIDsByUser(context.Context, int) ([]int, error) {
	return append([]int(nil), store.allowedCorpIDs...), nil
}

func dashboardAccessGuardRequest(guard *DashboardAccessGuard, method, target string, body io.Reader) *http.Request {
	request := httptest.NewRequest(method, target, body)
	tenantID, corpID := 9, 12
	isSuperAdmin := false
	if store, ok := guard.store.(*fakeDashboardAccessGuardStore); ok {
		tenantID = store.identity.TenantID
		isSuperAdmin = store.identity.IsSuperAdmin
		if len(store.allowedCorpIDs) == 1 {
			corpID = store.allowedCorpIDs[0]
		}
	}
	principal := dashboardprincipal.DashboardPrincipal{
		UserID: 7, TenantID: tenantID, CorpID: corpID,
		CorpStatus:   dashboardprincipal.CorpBindingStatusActive,
		IsSuperAdmin: isSuperAdmin, AuthVersion: 1,
	}
	return request.WithContext(dashboardprincipal.WithPrincipal(request.Context(), principal))
}

func newDashboardAccessGuardFixture(superadmin bool) (*DashboardAccessGuard, *fakeDashboardAccessGuardStore) {
	store := &fakeDashboardAccessGuardStore{
		identity:      DashboardAccessIdentity{UserID: 7, TenantID: 9, UserName: "测试用户", Status: 1, IsSuperAdmin: superadmin},
		identityFound: true,
		tenantAccess:  DashboardTenantAccess{TenantID: 9, Allowed: true},
		resources: []DashboardPermissionResource{{
			PermissionCode: "dashboard.contacts", Method: http.MethodGet,
			PathPattern: "/dashboard/workContact/{id}", ScopeRequired: true,
		}},
		catalog: []DashboardPermissionDefinition{{
			ID: 31, Code: "dashboard.contacts", Path: "/contact", Name: "客户", ScopeRequired: true,
		}},
		grants: []DashboardPermissionGrantFact{{
			TenantID: 9, PermissionID: 31, SourceType: PermissionSourceDirect,
			SourceID: 7, SourceName: "直接授权", SourceStatus: 1, Scope: string(DataScopeDepartment),
		}},
		employee: DashboardEmployeeScope{
			EmployeeID: 81, DepartmentIDs: []int{4}, DepartmentEmployeeIDs: []int{81, 82},
		},
		employeeFound:  true,
		allowedCorpIDs: []int{12},
	}
	guard := NewDashboardAccessGuard(store, NewDashboardAccessService(store))
	return guard, store
}

func TestDashboardAccessGuardRejectsMissingAuthentication(t *testing.T) {
	guard, store := newDashboardAccessGuardFixture(false)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/dashboard/workContact/123", nil)

	if guard.Authorize(recorder, request) {
		t.Fatal("Authorize returned true without authentication")
	}
	if recorder.Code != http.StatusUnauthorized || store.resourceCalls != 0 {
		t.Fatalf("status=%d resourceCalls=%d", recorder.Code, store.resourceCalls)
	}
}

func TestDashboardAccessGuardChecksTenantBeforeResourceAndPermission(t *testing.T) {
	guard, store := newDashboardAccessGuardFixture(false)
	store.tenantAccess = DashboardTenantAccess{TenantID: 9, Reason: DashboardTenantAccessReasonSubscriptionDenied}
	recorder := httptest.NewRecorder()
	request := dashboardAccessGuardRequest(guard, http.MethodGet, "/dashboard/workContact/123", nil)

	if !guard.Authorize(recorder, request) {
		t.Fatalf("principal-authenticated request rejected: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if store.tenantCalls != 0 {
		t.Fatalf("tenant gate was re-read after principal resolution: calls=%d", store.tenantCalls)
	}
}

func TestDashboardAccessGuardAllowsMappedPermissionAndWritesScopeContext(t *testing.T) {
	guard, _ := newDashboardAccessGuardFixture(false)
	recorder := httptest.NewRecorder()
	request := dashboardAccessGuardRequest(guard, http.MethodGet, "/dashboard/workContact/123", nil)

	if !guard.Authorize(recorder, request) {
		t.Fatalf("Authorize returned false: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	access, ok := DashboardAccessFromContext(request.Context())
	if !ok {
		t.Fatal("DashboardAccessContext missing")
	}
	if access.TenantID != 9 || access.CorpID != 12 || access.PermissionCode != "dashboard.contacts" || access.Scope != DataScopeDepartment || !access.ScopeRequired {
		t.Fatalf("access=%+v", access)
	}
	if len(access.AllowedEmployeeIDs) != 2 || access.AllowedEmployeeIDs[0] != 81 || access.AllowedEmployeeIDs[1] != 82 {
		t.Fatalf("AllowedEmployeeIDs=%v", access.AllowedEmployeeIDs)
	}
}

func TestDashboardAccessGuardDeniesMissingPermissionAndUnclassifiedAPI(t *testing.T) {
	for _, test := range []struct {
		name       string
		path       string
		superadmin bool
		mutate     func(*fakeDashboardAccessGuardStore)
	}{
		{name: "mapped without permission", path: "/dashboard/workContact/123", mutate: func(store *fakeDashboardAccessGuardStore) { store.grants = nil }},
		{name: "ordinary unknown", path: "/dashboard/newUnknown"},
		{name: "superadmin unknown", path: "/dashboard/newUnknown", superadmin: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			guard, store := newDashboardAccessGuardFixture(test.superadmin)
			if test.path == "/dashboard/newUnknown" {
				store.resources = nil
			}
			if test.mutate != nil {
				test.mutate(store)
			}
			recorder := httptest.NewRecorder()
			request := dashboardAccessGuardRequest(guard, http.MethodGet, test.path, nil)
			if guard.Authorize(recorder, request) {
				t.Fatal("Authorize returned true")
			}
			if recorder.Code != http.StatusForbidden || machineCode(t, recorder) != DashboardPermissionDeniedCode {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestDashboardAccessGuardSuperadminAndDenyOnlyPolicy(t *testing.T) {
	for _, test := range []struct {
		name       string
		path       string
		superadmin bool
		want       bool
	}{
		{name: "superadmin mapped", path: "/dashboard/workContact/123", superadmin: true, want: true},
		{name: "ordinary deny-only", path: "/dashboard/acceptance/phase35", want: false},
		{name: "superadmin deny-only", path: "/dashboard/acceptance/phase35", superadmin: true, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			guard, _ := newDashboardAccessGuardFixture(test.superadmin)
			recorder := httptest.NewRecorder()
			request := dashboardAccessGuardRequest(guard, http.MethodGet, test.path, nil)
			got := guard.Authorize(recorder, request)
			if got != test.want {
				t.Fatalf("Authorize=%v status=%d body=%s", got, recorder.Code, recorder.Body.String())
			}
			if !test.want && machineCode(t, recorder) != DashboardPermissionDeniedCode {
				t.Fatalf("body=%s", recorder.Body.String())
			}
		})
	}
}

func TestDashboardAccessGuardAllowsCompanyProfileForGrantedOrdinaryUser(t *testing.T) {
	guard, store := newDashboardAccessGuardFixture(false)
	store.resources = []DashboardPermissionResource{
		{PermissionCode: "dashboard.company_setting.website", Method: http.MethodGet, PathPattern: "/dashboard/company/profile", ScopeRequired: false},
		{PermissionCode: "dashboard.company_setting.website", Method: http.MethodGet, PathPattern: "/dashboard/providers/status", ScopeRequired: false},
	}
	store.catalog = []DashboardPermissionDefinition{{
		ID: 49, Code: "dashboard.company_setting.website", Path: "/company-setting/website", Name: "唯一企业资料",
	}}
	store.grants = []DashboardPermissionGrantFact{{
		TenantID: 9, PermissionID: 49, SourceType: PermissionSourceDirect, SourceID: 7, Scope: string(DataScopeTenant),
	}}
	for _, path := range []string{"/dashboard/company/profile", "/dashboard/providers/status"} {
		recorder := httptest.NewRecorder()
		request := dashboardAccessGuardRequest(guard, http.MethodGet, path, nil)
		if !guard.Authorize(recorder, request) {
			t.Fatalf("granted company profile request %s rejected: status=%d body=%s", path, recorder.Code, recorder.Body.String())
		}
		principal, err := dashboardprincipal.DashboardPrincipalFromContext(request.Context())
		if err != nil || !dashboardprincipal.HasPermissionCode(request.Context(), principal, "dashboard.company_setting.website") {
			t.Fatalf("granted company profile request %s lost service-layer permission context: principal=%+v err=%v", path, principal, err)
		}
	}
}

func TestDashboardAccessGuardDeniesCompanyProfileWithoutGrant(t *testing.T) {
	guard, store := newDashboardAccessGuardFixture(false)
	store.resources = []DashboardPermissionResource{
		{PermissionCode: "dashboard.company_setting.website", Method: http.MethodGet, PathPattern: "/dashboard/company/profile", ScopeRequired: false},
		{PermissionCode: "dashboard.company_setting.website", Method: http.MethodGet, PathPattern: "/dashboard/providers/status", ScopeRequired: false},
	}
	store.catalog = []DashboardPermissionDefinition{{
		ID: 49, Code: "dashboard.company_setting.website", Path: "/company-setting/website", Name: "唯一企业资料",
	}}
	store.grants = nil
	for _, path := range []string{"/dashboard/company/profile", "/dashboard/providers/status"} {
		recorder := httptest.NewRecorder()
		request := dashboardAccessGuardRequest(guard, http.MethodGet, path, nil)
		if guard.Authorize(recorder, request) {
			t.Fatalf("ungranted company profile request %s was allowed", path)
		}
		if recorder.Code != http.StatusForbidden || machineCode(t, recorder) != DashboardPermissionDeniedCode {
			t.Fatalf("path=%s status=%d body=%s", path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestDashboardAccessGuardExemptionsAreExactAndProtectedOnSessionRoutes(t *testing.T) {
	guard, store := newDashboardAccessGuardFixture(false)
	for _, test := range []struct {
		method string
		path   string
		want   bool
	}{
		{method: http.MethodPost, path: "/dashboard/user/auth", want: true},
		{method: http.MethodPost, path: "/dashboard/officialAccount/authEventCallback", want: true},
		{method: http.MethodGet, path: "/dashboard/user/securityMFA", want: true},
		{method: http.MethodGet, path: "/dashboard/user/securityMFAExtra", want: false},
		{method: http.MethodPost, path: "/dashboard/user/authExtra", want: false},
		{method: http.MethodGet, path: "/dashboard/officialAccount/authRedirect", want: false},
	} {
		recorder := httptest.NewRecorder()
		request := dashboardAccessGuardRequest(guard, test.method, test.path, nil)
		got := guard.Authorize(recorder, request)
		if got != test.want {
			t.Fatalf("%s %s Authorize=%v status=%d body=%s", test.method, test.path, got, recorder.Code, recorder.Body.String())
		}
	}
	if store.resourceCalls != 3 {
		t.Fatalf("resourceCalls=%d, only similar unclassified paths should reach mapping", store.resourceCalls)
	}

	guard, _ = newDashboardAccessGuardFixture(false)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/dashboard/user/securityMFA", nil)
	if guard.Authorize(recorder, request) || recorder.Code != http.StatusUnauthorized {
		t.Fatalf("protected exemption bypassed identity principal: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestDashboardPublicExemptionsAreAnExactPolicySubset(t *testing.T) {
	exact := make(map[string]bool)
	for _, contract := range ExactExemptDashboardRouteContracts() {
		exact[contract] = true
	}
	for _, contract := range PublicDashboardRouteContracts() {
		if !exact[contract] {
			t.Fatalf("public exemption %q is not in exact exemption policy", contract)
		}
	}
}

func TestDashboardIdentityAuthenticatedRoutesAreExactButNotPublic(t *testing.T) {
	exact := make(map[string]bool)
	for _, contract := range ExactExemptDashboardRouteContracts() {
		exact[contract] = true
	}
	public := make(map[string]bool)
	for _, contract := range PublicDashboardRouteContracts() {
		public[contract] = true
	}
	for _, contract := range []string{
		"POST /dashboard/auth/password/reset-request",
		"GET /dashboard/auth/session",
		"POST /dashboard/auth/logout",
		"PUT /dashboard/user/logout",
	} {
		if !exact[contract] {
			t.Fatalf("identity-authenticated route %q must remain a page-RBAC exact exemption", contract)
		}
		if public[contract] {
			t.Fatalf("identity-authenticated route %q must not be an identity-public exemption", contract)
		}
	}
	for _, contract := range []string{
		"POST /dashboard/user/auth",
		"POST /dashboard/user/authMFA",
		"POST /dashboard/auth/activate",
		"POST /dashboard/auth/password/reset",
	} {
		if !public[contract] || !exact[contract] {
			t.Fatalf("identity-public route %q must be both public and exact", contract)
		}
	}
}

func TestDashboardAccessGuardReplacesUntrustedCorpWithTenantValidatedCorp(t *testing.T) {
	guard, store := newDashboardAccessGuardFixture(false)
	store.allowedCorpIDs = []int{13}
	recorder := httptest.NewRecorder()
	request := dashboardAccessGuardRequest(guard, http.MethodGet, "/dashboard/workContact/123?corpId=12", nil)
	if !guard.Authorize(recorder, request) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	access, ok := DashboardAccessFromContext(request.Context())
	if !ok || access.CorpID != 13 {
		t.Fatalf("access=%+v ok=%v, request corp must not be trusted", access, ok)
	}
}

func TestDashboardAccessGuardRejectsMissingEmployeeForScopedResource(t *testing.T) {
	guard, store := newDashboardAccessGuardFixture(false)
	store.employee = DashboardEmployeeScope{}
	store.employeeFound = false
	recorder := httptest.NewRecorder()
	request := dashboardAccessGuardRequest(guard, http.MethodGet, "/dashboard/workContact/123", nil)
	if guard.Authorize(recorder, request) || machineCode(t, recorder) != DashboardPermissionDeniedCode {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestDashboardAccessContextFeedsLegacyAuthorizerForListDetailAndWrite(t *testing.T) {
	resolver := NewRBACResolver(panicRBACStore{})
	for _, request := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/dashboard/workContact/index"},
		{method: http.MethodGet, path: "/dashboard/workContact/show"},
		{method: http.MethodPut, path: "/dashboard/workContact/update"},
	} {
		t.Run(request.method+" "+request.path, func(t *testing.T) {
			ctx := WithDashboardAccessContext(context.Background(), DashboardAccessContext{
				UserID: 7, TenantID: 9, CorpID: 12, WorkEmployeeID: 81,
				PermissionCode: "dashboard.contacts", Scope: DataScopeDepartment,
				ScopeRequired: true, AllowedEmployeeIDs: []int{81, 82},
			})
			access, err := resolver.Resolve(ctx, 7, PermissionKey(request.path, request.method), 12, 81)
			if err != nil {
				t.Fatal(err)
			}
			if access.DataPermission != DataPermissionDepartment || len(access.DeptEmployeeIDs) != 2 {
				t.Fatalf("access=%+v", access)
			}
		})
	}
}

func TestDashboardAccessContextTranslatesTenantDepartmentAndSelfScopes(t *testing.T) {
	resolver := NewRBACResolver(panicRBACStore{})
	for _, test := range []struct {
		name       string
		scope      DataScope
		allowed    []int
		wantLegacy int
		wantIDs    []int
	}{
		{name: "tenant", scope: DataScopeTenant, wantLegacy: DataPermissionAll, wantIDs: []int{}},
		{name: "department", scope: DataScopeDepartment, allowed: []int{81, 82}, wantLegacy: DataPermissionDepartment, wantIDs: []int{81, 82}},
		{name: "self", scope: DataScopeSelf, allowed: []int{81}, wantLegacy: DataPermissionSelf, wantIDs: []int{81}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := WithDashboardAccessContext(context.Background(), DashboardAccessContext{
				UserID: 7, TenantID: 9, CorpID: 12, WorkEmployeeID: 81,
				PermissionCode: "dashboard.contacts", Scope: test.scope, ScopeRequired: true,
				AllowedEmployeeIDs: test.allowed,
			})
			access, err := resolver.Resolve(ctx, 7, "/dashboard/resource#get", 12, 81)
			if err != nil {
				t.Fatal(err)
			}
			if access.DataPermission != test.wantLegacy || !equalInts(access.DeptEmployeeIDs, test.wantIDs) {
				t.Fatalf("access=%+v", access)
			}
		})
	}
	ctx := WithDashboardAccessContext(context.Background(), DashboardAccessContext{
		UserID: 7, TenantID: 9, CorpID: 12, Scope: DataScopeSelf, ScopeRequired: true,
	})
	if _, err := resolver.Resolve(ctx, 7, "/dashboard/resource#get", 12, 0); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("missing employee error=%v", err)
	}
}

func TestDashboardAccessContextAllowsTenantScopeWithoutEmployeeBinding(t *testing.T) {
	resolver := NewRBACResolver(panicRBACStore{})
	ctx := WithDashboardAccessContext(context.Background(), DashboardAccessContext{
		UserID: 7, TenantID: 9, CorpID: 12,
		PermissionCode: "dashboard.chat.staff", Scope: DataScopeTenant,
		ScopeRequired: true, IsSuperAdmin: true,
	})

	access, err := resolver.Resolve(ctx, 7, "/dashboard/workMessage/fromUsers#get", 12, 0)
	if err != nil {
		t.Fatal(err)
	}
	if access.DataPermission != DataPermissionAll || access.WorkEmployeeID != 0 || len(access.DeptEmployeeIDs) != 0 {
		t.Fatalf("access=%+v", access)
	}
}

func TestDashboardGuardAllowsSuperadminTenantScopeWithoutEmployeeBinding(t *testing.T) {
	profile := DashboardAccessProfile{
		UserID: 7, TenantID: 9, CorpID: 12, IsSuperAdmin: true,
		EffectivePermissions: []EffectivePermission{{Code: "dashboard.chat.staff", Scope: DataScopeTenant}},
	}
	access, ok := dashboardContextForMatches(profile, []DashboardPermissionResource{{
		PermissionCode: "dashboard.chat.staff", Method: http.MethodGet,
		PathPattern: "/dashboard/workMessage/fromUsers", ScopeRequired: true,
	}})
	if !ok {
		t.Fatal("superadmin tenant scope was rejected without an employee binding")
	}
	if !access.IsSuperAdmin || access.Scope != DataScopeTenant || access.WorkEmployeeID != 0 || len(access.AllowedEmployeeIDs) != 0 {
		t.Fatalf("access=%+v", access)
	}

	profile.IsSuperAdmin = false
	if _, allowed := dashboardContextForMatches(profile, []DashboardPermissionResource{{
		PermissionCode: "dashboard.chat.staff", Method: http.MethodGet,
		PathPattern: "/dashboard/workMessage/fromUsers", ScopeRequired: true,
	}}); allowed {
		t.Fatal("ordinary tenant-scoped user bypassed the employee binding requirement")
	}
}

func equalInts(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func TestDashboardAccessGuardProvidesScopeContextForEveryCatalogScopedResource(t *testing.T) {
	raw, err := os.ReadFile("dashboard_page_catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Pages []struct {
			Code      string                        `json:"code"`
			Path      string                        `json:"path"`
			Name      string                        `json:"name"`
			Resources []DashboardPermissionResource `json:"resources"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	tested := 0
	for _, page := range catalog.Pages {
		for _, resource := range page.Resources {
			if !resource.ScopeRequired {
				continue
			}
			tested++
			resource.PermissionCode = page.Code
			store := &fakeDashboardAccessGuardStore{
				identity:       DashboardAccessIdentity{UserID: 7, TenantID: 9, UserName: "测试用户", Status: 1},
				identityFound:  true,
				tenantAccess:   DashboardTenantAccess{TenantID: 9, Allowed: true},
				resources:      []DashboardPermissionResource{resource},
				catalog:        []DashboardPermissionDefinition{{ID: 1, Code: page.Code, Path: page.Path, Name: page.Name, ScopeRequired: true}},
				grants:         []DashboardPermissionGrantFact{{TenantID: 9, PermissionID: 1, SourceType: PermissionSourceDirect, SourceID: 7, Scope: string(DataScopeSelf)}},
				employee:       DashboardEmployeeScope{EmployeeID: 81, DepartmentIDs: []int{4}, DepartmentEmployeeIDs: []int{81}},
				employeeFound:  true,
				allowedCorpIDs: []int{12},
			}
			guard := NewDashboardAccessGuard(store, NewDashboardAccessService(store))
			path := resource.PathPattern
			for strings.Contains(path, "{") {
				start := strings.Index(path, "{")
				end := strings.Index(path[start:], "}")
				if end < 0 {
					t.Fatalf("invalid path pattern %q", path)
				}
				path = path[:start] + "123" + path[start+end+1:]
			}
			recorder := httptest.NewRecorder()
			request := dashboardAccessGuardRequest(guard, resource.Method, path, nil)
			if !guard.Authorize(recorder, request) {
				t.Fatalf("%s %s (%s): status=%d body=%s", resource.Method, path, page.Code, recorder.Code, recorder.Body.String())
			}
			access, ok := DashboardAccessFromContext(request.Context())
			if !ok || !access.ScopeRequired || access.WorkEmployeeID != 81 || len(access.AllowedEmployeeIDs) != 1 || access.AllowedEmployeeIDs[0] != 81 {
				t.Fatalf("%s %s (%s): access=%+v ok=%v", resource.Method, path, page.Code, access, ok)
			}
		}
	}
	if tested == 0 {
		t.Fatal("catalog has no scopeRequired resources")
	}
}

func TestDashboardPageCatalogCustomerConversationResources(t *testing.T) {
	raw, err := os.ReadFile("dashboard_page_catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Pages []struct {
			Code      string                        `json:"code"`
			Resources []DashboardPermissionResource `json:"resources"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}

	var customerResources []DashboardPermissionResource
	for _, page := range catalog.Pages {
		if page.Code == "dashboard.chat.v2_customer" {
			customerResources = page.Resources
			break
		}
	}
	if customerResources == nil {
		t.Fatal("customer conversation page is missing")
	}
	for _, want := range []DashboardPermissionResource{
		{Method: http.MethodGet, PathPattern: "/dashboard/workMessage/customerDirectory", ScopeRequired: true},
		{Method: http.MethodGet, PathPattern: "/dashboard/workMessage/customerConversations", ScopeRequired: true},
		{Method: http.MethodGet, PathPattern: "/dashboard/workMessage/customerDetail", ScopeRequired: true},
		{Method: http.MethodPut, PathPattern: "/dashboard/workMessage/focus", ScopeRequired: true},
		{Method: http.MethodDelete, PathPattern: "/dashboard/workMessage/focus", ScopeRequired: true},
	} {
		found := false
		for _, resource := range customerResources {
			if resource.Method == want.Method && resource.PathPattern == want.PathPattern && resource.ScopeRequired == want.ScopeRequired {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("customer page missing resource %+v", want)
		}
	}
	for _, resource := range customerResources {
		if resource.PathPattern == "/dashboard/workContact/index" {
			t.Fatal("customer conversation page must not depend on /dashboard/workContact/index")
		}
	}
}

func TestDashboardPageCatalogCustomerInheritanceResources(t *testing.T) {
	raw, err := os.ReadFile("dashboard_page_catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Pages []struct {
			Code      string                        `json:"code"`
			Resources []DashboardPermissionResource `json:"resources"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	var resources []DashboardPermissionResource
	for _, page := range catalog.Pages {
		if page.Code == "dashboard.customer.inheritance" {
			resources = page.Resources
			break
		}
	}
	if resources == nil {
		t.Fatal("customer inheritance page is missing")
	}
	for _, want := range []DashboardPermissionResource{
		{Method: http.MethodGet, PathPattern: "/dashboard/contactTransfer/unassignedList"},
		{Method: http.MethodGet, PathPattern: "/dashboard/contactTransfer/info"},
		{Method: http.MethodGet, PathPattern: "/dashboard/contactTransfer/room"},
		{Method: http.MethodGet, PathPattern: "/dashboard/contactTransfer/log"},
		{Method: http.MethodGet, PathPattern: "/dashboard/workEmployee/index"},
		{Method: http.MethodPost, PathPattern: "/dashboard/contactTransfer/sync"},
		{Method: http.MethodPost, PathPattern: "/dashboard/contactTransfer/index"},
		{Method: http.MethodPost, PathPattern: "/dashboard/contactTransfer/room"},
	} {
		found := false
		for _, resource := range resources {
			if resource.Method == want.Method && resource.PathPattern == want.PathPattern && !resource.ScopeRequired {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("customer inheritance page missing resource %+v", want)
		}
	}
}

func TestDashboardPageCatalogConversationExportResources(t *testing.T) {
	raw, err := os.ReadFile("dashboard_page_catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Pages []struct {
			Code      string                        `json:"code"`
			Resources []DashboardPermissionResource `json:"resources"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	var resources []DashboardPermissionResource
	for _, page := range catalog.Pages {
		if page.Code == "dashboard.chat.export" {
			resources = page.Resources
			break
		}
	}
	if resources == nil {
		t.Fatal("conversation export page is missing")
	}
	for _, want := range []DashboardPermissionResource{
		{Method: http.MethodGet, PathPattern: "/dashboard/workMessage/exportCandidates"},
		{Method: http.MethodGet, PathPattern: "/dashboard/workMessage/exportTasks"},
		{Method: http.MethodPost, PathPattern: "/dashboard/workMessage/exportTasks"},
		{Method: http.MethodGet, PathPattern: "/dashboard/workMessage/exportDownload"},
	} {
		found := false
		for _, resource := range resources {
			if resource.Method == want.Method && resource.PathPattern == want.PathPattern {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("conversation export page missing resource %+v", want)
		}
	}
}

func TestDashboardResourceMatcherSupportsNamedSegmentsAndPrefersStatic(t *testing.T) {
	resources := []DashboardPermissionResource{
		{PermissionCode: "dynamic", Method: http.MethodGet, PathPattern: "/dashboard/reports/{id}"},
		{PermissionCode: "static", Method: http.MethodGet, PathPattern: "/dashboard/reports/overview"},
		{PermissionCode: "document", Method: http.MethodDelete, PathPattern: "/dashboard/knowledge-bases/{id}/documents/{documentId}"},
	}
	matches := matchingDashboardResources(resources, http.MethodGet, "/dashboard/reports/overview")
	if len(matches) != 1 || matches[0].PermissionCode != "static" {
		t.Fatalf("matches=%+v", matches)
	}
	documentMatches := matchingDashboardResources(resources, http.MethodDelete, "/dashboard/knowledge-bases/kb-1/documents/doc-1")
	if len(documentMatches) != 1 || documentMatches[0].PermissionCode != "document" {
		t.Fatalf("named segment matches=%+v", documentMatches)
	}
	if dashboardPathPatternMatches("/dashboard/reports/{id}/", "/dashboard/reports/12") {
		t.Fatal("trailing slash mismatch expanded the route")
	}
}

type panicRBACStore struct{}

func (panicRBACStore) UserByID(context.Context, int) (User, bool, error) {
	panic("legacy UserByID called")
}
func (panicRBACStore) RolesByUserTenant(context.Context, int, int) ([]Role, error) {
	panic("legacy roles called")
}
func (panicRBACStore) MenuByLinkURL(context.Context, string) (Menu, bool, error) {
	panic("legacy menu called")
}
func (panicRBACStore) RoleMenusByRoleIDs(context.Context, []int) ([]RoleMenu, error) {
	panic("legacy role menus called")
}
func (panicRBACStore) DepartmentEmployeeIDs(context.Context, int) ([]int, error) {
	panic("legacy scope called")
}

func machineCode(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		ErrorCode string `json:"errorCode"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v body=%s", err, recorder.Body.String())
	}
	return body.ErrorCode
}
