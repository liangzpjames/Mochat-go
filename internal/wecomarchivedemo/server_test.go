package wecomarchivedemo

import (
	"encoding/base64"
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

func TestAdminHandlerServesAuthenticatedArchiveMediaChunkWithoutLeaks(t *testing.T) {
	privatePEM := testPrivateKeyPEM(t)
	const sdkFileID = "MOCHAT-LOCAL-ACCEPTANCE-20260827-sdkfile-image-sensitive-tail"
	sdk := &fakeFinanceSDK{media: map[string][]MediaChunk{
		sdkFileID: {{Data: []byte("1234567"), NextIndexBuf: "chunk-1", Finished: false}},
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
		CorpID:     "ww-local-acceptance", ArchiveSecret: "archive-secret-sensitive", RSAPrivateKey: privatePEM,
	}
	handler := NewAdminHandler(config, store, archive)
	request := httptest.NewRequest(http.MethodPost, "/work-message/archive/media", strings.NewReader(`{"corp_id":4,"wx_corpid":"ww-local-acceptance","sdkFileId":"`+sdkFileID+`","indexBuf":""}`))
	request.Header.Set("Authorization", "Bearer "+config.AdminToken)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		ErrCode      int    `json:"errcode"`
		DataBase64   string `json:"dataBase64"`
		NextIndexBuf string `json:"nextIndexBuf"`
		Finished     bool   `json:"finished"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(response.DataBase64)
	if err != nil {
		t.Fatal(err)
	}
	if response.ErrCode != 0 || string(decoded) != "1234567" || response.NextIndexBuf != "chunk-1" || response.Finished {
		t.Fatalf("response=%+v decoded=%q", response, decoded)
	}
	for _, forbidden := range []string{sdkFileID, config.ArchiveSecret, "PRIVATE KEY", "random-key-sensitive"} {
		if strings.Contains(recorder.Body.String(), forbidden) {
			t.Fatalf("media response leaked protected value %q: %s", forbidden, recorder.Body.String())
		}
	}
}

func TestAdminHandlerRejectsInvalidArchiveMediaRequests(t *testing.T) {
	privatePEM := testPrivateKeyPEM(t)
	store, err := NewEvidenceStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	archive, err := NewArchiveService(&fakeFinanceSDK{}, privatePEM, store, 100, 5)
	if err != nil {
		t.Fatal(err)
	}
	config := Config{AdminToken: "admin-secret-with-at-least-forty-characters-123456", CorpID: "ww-bound", RSAPrivateKey: privatePEM}
	handler := NewAdminHandler(config, store, archive)
	tests := []struct {
		name       string
		method     string
		body       string
		wantStatus int
	}{
		{name: "missing bearer", method: http.MethodPost, body: `{"corp_id":4,"wx_corpid":"ww-bound","sdkFileId":"media"}`, wantStatus: http.StatusUnauthorized},
		{name: "wrong bearer", method: http.MethodPost, body: `{"corp_id":4,"wx_corpid":"ww-bound","sdkFileId":"media"}`, wantStatus: http.StatusUnauthorized},
		{name: "corp mismatch", method: http.MethodPost, body: `{"corp_id":4,"wx_corpid":"ww-other","sdkFileId":"media"}`, wantStatus: http.StatusBadRequest},
		{name: "empty sdk file", method: http.MethodPost, body: `{"corp_id":4,"wx_corpid":"ww-bound","sdkFileId":""}`, wantStatus: http.StatusBadRequest},
		{name: "oversized sdk file", method: http.MethodPost, body: `{"corp_id":4,"wx_corpid":"ww-bound","sdkFileId":"` + strings.Repeat("x", 4097) + `"}`, wantStatus: http.StatusBadRequest},
		{name: "oversized index", method: http.MethodPost, body: `{"corp_id":4,"wx_corpid":"ww-bound","sdkFileId":"media","indexBuf":"` + strings.Repeat("x", 1025) + `"}`, wantStatus: http.StatusBadRequest},
		{name: "oversized body", method: http.MethodPost, body: `{"padding":"` + strings.Repeat("x", 70<<10) + `"}`, wantStatus: http.StatusBadRequest},
		{name: "wrong method", method: http.MethodGet, body: "", wantStatus: http.StatusMethodNotAllowed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, "/work-message/archive/media", strings.NewReader(test.body))
			if test.name != "missing bearer" {
				token := config.AdminToken
				if test.name == "wrong bearer" {
					token = strings.Repeat("z", len(token))
				}
				request.Header.Set("Authorization", "Bearer "+token)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status=%d want=%d body=%s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
		})
	}
}

func TestAdminHandlerSanitizesArchiveMediaSDKError(t *testing.T) {
	privatePEM := testPrivateKeyPEM(t)
	const sdkFileID = "MOCHAT-LOCAL-ACCEPTANCE-20260827-sdkfile-secret-tail"
	store, err := NewEvidenceStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	archive, err := NewArchiveService(&fakeFinanceSDK{mediaErr: SDKError{Operation: "GetMediaData", Code: 10009}}, privatePEM, store, 100, 5)
	if err != nil {
		t.Fatal(err)
	}
	config := Config{AdminToken: "admin-secret-with-at-least-forty-characters-123456", CorpID: "ww-bound", ArchiveSecret: "archive-secret", RSAPrivateKey: privatePEM}
	handler := NewAdminHandler(config, store, archive)
	request := httptest.NewRequest(http.MethodPost, "/work-message/archive/media", strings.NewReader(`{"corp_id":4,"wx_corpid":"ww-bound","sdkFileId":"`+sdkFileID+`"}`))
	request.Header.Set("Authorization", "Bearer "+config.AdminToken)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadGateway || !strings.Contains(recorder.Body.String(), `"errcode":"ARCHIVE_MEDIA_SDK_ERROR"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	for _, forbidden := range []string{sdkFileID, config.ArchiveSecret, "PRIVATE KEY"} {
		if strings.Contains(recorder.Body.String(), forbidden) {
			t.Fatalf("error response leaked %q: %s", forbidden, recorder.Body.String())
		}
	}
}

func TestSanitizeArchiveMediaErrorUsesStrictTypedAllowlist(t *testing.T) {
	for _, test := range []struct {
		code       string
		wantCode   string
		wantStatus int
	}{
		{code: "MEDIA_MISSING", wantCode: "ARCHIVE_MEDIA_MISSING", wantStatus: http.StatusNotFound},
		{code: "MEDIA_CORRUPT", wantCode: "ARCHIVE_MEDIA_CORRUPT", wantStatus: http.StatusUnprocessableEntity},
		{code: "MEDIA_SDK_ERROR", wantCode: "ARCHIVE_MEDIA_SDK_ERROR", wantStatus: http.StatusBadGateway},
		{code: "SECRET_LOCATOR_DO_NOT_REFLECT", wantCode: "ARCHIVE_MEDIA_REQUEST_FAILED", wantStatus: http.StatusBadGateway},
	} {
		gotCode, gotStatus := sanitizeArchiveMediaError(testMediaCodedError{code: test.code})
		if gotCode != test.wantCode || gotStatus != test.wantStatus || strings.Contains(gotCode, "SECRET_LOCATOR") {
			t.Fatalf("code=%q status=%d", gotCode, gotStatus)
		}
	}
}

type testMediaCodedError struct{ code string }

func (e testMediaCodedError) Error() string          { return "untrusted: " + e.code }
func (e testMediaCodedError) MediaErrorCode() string { return e.code }

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
