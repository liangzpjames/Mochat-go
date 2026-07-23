package dashboard

import (
	"context"
	"net/http"
	"strings"
)

type OfficialAccountItem struct {
	ID       int
	Nickname string
	Avatar   string
}

type OfficialAccountSetItem struct {
	ID                int
	OfficialAccountID int
	Type              int
	CorpID            int
}

type OfficialAccountStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	OfficialAccountsByCorpID(ctx context.Context, corpID int) ([]OfficialAccountItem, error)
	OfficialAccountFirstByCorpID(ctx context.Context, corpID int) (OfficialAccountItem, bool, error)
	OfficialAccountByID(ctx context.Context, id int) (OfficialAccountItem, bool, error)
	OfficialAccountSetByCorpIDType(ctx context.Context, corpID int, accountType int) (OfficialAccountSetItem, bool, error)
	UpsertOfficialAccountSet(ctx context.Context, corpID int, accountType int, officialAccountID int, userID int) error
}

type OfficialAccountHandler struct {
	store      OfficialAccountStore
	cache      LoginCache
	resolver   UserIDResolver
	authorizer CorpAdminAuthorizer
	apiBaseURL string
}

func NewOfficialAccountHandler(store OfficialAccountStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, apiBaseURL string) *OfficialAccountHandler {
	return &OfficialAccountHandler{
		store:      store,
		cache:      cache,
		resolver:   resolver,
		authorizer: authorizer,
		apiBaseURL: strings.TrimRight(apiBaseURL, "/"),
	}
}

func (h *OfficialAccountHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	accountType, ok := optionalQueryInt(w, r, "type", "type 必须为整数")
	if !ok {
		return
	}
	if accountType == nil || *accountType <= 0 {
		items, err := h.store.OfficialAccountsByCorpID(r.Context(), corpID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		payload := make([]map[string]any, 0, len(items))
		for _, item := range items {
			payload = append(payload, h.accountPayload(item))
		}
		writeEnvelope(w, http.StatusOK, 200, "success", payload)
		return
	}
	set, found, err := h.store.OfficialAccountSetByCorpIDType(r.Context(), corpID, *accountType)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if found {
		item, found, err := h.store.OfficialAccountByID(r.Context(), set.OfficialAccountID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if !found {
			writeEnvelope(w, http.StatusOK, 200, "success", []any{})
			return
		}
		writeEnvelope(w, http.StatusOK, 200, "success", h.accountPayload(item))
		return
	}
	item, found, err := h.store.OfficialAccountFirstByCorpID(r.Context(), corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusOK, 200, "success", []any{})
		return
	}
	if err := h.store.UpsertOfficialAccountSet(r.Context(), corpID, *accountType, item.ID, userID); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", h.accountPayload(item))
}

func (h *OfficialAccountHandler) Set(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	accountType, ok := queryPositiveIntParam(w, r, "type", "type 必传", "type 必须为整数")
	if !ok {
		return
	}
	officialAccountID, ok := queryPositiveIntParam(w, r, "official_account_id", "official_account_id 必传", "official_account_id 必须为整数")
	if !ok {
		return
	}
	if err := h.store.UpsertOfficialAccountSet(r.Context(), corpID, accountType, officialAccountID, userID); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *OfficialAccountHandler) resolveAuthorized(w http.ResponseWriter, r *http.Request) (int, User, LoginCorpInfo, bool) {
	userID, user, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return 0, User{}, LoginCorpInfo{}, false
	}
	if h.authorizer != nil {
		corpID := 0
		if len(loginInfo.CorpIDs) > 0 {
			corpID = loginInfo.CorpIDs[0]
		}
		if _, err := h.authorizer.Resolve(r.Context(), userID, PermissionKeyFromRequest(r), corpID, loginInfo.WorkEmployeeID); err != nil {
			writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, err.Error(), nil)
			return 0, User{}, LoginCorpInfo{}, false
		}
	}
	return userID, user, loginInfo, true
}

func (h *OfficialAccountHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, LoginCorpInfo, bool) {
	userID, err := h.resolver.UserID(r)
	if err != nil || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return 0, User{}, LoginCorpInfo{}, false
	}
	user, found, err := h.store.UserByID(r.Context(), userID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, LoginCorpInfo{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
		return 0, User{}, LoginCorpInfo{}, false
	}
	cacheValue := ""
	if h.cache != nil {
		cacheValue, err = h.cache.UserCorpCache(r.Context(), userID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return 0, User{}, LoginCorpInfo{}, false
		}
	}
	loginInfo, err := ResolveValidatedLoginCorpInfoFromStore(r.Context(), r.Header, user, cacheValue, h.store)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, LoginCorpInfo{}, false
	}
	return userID, user, loginInfo, true
}

func (h *OfficialAccountHandler) accountPayload(item OfficialAccountItem) map[string]any {
	return map[string]any{
		"id":       item.ID,
		"nickname": item.Nickname,
		"avatar":   h.fullStaticURL(item.Avatar),
	}
}

func (h *OfficialAccountHandler) fullStaticURL(path string) string {
	if path == "" || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") || h.apiBaseURL == "" {
		return path
	}
	return h.apiBaseURL + "/static/" + strings.TrimLeft(path, "/")
}
