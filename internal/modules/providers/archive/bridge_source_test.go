package archive

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"jiyi/mochat-go/internal/modules/providers"
	"jiyi/mochat-go/internal/testfixtures/archivesource"
	"jiyi/mochat-go/internal/wecomarchivedemo"
)

func TestBridgeSourceUsesRealArchiveFixtureMixedShape(t *testing.T) {
	fixture, err := archivesource.NewArchiveFixture()
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	evidence, err := wecomarchivedemo.NewEvidenceStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := wecomarchivedemo.NewArchiveService(fixture, fixture.PrivateKeyPEM(), evidence, 100, 5)
	if err != nil {
		t.Fatal(err)
	}
	const token = "MOCHAT-LOCAL-ACCEPTANCE-BEARER-0123456789"
	const wxCorpID = "ww-local-acceptance"
	financeHandler := wecomarchivedemo.NewAdminHandler(wecomarchivedemo.Config{
		AdminToken: token, CorpID: wxCorpID, PullLimit: 100, TimeoutSeconds: 5,
	}, evidence, service)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorded := httptest.NewRecorder()
		financeHandler.ServeHTTP(recorded, r)
		for key, values := range recorded.Header() {
			w.Header()[key] = append([]string(nil), values...)
		}
		if recorded.Code != http.StatusOK || !strings.Contains(r.URL.Path, "messages") {
			w.WriteHeader(recorded.Code)
			_, _ = w.Write(recorded.Body.Bytes())
			return
		}
		var payload map[string]any
		if json.Unmarshal(recorded.Body.Bytes(), &payload) != nil {
			t.Fatal("invalid Finance adapter response")
		}
		items, _ := payload["messages"].([]any)
		if len(items) == 0 {
			items, _ = payload["chatdata"].([]any)
		}
		for _, item := range items {
			if object, ok := item.(map[string]any); ok {
				object["source_mode"] = IntegrationModeSelfBuilt
			}
		}
		payload["messages"] = items
		w.WriteHeader(recorded.Code)
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	client, err := NewBridgeArchiveClient(server.URL, token, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewBridgeSource(client, Scope{TenantID: 11, CorpID: 27}, wxCorpID, IntegrationModeSelfBuilt)
	if err != nil {
		t.Fatal(err)
	}
	page, err := source.Fetch(context.Background(), Scope{TenantID: 11, CorpID: 27}, Cursor{}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 10 {
		t.Fatalf("messages=%d", len(page.Messages))
	}
	fileIDs := fixture.MediaFileIDs()
	missing := page.Messages[7]
	if missing.MsgType != "image" || len(missing.Media) != 1 || missing.Media[0].SDKFileID != fileIDs["missing"] {
		t.Fatalf("independent missing media=%#v", missing)
	}
	mixed := page.Messages[8]
	wantIDs := []string{fileIDs["mixed"], fileIDs["corrupt"]}
	if mixed.MsgType != "mixed" || len(mixed.Media) != len(wantIDs) {
		t.Fatalf("mixed=%#v", mixed)
	}
	for index, wantID := range wantIDs {
		media := mixed.Media[index]
		if media.Type != "image" || media.SDKFileID != wantID || media.ExpectedSize <= 0 || media.ExpectedMD5 == "" {
			t.Fatalf("mixed media[%d]=%#v", index, media)
		}
		if strings.Contains(mixed.RawJSON, wantID) || strings.Contains(mixed.ContentRaw, wantID) {
			t.Fatal("mixed public payload leaked SDK locator")
		}
	}
	if page.Messages[9].MsgType != "future_archive_type" {
		t.Fatalf("unknown message=%#v", page.Messages[9])
	}
}

func TestBridgeMediaNon2xxReturnsBoundedSanitizedFetchError(t *testing.T) {
	const secretLocator = "MOCHAT-LOCAL-ACCEPTANCE-SECRET-LOCATOR"
	for _, code := range []string{"ARCHIVE_MEDIA_MISSING", "ARCHIVE_MEDIA_CORRUPT"} {
		t.Run(code, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"errcode":"` + code + `","errmsg":"` + secretLocator + `"}`))
			}))
			defer server.Close()
			client, err := NewBridgeArchiveClient(server.URL, "MOCHAT-LOCAL-ACCEPTANCE-BEARER-0123456789", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.FetchMedia(context.Background(), Scope{TenantID: 11, CorpID: 27}, "ww-local", secretLocator, "")
			var fetchErr *MediaFetchError
			if !errors.As(err, &fetchErr) || fetchErr.Code != code {
				t.Fatalf("error=%T %v", err, err)
			}
			if strings.Contains(err.Error(), secretLocator) || strings.Contains(err.Error(), "errmsg") {
				t.Fatalf("error leaked bridge response: %v", err)
			}
			if requests.Load() != 1 {
				t.Fatalf("terminal media response triggered %d requests, want no legacy endpoint fallback", requests.Load())
			}
		})
	}
}

func TestBridgeSourceParsesSupportedMessagesAndKeepsSDKFileIDInternal(t *testing.T) {
	const bearer = "MOCHAT-LOCAL-ACCEPTANCE-BEARER-0123456789"
	const sdkFileID = "MOCHAT-LOCAL-ACCEPTANCE-SDKFILE-image-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+bearer {
			t.Fatalf("authorization header = %q", r.Header.Get("Authorization"))
		}
		var request struct {
			TenantID int64  `json:"tenant_id"`
			CorpID   int64  `json:"corp_id"`
			WXCorpID string `json:"wx_corpid"`
			Mode     string `json:"integration_mode"`
			Seq      int64  `json:"seq"`
			Limit    int    `json:"limit"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.TenantID != 11 || request.CorpID != 27 || request.WXCorpID != "ww-local-acceptance" || request.Mode != IntegrationModeSelfBuilt || request.Seq != 40 || request.Limit != 9 {
			t.Fatalf("request = %#v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","messages":[
			{"source_mode":"self_built","seq":41,"msgid":"text-1","action":"send","from":"employee","tolist":["contact"],"msgtime":1700000000000,"msgtype":"text","text":{"content":"hello"}},
			{"source_mode":"self_built","seq":42,"msgid":"image-1","action":"send","from":"contact","tolist":["employee"],"msgtime":1700000001000,"msgtype":"image","image":{"sdkfileid":"` + sdkFileID + `","md5sum":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","filesize":21}},
			{"source_mode":"self_built","seq":43,"msgid":"voice-1","action":"send","from":"employee","tolist":["contact"],"msgtime":1700000002000,"msgtype":"voice","voice":{"sdkfileid":"voice-secret","voice_size":22,"play_length":3}},
			{"source_mode":"self_built","seq":44,"msgid":"video-1","action":"send","from":"employee","tolist":["contact"],"msgtime":1700000003000,"msgtype":"video","video":{"sdkfileid":"video-secret","filesize":23}},
			{"source_mode":"self_built","seq":45,"msgid":"file-1","action":"send","from":"employee","tolist":["contact"],"msgtime":1700000004000,"msgtype":"file","file":{"sdkfileid":"file-secret","filename":"../../proposal.pdf","fileext":"pdf","filesize":24}},
			{"source_mode":"self_built","seq":46,"msgid":"link-1","action":"send","from":"employee","tolist":["contact"],"msgtime":1700000005000,"msgtype":"link","link":{"title":"safe title","link_url":"https://example.invalid"}},
			{"source_mode":"self_built","seq":47,"msgid":"location-1","action":"send","from":"employee","tolist":["contact"],"msgtime":1700000006000,"msgtype":"location","location":{"address":"Shanghai","latitude":31.2,"longitude":121.5}},
			{"source_mode":"self_built","seq":48,"msgid":"mixed-1","action":"send","from":"employee","tolist":["contact"],"msgtime":1700000007000,"msgtype":"mixed","mixed":{"item":[{"type":"text","content":"mixed text"},{"type":"image","content":{"sdkfileid":"mixed-image-secret","filesize":25}}]}},
			{"source_mode":"self_built","seq":49,"msgid":"unknown-1","action":"send","from":"employee","tolist":["contact"],"msgtime":1700000008000,"msgtype":"future_type","future_type":{"sdkfileid":"unknown-secret","value":"retained"}}
		]}`))
	}))
	defer server.Close()

	client, err := NewBridgeArchiveClient(server.URL, bearer, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewBridgeSource(client, Scope{TenantID: 11, CorpID: 27}, "ww-local-acceptance", IntegrationModeSelfBuilt)
	if err != nil {
		t.Fatal(err)
	}
	page, err := source.Fetch(context.Background(), Scope{TenantID: 11, CorpID: 27}, Cursor{Sequence: 40}, 9)
	if err != nil {
		t.Fatal(err)
	}
	if source.Kind() != providers.SourceExternal || source.SourceID() != "wecom:self_built:ww-local-acceptance" || source.Namespace() != "wecom:self_built:ww-local-acceptance" {
		t.Fatalf("source identity = %s/%s/%s", source.Kind(), source.SourceID(), source.Namespace())
	}
	if len(page.Messages) != 9 || page.NextCursor.Sequence != 49 || !page.HasMore {
		t.Fatalf("page = %#v", page)
	}
	types := []string{"text", "image", "voice", "video", "file", "link", "location", "mixed", "future_type"}
	for index, message := range page.Messages {
		if message.MsgType != types[index] {
			t.Fatalf("message %d type=%q", index, message.MsgType)
		}
		if strings.Contains(strings.ToLower(message.ContentRaw), "sdkfileid") || strings.Contains(message.ContentRaw, "secret") ||
			strings.Contains(strings.ToLower(message.RawJSON), "sdkfileid") || strings.Contains(message.RawJSON, "secret") {
			t.Fatalf("public message leaked sdk file id: %#v", message)
		}
	}
	if got := page.Messages[1].Media; len(got) != 1 || got[0].SDKFileID != sdkFileID || got[0].ExpectedSize != 21 || got[0].ExpectedMD5 != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("image media = %#v", got)
	}
	if got := page.Messages[7].Media; len(got) != 1 || got[0].SDKFileID != "mixed-image-secret" {
		t.Fatalf("mixed media = %#v", got)
	}
	if got := page.Messages[8].Media; len(got) != 0 {
		t.Fatalf("unknown media must not be scheduled: %#v", got)
	}
}

func TestBridgeSourceRejectsMissingOrMismatchedResponseMode(t *testing.T) {
	for _, sourceMode := range []string{"", IntegrationModeThirdPartyDelegated} {
		t.Run("mode_"+sourceMode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"errcode":0,"messages":[{"source_mode":"` + sourceMode + `","seq":1,"msgid":"m-1","msgtype":"text","text":{"content":"hello"}}]}`))
			}))
			defer server.Close()
			client, _ := NewBridgeArchiveClient(server.URL, "MOCHAT-LOCAL-ACCEPTANCE-BEARER-0123456789", server.Client())
			source, _ := NewBridgeSource(client, Scope{TenantID: 11, CorpID: 27}, "ww-local", IntegrationModeSelfBuilt)
			page, err := source.Fetch(context.Background(), Scope{TenantID: 11, CorpID: 27}, Cursor{}, 10)
			if err == nil || len(page.Messages) != 0 || page.NextCursor.Sequence != 0 {
				t.Fatalf("page=%#v err=%v", page, err)
			}
		})
	}
}

func TestBridgeSourceFailsClosedOnScopeAuthCorpAndInvalidJSON(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "auth", status: http.StatusUnauthorized, body: `unauthorized`},
		{name: "corp", status: http.StatusBadRequest, body: `{"errcode":400,"errmsg":"archive corp binding mismatch"}`},
		{name: "bridge", status: http.StatusBadGateway, body: `{"errcode":502,"errmsg":"bridge failed"}`},
		{name: "invalid_json", status: http.StatusOK, body: `{`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			client, err := NewBridgeArchiveClient(server.URL, "MOCHAT-LOCAL-ACCEPTANCE-BEARER-0123456789", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			source, err := NewBridgeSource(client, Scope{TenantID: 11, CorpID: 27}, "ww-local-acceptance", IntegrationModeSelfBuilt)
			if err != nil {
				t.Fatal(err)
			}
			_, err = source.Fetch(context.Background(), Scope{TenantID: 11, CorpID: 27}, Cursor{}, 1)
			if err == nil || strings.Contains(err.Error(), "BEARER") {
				t.Fatalf("error = %v", err)
			}
		})
	}
	client, _ := NewBridgeArchiveClient("https://bridge.example", "MOCHAT-LOCAL-ACCEPTANCE-BEARER-0123456789", http.DefaultClient)
	source, _ := NewBridgeSource(client, Scope{TenantID: 11, CorpID: 27}, "ww-local-acceptance", IntegrationModeSelfBuilt)
	if _, err := source.Fetch(context.Background(), Scope{TenantID: 12, CorpID: 27}, Cursor{}, 1); err == nil {
		t.Fatal("scope mismatch unexpectedly succeeded")
	}
}

func TestBridgeSourceParsesDelegatedComponentWithoutPlaintextOrMedia(t *testing.T) {
	const bearer = "MOCHAT-LOCAL-ACCEPTANCE-BEARER-0123456789"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request["integration_mode"] != IntegrationModeThirdPartyDelegated || request["tenant_id"] != float64(11) {
			t.Fatalf("request=%#v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"messages":[{"source_mode":"third_party_delegated","seq":1,"msgid":"dz-1","msgtype":"voice","from":"a","tolist":["b"],"content_policy":"component","component_locator":{"msgid":"dz-1","public_key_ver":1,"encrypted_secret_key":"private-wrapped-key"}}]}`))
	}))
	defer server.Close()
	client, err := NewBridgeArchiveClient(server.URL, bearer, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewBridgeSource(client, Scope{TenantID: 11, CorpID: 27}, "ww-delegated", IntegrationModeThirdPartyDelegated)
	if err != nil {
		t.Fatal(err)
	}
	page, err := source.Fetch(context.Background(), Scope{TenantID: 11, CorpID: 27}, Cursor{}, 10)
	if err != nil || len(page.Messages) != 1 {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	message := page.Messages[0]
	if message.ContentPolicy != ContentPolicyComponent || message.Component == nil || message.Component.EncryptedSecretKey != "private-wrapped-key" || message.ContentText != "" || len(message.Media) != 0 {
		t.Fatalf("component message=%#v", message)
	}
	if strings.Contains(message.RawJSON, "private-wrapped-key") || strings.Contains(message.ContentRaw, "private-wrapped-key") {
		t.Fatalf("component locator leaked: %#v", message)
	}
}

func TestBridgeClientFetchesDelegatedComponentWithExactBinding(t *testing.T) {
	const bearer = "MOCHAT-LOCAL-ACCEPTANCE-BEARER-0123456789"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input map[string]any
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		if input["tenant_id"] != float64(11) || input["corp_id"] != float64(27) || input["integration_mode"] != IntegrationModeThirdPartyDelegated || input["encrypted_secret_key"] != "wrapped-key" {
			t.Fatalf("component input=%#v", input)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"msgtype":"voice","fileName":"message.wav","mimeType":"audio/wav","dataBase64":"Zml4dHVyZS1hdWRpbw=="}`))
	}))
	defer server.Close()
	client, err := NewBridgeArchiveClient(server.URL, bearer, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	content, err := client.FetchComponent(context.Background(), ComponentRequest{
		Scope: Scope{TenantID: 11, CorpID: 27}, WXCorpID: "ww-delegated", MessageID: "dz-1", PublicKeyVersion: 1, EncryptedSecretKey: "wrapped-key",
	})
	if err != nil || string(content.Data) != "fixture-audio" || content.MIMEType != "audio/wav" {
		t.Fatalf("content=%#v err=%v", content, err)
	}
}
