package wecomsuitecallback

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBridgeAuthorizationExchangerUsesInternalBearerAndStrictResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/suite/authorization/exchange" || r.Header.Get("Authorization") != "Bearer "+testBridgeCallbackBearer {
			t.Fatalf("request path=%s authorization=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tenantId":21,"corpId":"ww-delegated","permanentCode":"permanent-secret"}`))
	}))
	defer server.Close()
	exchanger, err := NewBridgeAuthorizationExchanger(server.URL, testBridgeCallbackBearer, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	authorization, err := exchanger.ExchangeAuthorization(context.Background(), "suite", "secret", "ticket", "auth-code")
	if err != nil || authorization.TenantID != 21 || authorization.CorpID != "ww-delegated" || authorization.PermanentCode != "permanent-secret" {
		t.Fatalf("authorization=%+v err=%v", authorization, err)
	}
}

func TestBridgeAuthorizationExchangerDoesNotLeakUpstreamBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "permanent-secret", http.StatusBadGateway)
	}))
	defer server.Close()
	exchanger, err := NewBridgeAuthorizationExchanger(server.URL, testBridgeCallbackBearer, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = exchanger.ExchangeAuthorization(context.Background(), "suite", "secret", "ticket", "auth-code")
	if err == nil || strings.Contains(err.Error(), "permanent-secret") {
		t.Fatalf("error=%v", err)
	}
}

const testBridgeCallbackBearer = "local-bridge-bearer-012345678901234567890123456789"
