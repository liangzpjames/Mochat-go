package dashboard

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type OperationLoadOAuthStore interface {
	OfficialAccountOAuthInfoByID(ctx context.Context, id int) (OfficialAccountOAuthInfo, bool, error)
	OfficialAccountOAuthInfoByCorpID(ctx context.Context, corpID int) (OfficialAccountOAuthInfo, bool, error)
	OfficialAccountOAuthInfoByAuthorizerAppID(ctx context.Context, authorizerAppID string) (OfficialAccountOAuthInfo, bool, error)
}

type OperationLoadOAuthHandler struct {
	store            OperationLoadOAuthStore
	session          OperationSessionStore
	oauthClient      OfficialAccountOAuthClient
	operationBaseURL string
	sessionTTL       time.Duration
}

func NewOperationLoadOAuthHandler(store OperationLoadOAuthStore, session OperationSessionStore, oauthClient OfficialAccountOAuthClient, operationBaseURL string) *OperationLoadOAuthHandler {
	return &OperationLoadOAuthHandler{
		store:            store,
		session:          session,
		oauthClient:      oauthClient,
		operationBaseURL: strings.TrimRight(operationBaseURL, "/"),
		sessionTTL:       defaultOperationSessionTTL,
	}
}

func (h *OperationLoadOAuthHandler) Load(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	params, err := operationRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	if pathTarget := strings.TrimPrefix(r.URL.Path, "/load/"); pathTarget != r.URL.Path && pathTarget != "" {
		if stringParam(params, "target") == "" {
			params["target"] = "/" + pathTarget
		}
	}
	target := stringParam(params, "target")
	if target == "" {
		target = h.operationBaseURL
	}
	sessionID, ok := h.ensureSessionID(w, r)
	if !ok {
		return
	}
	code := stringParam(params, "code")
	if code != "" {
		h.authCallback(w, r, params, sessionID, code, target)
		return
	}
	info, found := h.officialAccountInfo(w, r, params)
	if !found {
		return
	}
	if h.hasAuthorizedUser(w, r, sessionID, info) {
		http.Redirect(w, r, h.normalizeTarget(target), http.StatusFound)
		return
	}
	redirectURI := h.operationBaseURL + "/load?target=" + url.QueryEscape(h.normalizeTarget(target))
	if info.ID > 0 {
		redirectURI += "&official_account_id=" + strconv.Itoa(info.ID)
	} else if info.CorpID > 0 {
		redirectURI += "&corp_id=" + strconv.Itoa(info.CorpID)
	}
	oauthURL, err := h.oauthClient.OAuthURL(info, redirectURI)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	http.Redirect(w, r, oauthURL, http.StatusFound)
}

func (h *OperationLoadOAuthHandler) authCallback(w http.ResponseWriter, r *http.Request, params map[string]any, sessionID string, code string, target string) {
	info, found := h.officialAccountInfoForCallback(w, r, params)
	if !found {
		return
	}
	rawUser, err := h.oauthClient.OAuthUser(r.Context(), info, code)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	sessionValue := map[string]any{"raw": rawUser}
	for _, key := range operationWechatUserKeys(info) {
		if err := h.session.SetOperationSessionValue(r.Context(), sessionID, key, sessionValue, h.sessionTTL); err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
	}
	http.Redirect(w, r, h.normalizeTarget(target), http.StatusFound)
}

func (h *OperationLoadOAuthHandler) officialAccountInfoForCallback(w http.ResponseWriter, r *http.Request, params map[string]any) (OfficialAccountOAuthInfo, bool) {
	appID := stringParam(params, "appid")
	if appID != "" {
		info, found, err := h.store.OfficialAccountOAuthInfoByAuthorizerAppID(r.Context(), appID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return OfficialAccountOAuthInfo{}, false
		}
		if found {
			return info, true
		}
	}
	return h.officialAccountInfo(w, r, params)
}

func (h *OperationLoadOAuthHandler) officialAccountInfo(w http.ResponseWriter, r *http.Request, params map[string]any) (OfficialAccountOAuthInfo, bool) {
	if officialAccountID, exists, err := intParam(params, "official_account_id"); err == nil && exists && officialAccountID > 0 {
		info, found, err := h.store.OfficialAccountOAuthInfoByID(r.Context(), officialAccountID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return OfficialAccountOAuthInfo{}, false
		}
		if found {
			return info, true
		}
	}
	if corpID, exists, err := intParam(params, "corp_id"); err == nil && exists && corpID > 0 {
		info, found, err := h.store.OfficialAccountOAuthInfoByCorpID(r.Context(), corpID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return OfficialAccountOAuthInfo{}, false
		}
		if found {
			return info, true
		}
	}
	if id, exists, err := intParam(params, "id"); err == nil && exists && id > 0 {
		info, found, err := h.store.OfficialAccountOAuthInfoByCorpID(r.Context(), id)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return OfficialAccountOAuthInfo{}, false
		}
		if found {
			return info, true
		}
	}
	writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "公众号配置错误或不存在", nil)
	return OfficialAccountOAuthInfo{}, false
}

func (h *OperationLoadOAuthHandler) hasAuthorizedUser(w http.ResponseWriter, r *http.Request, sessionID string, info OfficialAccountOAuthInfo) bool {
	for _, key := range operationWechatUserKeys(info) {
		_, found, err := h.session.GetOperationSessionValue(r.Context(), sessionID, key)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return false
		}
		if found {
			return true
		}
	}
	return false
}

func (h *OperationLoadOAuthHandler) ensureSessionID(w http.ResponseWriter, r *http.Request) (string, bool) {
	if sessionID := operationSessionID(r); sessionID != "" {
		return sessionID, true
	}
	sessionID, err := newOperationSessionID()
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return "", false
	}
	http.SetCookie(w, &http.Cookie{
		Name:     operationSessionCookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   int(h.sessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return sessionID, true
}

func (h *OperationLoadOAuthHandler) normalizeTarget(target string) string {
	if strings.Contains(target, "http") {
		return target
	}
	if h.operationBaseURL == "" {
		return target
	}
	return h.operationBaseURL + target
}
