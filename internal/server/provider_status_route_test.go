package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/config"
)

func TestProviderStatusRouteReachesDedicatedHandler(t *testing.T) {
	called := false
	srv, err := New(config.Config{}, WithProviderStatusHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = r.Method == http.MethodGet && r.URL.Path == "/dashboard/providers/status"
		w.WriteHeader(http.StatusNoContent)
	})))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	srv.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/dashboard/providers/status", nil))
	if response.Code != http.StatusNoContent || !called {
		t.Fatalf("status=%d called=%t, want 204/true", response.Code, called)
	}
}

func TestProviderStatusRouteDoesNotBroadenMethods(t *testing.T) {
	called := false
	srv, err := New(config.Config{}, WithProviderStatusHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	})))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	srv.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/dashboard/providers/status", strings.NewReader(`{"tenantId":999}`)))
	if called || response.Code == http.StatusNoContent {
		t.Fatalf("POST reached handler=%t status=%d", called, response.Code)
	}
}
