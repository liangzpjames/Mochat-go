package dashboard

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type MenuListFilter struct {
	Name string
}

type MenuListItem struct {
	ID          int
	Name        string
	Level       int
	ParentID    int
	Icon        string
	Status      int
	OperateName string
	UpdatedAt   string
}

type MenuDetail struct {
	ID             int
	Name           string
	Level          int
	Status         int
	Icon           string
	LinkURL        string
	IsPageMenu     int
	LinkType       int
	DataPermission int
	Path           string
}

type MenuCreateValues struct {
	Name           string
	Level          int
	ParentID       int
	Path           string
	Icon           string
	LinkURL        string
	LinkType       int
	IsPageMenu     int
	DataPermission int
	OperateID      int
	OperateName    string
}

type MenuUpdateValues struct {
	Name                 string
	Icon                 string
	LinkURL              string
	LinkType             int
	IsPageMenu           int
	DataPermission       int
	UpdateIcon           bool
	UpdateLinkFields     bool
	UpdateDataPermission bool
}

type MenuAdminStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	MenuList(ctx context.Context, filter MenuListFilter) ([]MenuListItem, error)
	MenuDetailByID(ctx context.Context, menuID int) (MenuDetail, bool, error)
	MenuLinkURLExists(ctx context.Context, linkURL string, excludeMenuID int) (bool, error)
	CreateMenu(ctx context.Context, values MenuCreateValues) (int, error)
	UpdateMenu(ctx context.Context, menuID int, values MenuUpdateValues) (bool, error)
	UpdateMenuStatus(ctx context.Context, menuID int, status int, cascade bool) (bool, error)
	DeleteMenuCascade(ctx context.Context, menuID int) (bool, error)
}

type MenuAdminHandler struct {
	store      MenuAdminStore
	cache      LoginCache
	resolver   UserIDResolver
	authorizer CorpAdminAuthorizer
}

func NewMenuAdminHandler(store MenuAdminStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer) *MenuAdminHandler {
	return &MenuAdminHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer}
}

func (h *MenuAdminHandler) Index(w http.ResponseWriter, r *http.Request) {
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

	page := positiveQueryInt(r, "page", 1)
	perPage := positiveQueryInt(r, "perPage", 10)
	items, err := h.store.MenuList(r.Context(), MenuListFilter{Name: strings.TrimSpace(r.URL.Query().Get("name"))})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if len(items) == 0 {
		writeEnvelope(w, http.StatusOK, 200, "success", emptyMenuListPage(perPage))
		return
	}

	tree := menuListTree(items, 0, "")
	total := len(tree)
	totalPage := 0
	if total > 0 {
		totalPage = (total + perPage - 1) / perPage
	}
	offset := (page - 1) * perPage
	list := []map[string]any{}
	if offset < total {
		end := offset + perPage
		if end > total {
			end = total
		}
		list = tree[offset:end]
	}

	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{
			"perPage":   perPage,
			"total":     total,
			"totalPage": totalPage,
		},
		"list": list,
	})
}

func (h *MenuAdminHandler) Show(w http.ResponseWriter, r *http.Request) {
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

	menuID, err := positiveQueryIntRequired(r, "menuId")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "菜单id 必填", nil)
		return
	}

	menu, found, err := h.store.MenuDetailByID(r.Context(), menuID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "菜单不存在", nil)
		return
	}

	writeEnvelope(w, http.StatusOK, 200, "success", menuDetailPayload(menu))
}

func (h *MenuAdminHandler) Store(w http.ResponseWriter, r *http.Request) {
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

	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	values, ok := parseMenuCreateValues(w, params, user)
	if !ok {
		return
	}
	exists, err := h.store.MenuLinkURLExists(r.Context(), values.LinkURL, 0)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if exists {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, values.LinkURL+"-该LinkUrl已存在，不能重复", nil)
		return
	}
	if _, err := h.store.CreateMenu(r.Context(), values); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *MenuAdminHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
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
	menuID, okInt, err := intParam(params, "menuId")
	if err != nil || !okInt || menuID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "菜单id 必填", nil)
		return
	}
	menu, found, err := h.store.MenuDetailByID(r.Context(), menuID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "菜单不存在", nil)
		return
	}
	values, ok := parseMenuUpdateValues(w, params, menu)
	if !ok {
		return
	}
	if values.UpdateLinkFields {
		exists, err := h.store.MenuLinkURLExists(r.Context(), values.LinkURL, menuID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if exists {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, values.LinkURL+"-该LinkUrl已存在，不能重复", nil)
			return
		}
	}
	updated, err := h.store.UpdateMenu(r.Context(), menuID, values)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !updated {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "修改失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *MenuAdminHandler) StatusUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
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
	menuID, okInt, err := intParam(params, "menuId")
	if err != nil || !okInt || menuID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "菜单id 必填", nil)
		return
	}
	status, okInt, err := intParam(params, "status")
	if err != nil || !okInt || (status != 1 && status != 2) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "状态 值必须在列表内：[1,2]", nil)
		return
	}
	updated, err := h.store.UpdateMenuStatus(r.Context(), menuID, status, status == 2)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !updated {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "修改失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *MenuAdminHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
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
	menuID, okInt, err := intParam(params, "menuId")
	if err != nil || !okInt || menuID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "菜单id 必填", nil)
		return
	}
	deleted, err := h.store.DeleteMenuCascade(r.Context(), menuID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !deleted {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "子菜单删除失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *MenuAdminHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
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

func (h *MenuAdminHandler) authorize(ctx context.Context, r *http.Request, userID int, principalScope DashboardRequestScope) (AccessContext, error) {
	if h.authorizer == nil {
		return AccessContext{}, nil
	}
	corpID := 0
	if len(principalScope.CorpIDs) > 0 {
		corpID = principalScope.CorpIDs[0]
	}
	return h.authorizer.Resolve(ctx, userID, PermissionKeyFromRequest(r), corpID, principalScope.WorkEmployeeID)
}

func parseMenuCreateValues(w http.ResponseWriter, params map[string]any, user User) (MenuCreateValues, bool) {
	name := stringParam(params, "name")
	if name == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "菜单名称 必填", nil)
		return MenuCreateValues{}, false
	}
	level, ok, err := intParam(params, "level")
	if err != nil || !ok || level <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "菜单级别 必填", nil)
		return MenuCreateValues{}, false
	}
	values := MenuCreateValues{
		Name:           name,
		Level:          level,
		IsPageMenu:     1,
		DataPermission: 2,
		OperateID:      user.ID,
		OperateName:    user.Name,
	}
	switch level {
	case 1:
		values.ParentID = 0
		values.Path = ""
	case 2:
		firstMenuID, ok := requiredMenuID(w, params, "firstMenuId", "一级菜单id不能为空")
		if !ok {
			return MenuCreateValues{}, false
		}
		values.ParentID = firstMenuID
		values.Path = menuPath(firstMenuID)
	case 3:
		firstMenuID, ok := requiredMenuID(w, params, "firstMenuId", "一级菜单id不能为空")
		if !ok {
			return MenuCreateValues{}, false
		}
		secondMenuID, ok := requiredMenuID(w, params, "secondMenuId", "二级菜单id不能为空")
		if !ok {
			return MenuCreateValues{}, false
		}
		values.ParentID = secondMenuID
		values.Path = menuPath(firstMenuID, secondMenuID)
	case 4:
		firstMenuID, ok := requiredMenuID(w, params, "firstMenuId", "一级菜单id不能为空")
		if !ok {
			return MenuCreateValues{}, false
		}
		secondMenuID, ok := requiredMenuID(w, params, "secondMenuId", "二级菜单id不能为空")
		if !ok {
			return MenuCreateValues{}, false
		}
		thirdMenuID, ok := requiredMenuID(w, params, "thirdMenuId", "三级菜单id不能为空")
		if !ok {
			return MenuCreateValues{}, false
		}
		values.ParentID = thirdMenuID
		values.Path = menuPath(firstMenuID, secondMenuID, thirdMenuID)
	case 5:
		firstMenuID, ok := requiredMenuID(w, params, "firstMenuId", "一级菜单id不能为空")
		if !ok {
			return MenuCreateValues{}, false
		}
		secondMenuID, ok := requiredMenuID(w, params, "secondMenuId", "二级菜单id不能为空")
		if !ok {
			return MenuCreateValues{}, false
		}
		thirdMenuID, ok := requiredMenuID(w, params, "thirdMenuId", "三级菜单id不能为空")
		if !ok {
			return MenuCreateValues{}, false
		}
		fourthMenuID, ok := requiredMenuID(w, params, "fourthMenuId", "四级菜单id不能为空")
		if !ok {
			return MenuCreateValues{}, false
		}
		values.ParentID = fourthMenuID
		values.Path = menuPath(firstMenuID, secondMenuID, thirdMenuID, fourthMenuID)
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "菜单级别 必须为整型", nil)
		return MenuCreateValues{}, false
	}

	if level == 1 || level == 2 {
		values.Icon = stringParam(params, "icon")
		values.LinkType = 1
		values.LinkURL = "path/" + strconv.FormatInt(time.Now().UnixNano(), 10)
		return values, true
	}

	linkType, ok, err := intParam(params, "linkType")
	if err != nil || !ok || linkType <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "链接类型 必填", nil)
		return MenuCreateValues{}, false
	}
	linkURL := stringParam(params, "linkUrl")
	if linkURL == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "链接地址 必填", nil)
		return MenuCreateValues{}, false
	}
	values.LinkType = linkType
	values.LinkURL = linkURL
	if dataPermission, ok, err := intParam(params, "dataPermission"); err == nil && ok {
		values.DataPermission = dataPermission
	}
	if level == 4 || level == 5 {
		isPageMenu, ok, err := intParam(params, "isPageMenu")
		if err != nil || !ok || isPageMenu <= 0 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请选择页面菜单", nil)
			return MenuCreateValues{}, false
		}
		values.IsPageMenu = isPageMenu
	}
	return values, true
}

func parseMenuUpdateValues(w http.ResponseWriter, params map[string]any, menu MenuDetail) (MenuUpdateValues, bool) {
	name := stringParam(params, "name")
	if name == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "菜单名称 必填", nil)
		return MenuUpdateValues{}, false
	}
	values := MenuUpdateValues{Name: name, IsPageMenu: 1}
	if menu.Level == 1 || menu.Level == 2 || menu.Level == 3 {
		values.Icon = stringParam(params, "icon")
		values.UpdateIcon = true
	}
	if menu.Level == 3 || menu.Level == 4 || menu.Level == 5 {
		linkType, ok, err := intParam(params, "linkType")
		if err != nil || !ok || linkType <= 0 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "链接类型 必填", nil)
			return MenuUpdateValues{}, false
		}
		linkURL := stringParam(params, "linkUrl")
		if linkURL == "" {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "链接地址 必填", nil)
			return MenuUpdateValues{}, false
		}
		values.LinkType = linkType
		values.LinkURL = linkURL
		values.UpdateLinkFields = true
		if isPageMenu, ok, err := intParam(params, "isPageMenu"); err == nil && ok {
			values.IsPageMenu = isPageMenu
		}
		if dataPermission, ok, err := intParam(params, "dataPermission"); err == nil && ok {
			values.DataPermission = dataPermission
			values.UpdateDataPermission = true
		}
	}
	return values, true
}

func requiredMenuID(w http.ResponseWriter, params map[string]any, key string, message string) (int, bool) {
	value, ok, err := intParam(params, key)
	if err != nil || !ok || value <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, message, nil)
		return 0, false
	}
	return value, true
}

func menuPath(ids ...int) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, "#"+strconv.Itoa(id)+"#")
	}
	return strings.Join(parts, "-")
}

func emptyMenuListPage(perPage int) map[string]any {
	return map[string]any{
		"page": map[string]any{
			"perPage":   perPage,
			"total":     "0",
			"totalPage": "0",
		},
		"list": []map[string]any{},
	}
}

func menuListTree(items []MenuListItem, parentID int, path string) []map[string]any {
	tree := make([]map[string]any, 0)
	pathKey := 1
	for _, item := range items {
		if item.ParentID != parentID {
			continue
		}

		menuPath := strconv.Itoa(pathKey)
		if item.ParentID != 0 {
			menuPath = path + "-" + menuPath
		}
		pathKey++

		tree = append(tree, map[string]any{
			"id":          item.ID,
			"name":        item.Name,
			"level":       item.Level,
			"parentId":    item.ParentID,
			"icon":        item.Icon,
			"status":      item.Status,
			"operateName": item.OperateName,
			"updatedAt":   item.UpdatedAt,
			"menuId":      item.ID,
			"menuPath":    menuPath,
			"levelName":   menuLevelName(item.Level),
			"children":    menuListTree(items, item.ID, menuPath),
		})
	}
	return tree
}

func menuDetailPayload(menu MenuDetail) map[string]any {
	data := map[string]any{
		"menuId":         menu.ID,
		"name":           menu.Name,
		"level":          menu.Level,
		"levelName":      menuLevelName(menu.Level),
		"status":         menu.Status,
		"icon":           menu.Icon,
		"linkUrl":        menu.LinkURL,
		"isPageMenu":     menu.IsPageMenu,
		"linkType":       emptyWhenZero(menu.LinkType),
		"dataPermission": emptyWhenZero(menu.DataPermission),
		"firstMenuId":    "",
		"secondMenuId":   "",
		"thirdMenuId":    "",
		"fourthMenuId":   "",
	}

	if menu.Path == "" {
		return data
	}
	path := strings.ReplaceAll(menu.Path, "#", "")
	if !strings.Contains(path, "-") {
		data["firstMenuId"] = path
		return data
	}
	parts := strings.Split(path, "-")
	if len(parts) > 0 {
		data["firstMenuId"] = atoiOrEmpty(parts[0])
	}
	if len(parts) > 1 {
		data["secondMenuId"] = atoiOrEmpty(parts[1])
	}
	if len(parts) > 2 {
		data["thirdMenuId"] = atoiOrEmpty(parts[2])
	}
	if len(parts) > 3 {
		data["fourthMenuId"] = atoiOrEmpty(parts[3])
	}
	return data
}

func menuLevelName(level int) string {
	switch level {
	case 1:
		return "一级菜单"
	case 2:
		return "二级菜单"
	case 3:
		return "三级菜单"
	case 4:
		return "四级菜单"
	case 5:
		return "五级菜单"
	default:
		return ""
	}
}

func emptyWhenZero(value int) any {
	if value == 0 {
		return ""
	}
	return value
}

func atoiOrEmpty(raw string) any {
	if raw == "" {
		return ""
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return raw
	}
	return value
}
