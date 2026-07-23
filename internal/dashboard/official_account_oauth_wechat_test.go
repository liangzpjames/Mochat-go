package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestOfficialAccountOAuthClientUsesStoredComponentVerifyTicket(t *testing.T) {
	var tokenPayload map[string]any
	var preAuthAccessToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/component/api_component_token":
			if err := json.NewDecoder(r.Body).Decode(&tokenPayload); err != nil {
				t.Fatalf("decode token payload: %v", err)
			}
			if tokenPayload["component_verify_ticket"] != "stored-ticket" {
				t.Fatalf("component_verify_ticket = %#v", tokenPayload["component_verify_ticket"])
			}
			writeRaw(w, http.StatusOK, `{"errcode":0,"component_access_token":"component-token","expires_in":7200}`)
		case "/cgi-bin/component/api_create_preauthcode":
			preAuthAccessToken = r.URL.Query().Get("component_access_token")
			writeRaw(w, http.StatusOK, `{"errcode":0,"pre_auth_code":"pre-auth-code","expires_in":600}`)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewOfficialAccountOAuthClient(server.URL, "component-app", "component-secret", "").
		WithComponentVerifyTicketProvider(fakeComponentTicketProvider{ticket: "stored-ticket"})

	authURL, err := client.PreAuthorizationURL(context.Background(), "https://dashboard.example.com/authRedirect?corp_id=7")
	if err != nil {
		t.Fatalf("PreAuthorizationURL error: %v", err)
	}
	if preAuthAccessToken != "component-token" {
		t.Fatalf("component_access_token = %q", preAuthAccessToken)
	}
	parsed, err := url.Parse(strings.TrimSuffix(authURL, "#wechat_redirect"))
	if err != nil {
		t.Fatalf("parse auth url: %v", err)
	}
	query := parsed.Query()
	if query.Get("component_appid") != "component-app" || query.Get("pre_auth_code") != "pre-auth-code" {
		t.Fatalf("auth url = %s", authURL)
	}
	if query.Get("redirect_uri") != "https://dashboard.example.com/authRedirect?corp_id=7" {
		t.Fatalf("redirect_uri = %q", query.Get("redirect_uri"))
	}
}

type fakeComponentTicketProvider struct {
	ticket string
}

func (p fakeComponentTicketProvider) WeChatComponentVerifyTicket(_ context.Context, componentAppID string) (string, bool, error) {
	if componentAppID != "component-app" {
		return "", false, nil
	}
	return p.ticket, p.ticket != "", nil
}
