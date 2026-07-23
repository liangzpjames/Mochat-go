package dashboard

import (
	"context"
	"net/http"
)

type RoleOption struct {
	ID   int
	Name string
}

type RoleSelectStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	RolesByTenantID(ctx context.Context, tenantID int) ([]RoleOption, error)
}

type RoleSelectHandler struct {
	store    RoleSelectStore
	resolver UserIDResolver
}

func NewRoleSelectHandler(store RoleSelectStore, resolver UserIDResolver) *RoleSelectHandler {
	return &RoleSelectHandler{store: store, resolver: resolver}
}

func (h *RoleSelectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}

	userID, err := h.resolver.UserID(r)
	if err != nil || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	user, found, err := h.store.UserByID(r.Context(), userID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
		return
	}

	roles, err := h.store.RolesByTenantID(r.Context(), user.TenantID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	data := make([]map[string]any, 0, len(roles))
	for _, role := range roles {
		data = append(data, map[string]any{
			"roleId": role.ID,
			"name":   role.Name,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", data)
}
