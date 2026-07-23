package dashboard

import (
	"context"
	"net/http"
)

type MenuOption struct {
	ID             int
	Name           string
	Level          int
	ParentID       int
	DataPermission int
	Icon           string
}

type MenuReadStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	MenuOptions(ctx context.Context) ([]MenuOption, error)
	MenuIcons(ctx context.Context) ([]string, error)
}

type MenuReadHandler struct {
	store    MenuReadStore
	resolver UserIDResolver
}

func NewMenuReadHandler(store MenuReadStore, resolver UserIDResolver) *MenuReadHandler {
	return &MenuReadHandler{store: store, resolver: resolver}
}

func (h *MenuReadHandler) IconIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if !h.ensureUser(w, r) {
		return
	}

	icons, err := h.store.MenuIcons(r.Context())
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", icons)
}

func (h *MenuReadHandler) Select(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if !h.ensureUser(w, r) {
		return
	}

	menus, err := h.store.MenuOptions(r.Context())
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", menuOptionsTree(menus, 0))
}

func (h *MenuReadHandler) ensureUser(w http.ResponseWriter, r *http.Request) bool {
	userID, err := h.resolver.UserID(r)
	if err != nil || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return false
	}
	if _, found, err := h.store.UserByID(r.Context(), userID); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return false
	} else if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
		return false
	}
	return true
}

func menuOptionsTree(menus []MenuOption, parentID int) []map[string]any {
	tree := make([]map[string]any, 0)
	for _, menu := range menus {
		if menu.ParentID != parentID {
			continue
		}
		tree = append(tree, map[string]any{
			"id":             menu.ID,
			"menuId":         menu.ID,
			"name":           menu.Name,
			"level":          menu.Level,
			"parentId":       menu.ParentID,
			"dataPermission": menu.DataPermission,
			"children":       menuOptionsTree(menus, menu.ID),
		})
	}
	return tree
}
