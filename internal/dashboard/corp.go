package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type CorpSelectStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	CorpIDsByTenant(ctx context.Context, tenantID int) ([]int, error)
	CorpIDsByUser(ctx context.Context, userID int) ([]int, error)
	CorpsByIDsName(ctx context.Context, corpIDs []int, name string) ([]Corp, error)
}

type CorpBindStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	CorpIDsByTenant(ctx context.Context, tenantID int) ([]int, error)
	CorpIDsByUser(ctx context.Context, userID int) ([]int, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
}

type UserCorpCacheWriter interface {
	SetUserCorpCache(ctx context.Context, userID int, value string) error
}

type CorpSelectHandler struct {
	store    CorpSelectStore
	resolver UserIDResolver
}

type CorpBindHandler struct {
	store    CorpBindStore
	cache    UserCorpCacheWriter
	resolver UserIDResolver
}

func NewCorpSelectHandler(store CorpSelectStore, resolver UserIDResolver) *CorpSelectHandler {
	return &CorpSelectHandler{store: store, resolver: resolver}
}

func NewCorpBindHandler(store CorpBindStore, cache UserCorpCacheWriter, resolver UserIDResolver) *CorpBindHandler {
	return &CorpBindHandler{store: store, cache: cache, resolver: resolver}
}

func (h *CorpSelectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	var corpIDs []int
	if user.IsSuperAdmin == 1 {
		corpIDs, err = h.store.CorpIDsByTenant(r.Context(), user.TenantID)
	} else {
		corpIDs, err = h.store.CorpIDsByUser(r.Context(), user.ID)
	}
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	corps, err := h.store.CorpsByIDsName(r.Context(), corpIDs, r.URL.Query().Get("corpName"))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	data := make([]map[string]any, 0, len(corps))
	for _, corp := range corps {
		data = append(data, map[string]any{
			"corpId":   corp.ID,
			"corpName": corp.Name,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", data)
}

func (h *CorpBindHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}

	userID, err := h.resolver.UserID(r)
	if err != nil || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	corpID, err := parseCorpBindRequest(r)
	if err != nil || corpID < 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "企业授信ID 必填", nil)
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

	employeeID := 0
	var corpIDs []int
	if user.IsSuperAdmin == 1 {
		corpIDs, err = h.store.CorpIDsByTenant(r.Context(), user.TenantID)
	} else {
		corpIDs, err = h.store.CorpIDsByUser(r.Context(), user.ID)
	}
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if corpID > 0 && !containsInt(corpIDs, corpID) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "当前用户不归属该企业，不可操作", nil)
		return
	}

	if user.IsSuperAdmin != 1 && corpID > 0 {
		employeeID, err = h.store.EmployeeIDByUserCorp(r.Context(), user.ID, corpID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
	}

	value := strconv.Itoa(corpID) + "-" + strconv.Itoa(employeeID)
	if err := h.cache.SetUserCorpCache(r.Context(), user.ID, value); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func parseCorpBindRequest(r *http.Request) (int, error) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return 0, err
		}
		return strconv.Atoi(r.FormValue("corpId"))
	}

	var raw map[string]any
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		if err := r.ParseForm(); err != nil {
			return 0, err
		}
		return strconv.Atoi(r.FormValue("corpId"))
	}

	switch value := raw["corpId"].(type) {
	case json.Number:
		n, err := value.Int64()
		return int(n), err
	case float64:
		return int(value), nil
	case string:
		return strconv.Atoi(value)
	default:
		return 0, strconv.ErrSyntax
	}
}

func containsInt(values []int, needle int) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
