package wecomarchivedemo

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPublicHandlerDoesNotExposeAdminRoutesOrSecrets(t *testing.T) {
	config := Config{CallbackToken: "callback-secret", EncodingAESKey: callbackTestAESKey, AdminToken: "admin-secret"}
	store, err := NewEvidenceStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := NewPublicHandler(config, store, false)
	for _, path := range []string{"/admin/status", "/admin/pull"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d", path, recorder.Code)
		}
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("health status = %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), config.CallbackToken) || strings.Contains(recorder.Body.String(), config.AdminToken) {
		t.Fatalf("health response leaked a secret: %s", recorder.Body.String())
	}
}

func TestAdminHandlerRequiresBearerToken(t *testing.T) {
	config := Config{AdminToken: "admin-secret-with-at-least-forty-characters-123456"}
	store, err := NewEvidenceStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := NewAdminHandler(config, store, nil)
	request := httptest.NewRequest(http.MethodGet, "/admin/status", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", recorder.Code)
	}
	request = httptest.NewRequest(http.MethodGet, "/admin/status", nil)
	request.Header.Set("Authorization", "Bearer "+config.AdminToken)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("authenticated status = %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHTTPServerHasDefensiveTimeouts(t *testing.T) {
	server := newHTTPServer(":8080", http.NotFoundHandler())
	if server.ReadHeaderTimeout != 5*time.Second || server.ReadTimeout != 10*time.Second || server.WriteTimeout != 10*time.Second || server.IdleTimeout != 30*time.Second {
		t.Fatalf("unexpected server timeouts: %+v", server)
	}
}

func TestAdminHandlerRejectsEmptyConfiguredToken(t *testing.T) {
	store, err := NewEvidenceStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := NewAdminHandler(Config{}, store, nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/status", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("empty-token status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}
