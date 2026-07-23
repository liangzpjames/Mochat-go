package dashboard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	workFissionOfficialAccountType = 7
	operationSessionCookieName     = "MOCHAT_SESSION_ID"
	defaultOperationSessionTTL     = 5 * time.Hour
)

type OfficialAccountOAuthInfo struct {
	ID                     int
	CorpID                 int
	ComponentAppID         string
	AuthorizerAppID        string
	ComponentSecret        string
	ComponentToken         string
	ComponentAESKey        string
	AuthorizationCode      string
	PreAuthCode            string
	AuthorizerRefreshToken string
}

type WorkFissionOAuthStore interface {
	WorkFissionOperationFissionByID(ctx context.Context, id int) (WorkFissionOperationFission, bool, error)
	OfficialAccountOAuthInfoByCorpIDType(ctx context.Context, corpID int, accountType int) (OfficialAccountOAuthInfo, bool, error)
	OfficialAccountOAuthInfoByAuthorizerAppID(ctx context.Context, authorizerAppID string) (OfficialAccountOAuthInfo, bool, error)
}

type OperationSessionStore interface {
	GetOperationSessionValue(ctx context.Context, sessionID string, key string) (map[string]any, bool, error)
	SetOperationSessionValue(ctx context.Context, sessionID string, key string, value map[string]any, ttl time.Duration) error
}

type OfficialAccountOAuthClient interface {
	OAuthURL(info OfficialAccountOAuthInfo, redirectURI string) (string, error)
	OAuthUser(ctx context.Context, info OfficialAccountOAuthInfo, code string) (map[string]any, error)
}

type WorkFissionOAuthHandler struct {
	store            WorkFissionOAuthStore
	session          OperationSessionStore
	oauthClient      OfficialAccountOAuthClient
	operationBaseURL string
	sessionTTL       time.Duration
}

func NewWorkFissionOAuthHandler(store WorkFissionOAuthStore, session OperationSessionStore, oauthClient OfficialAccountOAuthClient, operationBaseURL string) *WorkFissionOAuthHandler {
	return &WorkFissionOAuthHandler{
		store:            store,
		session:          session,
		oauthClient:      oauthClient,
		operationBaseURL: strings.TrimRight(operationBaseURL, "/"),
		sessionTTL:       defaultOperationSessionTTL,
	}
}

func (h *WorkFissionOAuthHandler) Auth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	params, err := operationRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	target := stringParam(params, "target")
	if target == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "target 必传", nil)
		return
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

	info, found := h.officialAccountInfoForFission(w, r, params, "公众号配置错误或不存在")
	if !found {
		return
	}
	if h.hasAuthorizedUser(w, r, sessionID, info) {
		http.Redirect(w, r, h.normalizeTarget(target), http.StatusFound)
		return
	}

	redirectURI := h.operationBaseURL + "/auth/workFission?target=" + url.QueryEscape(h.normalizeTarget(target))
	oauthURL, err := h.oauthClient.OAuthURL(info, redirectURI)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	http.Redirect(w, r, oauthURL, http.StatusFound)
}

func (h *WorkFissionOAuthHandler) OpenUserInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	params := valuesToParams(r.URL.Query())
	info, found := h.officialAccountInfoForFission(w, r, params, "未找到授权公众号")
	if !found {
		return
	}
	sessionID := operationSessionID(r)
	if sessionID == "" {
		writeEnvelope(w, http.StatusOK, 200, "success", []any{})
		return
	}
	for _, key := range operationWechatUserKeys(info) {
		value, found, err := h.session.GetOperationSessionValue(r.Context(), sessionID, key)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if found {
			writeEnvelope(w, http.StatusOK, 200, "success", operationSessionRawUser(value))
			return
		}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *WorkFissionOAuthHandler) authCallback(w http.ResponseWriter, r *http.Request, params map[string]any, sessionID string, code string, target string) {
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

func (h *WorkFissionOAuthHandler) officialAccountInfoForCallback(w http.ResponseWriter, r *http.Request, params map[string]any) (OfficialAccountOAuthInfo, bool) {
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
	return h.officialAccountInfoForFission(w, r, params, "公众号配置错误或不存在")
}

func (h *WorkFissionOAuthHandler) officialAccountInfoForFission(w http.ResponseWriter, r *http.Request, params map[string]any, missingMessage string) (OfficialAccountOAuthInfo, bool) {
	fissionID, exists, err := intParam(params, "id")
	if err != nil || !exists || fissionID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "数据不存在", nil)
		return OfficialAccountOAuthInfo{}, false
	}
	fission, found, err := h.store.WorkFissionOperationFissionByID(r.Context(), fissionID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return OfficialAccountOAuthInfo{}, false
	}
	if !found || fission.CorpID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "数据不存在", nil)
		return OfficialAccountOAuthInfo{}, false
	}
	info, found, err := h.store.OfficialAccountOAuthInfoByCorpIDType(r.Context(), fission.CorpID, workFissionOfficialAccountType)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return OfficialAccountOAuthInfo{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, missingMessage, nil)
		return OfficialAccountOAuthInfo{}, false
	}
	return info, true
}

func (h *WorkFissionOAuthHandler) hasAuthorizedUser(w http.ResponseWriter, r *http.Request, sessionID string, info OfficialAccountOAuthInfo) bool {
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

func (h *WorkFissionOAuthHandler) ensureSessionID(w http.ResponseWriter, r *http.Request) (string, bool) {
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

func (h *WorkFissionOAuthHandler) normalizeTarget(target string) string {
	if strings.Contains(target, "http") {
		return target
	}
	if h.operationBaseURL == "" {
		return target
	}
	return h.operationBaseURL + target
}

func operationRequestParams(r *http.Request) (map[string]any, error) {
	params := valuesToParams(r.URL.Query())
	if r.Method != http.MethodPost {
		return params, nil
	}
	bodyParams, err := parseRequestParams(r)
	if err != nil {
		return nil, err
	}
	for key, value := range bodyParams {
		params[key] = value
	}
	return params, nil
}

func operationSessionID(r *http.Request) string {
	cookie, err := r.Cookie(operationSessionCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func newOperationSessionID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成会话失败: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func operationWechatUserKeys(info OfficialAccountOAuthInfo) []string {
	keys := make([]string, 0, 2)
	if strings.TrimSpace(info.AuthorizerAppID) != "" {
		keys = append(keys, "wechat_user_"+strings.TrimSpace(info.AuthorizerAppID))
	}
	if info.ID > 0 {
		keys = append(keys, "wechat_user_"+strconv.Itoa(info.ID))
	}
	return keys
}

func operationSessionRawUser(value map[string]any) any {
	if raw, ok := value["raw"]; ok {
		return raw
	}
	return value
}
