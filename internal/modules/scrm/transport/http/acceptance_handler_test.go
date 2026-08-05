package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type acceptanceStoreStub struct {
	created, verified, cleaned bool
	scope                      AcceptanceScope
}

func (s *acceptanceStoreStub) Create(_ context.Context, scope AcceptanceScope) (AcceptanceResult, error) {
	s.created = true
	s.scope = scope
	return AcceptanceResult{Prefix: AcceptancePrefix, ResourceIDs: []string{scope.EnvironmentID + "-CONTACT"}}, nil
}
func (s *acceptanceStoreStub) Verify(_ context.Context, scope AcceptanceScope) (AcceptanceResult, error) {
	s.verified = true
	s.scope = scope
	return AcceptanceResult{Prefix: AcceptancePrefix}, nil
}
func (s *acceptanceStoreStub) Cleanup(_ context.Context, scope AcceptanceScope) (AcceptanceResult, error) {
	s.cleaned = true
	s.scope = scope
	return AcceptanceResult{Prefix: AcceptancePrefix}, nil
}

type acceptancePrincipal struct{}

func (acceptancePrincipal) Resolve(*http.Request) (Principal, error) {
	return Principal{TenantID: 9, UserID: 7}, nil
}

func TestAcceptanceHandlerIsDisabledByDefault(t *testing.T) {
	store := &acceptanceStoreStub{}
	h := NewAcceptanceHandler(false, "", store, acceptancePrincipal{}, settingsAuth{})
	r := httptest.NewRequest(http.MethodGet, AcceptancePath+"?corpId=42&prefix=P35-ACCEPT-", nil)
	r.Header.Set(AcceptanceEnvironmentHeader, "P35-ACCEPT-TEST")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound || store.verified {
		t.Fatalf("status=%d verified=%v", w.Code, store.verified)
	}
}

func TestAcceptanceHandlerBindsAuthenticatedTenantCorpAndEnvironment(t *testing.T) {
	store := &acceptanceStoreStub{}
	h := NewAcceptanceHandler(true, "P35-ACCEPT-TEST", store, acceptancePrincipal{}, settingsAuth{})
	r := httptest.NewRequest(http.MethodPost, AcceptancePath+"?corpId=42&prefix=P35-ACCEPT-", strings.NewReader(`{"environmentId":"P35-ACCEPT-TEST","prefix":"P35-ACCEPT-"}`))
	r.Header.Set(AcceptanceEnvironmentHeader, "P35-ACCEPT-TEST")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK || !store.created {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if store.scope.TenantID != 9 || store.scope.CorpID != 42 || store.scope.ActorID != 7 || store.scope.EnvironmentID != "P35-ACCEPT-TEST" {
		t.Fatalf("scope=%+v", store.scope)
	}
}

func TestAcceptanceHandlerRejectsPrefixOrEnvironmentMismatch(t *testing.T) {
	for _, tc := range []struct{ name, query, header, body string }{
		{"bad prefix", "?corpId=42&prefix=ALL", "P35-ACCEPT-TEST", `{"environmentId":"P35-ACCEPT-TEST","prefix":"ALL"}`},
		{"header mismatch", "?corpId=42&prefix=P35-ACCEPT-", "P35-ACCEPT-OTHER", `{"environmentId":"P35-ACCEPT-TEST","prefix":"P35-ACCEPT-"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &acceptanceStoreStub{}
			h := NewAcceptanceHandler(true, "P35-ACCEPT-TEST", store, acceptancePrincipal{}, settingsAuth{})
			r := httptest.NewRequest(http.MethodDelete, AcceptancePath+tc.query, strings.NewReader(tc.body))
			r.Header.Set(AcceptanceEnvironmentHeader, tc.header)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest || store.cleaned {
				t.Fatalf("status=%d cleaned=%v", w.Code, store.cleaned)
			}
		})
	}
}
