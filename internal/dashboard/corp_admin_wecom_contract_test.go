package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestVerifyCompanyUsesRealCredentialProbes(t *testing.T) {
	const (
		corpID         = "ww-real-corp"
		employeeSecret = "employee-secret-not-for-output"
		contactSecret  = "contact-secret-not-for-output"
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/cgi-bin/gettoken":
			if query.Get("corpid") != corpID {
				t.Fatalf("gettoken corpid = %q", query.Get("corpid"))
			}
			switch query.Get("corpsecret") {
			case employeeSecret:
				_, _ = w.Write([]byte(`{"errcode":0,"access_token":"employee-token","expires_in":7200}`))
			case contactSecret:
				_, _ = w.Write([]byte(`{"errcode":0,"access_token":"contact-token","expires_in":7200}`))
			default:
				t.Fatalf("unexpected credential probe")
			}
		case "/cgi-bin/department/list":
			if query.Get("access_token") != "employee-token" || query.Get("userid") != "" || query.Get("external_userid") != "" {
				t.Fatalf("department probe query = %s", query.Encode())
			}
			_, _ = w.Write([]byte(`{"errcode":0,"department":[{"id":1,"name":"真实企业","parentid":0,"order":1}]}`))
		case "/cgi-bin/externalcontact/get_follow_user_list":
			if query.Get("access_token") != "contact-token" || query.Get("external_userid") != "" || query.Get("userid") != "" {
				t.Fatalf("contact probe query = %s", query.Encode())
			}
			_, _ = w.Write([]byte(`{"errcode":0,"follow_user":["visible-user"]}`))
		default:
			t.Fatalf("unexpected WeCom endpoint %s?%s", r.URL.Path, query.Encode())
		}
	}))
	defer server.Close()

	result, err := NewRoomWelcomeWeComClient(server.URL).VerifyCompany(context.Background(), corpID, employeeSecret, contactSecret)
	if err != nil {
		t.Fatalf("VerifyCompany() error = %v", err)
	}
	if result.WXCorpID != corpID || result.CorpName != "真实企业" {
		t.Fatalf("result = %+v", result)
	}
}

func TestVerifyCompanyFailsClosedWhenRootDepartmentNameIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/cgi-bin/gettoken":
			_, _ = w.Write([]byte(`{"errcode":0,"access_token":"probe-token","expires_in":7200}`))
		case "/cgi-bin/department/list":
			_, _ = w.Write([]byte(`{"errcode":0,"department":[{"id":1,"name":"","parentid":0,"order":1}]}`))
		case "/cgi-bin/externalcontact/get_follow_user_list":
			_, _ = w.Write([]byte(`{"errcode":0,"follow_user":[]}`))
		default:
			t.Fatalf("unexpected WeCom endpoint %s", r.URL.Path)
		}
	}))
	defer server.Close()

	_, err := NewRoomWelcomeWeComClient(server.URL).VerifyCompany(context.Background(), "ww-corp", "employee-secret", "contact-secret")
	if err == nil || !strings.Contains(err.Error(), "WECOM_CREDENTIAL_INVALID") {
		t.Fatalf("error = %v, want stable WECOM_CREDENTIAL_INVALID", err)
	}
}

func TestVerifyCompanyNeverReturnsProviderSecretInError(t *testing.T) {
	const secret = "employee-secret-must-not-leak"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/cgi-bin/gettoken" {
			_, _ = w.Write([]byte(`{"errcode":40001,"errmsg":"` + url.QueryEscape(secret) + `"}`))
			return
		}
		t.Fatalf("unexpected endpoint %s", r.URL.Path)
	}))
	defer server.Close()

	_, err := NewRoomWelcomeWeComClient(server.URL).VerifyCompany(context.Background(), "ww-corp", secret, "contact-secret")
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("error = %v leaks provider credential", err)
	}
}
