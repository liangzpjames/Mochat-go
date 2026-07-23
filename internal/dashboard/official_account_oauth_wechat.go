package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultWeChatAPIBaseURL = "https://api.weixin.qq.com"
	weChatOAuthURL          = "https://open.weixin.qq.com/connect/oauth2/authorize"
	weChatPreAuthURL        = "https://mp.weixin.qq.com/cgi-bin/componentloginpage"
)

type OfficialAccountOAuthHTTPClient struct {
	baseURL               string
	componentAppID        string
	componentSecret       string
	componentVerifyTicket string
	ticketProvider        WeChatComponentVerifyTicketProvider
	httpClient            *http.Client
	mu                    sync.Mutex
	componentAccessTokens map[string]cachedWeChatComponentToken
}

type WeChatComponentVerifyTicketProvider interface {
	WeChatComponentVerifyTicket(ctx context.Context, componentAppID string) (string, bool, error)
}

type cachedWeChatComponentToken struct {
	token     string
	expiresAt time.Time
}

func NewOfficialAccountOAuthClient(baseURL string, componentAppID string, componentSecret string, componentVerifyTicket string) *OfficialAccountOAuthHTTPClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultWeChatAPIBaseURL
	}
	return &OfficialAccountOAuthHTTPClient{
		baseURL:               baseURL,
		componentAppID:        strings.TrimSpace(componentAppID),
		componentSecret:       strings.TrimSpace(componentSecret),
		componentVerifyTicket: strings.TrimSpace(componentVerifyTicket),
		httpClient:            &http.Client{Timeout: 30 * time.Second},
		componentAccessTokens: map[string]cachedWeChatComponentToken{},
	}
}

func (c *OfficialAccountOAuthHTTPClient) WithComponentVerifyTicketProvider(provider WeChatComponentVerifyTicketProvider) *OfficialAccountOAuthHTTPClient {
	c.ticketProvider = provider
	return c
}

func (c *OfficialAccountOAuthHTTPClient) OAuthURL(info OfficialAccountOAuthInfo, redirectURI string) (string, error) {
	authorizerAppID := strings.TrimSpace(info.AuthorizerAppID)
	if authorizerAppID == "" {
		return "", fmt.Errorf("公众号 authorizer_appid 未配置")
	}
	componentAppID, _, _ := c.componentConfig(info)
	if componentAppID == "" {
		return "", fmt.Errorf("微信开放平台 component_appid 未配置")
	}
	values := url.Values{}
	values.Set("appid", authorizerAppID)
	values.Set("component_appid", componentAppID)
	values.Set("redirect_uri", redirectURI)
	values.Set("response_type", "code")
	values.Set("scope", "snsapi_userinfo")
	values.Set("state", "")
	return weChatOAuthURL + "?" + values.Encode() + "#wechat_redirect", nil
}

func (c *OfficialAccountOAuthHTTPClient) OAuthUser(ctx context.Context, info OfficialAccountOAuthInfo, code string) (map[string]any, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, fmt.Errorf("code 必填")
	}
	authorizerAppID := strings.TrimSpace(info.AuthorizerAppID)
	if authorizerAppID == "" {
		return nil, fmt.Errorf("公众号 authorizer_appid 未配置")
	}
	componentAppID, componentSecret, componentVerifyTicket, err := c.componentConfigWithTicket(ctx, info)
	if err != nil {
		return nil, err
	}
	if componentAppID == "" {
		return nil, fmt.Errorf("微信开放平台 component_appid 未配置")
	}
	if componentSecret == "" {
		return nil, fmt.Errorf("微信开放平台 component_appsecret 未配置")
	}
	if componentVerifyTicket == "" {
		return nil, fmt.Errorf("微信开放平台 component_verify_ticket 未配置")
	}

	componentAccessToken, err := c.componentAccessToken(ctx, componentAppID, componentSecret, componentVerifyTicket)
	if err != nil {
		return nil, err
	}
	webToken, err := c.webAccessToken(ctx, authorizerAppID, code, componentAppID, componentAccessToken)
	if err != nil {
		return nil, err
	}
	rawUser, err := c.userInfo(ctx, webToken.AccessToken, webToken.OpenID)
	if err != nil {
		return nil, err
	}
	if _, ok := rawUser["openid"]; !ok && webToken.OpenID != "" {
		rawUser["openid"] = webToken.OpenID
	}
	if _, ok := rawUser["unionid"]; !ok && webToken.UnionID != "" {
		rawUser["unionid"] = webToken.UnionID
	}
	return rawUser, nil
}

func (c *OfficialAccountOAuthHTTPClient) PreAuthorizationURL(ctx context.Context, redirectURI string) (string, error) {
	componentAppID, componentSecret, componentVerifyTicket, err := c.componentConfigWithTicket(ctx, OfficialAccountOAuthInfo{})
	if err != nil {
		return "", err
	}
	if componentAppID == "" {
		return "", fmt.Errorf("微信开放平台 component_appid 未配置")
	}
	if componentSecret == "" {
		return "", fmt.Errorf("微信开放平台 component_appsecret 未配置")
	}
	if componentVerifyTicket == "" {
		return "", fmt.Errorf("微信开放平台 component_verify_ticket 未配置")
	}
	componentAccessToken, err := c.componentAccessToken(ctx, componentAppID, componentSecret, componentVerifyTicket)
	if err != nil {
		return "", err
	}
	var response struct {
		weComBaseResponse
		PreAuthCode string `json:"pre_auth_code"`
		ExpiresIn   int    `json:"expires_in"`
	}
	values := url.Values{}
	values.Set("component_access_token", componentAccessToken)
	if err := c.postJSONWithQuery(ctx, "cgi-bin/component/api_create_preauthcode", values, map[string]any{"component_appid": componentAppID}, &response); err != nil {
		return "", err
	}
	if response.PreAuthCode == "" {
		return "", fmt.Errorf("微信接口未返回 pre_auth_code")
	}
	query := url.Values{}
	query.Set("component_appid", componentAppID)
	query.Set("pre_auth_code", response.PreAuthCode)
	query.Set("redirect_uri", redirectURI)
	query.Set("auth_type", "3")
	return weChatPreAuthURL + "?" + query.Encode(), nil
}

func (c *OfficialAccountOAuthHTTPClient) QueryAuthorization(ctx context.Context, authCode string) (OfficialAccountAuthorization, error) {
	authCode = strings.TrimSpace(authCode)
	if authCode == "" {
		return OfficialAccountAuthorization{}, fmt.Errorf("auth_code 必传")
	}
	componentAppID, componentSecret, componentVerifyTicket, err := c.componentConfigWithTicket(ctx, OfficialAccountOAuthInfo{})
	if err != nil {
		return OfficialAccountAuthorization{}, err
	}
	if componentAppID == "" {
		return OfficialAccountAuthorization{}, fmt.Errorf("微信开放平台 component_appid 未配置")
	}
	if componentSecret == "" {
		return OfficialAccountAuthorization{}, fmt.Errorf("微信开放平台 component_appsecret 未配置")
	}
	if componentVerifyTicket == "" {
		return OfficialAccountAuthorization{}, fmt.Errorf("微信开放平台 component_verify_ticket 未配置")
	}
	componentAccessToken, err := c.componentAccessToken(ctx, componentAppID, componentSecret, componentVerifyTicket)
	if err != nil {
		return OfficialAccountAuthorization{}, err
	}
	values := url.Values{}
	values.Set("component_access_token", componentAccessToken)
	request := map[string]any{
		"component_appid":    componentAppID,
		"authorization_code": authCode,
	}
	raw, err := c.postJSONBytes(ctx, "cgi-bin/component/api_query_auth", values, request)
	if err != nil {
		return OfficialAccountAuthorization{}, err
	}
	var base weComBaseResponse
	if err := json.Unmarshal(raw, &base); err != nil {
		return OfficialAccountAuthorization{}, err
	}
	if err := base.Err(); err != nil {
		return OfficialAccountAuthorization{}, err
	}
	var envelope struct {
		AuthorizationInfo map[string]json.RawMessage `json:"authorization_info"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return OfficialAccountAuthorization{}, err
	}
	var authorizerAppID string
	if err := json.Unmarshal(envelope.AuthorizationInfo["authorizer_appid"], &authorizerAppID); err != nil {
		return OfficialAccountAuthorization{}, err
	}
	if authorizerAppID == "" {
		return OfficialAccountAuthorization{}, fmt.Errorf("微信接口未返回 authorizer_appid")
	}
	var authorizerRefreshToken string
	if raw := envelope.AuthorizationInfo["authorizer_refresh_token"]; len(raw) > 0 {
		_ = json.Unmarshal(raw, &authorizerRefreshToken)
	}
	funcInfo := strings.TrimSpace(string(envelope.AuthorizationInfo["func_info"]))
	return OfficialAccountAuthorization{
		ComponentAppID:         componentAppID,
		ComponentSecret:        componentSecret,
		AuthorizedStatus:       1,
		AuthorizerAppID:        authorizerAppID,
		AuthorizerRefreshToken: authorizerRefreshToken,
		AuthorizationCode:      authCode,
		FuncInfo:               funcInfo,
	}, nil
}

func (c *OfficialAccountOAuthHTTPClient) SendCustomerTextFromAuthCode(ctx context.Context, authCode string, toUser string, content string) error {
	authorization, err := c.QueryAuthorization(ctx, authCode)
	if err != nil {
		return err
	}
	authorizerAppID := strings.TrimSpace(authorization.AuthorizerAppID)
	refreshToken := strings.TrimSpace(authorization.AuthorizerRefreshToken)
	if authorizerAppID == "" {
		return fmt.Errorf("微信接口未返回 authorizer_appid")
	}
	if refreshToken == "" {
		return fmt.Errorf("微信接口未返回 authorizer_refresh_token")
	}
	componentAppID, componentSecret, componentVerifyTicket, err := c.componentConfigWithTicket(ctx, OfficialAccountOAuthInfo{})
	if err != nil {
		return err
	}
	componentAccessToken, err := c.componentAccessToken(ctx, componentAppID, componentSecret, componentVerifyTicket)
	if err != nil {
		return err
	}
	authorizerAccessToken, err := c.authorizerAccessToken(ctx, componentAppID, componentAccessToken, authorizerAppID, refreshToken)
	if err != nil {
		return err
	}
	values := url.Values{}
	values.Set("access_token", authorizerAccessToken)
	request := map[string]any{
		"touser":  strings.TrimSpace(toUser),
		"msgtype": "text",
		"text": map[string]any{
			"content": content,
		},
	}
	var response weComBaseResponse
	return c.postJSONWithQuery(ctx, "cgi-bin/message/custom/send", values, request, &response)
}

func (c *OfficialAccountOAuthHTTPClient) AuthorizerInfo(ctx context.Context, authorizerAppID string) (OfficialAccountAuthorizerProfile, error) {
	authorizerAppID = strings.TrimSpace(authorizerAppID)
	if authorizerAppID == "" {
		return OfficialAccountAuthorizerProfile{}, fmt.Errorf("authorizer_appid 必传")
	}
	componentAppID, componentSecret, componentVerifyTicket, err := c.componentConfigWithTicket(ctx, OfficialAccountOAuthInfo{})
	if err != nil {
		return OfficialAccountAuthorizerProfile{}, err
	}
	if componentAppID == "" {
		return OfficialAccountAuthorizerProfile{}, fmt.Errorf("微信开放平台 component_appid 未配置")
	}
	if componentSecret == "" {
		return OfficialAccountAuthorizerProfile{}, fmt.Errorf("微信开放平台 component_appsecret 未配置")
	}
	if componentVerifyTicket == "" {
		return OfficialAccountAuthorizerProfile{}, fmt.Errorf("微信开放平台 component_verify_ticket 未配置")
	}
	componentAccessToken, err := c.componentAccessToken(ctx, componentAppID, componentSecret, componentVerifyTicket)
	if err != nil {
		return OfficialAccountAuthorizerProfile{}, err
	}
	var response struct {
		weComBaseResponse
		AuthorizerInfo struct {
			NickName        string         `json:"nick_name"`
			HeadImg         string         `json:"head_img"`
			ServiceTypeInfo map[string]any `json:"service_type_info"`
			VerifyTypeInfo  map[string]any `json:"verify_type_info"`
			UserName        string         `json:"user_name"`
			PrincipalName   string         `json:"principal_name"`
			Alias           string         `json:"alias"`
			BusinessInfo    map[string]any `json:"business_info"`
			QRCodeURL       string         `json:"qrcode_url"`
		} `json:"authorizer_info"`
	}
	values := url.Values{}
	values.Set("component_access_token", componentAccessToken)
	request := map[string]any{
		"component_appid":  componentAppID,
		"authorizer_appid": authorizerAppID,
	}
	if err := c.postJSONWithQuery(ctx, "cgi-bin/component/api_get_authorizer_info", values, request, &response); err != nil {
		return OfficialAccountAuthorizerProfile{}, err
	}
	info := response.AuthorizerInfo
	return OfficialAccountAuthorizerProfile{
		Nickname:        info.NickName,
		HeadImg:         info.HeadImg,
		Avatar:          info.HeadImg,
		ServiceTypeInfo: intFromAny(info.ServiceTypeInfo["id"]),
		VerifyTypeInfo:  intFromAny(info.VerifyTypeInfo["id"]),
		UserName:        info.UserName,
		PrincipalName:   info.PrincipalName,
		Alias:           info.Alias,
		BusinessInfo:    officialAccountJSON(info.BusinessInfo),
		QRCodeURL:       info.QRCodeURL,
		LocalQRCodeURL:  "",
	}, nil
}

func (c *OfficialAccountOAuthHTTPClient) componentConfig(info OfficialAccountOAuthInfo) (string, string, string) {
	componentAppID := strings.TrimSpace(info.ComponentAppID)
	if componentAppID == "" {
		componentAppID = c.componentAppID
	}
	componentSecret := strings.TrimSpace(info.ComponentSecret)
	if componentSecret == "" {
		componentSecret = c.componentSecret
	}
	return componentAppID, componentSecret, c.componentVerifyTicket
}

func (c *OfficialAccountOAuthHTTPClient) componentConfigWithTicket(ctx context.Context, info OfficialAccountOAuthInfo) (string, string, string, error) {
	componentAppID, componentSecret, componentVerifyTicket := c.componentConfig(info)
	if strings.TrimSpace(componentVerifyTicket) != "" || c.ticketProvider == nil || strings.TrimSpace(componentAppID) == "" {
		return componentAppID, componentSecret, strings.TrimSpace(componentVerifyTicket), nil
	}
	storedTicket, found, err := c.ticketProvider.WeChatComponentVerifyTicket(ctx, componentAppID)
	if err != nil {
		return componentAppID, componentSecret, "", err
	}
	if found {
		componentVerifyTicket = strings.TrimSpace(storedTicket)
	}
	return componentAppID, componentSecret, strings.TrimSpace(componentVerifyTicket), nil
}

func (c *OfficialAccountOAuthHTTPClient) componentAccessToken(ctx context.Context, componentAppID string, componentSecret string, componentVerifyTicket string) (string, error) {
	cacheKey := componentAppID + ":" + componentSecret + ":" + componentVerifyTicket
	now := time.Now()
	c.mu.Lock()
	if cached, ok := c.componentAccessTokens[cacheKey]; ok && cached.token != "" && now.Before(cached.expiresAt.Add(-5*time.Minute)) {
		c.mu.Unlock()
		return cached.token, nil
	}
	c.mu.Unlock()

	var response struct {
		weComBaseResponse
		ComponentAccessToken string `json:"component_access_token"`
		ExpiresIn            int    `json:"expires_in"`
	}
	request := map[string]any{
		"component_appid":         componentAppID,
		"component_appsecret":     componentSecret,
		"component_verify_ticket": componentVerifyTicket,
	}
	if err := c.postJSON(ctx, "cgi-bin/component/api_component_token", request, &response); err != nil {
		return "", err
	}
	if response.ComponentAccessToken == "" {
		return "", fmt.Errorf("微信接口未返回 component_access_token")
	}
	expiresIn := response.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 7200
	}
	c.mu.Lock()
	c.componentAccessTokens[cacheKey] = cachedWeChatComponentToken{
		token:     response.ComponentAccessToken,
		expiresAt: now.Add(time.Duration(expiresIn) * time.Second),
	}
	c.mu.Unlock()
	return response.ComponentAccessToken, nil
}

func (c *OfficialAccountOAuthHTTPClient) authorizerAccessToken(ctx context.Context, componentAppID string, componentAccessToken string, authorizerAppID string, authorizerRefreshToken string) (string, error) {
	var response struct {
		weComBaseResponse
		AuthorizerAccessToken  string `json:"authorizer_access_token"`
		ExpiresIn              int    `json:"expires_in"`
		AuthorizerRefreshToken string `json:"authorizer_refresh_token"`
	}
	values := url.Values{}
	values.Set("component_access_token", componentAccessToken)
	request := map[string]any{
		"component_appid":          componentAppID,
		"authorizer_appid":         authorizerAppID,
		"authorizer_refresh_token": authorizerRefreshToken,
	}
	if err := c.postJSONWithQuery(ctx, "cgi-bin/component/api_authorizer_token", values, request, &response); err != nil {
		return "", err
	}
	if response.AuthorizerAccessToken == "" {
		return "", fmt.Errorf("微信接口未返回 authorizer_access_token")
	}
	return response.AuthorizerAccessToken, nil
}

type officialAccountWebAccessToken struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	OpenID       string `json:"openid"`
	Scope        string `json:"scope"`
	UnionID      string `json:"unionid"`
}

func (c *OfficialAccountOAuthHTTPClient) webAccessToken(ctx context.Context, authorizerAppID string, code string, componentAppID string, componentAccessToken string) (officialAccountWebAccessToken, error) {
	values := url.Values{}
	values.Set("appid", authorizerAppID)
	values.Set("code", code)
	values.Set("grant_type", "authorization_code")
	values.Set("component_appid", componentAppID)
	values.Set("component_access_token", componentAccessToken)
	var response struct {
		weComBaseResponse
		officialAccountWebAccessToken
	}
	if err := c.getJSON(ctx, "sns/oauth2/component/access_token", values, &response); err != nil {
		return officialAccountWebAccessToken{}, err
	}
	if response.AccessToken == "" {
		return officialAccountWebAccessToken{}, fmt.Errorf("微信接口未返回 access_token")
	}
	if response.OpenID == "" {
		return officialAccountWebAccessToken{}, fmt.Errorf("微信接口未返回 openid")
	}
	return response.officialAccountWebAccessToken, nil
}

func (c *OfficialAccountOAuthHTTPClient) userInfo(ctx context.Context, accessToken string, openID string) (map[string]any, error) {
	values := url.Values{}
	values.Set("access_token", accessToken)
	values.Set("openid", openID)
	values.Set("lang", "zh_CN")
	raw := map[string]any{}
	if err := c.getJSONMap(ctx, "sns/userinfo", values, raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (c *OfficialAccountOAuthHTTPClient) postJSON(ctx context.Context, path string, payload any, out interface{ Err() error }) error {
	return c.postJSONWithQuery(ctx, path, nil, payload, out)
}

func (c *OfficialAccountOAuthHTTPClient) postJSONWithQuery(ctx context.Context, path string, values url.Values, payload any, out interface{ Err() error }) error {
	raw, err := c.postJSONBytes(ctx, path, values, payload)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return err
	}
	return out.Err()
}

func (c *OfficialAccountOAuthHTTPClient) postJSONBytes(ctx context.Context, path string, values url.Values, payload any) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL(path, values), bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("微信接口 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (c *OfficialAccountOAuthHTTPClient) getJSON(ctx context.Context, path string, values url.Values, out interface{ Err() error }) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL(path, values), nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("微信接口 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return err
	}
	return out.Err()
}

func (c *OfficialAccountOAuthHTTPClient) getJSONMap(ctx context.Context, path string, values url.Values, out map[string]any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL(path, values), nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("微信接口 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	decoder := json.NewDecoder(resp.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&out); err != nil {
		return err
	}
	if errCode, ok := out["errcode"]; ok && fmt.Sprint(errCode) != "0" {
		if errMsg, ok := out["errmsg"]; ok && strings.TrimSpace(fmt.Sprint(errMsg)) != "" {
			return fmt.Errorf("%s", strings.TrimSpace(fmt.Sprint(errMsg)))
		}
		return fmt.Errorf("errcode=%s", fmt.Sprint(errCode))
	}
	return nil
}

func (c *OfficialAccountOAuthHTTPClient) apiURL(path string, values url.Values) string {
	if values == nil {
		values = url.Values{}
	}
	rawQuery := values.Encode()
	if rawQuery == "" {
		return c.baseURL + "/" + strings.TrimLeft(path, "/")
	}
	return c.baseURL + "/" + strings.TrimLeft(path, "/") + "?" + rawQuery
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		integer, _ := typed.Int64()
		return int(integer)
	case string:
		var integer int
		_, _ = fmt.Sscanf(strings.TrimSpace(typed), "%d", &integer)
		return integer
	default:
		return 0
	}
}
