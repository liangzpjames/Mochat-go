package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

const (
	DataPermissionAll        = 0
	DataPermissionDepartment = 1
	DataPermissionSelf       = 2
)

var ErrPermissionDenied = errors.New("permission denied")

type Role struct {
	ID             int
	DataPermission string
	Status         int
}

type RoleMenu struct {
	RoleID int
	MenuID int
}

type AccessContext struct {
	User            User
	PermissionKey   string
	RoleID          int
	CorpID          int
	WorkEmployeeID  int
	DataPermission  int
	DeptEmployeeIDs []int
}

type RBACStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	RolesByUserTenant(ctx context.Context, userID int, tenantID int) ([]Role, error)
	MenuByLinkURL(ctx context.Context, linkURL string) (Menu, bool, error)
	RoleMenusByRoleIDs(ctx context.Context, roleIDs []int) ([]RoleMenu, error)
	DepartmentEmployeeIDs(ctx context.Context, employeeID int) ([]int, error)
}

type RBACResolver struct {
	store RBACStore
}

func NewRBACResolver(store RBACStore) *RBACResolver {
	return &RBACResolver{store: store}
}

func PermissionKey(path string, method string) string {
	return path + "#" + strings.ToLower(method)
}

func PermissionKeyFromRequest(r *http.Request) string {
	return PermissionKey(r.URL.Path, r.Method)
}

func menuLinkURLFromPermissionKey(permissionKey string) string {
	linkURL, _, _ := strings.Cut(permissionKey, "#")
	return linkURL
}

func requestWithPermissionPath(r *http.Request, path string) *http.Request {
	if r.URL.Path == path {
		return r
	}
	cloned := r.Clone(r.Context())
	urlCopy := *r.URL
	urlCopy.Path = path
	urlCopy.RawPath = ""
	cloned.URL = &urlCopy
	return cloned
}

func (r *RBACResolver) Resolve(
	ctx context.Context,
	userID int,
	permissionKey string,
	corpID int,
	workEmployeeID int,
) (AccessContext, error) {
	user, ok, err := r.store.UserByID(ctx, userID)
	if err != nil {
		return AccessContext{}, err
	}
	if !ok {
		return AccessContext{}, ErrUnauthorized
	}

	access := AccessContext{
		User:           user,
		PermissionKey:  permissionKey,
		CorpID:         corpID,
		WorkEmployeeID: workEmployeeID,
	}

	if user.IsSuperAdmin == 1 {
		access.DataPermission = DataPermissionAll
		return access, nil
	}

	roles, err := r.store.RolesByUserTenant(ctx, user.ID, user.TenantID)
	if err != nil {
		return AccessContext{}, err
	}
	if len(roles) == 0 {
		return AccessContext{}, ErrPermissionDenied
	}

	menu, ok, err := r.store.MenuByLinkURL(ctx, menuLinkURLFromPermissionKey(permissionKey))
	if err != nil {
		return AccessContext{}, err
	}
	if !ok {
		return AccessContext{}, ErrPermissionDenied
	}

	roleByID := make(map[int]Role, len(roles))
	roleIDs := make([]int, 0, len(roles))
	for _, role := range roles {
		roleByID[role.ID] = role
		roleIDs = append(roleIDs, role.ID)
	}

	roleMenus, err := r.store.RoleMenusByRoleIDs(ctx, roleIDs)
	if err != nil {
		return AccessContext{}, err
	}

	roleIDByMenuID := make(map[int]int, len(roleMenus))
	for _, roleMenu := range roleMenus {
		roleIDByMenuID[roleMenu.MenuID] = roleMenu.RoleID
	}

	roleID, ok := roleIDByMenuID[menu.ID]
	if !ok {
		return AccessContext{}, ErrPermissionDenied
	}

	access.RoleID = roleID
	access.DataPermission = resolveRouteDataPermission(menu, roleByID[roleID], corpID)
	switch access.DataPermission {
	case DataPermissionDepartment:
		employeeIDs, err := r.store.DepartmentEmployeeIDs(ctx, workEmployeeID)
		if err != nil {
			return AccessContext{}, err
		}
		if len(employeeIDs) == 0 && workEmployeeID > 0 {
			employeeIDs = []int{workEmployeeID}
		}
		access.DeptEmployeeIDs = employeeIDs
	case DataPermissionSelf:
		if workEmployeeID > 0 {
			access.DeptEmployeeIDs = []int{workEmployeeID}
		}
	default:
		access.DeptEmployeeIDs = []int{}
	}

	return access, nil
}

func resolveRouteDataPermission(menu Menu, role Role, corpID int) int {
	if menu.DataPermission == DataPermissionSelf {
		return DataPermissionAll
	}

	dataPermission := menu.DataPermission
	for _, setting := range parseRoleDataPermission(role.DataPermission) {
		if setting.CorpID == corpID {
			dataPermission = setting.PermissionType
			break
		}
	}
	return dataPermission
}

type roleDataPermissionSetting struct {
	CorpID         int `json:"corpId"`
	CorpIDSnake    int `json:"corp_id"`
	PermissionType int `json:"permissionType"`
	Permission     int `json:"permission_type"`
}

func parseRoleDataPermission(raw string) []roleDataPermissionSetting {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}

	var settings []roleDataPermissionSetting
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return nil
	}
	for index := range settings {
		if settings[index].CorpID == 0 {
			settings[index].CorpID = settings[index].CorpIDSnake
		}
		if settings[index].PermissionType == 0 {
			settings[index].PermissionType = settings[index].Permission
		}
	}
	return settings
}
