package dashboard

import (
	"context"
	"net/http"
	"sort"
	"strings"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

const (
	DashboardPermissionDeniedCode          = "DASHBOARD_PERMISSION_DENIED"
	DashboardCorpConfigurationRequiredCode = "CORP_CONFIGURATION_REQUIRED"
)

type DashboardAccessContext struct {
	UserID                int
	UserName              string
	TenantID              int
	CorpID                int
	WorkEmployeeID        int
	PermissionCode        string
	PermissionCodes       []string
	PermissionScopes      map[string]DataScope
	Scope                 DataScope
	ScopeRequired         bool
	DepartmentIDs         []int
	DepartmentEmployeeIDs []int
	AllowedEmployeeIDs    []int
	IsSuperAdmin          bool
}

type dashboardAccessContextKey struct{}

func WithDashboardAccessContext(ctx context.Context, access DashboardAccessContext) context.Context {
	access.PermissionCodes = append([]string(nil), access.PermissionCodes...)
	if access.PermissionScopes != nil {
		permissionScopes := make(map[string]DataScope, len(access.PermissionScopes))
		for code, scope := range access.PermissionScopes {
			permissionScopes[code] = scope
		}
		access.PermissionScopes = permissionScopes
	}
	access.DepartmentIDs = append([]int(nil), access.DepartmentIDs...)
	access.DepartmentEmployeeIDs = append([]int(nil), access.DepartmentEmployeeIDs...)
	access.AllowedEmployeeIDs = append([]int(nil), access.AllowedEmployeeIDs...)
	return context.WithValue(ctx, dashboardAccessContextKey{}, access)
}

func DashboardAccessFromContext(ctx context.Context) (DashboardAccessContext, bool) {
	access, ok := ctx.Value(dashboardAccessContextKey{}).(DashboardAccessContext)
	return access, ok
}

type DashboardPermissionResourceStore interface {
	DashboardPermissionResources(ctx context.Context, method string) ([]DashboardPermissionResource, error)
}

type DashboardAccessGuardStore interface {
	DashboardAccessStore
	DashboardTenantAccessStore
	DashboardPermissionResourceStore
}

type DashboardAccessGuard struct {
	store  DashboardAccessGuardStore
	access *DashboardAccessService
}

func NewDashboardAccessGuard(store DashboardAccessGuardStore, access *DashboardAccessService) *DashboardAccessGuard {
	if access == nil {
		access = NewDashboardAccessService(store)
	}
	return &DashboardAccessGuard{store: store, access: access}
}

func (guard *DashboardAccessGuard) Authorize(w http.ResponseWriter, request *http.Request) bool {
	if guard == nil || guard.store == nil || guard.access == nil || request == nil {
		writeMachineEnvelope(w, http.StatusInternalServerError, "DASHBOARD_ACCESS_ERROR", "dashboard access unavailable", nil)
		return false
	}
	method, path := strings.ToUpper(request.Method), request.URL.Path
	if isDashboardSaaSPath(path) {
		return true
	}
	contract := method + " " + path
	if isPublicDashboardExemption(contract) {
		return true
	}

	principal, err := DashboardPrincipalFromContext(request.Context())
	if err != nil || principal.UserID <= 0 || principal.TenantID <= 0 || principal.CorpID <= 0 {
		writeMachineEnvelope(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized", nil)
		return false
	}
	if principal.CorpStatus == dashboardprincipal.CorpBindingStatusSuspended {
		writeMachineEnvelope(w, http.StatusForbidden, DashboardTenantAccessDeniedCode, "tenant access denied", nil)
		return false
	}
	userID := principal.UserID
	if principal.CorpStatus == dashboardprincipal.CorpBindingStatusPending {
		if isPendingCompanySyncContract(contract) {
			writeDashboardCorpConfigurationRequired(w)
			return false
		}
		if isPendingCompanyConfigurationContract(contract) {
			if !principal.IsSuperAdmin {
				writeDashboardPermissionDenied(w)
				return false
			}
			guard.attachIdentityContext(request, principal)
			return true
		}
	}
	if method != http.MethodGet && method != http.MethodHead && dashboardPathPatternMatches("/dashboard/archive/media/{id}/content", path) {
		// The principal boundary still applies, but unsupported methods do not
		// need a fabricated RBAC resource merely to reach the handler's 405.
		guard.attachIdentityContext(request, principal)
		return true
	}

	if contract == "GET /dashboard/access/profile" {
		guard.attachIdentityContext(request, principal)
		return true
	}
	if dashboardContractContains(ExactExemptDashboardRouteContracts(), contract) {
		return true
	}
	if dashboardContractContains(DenyOnlyDashboardRouteContracts(), contract) {
		if !principal.IsSuperAdmin {
			writeDashboardPermissionDenied(w)
			return false
		}
		guard.attachIdentityContext(request, principal)
		return true
	}
	if isDashboardAccessManagementRoute(contract) {
		if !principal.IsSuperAdmin {
			writeDashboardPermissionDenied(w)
			return false
		}
		guard.attachIdentityContext(request, principal)
		return true
	}

	resources, err := guard.store.DashboardPermissionResources(request.Context(), method)
	if err != nil {
		writeMachineEnvelope(w, http.StatusInternalServerError, "DASHBOARD_ACCESS_ERROR", "dashboard access unavailable", nil)
		return false
	}
	matches := matchingDashboardResources(resources, method, path)
	if len(matches) == 0 {
		writeDashboardPermissionDenied(w)
		return false
	}

	corpID := principal.CorpID
	if corpID <= 0 {
		writeDashboardPermissionDenied(w)
		return false
	}
	profile, err := guard.access.Resolve(request.Context(), userID, corpID)
	if err != nil {
		if err == ErrUnauthorized {
			writeMachineEnvelope(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized", nil)
		} else {
			writeMachineEnvelope(w, http.StatusInternalServerError, "DASHBOARD_ACCESS_ERROR", "dashboard access unavailable", nil)
		}
		return false
	}
	access, ok := dashboardContextForMatches(profile, matches)
	if !ok {
		writeDashboardPermissionDenied(w)
		return false
	}
	ctx := WithDashboardAccessContext(request.Context(), access)
	ctx = dashboardprincipal.WithCapabilityAccess(ctx, access.IsSuperAdmin, access.PermissionCodes)
	*request = *request.WithContext(ctx)
	return true
}

func (guard *DashboardAccessGuard) authorizePendingBinding(w http.ResponseWriter, request *http.Request, principal dashboardprincipal.DashboardPrincipal, contract string) bool {
	if contract == "GET /dashboard/access/profile" || contract == "GET /dashboard/auth/session" ||
		contract == "POST /dashboard/auth/logout" || contract == "PUT /dashboard/user/logout" {
		guard.attachIdentityContext(request, principal)
		return true
	}
	if isPendingCompanySyncContract(contract) {
		writeDashboardCorpConfigurationRequired(w)
		return false
	}
	if isPendingCompanyConfigurationContract(contract) {
		if !principal.IsSuperAdmin {
			writeDashboardPermissionDenied(w)
			return false
		}
		guard.attachIdentityContext(request, principal)
		return true
	}
	writeDashboardCorpConfigurationRequired(w)
	return false
}

func isPendingCompanyConfigurationContract(contract string) bool {
	switch contract {
	case "GET /dashboard/company/profile", "PUT /dashboard/company/profile",
		"PUT /dashboard/company/wecom-credentials", "PUT /dashboard/company/agent-credentials",
		"PUT /dashboard/company/application-credentials", "PUT /dashboard/company/archive-credentials",
		"GET /dashboard/company/callback-configuration", "POST /dashboard/company/callback-configuration/regenerate",
		"POST /dashboard/company/verify",
		"GET /dashboard/company/audits", "GET /dashboard/providers/status", "GET /dashboard/company/sync-status",
		"GET /dashboard/company/archive-sync-status":
		return true
	default:
		return false
	}
}

func isPendingCompanySyncContract(contract string) bool {
	return contract == "POST /dashboard/company/employee-sync" || contract == "POST /dashboard/company/archive-sync"
}

func isDashboardAccessManagementRoute(contract string) bool {
	method, path, ok := strings.Cut(contract, " ")
	if !ok || !strings.HasPrefix(path, "/dashboard/access/") || path == "/dashboard/access/profile" {
		return false
	}
	return method == http.MethodGet || method == http.MethodPost || method == http.MethodPut || method == http.MethodDelete
}

func (guard *DashboardAccessGuard) attachIdentityContext(request *http.Request, principal dashboardprincipal.DashboardPrincipal) {
	access := DashboardAccessContext{
		UserID: principal.UserID, TenantID: principal.TenantID,
		CorpID: principal.CorpID, Scope: DataScopeTenant, IsSuperAdmin: principal.IsSuperAdmin,
	}
	ctx := WithDashboardAccessContext(request.Context(), access)
	ctx = dashboardprincipal.WithCapabilityAccess(ctx, principal.IsSuperAdmin, nil)
	*request = *request.WithContext(ctx)
}

func dashboardContextForMatches(profile DashboardAccessProfile, matches []DashboardPermissionResource) (DashboardAccessContext, bool) {
	permissionByCode := make(map[string]EffectivePermission, len(profile.EffectivePermissions))
	for _, permission := range profile.EffectivePermissions {
		permissionByCode[permission.Code] = permission
	}
	codes := make([]string, 0, len(matches))
	permissionScopes := make(map[string]DataScope, len(matches))
	scope := DataScope("")
	scopeRequired := false
	for _, resource := range matches {
		permission, allowed := permissionByCode[resource.PermissionCode]
		if !allowed {
			continue
		}
		codes = append(codes, resource.PermissionCode)
		permissionScopes[resource.PermissionCode] = permission.Scope
		scope = MergeDataScopes(scope, permission.Scope)
		scopeRequired = scopeRequired || resource.ScopeRequired
	}
	if len(codes) == 0 {
		return DashboardAccessContext{}, false
	}
	sort.Strings(codes)
	codes = uniqueStrings(codes)
	allowedEmployeeIDs := []int{}
	if scopeRequired {
		allowsUnboundSuperadmin := profile.IsSuperAdmin && scope == DataScopeTenant
		if profile.CorpID <= 0 || (profile.WorkEmployeeID <= 0 && !allowsUnboundSuperadmin) {
			return DashboardAccessContext{}, false
		}
		switch scope {
		case DataScopeTenant:
			// Tenant scope does not constrain the employee set. Superadmins may be
			// provisioned by SaaS without a matching WeCom employee record.
		case DataScopeDepartment:
			allowedEmployeeIDs = uniquePositiveInts(profile.DepartmentEmployeeIDs)
			if len(allowedEmployeeIDs) == 0 {
				return DashboardAccessContext{}, false
			}
		case DataScopeSelf:
			allowedEmployeeIDs = []int{profile.WorkEmployeeID}
		default:
			return DashboardAccessContext{}, false
		}
	}
	return DashboardAccessContext{
		UserID: profile.UserID, UserName: profile.UserName, TenantID: profile.TenantID,
		CorpID: profile.CorpID, WorkEmployeeID: profile.WorkEmployeeID,
		PermissionCode: codes[0], PermissionCodes: codes, PermissionScopes: permissionScopes, Scope: scope, ScopeRequired: scopeRequired,
		DepartmentIDs:         append([]int(nil), profile.DepartmentIDs...),
		DepartmentEmployeeIDs: append([]int(nil), profile.DepartmentEmployeeIDs...),
		AllowedEmployeeIDs:    allowedEmployeeIDs, IsSuperAdmin: profile.IsSuperAdmin,
	}, true
}

func matchingDashboardResources(resources []DashboardPermissionResource, method, path string) []DashboardPermissionResource {
	staticMatches := make([]DashboardPermissionResource, 0)
	dynamicMatches := make([]DashboardPermissionResource, 0)
	for _, resource := range resources {
		if strings.EqualFold(resource.Method, method) && dashboardPathPatternMatches(resource.PathPattern, path) {
			if resource.PathPattern == path {
				staticMatches = append(staticMatches, resource)
			} else {
				dynamicMatches = append(dynamicMatches, resource)
			}
		}
	}
	matches := dynamicMatches
	if len(staticMatches) > 0 {
		matches = staticMatches
	}
	sort.SliceStable(matches, func(left, right int) bool {
		return matches[left].PermissionCode < matches[right].PermissionCode
	})
	return matches
}

func dashboardPathPatternMatches(pattern, path string) bool {
	if pattern == path {
		return true
	}
	if strings.HasPrefix(pattern, "/") != strings.HasPrefix(path, "/") || strings.HasSuffix(pattern, "/") != strings.HasSuffix(path, "/") {
		return false
	}
	patternSegments := strings.Split(strings.TrimPrefix(pattern, "/"), "/")
	pathSegments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(patternSegments) != len(pathSegments) {
		return false
	}
	for index := range patternSegments {
		segment := patternSegments[index]
		if len(segment) > 2 && strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			if pathSegments[index] == "" {
				return false
			}
			continue
		}
		if segment != pathSegments[index] {
			return false
		}
	}
	return true
}

func dashboardContractContains(contracts []string, requestContract string) bool {
	method, path, ok := strings.Cut(requestContract, " ")
	if !ok {
		return false
	}
	for _, contract := range contracts {
		contractMethod, pattern, valid := strings.Cut(contract, " ")
		if valid && contractMethod == method && (pattern == path || (strings.Contains(pattern, "{") && dashboardPathPatternMatches(pattern, path))) {
			return true
		}
	}
	return false
}

func isPublicDashboardExemption(contract string) bool {
	return dashboardContractContains(PublicDashboardRouteContracts(), contract)
}

func isDashboardSaaSPath(path string) bool {
	for _, prefix := range []string{"/dashboard/saasAdmin/", "/dashboard/saasAlert/", "/dashboard/saasBilling/"} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func writeDashboardPermissionDenied(w http.ResponseWriter) {
	writeMachineEnvelope(w, http.StatusForbidden, DashboardPermissionDeniedCode, "dashboard permission denied", nil)
}

func writeDashboardCorpConfigurationRequired(w http.ResponseWriter) {
	writeMachineEnvelope(w, http.StatusForbidden, DashboardCorpConfigurationRequiredCode, "corp configuration required", nil)
}

func uniqueStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}
