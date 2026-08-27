package archivebridge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/archivefixture"
	"jiyi/mochat-go/internal/wecomarchivedemo"
)

const testBridgeBearer = "local-bridge-bearer-012345678901234567890123456789"

type fakeFinanceDriver struct{}

func (fakeFinanceDriver) FetchPage(context.Context, uint64, uint32) (wecomarchivedemo.ArchivePage, error) {
	return wecomarchivedemo.ArchivePage{Messages: []json.RawMessage{json.RawMessage(`{"seq":1,"msgid":"finance-1","msgtype":"text","from":"a","tolist":["b"],"text":{"content":"hello"}}`)}}, nil
}

func (fakeFinanceDriver) FetchMediaWithTimeout(context.Context, string, string, int) (wecomarchivedemo.MediaChunk, error) {
	return wecomarchivedemo.MediaChunk{Data: []byte("chunk"), Finished: true}, nil
}

func TestHandlerRequiresBearerAndExactBinding(t *testing.T) {
	store := NewStore()
	if err := store.RegisterFinance(Binding{TenantID: 11, CorpID: 27, WXCorpID: "ww-self", IntegrationMode: ModeSelfBuilt}, fakeFinanceDriver{}); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(Config{BearerToken: testBridgeBearer}, store)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	for _, tc := range []struct {
		bearer string
		body   string
		want   int
	}{
		{"", `{"tenant_id":11,"corp_id":27,"wx_corpid":"ww-self","integration_mode":"self_built","seq":0,"limit":10}`, http.StatusUnauthorized},
		{testBridgeBearer, `{"tenant_id":12,"corp_id":27,"wx_corpid":"ww-self","integration_mode":"self_built","seq":0,"limit":10}`, http.StatusConflict},
		{testBridgeBearer, `{"tenant_id":11,"corp_id":27,"wx_corpid":"ww-self","integration_mode":"third_party_delegated","seq":0,"limit":10}`, http.StatusConflict},
		{testBridgeBearer, `{"tenant_id":11,"corp_id":27,"wx_corpid":"ww-self","integration_mode":"self_built","seq":0,"limit":10,"unknown":true}`, http.StatusBadRequest},
		{testBridgeBearer, `{"tenant_id":11,"corp_id":27,"wx_corpid":"ww-self","integration_mode":"self_built","seq":0,"limit":10}`, http.StatusOK},
	} {
		request, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/archive/messages", strings.NewReader(tc.body))
		if tc.bearer != "" {
			request.Header.Set("Authorization", "Bearer "+tc.bearer)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != tc.want {
			t.Fatalf("body=%s status=%d want=%d", tc.body, response.StatusCode, tc.want)
		}
	}
}

func TestHandlerServesFinanceMessagesAndMedia(t *testing.T) {
	store := NewStore()
	binding := Binding{TenantID: 11, CorpID: 27, WXCorpID: "ww-self", IntegrationMode: ModeSelfBuilt}
	if err := store.RegisterFinance(binding, fakeFinanceDriver{}); err != nil {
		t.Fatal(err)
	}
	handler, _ := NewHandler(Config{BearerToken: testBridgeBearer}, store)
	response := postBridge(t, handler, "/v1/archive/messages", map[string]any{
		"tenant_id": 11, "corp_id": 27, "wx_corpid": "ww-self", "integration_mode": ModeSelfBuilt, "seq": 0, "limit": 10,
	})
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"content_policy":"plaintext"`) || !strings.Contains(response.Body.String(), `"hello"`) {
		t.Fatalf("messages response=%d %s", response.Code, response.Body.String())
	}
	response = postBridge(t, handler, "/v1/archive/media/chunks", map[string]any{
		"tenant_id": 11, "corp_id": 27, "wx_corpid": "ww-self", "integration_mode": ModeSelfBuilt, "sdkFileId": "media-1", "indexBuf": "", "timeoutSeconds": 5,
	})
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), base64.StdEncoding.EncodeToString([]byte("chunk"))) {
		t.Fatalf("media response=%d %s", response.Code, response.Body.String())
	}
}

func TestHandlerServesDelegatedMetadataAndComponentWithoutPlaintextLeak(t *testing.T) {
	provider, err := archivefixture.NewDataZoneProvider("ww-delegated")
	if err != nil {
		t.Fatal(err)
	}
	item, err := provider.Append(archivefixture.DataZoneContent{
		Sequence: 1, MessageID: "data-zone-1", Type: "voice", Sender: "a", Receivers: []string{"b"},
		Body: []byte("fixture-private-audio"), FileName: "message.wav", MIMEType: "audio/wav",
	})
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore()
	binding := Binding{TenantID: 21, CorpID: 37, WXCorpID: "ww-delegated", IntegrationMode: ModeThirdPartyDelegated}
	if err := store.RegisterDataZone(binding, provider); err != nil {
		t.Fatal(err)
	}
	handler, _ := NewHandler(Config{BearerToken: testBridgeBearer}, store)
	response := postBridge(t, handler, "/v1/archive/messages", map[string]any{
		"tenant_id": 21, "corp_id": 37, "wx_corpid": "ww-delegated", "integration_mode": ModeThirdPartyDelegated, "seq": 0, "limit": 10,
	})
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"content_policy":"component"`) || !strings.Contains(response.Body.String(), item.EncryptedSecretKey) || strings.Contains(response.Body.String(), "fixture-private-audio") {
		t.Fatalf("metadata response=%d %s", response.Code, response.Body.String())
	}
	response = postBridge(t, handler, "/v1/archive/component/session", map[string]any{
		"tenant_id": 21, "corp_id": 37, "wx_corpid": "ww-delegated", "integration_mode": ModeThirdPartyDelegated,
		"msgid": item.MessageID, "public_key_ver": item.PublicKeyVersion, "encrypted_secret_key": item.EncryptedSecretKey,
	})
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(base64.StdEncoding.EncodeToString([]byte("fixture-private-audio")))) {
		t.Fatalf("component response=%d %s", response.Code, response.Body.String())
	}
	response = postBridge(t, handler, "/v1/archive/media/chunks", map[string]any{
		"tenant_id": 21, "corp_id": 37, "wx_corpid": "ww-delegated", "integration_mode": ModeThirdPartyDelegated, "sdkFileId": "not-allowed", "timeoutSeconds": 5,
	})
	if response.Code != http.StatusConflict {
		t.Fatalf("delegated media status=%d body=%s", response.Code, response.Body.String())
	}
}

func postBridge(t *testing.T, handler http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	request.Header.Set("Authorization", "Bearer "+testBridgeBearer)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
