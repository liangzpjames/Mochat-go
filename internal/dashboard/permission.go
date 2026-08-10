package dashboard

import (
	"context"
	"net/http"
	"strconv"
	"strings"
)

type Menu struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Level          int    `json:"level"`
	DataPermission int    `json:"dataPermission"`
	Icon           string `json:"icon"`
	LinkType       int    `json:"linkType"`
	LinkURL        string `json:"linkUrl"`
	ParentID       int    `json:"parentId"`
	IsPageMenu     int    `json:"isPageMenu"`
	MenuID         int    `json:"menuId"`
	Children       []Menu `json:"children"`
	Sort           int    `json:"-"`
}

type PermissionStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	RoleIDByUserTenant(ctx context.Context, userID int, tenantID int) (int, bool, error)
	MenuIDsByRole(ctx context.Context, roleID int) ([]int, error)
	PageMenus(ctx context.Context) ([]Menu, error)
	MenusByIDs(ctx context.Context, menuIDs []int) ([]Menu, error)
}

type PermissionByUserHandler struct {
	store    PermissionStore
	resolver UserIDResolver
}

func NewPermissionByUserHandler(store PermissionStore, resolver UserIDResolver) *PermissionByUserHandler {
	return &PermissionByUserHandler{store: store, resolver: resolver}
}

func (h *PermissionByUserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}

	userID, err := h.resolver.UserID(r)
	if err != nil || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	user, ok, err := h.store.UserByID(r.Context(), userID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !ok {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
		return
	}
	if !requireTenantSuperAdmin(w, user) {
		return
	}

	menus, err := h.store.PageMenus(r.Context())
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	menus = normalizePageMenus(menus)
	writeEnvelope(w, http.StatusOK, 200, "success", buildMenuTree(menus, 0))
}

func normalizePageMenus(menus []Menu) []Menu {
	out := make([]Menu, 0, len(menus))
	for _, menu := range menus {
		if menu.IsPageMenu != 1 && menu.IsPageMenu != 2 {
			continue
		}
		menu.LinkURL = strings.ReplaceAll(menu.LinkURL, "/dashboard", "")
		menu.MenuID = menu.ID
		menu.Children = []Menu{}
		out = append(out, menu)
	}
	return addAutoTagRouteRegistrationChildren(out)
}

func buildMenuTree(menus []Menu, parentID int) []Menu {
	tree := make([]Menu, 0)
	for _, menu := range menus {
		if menu.ParentID != parentID {
			continue
		}
		menu.Children = buildMenuTree(menus, menu.ID)
		tree = append(tree, menu)
	}
	return tree
}

var autoTagHiddenIndexRoutes = map[string]struct{}{
	"/autoTag/keywordIndex":  {},
	"/autoTag/joinRoomIndex": {},
	"/autoTag/dayPartIndex":  {},
}

func addAutoTagRouteRegistrationChildren(menus []Menu) []Menu {
	out := make([]Menu, 0, len(menus)+len(autoTagHiddenIndexRoutes))
	existing := make(map[string]struct{}, len(menus))
	for _, menu := range menus {
		existing[menuChildKey(menu.ParentID, menu.LinkURL)] = struct{}{}
	}

	nextSyntheticID := -100000
	for _, menu := range menus {
		out = append(out, menu)
		if _, ok := autoTagHiddenIndexRoutes[menu.LinkURL]; !ok {
			continue
		}
		key := menuChildKey(menu.ID, menu.LinkURL)
		if _, exists := existing[key]; exists {
			continue
		}

		child := menu
		child.ID = nextSyntheticID
		child.MenuID = nextSyntheticID
		child.ParentID = menu.ID
		child.Level = menu.Level + 1
		child.Children = []Menu{}
		out = append(out, child)
		existing[key] = struct{}{}
		nextSyntheticID--
	}
	return out
}

func menuChildKey(parentID int, linkURL string) string {
	return strconv.Itoa(parentID) + "\x00" + strings.TrimSpace(linkURL)
}
