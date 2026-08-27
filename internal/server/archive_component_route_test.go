package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"jiyi/mochat-go/internal/config"
)

func TestArchiveComponentRoutesDispatchToAuthenticatedHandler(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("X-Handled-Method", request.Method)
		w.WriteHeader(http.StatusNoContent)
	})
	srv, err := New(config.Config{}, WithArchiveComponentHandler(handler))
	if err != nil {
		t.Fatal(err)
	}
	paths := []struct{ method, path string }{
		{http.MethodPost, "/dashboard/archive/components/8ff7bf2d-5604-43bc-a600-3ec91d575085/session"},
		{http.MethodGet, "/dashboard/archive/components/session/token"},
	}
	for _, item := range paths {
		response := httptest.NewRecorder()
		srv.ServeHTTP(response, httptest.NewRequest(item.method, item.path, nil))
		if response.Code != http.StatusNoContent || response.Header().Get("X-Handled-Method") != item.method {
			t.Fatalf("%s %s status/header=%d/%q", item.method, item.path, response.Code, response.Header().Get("X-Handled-Method"))
		}
	}
	if !containsString(srv.migratedRoutes(), "POST /dashboard/archive/components/{id}/session") || !containsString(srv.migratedRoutes(), "GET /dashboard/archive/components/session/{token}") {
		t.Fatalf("component routes absent: %v", srv.migratedRoutes())
	}
}
