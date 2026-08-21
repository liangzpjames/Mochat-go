package dashboard

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoomWelcomeWeComClientDeletesContactWay(t *testing.T) {
	var gotConfigID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cgi-bin/gettoken" {
			_, _ = w.Write([]byte(`{"errcode":0,"access_token":"token-1","expires_in":7200}`))
			return
		}
		if r.URL.Path != "/cgi-bin/externalcontact/del_contact_way" || r.URL.Query().Get("access_token") != "token-1" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		_ = json.Unmarshal(body, &payload)
		gotConfigID = payload["config_id"]
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer server.Close()

	client := NewRoomWelcomeWeComClient(server.URL)
	if err := client.DeleteContactWay(context.Background(), RoomWelcomeCorpCredential{WXCorpID: "ww-test", ContactSecret: "secret"}, "config-11"); err != nil {
		t.Fatal(err)
	}
	if gotConfigID != "config-11" {
		t.Fatalf("config id = %q", gotConfigID)
	}
}

func TestChannelCodeBatchInvalidateReportsPartialProviderFailure(t *testing.T) {
	store := &fakeChannelCodeLifecycleStore{
		fakeChannelCodeStore: &fakeChannelCodeStore{users: map[int]User{1: {ID: 1}}},
		configs: map[int]channelCodeProviderConfig{
			11: {credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-test", ContactSecret: "secret"}, configID: "config-11"},
			12: {credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-test", ContactSecret: "secret"}, configID: "config-12"},
		},
	}
	client := &fakeChannelCodeLifecycleWeComClient{failConfigID: "config-12"}
	handler := NewChannelCodeHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{})
	handler.wecom = client

	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/channelCode/batchInvalidate", strings.NewReader(`{"ids":[11,12]}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.BatchInvalidate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	items := body["data"].(map[string]any)["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items = %#v", items)
	}
	if items[0].(map[string]any)["success"] != true || items[1].(map[string]any)["success"] != false {
		t.Fatalf("partial result = %#v", items)
	}
	if len(store.states) != 1 || store.states[0].id != 11 || store.states[0].state != "invalidated" {
		t.Fatalf("states = %#v", store.states)
	}
}

type channelCodeProviderConfig struct {
	credential RoomWelcomeCorpCredential
	configID   string
}

type fakeChannelCodeLifecycleStore struct {
	*fakeChannelCodeStore
	configs map[int]channelCodeProviderConfig
	states  []struct {
		id    int
		state string
	}
}

func (s *fakeChannelCodeLifecycleStore) ChannelCodeProviderConfig(_ context.Context, id int, _ int) (RoomWelcomeCorpCredential, string, bool, error) {
	config, ok := s.configs[id]
	if !ok {
		return RoomWelcomeCorpCredential{}, "", false, nil
	}
	return config.credential, config.configID, true, nil
}

func (s *fakeChannelCodeLifecycleStore) SetChannelCodeLifecycle(_ context.Context, id int, _ int, state string, _ string, _ string) error {
	s.states = append(s.states, struct {
		id    int
		state string
	}{id: id, state: state})
	return nil
}

type fakeChannelCodeLifecycleWeComClient struct {
	fakeChannelCodeWeComClient
	failConfigID string
}

func (c *fakeChannelCodeLifecycleWeComClient) DeleteContactWay(_ context.Context, _ RoomWelcomeCorpCredential, configID string) error {
	if configID == c.failConfigID {
		return context.DeadlineExceeded
	}
	return nil
}
