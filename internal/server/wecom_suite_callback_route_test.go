package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"jiyi/mochat-go/internal/config"
)

func TestWeComSuiteCallbackRouteDispatchesOnlyExactPublicPath(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	server, err := New(config.Config{}, WithWeComSuiteCallbackHandler(handler))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path string
		want int
	}{{"/wecom/suite/callback", http.StatusNoContent}, {"/wecom/suite/callback/extra", http.StatusNotFound}} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, test.path, nil))
		if response.Code != test.want {
			t.Fatalf("path=%s status=%d want=%d", test.path, response.Code, test.want)
		}
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/wecom/suite/callback", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("GET callback status=%d", response.Code)
	}
}
