package archive

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
	server := httptest.NewServer(wecomarchivedemo.NewAdminHandler(wecomarchivedemo.Config{
		AdminToken: token, CorpID: wxCorpID, PullLimit: 100, TimeoutSeconds: 5,
	}, evidence, service))
	defer server.Close()

	client, err := NewBridgeArchiveClient(server.URL, token, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewBridgeSource(client, Scope{TenantID: 11, CorpID: 27}, wxCorpID)
	if err != nil {
		t.Fatal(err)
	}
	page, err := source.Fetch(context.Background(), Scope{TenantID: 11, CorpID: 27}, Cursor{}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 9 {
		t.Fatalf("messages=%d", len(page.Messages))
	}
	mixed := page.Messages[7]
	fileIDs := fixture.MediaFileIDs()
	wantIDs := []string{fileIDs["mixed"], fileIDs["missing"], fileIDs["corrupt"]}
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
}

func TestBridgeMediaNon2xxReturnsBoundedSanitizedFetchError(t *testing.T) {
	const secretLocator = "MOCHAT-LOCAL-ACCEPTANCE-SECRET-LOCATOR"
	for _, code := range []string{"ARCHIVE_MEDIA_MISSING", "ARCHIVE_MEDIA_CORRUPT"} {
		t.Run(code, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
			CorpID   int64  `json:"corp_id"`
			WXCorpID string `json:"wx_corpid"`
			Seq      int64  `json:"seq"`
			Limit    int    `json:"limit"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.CorpID != 27 || request.WXCorpID != "ww-local-acceptance" || request.Seq != 40 || request.Limit != 9 {
			t.Fatalf("request = %#v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","messages":[
			{"seq":41,"msgid":"text-1","action":"send","from":"employee","tolist":["contact"],"msgtime":1700000000000,"msgtype":"text","text":{"content":"hello"}},
			{"seq":42,"msgid":"image-1","action":"send","from":"contact","tolist":["employee"],"msgtime":1700000001000,"msgtype":"image","image":{"sdkfileid":"` + sdkFileID + `","md5sum":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","filesize":21}},
			{"seq":43,"msgid":"voice-1","action":"send","from":"employee","tolist":["contact"],"msgtime":1700000002000,"msgtype":"voice","voice":{"sdkfileid":"voice-secret","voice_size":22,"play_length":3}},
			{"seq":44,"msgid":"video-1","action":"send","from":"employee","tolist":["contact"],"msgtime":1700000003000,"msgtype":"video","video":{"sdkfileid":"video-secret","filesize":23}},
			{"seq":45,"msgid":"file-1","action":"send","from":"employee","tolist":["contact"],"msgtime":1700000004000,"msgtype":"file","file":{"sdkfileid":"file-secret","filename":"../../proposal.pdf","fileext":"pdf","filesize":24}},
			{"seq":46,"msgid":"link-1","action":"send","from":"employee","tolist":["contact"],"msgtime":1700000005000,"msgtype":"link","link":{"title":"safe title","link_url":"https://example.invalid"}},
			{"seq":47,"msgid":"location-1","action":"send","from":"employee","tolist":["contact"],"msgtime":1700000006000,"msgtype":"location","location":{"address":"Shanghai","latitude":31.2,"longitude":121.5}},
			{"seq":48,"msgid":"mixed-1","action":"send","from":"employee","tolist":["contact"],"msgtime":1700000007000,"msgtype":"mixed","mixed":{"item":[{"type":"text","content":"mixed text"},{"type":"image","content":{"sdkfileid":"mixed-image-secret","filesize":25}}]}},
			{"seq":49,"msgid":"unknown-1","action":"send","from":"employee","tolist":["contact"],"msgtime":1700000008000,"msgtype":"future_type","future_type":{"sdkfileid":"unknown-secret","value":"retained"}}
		]}`))
	}))
	defer server.Close()

	client, err := NewBridgeArchiveClient(server.URL, bearer, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewBridgeSource(client, Scope{TenantID: 11, CorpID: 27}, "ww-local-acceptance")
	if err != nil {
		t.Fatal(err)
	}
	page, err := source.Fetch(context.Background(), Scope{TenantID: 11, CorpID: 27}, Cursor{Sequence: 40}, 9)
	if err != nil {
		t.Fatal(err)
	}
	if source.Kind() != providers.SourceExternal || source.SourceID() != "wecom:ww-local-acceptance" || source.Namespace() != "wecom:ww-local-acceptance" {
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
			source, err := NewBridgeSource(client, Scope{TenantID: 11, CorpID: 27}, "ww-local-acceptance")
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
	source, _ := NewBridgeSource(client, Scope{TenantID: 11, CorpID: 27}, "ww-local-acceptance")
	if _, err := source.Fetch(context.Background(), Scope{TenantID: 12, CorpID: 27}, Cursor{}, 1); err == nil {
		t.Fatal("scope mismatch unexpectedly succeeded")
	}
}
