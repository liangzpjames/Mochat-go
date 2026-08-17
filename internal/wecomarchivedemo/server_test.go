package wecomarchivedemo

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	config := Config{AdminToken: "admin-secret"}
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
	request.Header.Set("Authorization", "Bearer admin-secret")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("authenticated status = %d body=%s", recorder.Code, recorder.Body.String())
	}
}
