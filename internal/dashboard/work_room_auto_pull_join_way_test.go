package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestWorkRoomAutoPullJoinWayCreateGetsRealQRCode(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/cgi-bin/gettoken":
			_, _ = w.Write([]byte(`{"errcode":0,"access_token":"contact-token","expires_in":7200}`))
		case "/cgi-bin/externalcontact/groupchat/add_join_way":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(body["chat_id_list"], []any{"chat-b", "chat-a"}) {
				t.Fatalf("chat_id_list=%#v", body["chat_id_list"])
			}
			if body["auto_create_room"] != float64(1) || body["room_base_name"] != "服务群" || body["room_base_id"] != float64(8) {
				t.Fatalf("body=%#v", body)
			}
			if body["scene"] != float64(2) || body["remark"] != "售后入群" {
				t.Fatalf("scene/remark body=%#v", body)
			}
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","config_id":"join-1"}`))
		case "/cgi-bin/externalcontact/groupchat/get_join_way":
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","join_way":{"config_id":"join-1","qr_code":"https://wecom.example/join.png"}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewWorkRoomAutoPullJoinWayWeComClient(server.URL)
	result, err := client.CreateJoinWay(context.Background(), RoomWelcomeCorpCredential{WXCorpID: "ww", ContactSecret: "secret"}, WorkRoomAutoPullJoinWayPayload{
		QRCodeName: "售后入群", ChatIDs: []string{"chat-b", "chat-a"}, AutoCreateRoom: true, RoomBaseName: "服务群", RoomBaseID: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ConfigID != "join-1" || result.QRCodeURL != "https://wecom.example/join.png" {
		t.Fatalf("result=%#v", result)
	}
	if !reflect.DeepEqual(paths, []string{"/cgi-bin/gettoken", "/cgi-bin/externalcontact/groupchat/add_join_way", "/cgi-bin/externalcontact/groupchat/get_join_way"}) {
		t.Fatalf("paths=%#v", paths)
	}
}

func TestWorkRoomAutoPullJoinWayCreateCleansRemoteConfigWhenGetFails(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/cgi-bin/gettoken":
			_, _ = w.Write([]byte(`{"errcode":0,"access_token":"contact-token","expires_in":7200}`))
		case "/cgi-bin/externalcontact/groupchat/add_join_way":
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","config_id":"join-failed-get"}`))
		case "/cgi-bin/externalcontact/groupchat/get_join_way":
			_, _ = w.Write([]byte(`{"errcode":40058,"errmsg":"invalid config_id"}`))
		case "/cgi-bin/externalcontact/groupchat/del_join_way":
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewWorkRoomAutoPullJoinWayWeComClient(server.URL)
	_, err := client.CreateJoinWay(context.Background(), RoomWelcomeCorpCredential{WXCorpID: "ww", ContactSecret: "secret"}, WorkRoomAutoPullJoinWayPayload{QRCodeName: "售后入群", ChatIDs: []string{"chat-1"}})
	if err == nil {
		t.Fatal("expected get_join_way error")
	}
	if !reflect.DeepEqual(paths, []string{"/cgi-bin/gettoken", "/cgi-bin/externalcontact/groupchat/add_join_way", "/cgi-bin/externalcontact/groupchat/get_join_way", "/cgi-bin/externalcontact/groupchat/del_join_way"}) {
		t.Fatalf("paths=%#v", paths)
	}
}

func TestWorkRoomAutoPullJoinWayCreateCleansRemoteConfigWhenQRCodeIsEmpty(t *testing.T) {
	var deleted bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/gettoken":
			_, _ = w.Write([]byte(`{"errcode":0,"access_token":"contact-token","expires_in":7200}`))
		case "/cgi-bin/externalcontact/groupchat/add_join_way":
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","config_id":"join-empty-qr"}`))
		case "/cgi-bin/externalcontact/groupchat/get_join_way":
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","join_way":{"config_id":"join-empty-qr","qr_code":""}}`))
		case "/cgi-bin/externalcontact/groupchat/del_join_way":
			deleted = true
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewWorkRoomAutoPullJoinWayWeComClient(server.URL)
	_, err := client.CreateJoinWay(context.Background(), RoomWelcomeCorpCredential{WXCorpID: "ww", ContactSecret: "secret"}, WorkRoomAutoPullJoinWayPayload{QRCodeName: "售后入群", ChatIDs: []string{"chat-1"}})
	if err == nil || !deleted {
		t.Fatalf("err=%v deleted=%v", err, deleted)
	}
}

func TestWorkRoomAutoPullJoinWayCreateRejectsProviderErrcodeBeforePersistence(t *testing.T) {
	var addCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/gettoken":
			_, _ = w.Write([]byte(`{"errcode":0,"access_token":"contact-token","expires_in":7200}`))
		case "/cgi-bin/externalcontact/groupchat/add_join_way":
			addCalls++
			_, _ = w.Write([]byte(`{"errcode":48002,"errmsg":"api unauthorized"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewWorkRoomAutoPullJoinWayWeComClient(server.URL)
	_, err := client.CreateJoinWay(context.Background(), RoomWelcomeCorpCredential{WXCorpID: "ww", ContactSecret: "secret"}, WorkRoomAutoPullJoinWayPayload{QRCodeName: "售后入群", ChatIDs: []string{"chat-1"}})
	if err == nil || addCalls != 1 {
		t.Fatalf("err=%v addCalls=%d", err, addCalls)
	}
}

func TestWorkRoomAutoPullJoinWayUpdateAndDelete(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cgi-bin/gettoken" {
			_, _ = w.Write([]byte(`{"errcode":0,"access_token":"contact-token","expires_in":7200}`))
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		body["path"] = r.URL.Path
		requests = append(requests, body)
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer server.Close()

	client := NewWorkRoomAutoPullJoinWayWeComClient(server.URL)
	credential := RoomWelcomeCorpCredential{WXCorpID: "ww", ContactSecret: "secret"}
	payload := WorkRoomAutoPullJoinWayPayload{QRCodeName: "服务群", ChatIDs: []string{"chat-1"}}
	if err := client.UpdateJoinWay(context.Background(), credential, "join-1", payload); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteJoinWay(context.Background(), credential, "join-1"); err != nil {
		t.Fatal(err)
	}
	if requests[0]["path"] != "/cgi-bin/externalcontact/groupchat/update_join_way" || requests[0]["config_id"] != "join-1" {
		t.Fatalf("update=%#v", requests[0])
	}
	if requests[1]["path"] != "/cgi-bin/externalcontact/groupchat/del_join_way" || requests[1]["config_id"] != "join-1" {
		t.Fatalf("delete=%#v", requests[1])
	}
}
