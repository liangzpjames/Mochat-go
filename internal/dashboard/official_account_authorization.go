package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type OfficialAccountAuthorization struct {
	ComponentAppID         string
	ComponentSecret        string
	ComponentToken         string
	ComponentAESKey        string
	AuthorizedStatus       int
	AuthorizerAppID        string
	AuthorizerRefreshToken string
	AuthorizationCode      string
	PreAuthCode            string
	FuncInfo               string
	CorpID                 int
	CreateTime             int64
}

type OfficialAccountAuthorizerProfile struct {
	Nickname        string
	HeadImg         string
	Avatar          string
	ServiceTypeInfo int
	VerifyTypeInfo  int
	UserName        string
	PrincipalName   string
	Alias           string
	BusinessInfo    string
	QRCodeURL       string
	LocalQRCodeURL  string
}

type OfficialAccountAuthorizationStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	UpsertOfficialAccountAuthorization(ctx context.Context, values OfficialAccountAuthorization) (int, error)
	UpdateOfficialAccountAuthorizerProfile(ctx context.Context, id int, values OfficialAccountAuthorizerProfile) error
}

type OfficialAccountOpenPlatformClient interface {
	PreAuthorizationURL(ctx context.Context, redirectURI string) (string, error)
	QueryAuthorization(ctx context.Context, authCode string) (OfficialAccountAuthorization, error)
	AuthorizerInfo(ctx context.Context, authorizerAppID string) (OfficialAccountAuthorizerProfile, error)
}

type OfficialAccountAuthorizationHandler struct {
	store            OfficialAccountAuthorizationStore
	cache            LoginCache
	resolver         UserIDResolver
	authorizer       CorpAdminAuthorizer
	client           OfficialAccountOpenPlatformClient
	dashboardBaseURL string
	componentAppID   string
	componentSecret  string
	componentToken   string
	componentAESKey  string
}

func NewOfficialAccountAuthorizationHandler(store OfficialAccountAuthorizationStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, client OfficialAccountOpenPlatformClient, dashboardBaseURL string, componentAppID string, componentSecret string, componentToken string, componentAESKey string) *OfficialAccountAuthorizationHandler {
	return &OfficialAccountAuthorizationHandler{
		store:            store,
		cache:            cache,
		resolver:         resolver,
		authorizer:       authorizer,
		client:           client,
		dashboardBaseURL: strings.TrimRight(dashboardBaseURL, "/"),
		componentAppID:   strings.TrimSpace(componentAppID),
		componentSecret:  strings.TrimSpace(componentSecret),
		componentToken:   strings.TrimSpace(componentToken),
		componentAESKey:  strings.TrimSpace(componentAESKey),
	}
}

func (h *OfficialAccountAuthorizationHandler) GetPreAuthURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	if len(loginInfo.CorpIDs) != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "未选择登录企业，不可操作", nil)
		return
	}
	if h.componentAppID == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请检查开放平台参数是否正确配置!", nil)
		return
	}
	redirectURI := h.dashboardBaseURL + "/authRedirect?corp_id=" + strconv.Itoa(loginInfo.CorpIDs[0])
	authURL, err := h.client.PreAuthorizationURL(r.Context(), redirectURI)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"url": authURL})
}

func (h *OfficialAccountAuthorizationHandler) AuthRedirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	params, err := operationRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	authCode := stringParam(params, "auth_code")
	if authCode == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "auth_code 必传", nil)
		return
	}
	corpID, exists, err := intParam(params, "corp_id")
	if err != nil || !exists || corpID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "corp_id 必传", nil)
		return
	}
	authorization, err := h.client.QueryAuthorization(r.Context(), authCode)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	authorization.ComponentAppID = firstNonEmpty(authorization.ComponentAppID, h.componentAppID)
	authorization.ComponentSecret = firstNonEmpty(authorization.ComponentSecret, h.componentSecret)
	authorization.ComponentToken = h.componentToken
	authorization.ComponentAESKey = h.componentAESKey
	authorization.CorpID = corpID
	if authorization.AuthorizedStatus == 0 {
		authorization.AuthorizedStatus = 1
	}
	id, err := h.store.UpsertOfficialAccountAuthorization(r.Context(), authorization)
	if err != nil {
		if writeSaaSQuotaError(w, err) {
			return
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if id > 0 && authorization.AuthorizerAppID != "" {
		profile, err := h.client.AuthorizerInfo(r.Context(), authorization.AuthorizerAppID)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return
		}
		if err := h.store.UpdateOfficialAccountAuthorizerProfile(r.Context(), id, profile); err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
	}
	http.Redirect(w, r, h.dashboardBaseURL+"/officialAccount/index", http.StatusFound)
}

func (h *OfficialAccountAuthorizationHandler) resolveAuthorized(w http.ResponseWriter, r *http.Request) (int, User, LoginCorpInfo, bool) {
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

func (h *OfficialAccountAuthorizationHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, LoginCorpInfo, bool) {
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

func officialAccountJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(raw)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
