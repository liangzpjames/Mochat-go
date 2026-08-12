package dashboard

import (
	"context"
	"sort"
	"strconv"
	"strings"
)

type DataScope string

const (
	DataScopeSelf       DataScope = "self"
	DataScopeDepartment DataScope = "department"
	DataScopeTenant     DataScope = "tenant"
)

const (
	PermissionSourceDirect     = "direct"
	PermissionSourceRole       = "role"
	PermissionSourceSuperadmin = "superadmin"
)

type DashboardAccessIdentity struct {
	UserID       int
	TenantID     int
	UserName     string
	Status       int
	IsSuperAdmin bool
}

type DashboardPermissionDefinition struct {
	ID             int64  `json:"id"`
	Code           string `json:"code"`
	Path           string `json:"path"`
	Name           string `json:"name"`
	GroupCode      string `json:"groupCode"`
	Sort           int    `json:"sort"`
	SuperadminOnly bool   `json:"superadminOnly"`
	ScopeRequired  bool   `json:"scopeRequired"`
}

type DashboardPermissionResource struct {
	PermissionCode string `json:"permissionCode"`
	Method         string `json:"method"`
	PathPattern    string `json:"pathPattern"`
	ScopeRequired  bool   `json:"scopeRequired"`
}

type DashboardEmployeeScope struct {
	EmployeeID            int
	DepartmentIDs         []int
	DepartmentEmployeeIDs []int
}

type DashboardPermissionGrantFact struct {
	TenantID      int
	PermissionID  int64
	SourceType    string
	SourceID      int
	SourceName    string
	SourceStatus  int
	SourceDeleted bool
	Scope         string
}

type PermissionSource struct {
	Type  string    `json:"type"`
	ID    int       `json:"id"`
	Name  string    `json:"name"`
	Scope DataScope `json:"scope"`
}

type EffectivePermission struct {
	Code    string             `json:"code"`
	Path    string             `json:"path"`
	Name    string             `json:"name"`
	Scope   DataScope          `json:"scope"`
	Sources []PermissionSource `json:"sources"`
}

type DashboardAccessProfile struct {
	UserID                int                             `json:"userId"`
	UserName              string                          `json:"userName"`
	TenantID              int                             `json:"tenantId"`
	CorpID                int                             `json:"corpId"`
	WorkEmployeeID        int                             `json:"workEmployeeId"`
	DepartmentIDs         []int                           `json:"departmentIds"`
	DepartmentEmployeeIDs []int                           `json:"departmentEmployeeIds"`
	IsSuperAdmin          bool                            `json:"isSuperAdmin"`
	CorpBindingStatus     string                          `json:"corpBindingStatus"`
	Catalog               []DashboardPermissionDefinition `json:"catalog"`
	EffectivePermissions  []EffectivePermission           `json:"effectivePermissions"`
	AllowedRoutes         []string                        `json:"allowedRoutes"`
}

type DashboardAccessStore interface {
	DashboardAccessIdentity(ctx context.Context, userID int) (DashboardAccessIdentity, bool, error)
	DashboardPermissionCatalog(ctx context.Context) ([]DashboardPermissionDefinition, error)
	DashboardPermissionGrants(ctx context.Context, tenantID, userID int) ([]DashboardPermissionGrantFact, error)
	DashboardEmployeeScope(ctx context.Context, tenantID, userID, corpID int) (DashboardEmployeeScope, bool, error)
}

type DashboardAccessService struct {
	store DashboardAccessStore
}

func NewDashboardAccessService(store DashboardAccessStore) *DashboardAccessService {
	return &DashboardAccessService{store: store}
}

func (service *DashboardAccessService) Resolve(ctx context.Context, userID, corpID int) (DashboardAccessProfile, error) {
	identity, found, err := service.store.DashboardAccessIdentity(ctx, userID)
	if err != nil {
		return DashboardAccessProfile{}, err
	}
	if !found || identity.Status != 1 || identity.UserID != userID || identity.TenantID <= 0 {
		return DashboardAccessProfile{}, ErrUnauthorized
	}
	catalog, err := service.store.DashboardPermissionCatalog(ctx)
	if err != nil {
		return DashboardAccessProfile{}, err
	}
	sort.SliceStable(catalog, func(left, right int) bool {
		if catalog[left].Sort == catalog[right].Sort {
			return catalog[left].Code < catalog[right].Code
		}
		return catalog[left].Sort < catalog[right].Sort
	})
	profile := DashboardAccessProfile{
		UserID: identity.UserID, UserName: identity.UserName, TenantID: identity.TenantID,
		CorpID: corpID, IsSuperAdmin: identity.IsSuperAdmin, Catalog: catalog,
		EffectivePermissions: []EffectivePermission{}, AllowedRoutes: []string{},
		DepartmentIDs: []int{}, DepartmentEmployeeIDs: []int{},
	}
	if corpID > 0 {
		employeeScope, found, err := service.store.DashboardEmployeeScope(ctx, identity.TenantID, identity.UserID, corpID)
		if err != nil {
			return DashboardAccessProfile{}, err
		}
		if found {
			profile.WorkEmployeeID = employeeScope.EmployeeID
			profile.DepartmentIDs = append([]int(nil), employeeScope.DepartmentIDs...)
			profile.DepartmentEmployeeIDs = append([]int(nil), employeeScope.DepartmentEmployeeIDs...)
		}
	}
	if identity.IsSuperAdmin {
		for _, permission := range catalog {
			profile.EffectivePermissions = append(profile.EffectivePermissions, EffectivePermission{
				Code: permission.Code, Path: permission.Path, Name: permission.Name, Scope: DataScopeTenant,
				Sources: []PermissionSource{{Type: PermissionSourceSuperadmin, ID: identity.UserID, Name: "superadmin", Scope: DataScopeTenant}},
			})
		}
		return finalizeDashboardAccessProfile(profile), nil
	}

	grants, err := service.store.DashboardPermissionGrants(ctx, identity.TenantID, identity.UserID)
	if err != nil {
		return DashboardAccessProfile{}, err
	}
	permissionsByID := make(map[int64]DashboardPermissionDefinition, len(catalog))
	for _, permission := range catalog {
		permissionsByID[permission.ID] = permission
	}
	effectiveByID := make(map[int64]*EffectivePermission)
	sourceKeysByPermission := make(map[int64]map[string]bool)
	for _, grant := range grants {
		permission, ok := permissionsByID[grant.PermissionID]
		if !ok || permission.SuperadminOnly || grant.TenantID != identity.TenantID || !dashboardGrantSourceActive(grant) {
			continue
		}
		scope, valid := dashboardGrantScope(grant)
		if !valid && permission.ScopeRequired {
			continue
		}
		if !valid {
			scope = DataScopeSelf
		}
		sourceKey := grant.SourceType + ":" + strconv.Itoa(grant.SourceID)
		effective := effectiveByID[permission.ID]
		if effective == nil {
			effective = &EffectivePermission{Code: permission.Code, Path: permission.Path, Name: permission.Name, Scope: scope, Sources: []PermissionSource{}}
			effectiveByID[permission.ID] = effective
			sourceKeysByPermission[permission.ID] = make(map[string]bool)
		} else {
			if sourceKeysByPermission[permission.ID][sourceKey] {
				continue
			}
			effective.Scope = MergeDataScopes(effective.Scope, scope)
		}
		sourceKeysByPermission[permission.ID][sourceKey] = true
		effective.Sources = append(effective.Sources, PermissionSource{Type: grant.SourceType, ID: grant.SourceID, Name: grant.SourceName, Scope: scope})
	}
	for _, permission := range effectiveByID {
		sort.SliceStable(permission.Sources, func(left, right int) bool {
			if permission.Sources[left].Type == permission.Sources[right].Type {
				return permission.Sources[left].ID < permission.Sources[right].ID
			}
			return permission.Sources[left].Type < permission.Sources[right].Type
		})
		profile.EffectivePermissions = append(profile.EffectivePermissions, *permission)
	}
	return finalizeDashboardAccessProfile(profile), nil
}

func dashboardGrantScope(grant DashboardPermissionGrantFact) (DataScope, bool) {
	switch grant.SourceType {
	case PermissionSourceDirect:
		if strings.TrimSpace(grant.Scope) == "" {
			return DataScopeSelf, true
		}
	case PermissionSourceRole:
	default:
		return "", false
	}
	scope := DataScope(strings.TrimSpace(grant.Scope))
	return scope, scope == DataScopeSelf || scope == DataScopeDepartment || scope == DataScopeTenant
}

func dashboardGrantSourceActive(grant DashboardPermissionGrantFact) bool {
	switch grant.SourceType {
	case PermissionSourceDirect:
		return true
	case PermissionSourceRole:
		return grant.SourceStatus == 1 && !grant.SourceDeleted
	default:
		return false
	}
}

func MergeDataScopes(left, right DataScope) DataScope {
	weight := func(scope DataScope) int {
		switch scope {
		case DataScopeTenant:
			return 3
		case DataScopeDepartment:
			return 2
		case DataScopeSelf:
			return 1
		default:
			return 0
		}
	}
	if weight(right) > weight(left) {
		return right
	}
	return left
}

func finalizeDashboardAccessProfile(profile DashboardAccessProfile) DashboardAccessProfile {
	sort.SliceStable(profile.EffectivePermissions, func(left, right int) bool {
		return profile.EffectivePermissions[left].Code < profile.EffectivePermissions[right].Code
	})
	profile.AllowedRoutes = make([]string, 0, len(profile.EffectivePermissions))
	for _, permission := range profile.EffectivePermissions {
		profile.AllowedRoutes = append(profile.AllowedRoutes, permission.Path)
	}
	return profile
}

func (profile DashboardAccessProfile) AllowsPage(path string) bool {
	for _, allowed := range profile.AllowedRoutes {
		if allowed == path {
			return true
		}
	}
	return false
}

func (profile DashboardAccessProfile) ScopeFor(code string) DataScope {
	for _, permission := range profile.EffectivePermissions {
		if permission.Code == code {
			return permission.Scope
		}
	}
	return ""
}
