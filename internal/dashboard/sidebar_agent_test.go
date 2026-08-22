package dashboard

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/authjwt"
)

func TestSidebarAgentOAuthReturnsAuthURL(t *testing.T) {
	store := &fakeSidebarAgentStore{
		credential: SidebarAgentCredential{ID: 9, CorpID: 7, WXCorpID: "wx-corp", WXAgentID: "100001", WXSecret: "agent-secret"},
	}
	wecom := &fakeSidebarAgentWeCom{authURL: "https://open.weixin.qq.com/connect/oauth2/authorize?appid=wx-corp"}
	handler := NewSidebarAgentHandler(store, wecom, nil, "https://api.example.com", "https://sidebar.example.com", "simple-secret", "sidebar-secret", time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/sidebar/agent/oauth?agentId=9&act=medium&isJsRedirect=1", nil)
	rec := httptest.NewRecorder()
	handler.OAuth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	data := body.Data.(map[string]any)
	if data["url"] != wecom.authURL {
		t.Fatalf("url = %#v", data["url"])
	}
	if !strings.Contains(wecom.lastRedirectURI, "agentId=9") || !strings.Contains(wecom.lastRedirectURI, "isJsRedirect=1") {
		t.Fatalf("redirect uri = %q", wecom.lastRedirectURI)
	}
}

func TestSidebarAgentOAuthCodeSignsDashboardJWT(t *testing.T) {
	store := &fakeSidebarAgentStore{
		credential: SidebarAgentCredential{ID: 9, CorpID: 7, WXCorpID: "wx-corp", WXAgentID: "100001", WXSecret: "agent-secret"},
		login:      SidebarAgentEmployeeLogin{EmployeeID: 5, CorpID: 7, UserID: 3, UserStatus: 1},
	}
	wecom := &fakeSidebarAgentWeCom{userID: "go-user"}
	handler := NewSidebarAgentHandler(store, wecom, nil, "https://api.example.com", "https://sidebar.example.com", "simple-secret", "sidebar-secret", time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/sidebar/agent/oauth?agentId=9&code=ok", nil)
	rec := httptest.NewRecorder()
	handler.OAuth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	data := body.Data.(map[string]any)
	token := data["token"].(string)
	uid, err := (authjwt.Parser{Secret: "simple-secret", Prefix: "default", SkipBlacklist: true}).UserID(requestWithBearer(token))
	if err != nil {
		t.Fatal(err)
	}
	if uid != 3 {
		t.Fatalf("uid = %d", uid)
	}
	if data["expire"].(float64) != 3600 {
		t.Fatalf("expire = %#v", data["expire"])
	}
}

func TestSidebarAgentAuthCodeRedirectsWithSidebarJWT(t *testing.T) {
	store := &fakeSidebarAgentStore{
		credential: SidebarAgentCredential{ID: 9, CorpID: 7, WXCorpID: "wx-corp", WXAgentID: "100001", WXSecret: "agent-secret"},
		login:      SidebarAgentEmployeeLogin{EmployeeID: 5, CorpID: 7, UserID: 3, UserStatus: 1},
	}
	wecom := &fakeSidebarAgentWeCom{userID: "go-user"}
	handler := NewSidebarAgentHandler(store, wecom, nil, "https://api.example.com", "https://sidebar.example.com", "simple-secret", "sidebar-secret", time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/sidebar/agent/auth?agentId=9&target=/contact&code=ok", nil)
	rec := httptest.NewRecorder()
	handler.Auth(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	location := rec.Header().Get("Location")
	if !strings.HasPrefix(location, "https://sidebar.example.com/auth?") {
		t.Fatalf("location = %q", location)
	}
	state := mustDecodeState(t, location)
	data := state["data"].(map[string]any)
	token := data["token"].(string)
	uid, err := (authjwt.Parser{Secret: "sidebar-secret", Prefix: "default", SkipBlacklist: true}).UserID(requestWithBearer(token))
	if err != nil {
		t.Fatal(err)
	}
	if uid != 5 {
		t.Fatalf("uid = %d", uid)
	}
}

func TestSidebarAgentJSSDKUsesSidebarEmployeeCorp(t *testing.T) {
	store := &fakeSidebarAgentStore{
		sidebarEmployee: SidebarEmployee{ID: 5, CorpID: 7, LogUserID: 3},
		credential:      SidebarAgentCredential{ID: 9, CorpID: 7, WXCorpID: "wx-corp", WXAgentID: "100001", WXSecret: "agent-secret"},
	}
	wecom := &fakeSidebarAgentWeCom{jssdk: map[string]any{"agentid": "100001", "signature": "sig"}}
	handler := NewSidebarAgentHandler(store, wecom, HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"}, "https://api.example.com", "https://sidebar.example.com", "simple-secret", "sidebar-secret", time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/sidebar/agent/jssdkConfig?agentId=9&uriPath=https%3A%2F%2Fsidebar.example.com%2Fcontact", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.AgentJSSDKConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !wecom.lastAgentConfig || wecom.lastURI != "https://sidebar.example.com/contact" {
		t.Fatalf("agent=%v uri=%q", wecom.lastAgentConfig, wecom.lastURI)
	}
}

func TestSidebarWxJSSDKRequiresSidebarEmployee(t *testing.T) {
	store := &fakeSidebarAgentStore{credential: SidebarAgentCredential{CorpID: 7, WXCorpID: "wx-corp"}}
	wecom := &fakeSidebarAgentWeCom{jssdk: map[string]any{"signature": "sig"}}
	handler := NewSidebarAgentHandler(store, wecom, HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"}, "https://api.example.com", "https://sidebar.example.com", "simple-secret", "sidebar-secret", time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/sidebar/wxJsSdk/config?corpId=7&uriPath=%2Fcontact", nil)
	rec := httptest.NewRecorder()
	handler.WxJSSDKConfig(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
	if wecom.lastURI != "" {
		t.Fatal("anonymous request reached the WeCom JSSDK client")
	}
}

func TestSidebarWxJSSDKRejectsClientCorpDifferentFromEmployee(t *testing.T) {
	store := &fakeSidebarAgentStore{
		sidebarEmployee: SidebarEmployee{ID: 5, CorpID: 7},
		credential:      SidebarAgentCredential{CorpID: 7, WXCorpID: "wx-corp"},
	}
	wecom := &fakeSidebarAgentWeCom{jssdk: map[string]any{"signature": "sig"}}
	handler := NewSidebarAgentHandler(store, wecom, HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"}, "https://api.example.com", "https://sidebar.example.com", "simple-secret", "sidebar-secret", time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/sidebar/wxJsSdk/config?corpId=8&uriPath=%2Fcontact", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.WxJSSDKConfig(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if wecom.lastURI != "" {
		t.Fatal("cross-corp request reached the WeCom JSSDK client")
	}
}

func TestSidebarWxJSSDKUsesAuthenticatedEmployeeCorp(t *testing.T) {
	store := &fakeSidebarAgentStore{
		sidebarEmployee: SidebarEmployee{ID: 5, CorpID: 7},
		credential:      SidebarAgentCredential{CorpID: 7, WXCorpID: "wx-corp"},
	}
	wecom := &fakeSidebarAgentWeCom{jssdk: map[string]any{"signature": "sig"}}
	handler := NewSidebarAgentHandler(store, wecom, HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"}, "https://api.example.com", "https://sidebar.example.com", "simple-secret", "sidebar-secret", time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/sidebar/wxJsSdk/config?corpId=7&uriPath=%2Fcontact%3Ftab%3D1", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.WxJSSDKConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if wecom.lastURI != "https://sidebar.example.com/contact?tab=1" {
		t.Fatalf("signed URI = %q", wecom.lastURI)
	}
}

func TestSidebarAgentJSSDKRejectsAgentFromAnotherCorp(t *testing.T) {
	store := &fakeSidebarAgentStore{
		sidebarEmployee: SidebarEmployee{ID: 5, CorpID: 7, LogUserID: 3},
		credential:      SidebarAgentCredential{ID: 9, CorpID: 8, WXCorpID: "other-corp", WXAgentID: "100002", WXSecret: "other-secret"},
	}
	wecom := &fakeSidebarAgentWeCom{jssdk: map[string]any{"agentid": "100002", "signature": "sig"}}
	handler := NewSidebarAgentHandler(store, wecom, HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"}, "https://api.example.com", "https://sidebar.example.com", "simple-secret", "sidebar-secret", time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/sidebar/agent/jssdkConfig?agentId=9&uriPath=https%3A%2F%2Fsidebar.example.com%2Fcontact", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.AgentJSSDKConfig(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if wecom.lastAgentConfig {
		t.Fatal("cross-corp credential reached the WeCom JSSDK client")
	}
}

func TestSidebarAgentJSSDKRejectsURLFromAnotherOrigin(t *testing.T) {
	store := &fakeSidebarAgentStore{
		sidebarEmployee: SidebarEmployee{ID: 5, CorpID: 7, LogUserID: 3},
		credential:      SidebarAgentCredential{ID: 9, CorpID: 7, WXCorpID: "wx-corp", WXAgentID: "100001", WXSecret: "agent-secret"},
	}
	wecom := &fakeSidebarAgentWeCom{jssdk: map[string]any{"agentid": "100001", "signature": "sig"}}
	handler := NewSidebarAgentHandler(store, wecom, HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"}, "https://api.example.com", "https://sidebar.example.com", "simple-secret", "sidebar-secret", time.Hour)

	query := url.Values{"agentId": {"9"}, "uriPath": {"https://outside.example.com/contact"}}
	req := httptest.NewRequest(http.MethodGet, "/sidebar/agent/jssdkConfig?"+query.Encode(), nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.AgentJSSDKConfig(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if wecom.lastAgentConfig {
		t.Fatal("off-origin URL reached the WeCom JSSDK client")
	}
}

func TestSidebarAgentJSSDKRejectsUnsafeURLForms(t *testing.T) {
	for _, tc := range []struct {
		name string
		uri  string
	}{
		{name: "userinfo", uri: "https://attacker@sidebar.example.com/contact"},
		{name: "fragment", uri: "https://sidebar.example.com/contact#section"},
		{name: "control character", uri: "https://sidebar.example.com/contact\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeSidebarAgentStore{
				sidebarEmployee: SidebarEmployee{ID: 5, CorpID: 7, LogUserID: 3},
				credential:      SidebarAgentCredential{ID: 9, CorpID: 7, WXCorpID: "wx-corp", WXAgentID: "100001", WXSecret: "agent-secret"},
			}
			wecom := &fakeSidebarAgentWeCom{jssdk: map[string]any{"agentid": "100001", "signature": "sig"}}
			handler := NewSidebarAgentHandler(store, wecom, HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"}, "https://api.example.com", "https://sidebar.example.com", "simple-secret", "sidebar-secret", time.Hour)

			query := url.Values{"agentId": {"9"}, "uriPath": {tc.uri}}
			req := httptest.NewRequest(http.MethodGet, "/sidebar/agent/jssdkConfig?"+query.Encode(), nil)
			req.Header.Set("X-Mochat-Go-Employee-ID", "5")
			rec := httptest.NewRecorder()
			handler.AgentJSSDKConfig(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
			}
			if wecom.lastAgentConfig {
				t.Fatal("unsafe URL reached the WeCom JSSDK client")
			}
		})
	}
}

func TestSidebarAgentJSSDKPreservesSameOriginURLQuery(t *testing.T) {
	store := &fakeSidebarAgentStore{
		sidebarEmployee: SidebarEmployee{ID: 5, CorpID: 7, LogUserID: 3},
		credential:      SidebarAgentCredential{ID: 9, CorpID: 7, WXCorpID: "wx-corp", WXAgentID: "100001", WXSecret: "agent-secret"},
	}
	wecom := &fakeSidebarAgentWeCom{jssdk: map[string]any{"agentid": "100001", "signature": "sig"}}
	handler := NewSidebarAgentHandler(store, wecom, HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"}, "https://api.example.com", "https://sidebar.example.com", "simple-secret", "sidebar-secret", time.Hour)

	uri := "https://sidebar.example.com/contact?keyword=a%20b&tab=1"
	query := url.Values{"agentId": {"9"}, "uriPath": {uri}}
	req := httptest.NewRequest(http.MethodGet, "/sidebar/agent/jssdkConfig?"+query.Encode(), nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.AgentJSSDKConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if wecom.lastURI != uri {
		t.Fatalf("signed URI = %q, want %q", wecom.lastURI, uri)
	}
}

func requestWithBearer(token string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func mustDecodeState(t *testing.T, location string) map[string]any {
	t.Helper()
	parsed, err := urlParse(location)
	if err != nil {
		t.Fatal(err)
	}
	raw := parsed.Query().Get("state")
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal(decoded, &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func urlParse(raw string) (*url.URL, error) {
	return url.Parse(raw)
}

type fakeSidebarAgentStore struct {
	credential      SidebarAgentCredential
	sidebarEmployee SidebarEmployee
	login           SidebarAgentEmployeeLogin
}

func (s *fakeSidebarAgentStore) WorkAgentCredentialByID(_ context.Context, agentID int) (SidebarAgentCredential, bool, error) {
	if s.credential.ID == agentID {
		return s.credential, true, nil
	}
	return SidebarAgentCredential{}, false, nil
}

func (s *fakeSidebarAgentStore) SidebarAgentCorpCredentialByID(_ context.Context, corpID int) (SidebarAgentCredential, bool, error) {
	if s.credential.CorpID == corpID {
		return s.credential, true, nil
	}
	return SidebarAgentCredential{}, false, nil
}

func (s *fakeSidebarAgentStore) SidebarEmployeeByID(_ context.Context, employeeID int) (SidebarEmployee, bool, error) {
	if s.sidebarEmployee.ID == employeeID {
		return s.sidebarEmployee, true, nil
	}
	return SidebarEmployee{}, false, nil
}

func (s *fakeSidebarAgentStore) SidebarEmployeeLoginByWXUserID(_ context.Context, _ string, corpID int) (SidebarAgentEmployeeLogin, bool, error) {
	if s.login.CorpID == corpID {
		return s.login, true, nil
	}
	return SidebarAgentEmployeeLogin{}, false, nil
}

type fakeSidebarAgentWeCom struct {
	authURL         string
	userID          string
	jssdk           map[string]any
	lastRedirectURI string
	lastAgentConfig bool
	lastURI         string
}

func (c *fakeSidebarAgentWeCom) OAuthURL(_ SidebarAgentCredential, redirectURI string) string {
	c.lastRedirectURI = redirectURI
	if c.authURL != "" {
		return c.authURL
	}
	return "https://open.weixin.qq.com/connect/oauth2/authorize"
}

func (c *fakeSidebarAgentWeCom) OAuthUserID(_ context.Context, _ SidebarAgentCredential, _ string) (string, error) {
	return c.userID, nil
}

func (c *fakeSidebarAgentWeCom) JSSDKConfig(_ context.Context, _ SidebarAgentCredential, agent bool, uri string, _ []string) (map[string]any, error) {
	c.lastAgentConfig = agent
	c.lastURI = uri
	out := map[string]any{}
	for key, value := range c.jssdk {
		out[key] = value
	}
	return out, nil
}
