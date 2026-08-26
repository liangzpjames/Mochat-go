package wecomarchivedemo

import (
	"encoding/json"
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

func TestAdminHandlerServesAuthenticatedArchiveBridgePage(t *testing.T) {
	privatePEM, encryptedRandomKey := archiveRSAFixture(t, []byte("session-key"))
	chatData, _ := json.Marshal(map[string]any{"errcode": 0, "chatdata": []map[string]any{{
		"seq": 41, "msgid": "msg-live-41", "publickey_ver": 3,
		"encrypt_random_key": encryptedRandomKey, "encrypt_chat_msg": "cipher-41",
	}}})
	sdk := &fakeFinanceSDK{chatData: chatData, plain: map[string][]byte{
		"cipher-41": []byte(`{"msgid":"msg-live-41","msgtype":"text","text":{"content":"live marker"}}`),
	}}
	store, err := NewEvidenceStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	archive, err := NewArchiveService(sdk, privatePEM, store, 100, 5)
	if err != nil {
		t.Fatal(err)
	}
	config := Config{
		AdminToken: "admin-secret-with-at-least-forty-characters-123456",
		CorpID:     "ww-live", ArchiveSecret: "archive-secret", RSAPrivateKey: privatePEM,
	}
	handler := NewAdminHandler(config, store, archive)

	request := httptest.NewRequest(http.MethodPost, "/work-message/archive/messages",
		strings.NewReader(`{"corp_id":4,"wx_corpid":"ww-live","seq":40,"limit":10}`))
	request.Header.Set("Authorization", "Bearer "+config.AdminToken)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"msgid":"msg-live-41"`) || !strings.Contains(recorder.Body.String(), `"seq":41`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	for _, forbidden := range []string{config.ArchiveSecret, "PRIVATE KEY"} {
		if strings.Contains(recorder.Body.String(), forbidden) {
			t.Fatalf("bridge response leaked protected configuration: %s", recorder.Body.String())
		}
	}

	request = httptest.NewRequest(http.MethodPost, "/work-message/archive/messages",
		strings.NewReader(`{"corp_id":4,"wx_corpid":"ww-other","seq":40,"limit":10}`))
	request.Header.Set("Authorization", "Bearer "+config.AdminToken)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("mismatched corp status=%d body=%s", recorder.Code, recorder.Body.String())
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
