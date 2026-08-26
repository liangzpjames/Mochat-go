package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/dashboard"
)

func TestArchiveMediaContentRouteDispatchesGETAndHEADAndIsNotPublic(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("X-Handled-Method", request.Method)
		w.WriteHeader(http.StatusNoContent)
	})
	srv, err := New(config.Config{}, WithArchiveMediaContentHandler(handler))
	if err != nil {
		t.Fatal(err)
	}
	path := "/dashboard/archive/media/8ff7bf2d-5604-43bc-a600-3ec91d575085/content"
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		response := httptest.NewRecorder()
		srv.ServeHTTP(response, httptest.NewRequest(method, path, nil))
		if response.Code != http.StatusNoContent || response.Header().Get("X-Handled-Method") != method {
			t.Fatalf("%s status/header = %d/%q", method, response.Code, response.Header().Get("X-Handled-Method"))
		}
	}
	if !containsString(srv.migratedRoutes(), "GET|HEAD /dashboard/archive/media/{id}/content") {
		t.Fatalf("media route absent from migrated routes: %v", srv.migratedRoutes())
	}
	for _, contract := range dashboard.PublicDashboardRouteContracts() {
		if contract == "GET /dashboard/archive/media/{id}/content" || contract == "HEAD /dashboard/archive/media/{id}/content" {
			t.Fatalf("archive media route must not be public: %v", dashboard.PublicDashboardRouteContracts())
		}
	}
}

func TestArchiveMediaContentRouteDispatchesUnsupportedMethodsForStable405(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
	})
	srv, err := New(config.Config{}, WithArchiveMediaContentHandler(handler))
	if err != nil {
		t.Fatal(err)
	}
	path := "/dashboard/archive/media/8ff7bf2d-5604-43bc-a600-3ec91d575085/content"
	response := httptest.NewRecorder()
	srv.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, nil))
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("status/allow=%d/%q want=405/GET, HEAD", response.Code, response.Header().Get("Allow"))
	}
}
