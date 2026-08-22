package dashboard

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/authjwt"
)

type SidebarAgentCredential struct {
	ID             int
	CorpID         int
	WXCorpID       string
	EmployeeSecret string
	ContactSecret  string
	WXAgentID      string
	WXSecret       string
}

type SidebarAgentEmployeeLogin struct {
	EmployeeID int
	CorpID     int
	UserID     int
	UserStatus int
}

type SidebarAgentStore interface {
	WorkAgentCredentialByID(ctx context.Context, agentID int) (SidebarAgentCredential, bool, error)
	SidebarAgentCorpCredentialByID(ctx context.Context, corpID int) (SidebarAgentCredential, bool, error)
	SidebarEmployeeByID(ctx context.Context, employeeID int) (SidebarEmployee, bool, error)
	SidebarEmployeeLoginByWXUserID(ctx context.Context, wxUserID string, corpID int) (SidebarAgentEmployeeLogin, bool, error)
}

type SidebarAgentWeComClient interface {
	OAuthURL(credential SidebarAgentCredential, redirectURI string) string
	OAuthUserID(ctx context.Context, credential SidebarAgentCredential, code string) (string, error)
	JSSDKConfig(ctx context.Context, credential SidebarAgentCredential, agent bool, uri string, jsAPIs []string) (map[string]any, error)
}

type SidebarAgentHandler struct {
	store          SidebarAgentStore
	wecom          SidebarAgentWeComClient
	resolver       UserIDResolver
	apiBaseURL     string
	sidebarBaseURL string
	simpleSecret   string
	sidebarSecret  string
	ttl            time.Duration
	now            func() time.Time
}

func NewSidebarAgentHandler(store SidebarAgentStore, wecom SidebarAgentWeComClient, resolver UserIDResolver, apiBaseURL string, sidebarBaseURL string, simpleSecret string, sidebarSecret string, ttl time.Duration) *SidebarAgentHandler {
	return &SidebarAgentHandler{
		store:          store,
		wecom:          wecom,
		resolver:       resolver,
		apiBaseURL:     strings.TrimRight(apiBaseURL, "/"),
		sidebarBaseURL: strings.TrimRight(sidebarBaseURL, "/"),
		simpleSecret:   simpleSecret,
		sidebarSecret:  sidebarSecret,
		ttl:            ttl,
	}
}

func (h *SidebarAgentHandler) Auth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	params, err := sidebarAgentParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	target := strings.TrimSpace(stringParam(params, "target"))
	if target == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "跳转地址不能为空", nil)
		return
	}
	agentID, ok, err := intParam(params, "agentId")
	if !ok || agentID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "应用Id不能为空", nil)
		return
	}
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "应用Id必须为整型", nil)
		return
	}
	targetURL := h.normalizeTarget(target)
	credential, found, err := h.store.WorkAgentCredentialByID(r.Context(), agentID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		http.Redirect(w, r, h.sidebarAuthRedirect(agentID, http.StatusBadRequest, "应用不存在", nil, targetURL), http.StatusFound)
		return
	}
	code := strings.TrimSpace(stringParam(params, "code"))
	if code == "" {
		redirectURI := h.apiBaseURL + "/sidebar/agent/auth?" + url.Values{
			"agentId": {strconv.Itoa(agentID)},
			"target":  {targetURL},
		}.Encode()
		http.Redirect(w, r, h.wecom.OAuthURL(credential, redirectURI), http.StatusFound)
		return
	}
	data, err := h.sidebarTokenByCode(r.Context(), credential, code, r, true)
	if err != nil {
		http.Redirect(w, r, h.sidebarAuthRedirect(agentID, http.StatusInternalServerError, err.Error(), nil, targetURL), http.StatusFound)
		return
	}
	http.Redirect(w, r, h.sidebarAuthRedirect(agentID, 200, "", data, targetURL), http.StatusFound)
}

func (h *SidebarAgentHandler) OAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	agentID, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("agentId")))
	if err != nil || agentID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "应用ID必须", nil)
		return
	}
	credential, found, err := h.store.WorkAgentCredentialByID(r.Context(), agentID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "应用不存在", nil)
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		callbackURL := h.apiBaseURL + "/sidebar/agent/oauth?" + url.Values{
			"act":          {r.URL.Query().Get("act")},
			"agentId":      {strconv.Itoa(agentID)},
			"isJsRedirect": {r.URL.Query().Get("isJsRedirect")},
		}.Encode()
		writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"url": h.wecom.OAuthURL(credential, callbackURL)})
		return
	}
	data, err := h.dashboardTokenByCode(r.Context(), credential, code, r)
	if err != nil {
		if truthyQuery(r, "isJsRedirect") {
			http.Redirect(w, r, h.sidebarCodeAuthRedirect(r, credential.CorpID, http.StatusInternalServerError, err.Error(), nil), http.StatusFound)
			return
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if truthyQuery(r, "isJsRedirect") {
		http.Redirect(w, r, h.sidebarCodeAuthRedirect(r, credential.CorpID, 200, "", data), http.StatusFound)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", data)
}

func (h *SidebarAgentHandler) AgentJSSDKConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if h.resolver == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "sidebar employee resolver not configured", nil)
		return
	}
	employeeID, err := h.resolver.UserID(r)
	if err != nil || employeeID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return
	}
	employee, found, err := h.store.SidebarEmployeeByID(r.Context(), employeeID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "employee not found", nil)
		return
	}
	agentID := queryInt(r, "agentId")
	uriPath := r.URL.Query().Get("uriPath")
	h.writeJSSDKConfig(w, r, employee.CorpID, agentID, uriPath, agentJSSDKAPIs)
}

func (h *SidebarAgentHandler) WxJSSDKConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if h.resolver == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "sidebar employee resolver not configured", nil)
		return
	}
	employeeID, err := h.resolver.UserID(r)
	if err != nil || employeeID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return
	}
	employee, found, err := h.store.SidebarEmployeeByID(r.Context(), employeeID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "employee not found", nil)
		return
	}
	requestedCorpID := queryInt(r, "corpId")
	if requestedCorpID > 0 && requestedCorpID != employee.CorpID {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "forbidden", nil)
		return
	}
	uriPath := h.sidebarBaseURL + r.URL.Query().Get("uriPath")
	h.writeJSSDKConfig(w, r, employee.CorpID, queryInt(r, "agentId"), uriPath, wxJSSDKAPIs)
}

func (h *SidebarAgentHandler) writeJSSDKConfig(w http.ResponseWriter, r *http.Request, corpID int, agentID int, uri string, jsAPIs []string) {
	normalizedURI, ok := h.normalizeJSSDKURI(uri)
	if !ok {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "forbidden", nil)
		return
	}
	credential := SidebarAgentCredential{CorpID: corpID}
	if agentID > 0 {
		agentCredential, found, err := h.store.WorkAgentCredentialByID(r.Context(), agentID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if !found {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "应用不存在", nil)
			return
		}
		if agentCredential.CorpID != corpID {
			writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "forbidden", nil)
			return
		}
		credential = agentCredential
	} else {
		corpCredential, found, err := h.store.SidebarAgentCorpCredentialByID(r.Context(), corpID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if !found {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "应用对应的企业不存在", nil)
			return
		}
		credential = corpCredential
	}
	config, err := h.wecom.JSSDKConfig(r.Context(), credential, agentID > 0, normalizedURI, jsAPIs)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", config)
}

func (h *SidebarAgentHandler) normalizeJSSDKURI(raw string) (string, bool) {
	if strings.Contains(raw, "#") {
		return "", false
	}
	uri, err := url.ParseRequestURI(raw)
	if err != nil || !uri.IsAbs() || uri.Opaque != "" || uri.Host == "" || uri.User != nil || uri.Fragment != "" {
		return "", false
	}
	if !strings.EqualFold(uri.Scheme, "http") && !strings.EqualFold(uri.Scheme, "https") {
		return "", false
	}
	base, err := url.ParseRequestURI(h.sidebarBaseURL)
	if err != nil || !strings.EqualFold(uri.Scheme, base.Scheme) || !strings.EqualFold(uri.Host, base.Host) {
		return "", false
	}
	return uri.String(), true
}

func (h *SidebarAgentHandler) sidebarTokenByCode(ctx context.Context, credential SidebarAgentCredential, code string, r *http.Request, sidebar bool) (map[string]any, error) {
	wxUserID, err := h.wecom.OAuthUserID(ctx, credential, code)
	if err != nil {
		return nil, err
	}
	login, found, err := h.store.SidebarEmployeeLoginByWXUserID(ctx, wxUserID, credential.CorpID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("此员工暂未同步,请联系管理员!")
	}
	secret := h.simpleSecret
	uid := login.UserID
	if sidebar {
		secret = h.sidebarSecret
		uid = login.EmployeeID
	}
	token, _, err := authjwt.MakeToken(authjwt.TokenOptions{
		Secret: secret,
		TTL:    h.ttl,
		Now:    h.currentTime(),
		UID:    uid,
		Issuer: requestIssuer(r),
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"token": token, "expire": h.expireSeconds()}, nil
}

func (h *SidebarAgentHandler) dashboardTokenByCode(ctx context.Context, credential SidebarAgentCredential, code string, r *http.Request) (map[string]any, error) {
	wxUserID, err := h.wecom.OAuthUserID(ctx, credential, code)
	if err != nil {
		return nil, err
	}
	login, found, err := h.store.SidebarEmployeeLoginByWXUserID(ctx, wxUserID, credential.CorpID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("此员工未同步")
	}
	if login.UserID <= 0 {
		return nil, fmt.Errorf("此员工未关联子账户")
	}
	if login.UserStatus != 1 {
		return nil, fmt.Errorf("账户已禁用，无法登录")
	}
	token, _, err := authjwt.MakeToken(authjwt.TokenOptions{
		Secret: h.simpleSecret,
		TTL:    h.ttl,
		Now:    h.currentTime(),
		UID:    login.UserID,
		Issuer: requestIssuer(r),
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"token": token, "expire": h.expireSeconds()}, nil
}

func (h *SidebarAgentHandler) normalizeTarget(target string) string {
	if strings.Contains(target, "http") {
		return target
	}
	return h.sidebarBaseURL + target
}

func (h *SidebarAgentHandler) sidebarAuthRedirect(agentID int, code int, msg string, data any, target string) string {
	values := url.Values{}
	values.Set("agentId", strconv.Itoa(agentID))
	values.Set("state", sidebarAgentState(code, msg, data))
	values.Set("target", target)
	return h.sidebarBaseURL + "/auth?" + values.Encode()
}

func (h *SidebarAgentHandler) sidebarCodeAuthRedirect(r *http.Request, corpID int, code int, msg string, data map[string]any) string {
	payload := map[string]any{}
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			payload[key] = values[0]
		}
	}
	payload["corpId"] = corpID
	for key, value := range data {
		payload[key] = value
	}
	return h.sidebarBaseURL + "/codeAuth?callValues=" + sidebarAgentState(code, msg, payload)
}

func (h *SidebarAgentHandler) currentTime() time.Time {
	if h.now != nil {
		return h.now()
	}
	return time.Now()
}

func (h *SidebarAgentHandler) expireSeconds() int64 {
	ttl := h.ttl
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return int64(ttl / time.Second)
}

func sidebarAgentState(code int, msg string, data any) string {
	raw, _ := json.Marshal(envelope{Code: code, Msg: msg, Data: data})
	return base64.StdEncoding.EncodeToString(raw)
}

func sidebarAgentParams(r *http.Request) (map[string]any, error) {
	params := valuesToParams(r.URL.Query())
	if r.Method == http.MethodPost {
		bodyParams, err := parseRequestParams(r)
		if err != nil {
			return nil, err
		}
		for key, value := range bodyParams {
			params[key] = value
		}
	}
	return params, nil
}

func queryInt(r *http.Request, key string) int {
	value, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get(key)))
	return value
}

func truthyQuery(r *http.Request, key string) bool {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	return raw == "1" || strings.EqualFold(raw, "true")
}

var wxJSSDKAPIs = []string{
	"getCurExternalContact",
	"sendChatMessage",
	"getContext",
	"shareAppMessage",
}

var agentJSSDKAPIs = []string{
	"getCurExternalContact",
	"sendChatMessage",
	"getContext",
	"shareAppMessage",
	"navigateToAddCustomer",
}
