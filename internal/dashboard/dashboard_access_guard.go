package dashboard

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"
)

const DashboardPermissionDeniedCode = "DASHBOARD_PERMISSION_DENIED"

type DashboardAccessContext struct {
	UserID             int
	UserName           string
	TenantID           int
	CorpID             int
	WorkEmployeeID     int
	PermissionCode     string
	PermissionCodes    []string
	Scope              DataScope
	ScopeRequired      bool
	DepartmentIDs      []int
	AllowedEmployeeIDs []int
	IsSuperAdmin       bool
}

type dashboardAccessContextKey struct{}

func WithDashboardAccessContext(ctx context.Context, access DashboardAccessContext) context.Context {
	access.PermissionCodes = append([]string(nil), access.PermissionCodes...)
	access.DepartmentIDs = append([]int(nil), access.DepartmentIDs...)
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
	loginCorpEmployeeStore
	loginCorpAccessStore
}

type DashboardAccessGuard struct {
	store    DashboardAccessGuardStore
	cache    LoginCache
	resolver UserIDResolver
	access   *DashboardAccessService
	now      func() time.Time
}

func NewDashboardAccessGuard(store DashboardAccessGuardStore, cache LoginCache, resolver UserIDResolver, access *DashboardAccessService) *DashboardAccessGuard {
	if access == nil {
		access = NewDashboardAccessService(store)
	}
	return &DashboardAccessGuard{store: store, cache: cache, resolver: resolver, access: access, now: time.Now}
}

func (guard *DashboardAccessGuard) Authorize(w http.ResponseWriter, request *http.Request) bool {
	if guard == nil || guard.store == nil || guard.resolver == nil || guard.access == nil {
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

	userID, err := guard.resolver.UserID(request)
	if err != nil || userID <= 0 {
		writeMachineEnvelope(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized", nil)
		return false
	}
	identity, found, err := guard.store.DashboardAccessIdentity(request.Context(), userID)
	if err != nil {
		writeMachineEnvelope(w, http.StatusInternalServerError, "DASHBOARD_ACCESS_ERROR", "dashboard access unavailable", nil)
		return false
	}
	if !found || identity.UserID != userID || identity.TenantID <= 0 || identity.Status != 1 {
		writeMachineEnvelope(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized", nil)
		return false
	}
	now := time.Now()
	if guard.now != nil {
		now = guard.now()
	}
	tenantAccess, err := guard.store.DashboardTenantAccess(request.Context(), identity.TenantID, now)
	if err != nil {
		writeMachineEnvelope(w, http.StatusInternalServerError, "DASHBOARD_ACCESS_ERROR", "dashboard access unavailable", nil)
		return false
	}
	if !tenantAccess.Allowed || tenantAccess.TenantID != identity.TenantID {
		writeMachineEnvelope(w, http.StatusForbidden, DashboardTenantAccessDeniedCode, "tenant access denied", nil)
		return false
	}

	if contract == "GET /dashboard/access/profile" {
		corpID, err := guard.validatedCorpID(request, identity)
		if err != nil {
			writeMachineEnvelope(w, http.StatusInternalServerError, "DASHBOARD_ACCESS_ERROR", "dashboard access unavailable", nil)
			return false
		}
		guard.attachIdentityContext(request, identity, corpID)
		return true
	}
	if dashboardContractContains(ExactExemptDashboardRouteContracts(), contract) {
		return true
	}
	if dashboardContractContains(DenyOnlyDashboardRouteContracts(), contract) {
		if !identity.IsSuperAdmin {
			writeDashboardPermissionDenied(w)
			return false
		}
		guard.attachIdentityContext(request, identity)
		return true
	}
	if isDashboardAccessManagementRoute(contract) {
		if !identity.IsSuperAdmin {
			writeDashboardPermissionDenied(w)
			return false
		}
		guard.attachIdentityContext(request, identity)
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

	corpID, err := guard.validatedCorpID(request, identity)
	if err != nil {
		writeMachineEnvelope(w, http.StatusInternalServerError, "DASHBOARD_ACCESS_ERROR", "dashboard access unavailable", nil)
		return false
	}
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
	*request = *request.WithContext(WithDashboardAccessContext(request.Context(), access))
	return true
}

func isDashboardAccessManagementRoute(contract string) bool {
	method, path, ok := strings.Cut(contract, " ")
	if !ok || !strings.HasPrefix(path, "/dashboard/access/") || path == "/dashboard/access/profile" {
		return false
	}
	return method == http.MethodGet || method == http.MethodPost || method == http.MethodPut || method == http.MethodDelete
}

func (guard *DashboardAccessGuard) validatedCorpID(request *http.Request, identity DashboardAccessIdentity) (int, error) {
	cacheValue := ""
	if guard.cache != nil {
		value, err := guard.cache.UserCorpCache(request.Context(), identity.UserID)
		if err != nil {
			return 0, err
		}
		cacheValue = value
	}
	user := User{ID: identity.UserID, Name: identity.UserName, TenantID: identity.TenantID, Status: identity.Status}
	if identity.IsSuperAdmin {
		user.IsSuperAdmin = 1
	}
	info, err := ResolveValidatedLoginCorpInfoFromStore(request.Context(), request.Header, user, cacheValue, guard.store)
	if err != nil || len(info.CorpIDs) != 1 {
		return 0, err
	}
	return info.CorpIDs[0], nil
}

func (guard *DashboardAccessGuard) attachIdentityContext(request *http.Request, identity DashboardAccessIdentity, corpIDs ...int) {
	corpID := 0
	if len(corpIDs) > 0 {
		corpID = corpIDs[0]
	}
	access := DashboardAccessContext{
		UserID: identity.UserID, UserName: identity.UserName, TenantID: identity.TenantID,
		CorpID: corpID, Scope: DataScopeTenant, IsSuperAdmin: identity.IsSuperAdmin,
	}
	*request = *request.WithContext(WithDashboardAccessContext(request.Context(), access))
}

func dashboardContextForMatches(profile DashboardAccessProfile, matches []DashboardPermissionResource) (DashboardAccessContext, bool) {
	permissionByCode := make(map[string]EffectivePermission, len(profile.EffectivePermissions))
	for _, permission := range profile.EffectivePermissions {
		permissionByCode[permission.Code] = permission
	}
	codes := make([]string, 0, len(matches))
	scope := DataScope("")
	scopeRequired := false
	for _, resource := range matches {
		permission, allowed := permissionByCode[resource.PermissionCode]
		if !allowed {
			continue
		}
		codes = append(codes, resource.PermissionCode)
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
		if profile.CorpID <= 0 || profile.WorkEmployeeID <= 0 {
			return DashboardAccessContext{}, false
		}
		switch scope {
		case DataScopeTenant:
			// Tenant scope still requires a tenant-validated employee fact, but does
			// not constrain the employee set.
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
		PermissionCode: codes[0], PermissionCodes: codes, Scope: scope, ScopeRequired: scopeRequired,
		DepartmentIDs:      append([]int(nil), profile.DepartmentIDs...),
		AllowedEmployeeIDs: allowedEmployeeIDs, IsSuperAdmin: profile.IsSuperAdmin,
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
		if segment == "{id}" {
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
