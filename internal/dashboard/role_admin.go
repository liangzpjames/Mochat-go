package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type RoleListFilter struct {
	TenantID int
	CorpID   int
	Name     string
	Page     int
	PerPage  int
}

type RoleListItem struct {
	ID          int
	Name        string
	Remarks     string
	UpdatedAt   string
	Status      int
	EmployeeNum int
}

type RoleListPage struct {
	Items     []RoleListItem
	Total     int
	TotalPage int
}

type RoleDetail struct {
	ID             int
	Name           string
	Remarks        string
	DataPermission string
}

type RolePermissionMenu struct {
	ID         int
	ParentID   int
	Name       string
	Level      int
	IsPageMenu int
}

type RoleEmployeeFilter struct {
	RoleID  int
	CorpID  int
	Page    int
	PerPage int
}

type RoleEmployee struct {
	ID         int
	Name       string
	Mobile     string
	Email      string
	Department string
}

type RoleEmployeePage struct {
	Items     []RoleEmployee
	Total     int
	TotalPage int
}

type RoleCreateValues struct {
	TenantID       int
	Name           string
	Remarks        string
	OperateID      int
	OperateName    string
	DataPermission string
}

type RoleUpdateValues struct {
	Name           string
	Remarks        string
	OperateID      int
	OperateName    string
	DataPermission string
}

type RoleAdminStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	RoleList(ctx context.Context, filter RoleListFilter) (RoleListPage, error)
	RoleDetailByIDTenant(ctx context.Context, roleID int, tenantID int) (RoleDetail, bool, error)
	RolePermissionMenus(ctx context.Context, roleID int) ([]RolePermissionMenu, []int, error)
	RoleEmployees(ctx context.Context, filter RoleEmployeeFilter) (RoleEmployeePage, error)
	RoleNameExists(ctx context.Context, name string, tenantID int, excludeRoleID int) (bool, error)
	CreateRole(ctx context.Context, values RoleCreateValues, copyFromRoleID int) (int, error)
	UpdateRole(ctx context.Context, roleID int, tenantID int, values RoleUpdateValues) (bool, error)
	UpdateRoleStatus(ctx context.Context, roleID int, tenantID int, status int) (bool, error)
	DeleteRole(ctx context.Context, roleID int, tenantID int) (bool, error)
	RoleEmployeeCount(ctx context.Context, roleID int, corpID int) (int, error)
	ExpandedMenuIDs(ctx context.Context, menuIDs []int) ([]int, error)
	ReplaceRoleMenus(ctx context.Context, roleID int, menuIDs []int) error
}

type RoleAdminHandler struct {
	store      RoleAdminStore
	cache      LoginCache
	resolver   UserIDResolver
	authorizer CorpAdminAuthorizer
}

func NewRoleAdminHandler(store RoleAdminStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer) *RoleAdminHandler {
	return &RoleAdminHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer}
}

func (h *RoleAdminHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorize(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}

	filter := RoleListFilter{
		TenantID: user.TenantID,
		CorpID:   corpID,
		Name:     strings.TrimSpace(r.URL.Query().Get("name")),
		Page:     positiveQueryInt(r, "page", 1),
		PerPage:  positiveQueryInt(r, "perPage", 10),
	}
	page, err := h.store.RoleList(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	list := make([]map[string]any, 0, len(page.Items))
	for _, role := range page.Items {
		list = append(list, map[string]any{
			"roleId":      role.ID,
			"name":        role.Name,
			"remarks":     role.Remarks,
			"updatedAt":   role.UpdatedAt,
			"status":      role.Status,
			"employeeNum": role.EmployeeNum,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{
			"perPage":   filter.PerPage,
			"total":     page.Total,
			"totalPage": page.TotalPage,
		},
		"list": list,
	})
}

func (h *RoleAdminHandler) Show(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorize(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}

	roleID, err := positiveQueryIntRequired(r, "roleId")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "角色ID 必填", nil)
		return
	}
	role, found, err := h.store.RoleDetailByIDTenant(r.Context(), roleID, user.TenantID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "角色不存在", nil)
		return
	}

	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"roleId":         role.ID,
		"name":           role.Name,
		"remarks":        role.Remarks,
		"dataPermission": roleDataPermissionForCorp(role.DataPermission, corpID),
	})
}

func (h *RoleAdminHandler) PermissionShow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorize(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}

	roleID, err := positiveQueryIntRequired(r, "roleId")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "角色ID必须", nil)
		return
	}
	menus, checkedIDs, err := h.store.RolePermissionMenus(r.Context(), roleID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	checked := make(map[int]struct{}, len(checkedIDs))
	for _, id := range checkedIDs {
		checked[id] = struct{}{}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", rolePermissionTree(menus, 0, checked))
}

func (h *RoleAdminHandler) ShowEmployee(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorize(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}

	roleID, err := queryIntRequired(r, "roleId")
	if err != nil || roleID < 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "角色ID 必填", nil)
		return
	}
	filter := RoleEmployeeFilter{
		RoleID:  roleID,
		CorpID:  corpID,
		Page:    positiveQueryInt(r, "page", 1),
		PerPage: positiveQueryInt(r, "perPage", 10),
	}
	page, err := h.store.RoleEmployees(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	list := make([]map[string]any, 0, len(page.Items))
	for _, employee := range page.Items {
		list = append(list, map[string]any{
			"employeeId":   employee.ID,
			"employeeName": employee.Name,
			"phone":        employee.Mobile,
			"email":        employee.Email,
			"department":   employee.Department,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{
			"perPage":   filter.PerPage,
			"total":     page.Total,
			"totalPage": page.TotalPage,
		},
		"list": list,
	})
}

func (h *RoleAdminHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorize(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}

	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	name := stringParam(params, "name")
	if name == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "角色名称 必填", nil)
		return
	}
	dataPermission, ok, err := intParam(params, "dataPermission")
	if err != nil || !ok || dataPermission <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "数据权限 必填", nil)
		return
	}
	copyFromRoleID, _, err := intParam(params, "roleId")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "角色id 必须为整型", nil)
		return
	}
	exists, err := h.store.RoleNameExists(r.Context(), name, user.TenantID, 0)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if exists {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, name+"-该角色已存在", nil)
		return
	}

	_, err = h.store.CreateRole(r.Context(), RoleCreateValues{
		TenantID:       user.TenantID,
		Name:           name,
		Remarks:        stringParam(params, "remarks"),
		OperateID:      user.ID,
		OperateName:    user.Name,
		DataPermission: roleDataPermissionJSON(corpID, dataPermission),
	}, copyFromRoleID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *RoleAdminHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorize(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}

	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	roleID, ok, err := intParam(params, "roleId")
	if err != nil || !ok || roleID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "角色id 必填", nil)
		return
	}
	name := stringParam(params, "name")
	if name == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "角色名称 必填", nil)
		return
	}
	remarks := stringParam(params, "remarks")
	if remarks == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "角色描述 必填", nil)
		return
	}
	dataPermission, ok, err := intParam(params, "dataPermission")
	if err != nil || !ok || dataPermission <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "数据权限 必填", nil)
		return
	}

	role, found, err := h.store.RoleDetailByIDTenant(r.Context(), roleID, user.TenantID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "角色不存在", nil)
		return
	}
	exists, err := h.store.RoleNameExists(r.Context(), name, user.TenantID, roleID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if exists {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, name+"-该角色已存在", nil)
		return
	}

	updated, err := h.store.UpdateRole(r.Context(), roleID, user.TenantID, RoleUpdateValues{
		Name:           name,
		Remarks:        remarks,
		OperateID:      user.ID,
		OperateName:    user.Name,
		DataPermission: mergeRoleDataPermissionJSON(role.DataPermission, corpID, dataPermission),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !updated {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "角色更新失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *RoleAdminHandler) StatusUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorize(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}

	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	roleID, ok, err := intParam(params, "roleId")
	if err != nil || !ok || roleID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "角色id 必填", nil)
		return
	}
	status, ok, err := intParam(params, "status")
	if err != nil || !ok || (status != 1 && status != 2) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "状态 值必须在列表内：[1,2]", nil)
		return
	}
	role, found, err := h.store.RoleDetailByIDTenant(r.Context(), roleID, user.TenantID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "角色不存在", nil)
		return
	}
	if isSystemPresetRole(role) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "系统预置角色不可停用，请新建角色后调整", nil)
		return
	}
	updated, err := h.store.UpdateRoleStatus(r.Context(), roleID, user.TenantID, status)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !updated {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "角色状态修改失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *RoleAdminHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorize(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}

	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	roleID, ok, err := intParam(params, "roleId")
	if err != nil || !ok || roleID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "角色id 必填", nil)
		return
	}
	role, found, err := h.store.RoleDetailByIDTenant(r.Context(), roleID, user.TenantID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "角色不存在", nil)
		return
	}
	if isSystemPresetRole(role) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "系统预置角色不可删除", nil)
		return
	}
	employeeCount, err := h.store.RoleEmployeeCount(r.Context(), roleID, corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if employeeCount > 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "角色下有成员，不能删除角色", nil)
		return
	}
	deleted, err := h.store.DeleteRole(r.Context(), roleID, user.TenantID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !deleted {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "删除失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *RoleAdminHandler) PermissionStore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorize(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}

	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	roleID, ok, err := intParam(params, "roleId")
	if err != nil || !ok || roleID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "角色必须", nil)
		return
	}
	menuIDs, err := intSliceParam(params, "menuIds")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "菜单id 必须为整型", nil)
		return
	}
	menuIDs, err = h.store.ExpandedMenuIDs(r.Context(), menuIDs)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if err := h.store.ReplaceRoleMenus(r.Context(), roleID, menuIDs); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "角色授权失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func isSystemPresetRole(role RoleDetail) bool {
	remarks := strings.TrimSpace(role.Remarks)
	return remarks == "系统预置全权限角色" || remarks == "bootstrap full-access role"
}

func (h *RoleAdminHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
	requestPrincipal, err := DashboardPrincipalFromContext(r.Context())
	userID := requestPrincipal.UserID
	if err != nil || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	user, found, err := h.store.UserByID(r.Context(), userID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	if !requireTenantSuperAdmin(w, user) {
		return 0, User{}, DashboardRequestScope{}, false
	}

	principalScope, err := DashboardRequestScopeFromContext(r.Context())
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	return userID, user, principalScope, true
}

func (h *RoleAdminHandler) authorize(ctx context.Context, r *http.Request, userID int, principalScope DashboardRequestScope) (AccessContext, error) {
	if h.authorizer == nil {
		return AccessContext{}, nil
	}
	corpID := 0
	if len(principalScope.CorpIDs) > 0 {
		corpID = principalScope.CorpIDs[0]
	}
	return h.authorizer.Resolve(ctx, userID, PermissionKeyFromRequest(r), corpID, principalScope.WorkEmployeeID)
}

func roleDataPermissionForCorp(raw string, corpID int) int {
	for _, setting := range parseRoleDataPermission(raw) {
		if setting.CorpID == corpID {
			return setting.PermissionType
		}
	}
	return 0
}

type roleDataPermissionOutput struct {
	CorpID         int `json:"corpId"`
	PermissionType int `json:"permissionType"`
}

func roleDataPermissionJSON(corpID int, dataPermission int) string {
	payload, _ := json.Marshal([]roleDataPermissionOutput{{CorpID: corpID, PermissionType: dataPermission}})
	return string(payload)
}

func mergeRoleDataPermissionJSON(raw string, corpID int, dataPermission int) string {
	settings := parseRoleDataPermission(raw)
	output := make([]roleDataPermissionOutput, 0, len(settings)+1)
	updated := false
	for _, setting := range settings {
		if setting.CorpID == corpID {
			setting.PermissionType = dataPermission
			updated = true
		}
		output = append(output, roleDataPermissionOutput{
			CorpID:         setting.CorpID,
			PermissionType: setting.PermissionType,
		})
	}
	if !updated {
		output = append(output, roleDataPermissionOutput{CorpID: corpID, PermissionType: dataPermission})
	}
	payload, _ := json.Marshal(output)
	return string(payload)
}

func rolePermissionTree(menus []RolePermissionMenu, parentID int, checked map[int]struct{}) []map[string]any {
	tree := make([]map[string]any, 0)
	for _, menu := range menus {
		if menu.ParentID != parentID {
			continue
		}
		children := rolePermissionTree(menus, menu.ID, checked)
		status := 1
		if len(children) == 0 {
			if _, ok := checked[menu.ID]; ok {
				status = 2
			}
		} else {
			total, unchecked, full := len(children), 0, 0
			for _, child := range children {
				switch child["checked"] {
				case 1:
					unchecked++
				case 2:
					full++
				}
			}
			if total == full {
				status = 2
			} else if total != unchecked {
				status = 3
			}
		}
		tree = append(tree, map[string]any{
			"id":         menu.ID,
			"parentId":   menu.ParentID,
			"name":       menu.Name,
			"level":      menu.Level,
			"isPageMenu": menu.IsPageMenu,
			"checked":    status,
			"children":   children,
		})
	}
	return tree
}

func queryIntRequired(r *http.Request, key string) (int, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return 0, strconv.ErrSyntax
	}
	return strconv.Atoi(raw)
}
