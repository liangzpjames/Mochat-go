package modules

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestRouterMatchesMethodAndPath(t *testing.T) {
	router := NewRouter()
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if err := router.Handle(http.MethodGet, "/api/example", handler); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/example", nil)
	matched, ok := router.Match(request)
	if !ok {
		t.Fatal("route did not match")
	}
	response := httptest.NewRecorder()
	matched.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestRouterDoesNotMatchDifferentMethod(t *testing.T) {
	router := NewRouter()
	if err := router.Handle(http.MethodGet, "/api/example", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})); err != nil {
		t.Fatal(err)
	}

	if _, ok := router.Match(httptest.NewRequest(http.MethodPost, "/api/example", nil)); ok {
		t.Fatal("route matched a different method")
	}
}

func TestRouterRejectsDuplicateRoute(t *testing.T) {
	router := NewRouter()
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	if err := router.Handle("GET", "/api/example", handler); err != nil {
		t.Fatal(err)
	}
	if err := router.Handle("GET", "/api/example", handler); !errors.Is(err, ErrDuplicateRoute) {
		t.Fatalf("error = %v", err)
	}
}

func TestRouterRejectsInvalidRoute(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	for _, tc := range []struct {
		name    string
		method  string
		pattern string
		handler http.Handler
	}{
		{name: "empty method", method: "", pattern: "/api/example", handler: handler},
		{name: "relative path", method: http.MethodGet, pattern: "api/example", handler: handler},
		{name: "query string", method: http.MethodGet, pattern: "/api/example?query=value", handler: handler},
		{name: "nil handler", method: http.MethodGet, pattern: "/api/example", handler: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := NewRouter().Handle(tc.method, tc.pattern, tc.handler); err == nil {
				t.Fatal("expected invalid route error")
			}
		})
	}
}

func TestRouterAllowsConcurrentReadsAfterRegistration(t *testing.T) {
	router := NewRouter()
	if err := router.Handle(http.MethodGet, "/api/example", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/example", nil)
	var group sync.WaitGroup
	for range 32 {
		group.Go(func() {
			if _, ok := router.Match(request); !ok {
				t.Error("route did not match")
			}
		})
	}
	group.Wait()
}

func TestRouterServeHTTPReturnsJSONNotFoundForUnmatchedRoute(t *testing.T) {
	response := httptest.NewRecorder()
	NewRouter().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/missing", nil))

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
}
