package dashboard

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWorkMessageArchiveSyncCronFetchesAndAdvancesCursor(t *testing.T) {
	store := &fakeWorkMessageArchiveSyncStore{
		corps: []WorkMessageArchiveCorp{{
			CorpID: 7, WXCorpID: "ww-go", ChatSecret: "archive-secret",
			RSAPublicKey: "archive-public", RSAPrivateKey: "archive-private",
		}},
		cursor: 10,
		upserts: map[string]WorkMessageArchiveUpsertResult{
			"archive-msg-11": {Inserted: true, Resolved: true},
			"archive-msg-12": {Inserted: true, Resolved: true},
		},
	}
	client := &fakeWorkMessageArchiveSyncClient{messages: []WorkMessageArchiveMessage{
		{Seq: 10, MsgID: "old-msg"},
		{Seq: 11, MsgID: "archive-msg-11", From: "employee-a", ToList: []string{"external-a"}, MsgType: "text", ContentText: "报价"},
		{Seq: 12, MsgID: "archive-msg-12", From: "external-a", ToList: []string{"employee-a"}, MsgType: "text", ContentText: "马上下单"},
	}}
	cron := NewWorkMessageArchiveSyncCron(store, client, log.New(io.Discard, "", 0)).WithLimit(2)

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.seq != 10 || client.limit != 2 {
		t.Fatalf("client request = seq %d limit %d", client.seq, client.limit)
	}
	if len(store.messages) != 2 {
		t.Fatalf("upsert count = %d", len(store.messages))
	}
	if store.updatedSeq != 12 {
		t.Fatalf("updated cursor = %d", store.updatedSeq)
	}
}

func TestWorkMessageArchiveSyncCronRequiresDependencies(t *testing.T) {
	err := NewWorkMessageArchiveSyncCron(nil, nil, nil).RunOnce(context.Background())
	if err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestWorkMessageArchiveBridgeClientPostsAndParsesMessages(t *testing.T) {
	var requestPath string
	var authorization string
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		authorization = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"errcode": 0,
			"errmsg": "ok",
			"messages": [{
				"seq": 42,
				"msgid": "archive-msg-42",
				"action": "send",
				"from": "employee-a",
				"tolist": ["external-a"],
				"msgtype": "text",
				"msgtime": 1783342800000,
				"text": {"content": "需要报价"}
			}]
		}`))
	}))
	defer server.Close()

	client := NewWorkMessageArchiveBridgeClient(server.URL+"/", "bridge-token")
	messages, err := client.FetchWorkMessageArchive(context.Background(), WorkMessageArchiveCorp{
		CorpID:        7,
		WXCorpID:      "ww-go",
		ChatSecret:    "archive-secret",
		RSAPublicKey:  "archive-public",
		RSAPrivateKey: "archive-private",
	}, 41, 100)
	if err != nil {
		t.Fatal(err)
	}
	if requestPath != "/work-message/archive/messages" || authorization != "Bearer bridge-token" {
		t.Fatalf("request path/auth = %s %s", requestPath, authorization)
	}
	if requestBody["wx_corpid"] != "ww-go" || int(requestBody["corp_id"].(float64)) != 7 ||
		int(requestBody["seq"].(float64)) != 41 || int(requestBody["limit"].(float64)) != 100 {
		t.Fatalf("request body = %#v", requestBody)
	}
	for _, forbidden := range []string{"chat_secret", "rsa_public_key", "rsa_private_key"} {
		if _, ok := requestBody[forbidden]; ok {
			t.Fatalf("bridge request leaked %s: %#v", forbidden, requestBody)
		}
	}
	if len(messages) != 1 {
		t.Fatalf("messages = %#v", messages)
	}
	message := messages[0]
	if message.Seq != 42 || message.MsgID != "archive-msg-42" || message.From != "employee-a" || message.ToList[0] != "external-a" || message.MsgType != "text" || message.ContentText != "需要报价" {
		t.Fatalf("parsed message = %#v", message)
	}
	wantTime := time.UnixMilli(1783342800000)
	if !message.MsgTime.Equal(wantTime) {
		t.Fatalf("msg time = %s want %s", message.MsgTime, wantTime)
	}
}

type fakeWorkMessageArchiveSyncStore struct {
	corps      []WorkMessageArchiveCorp
	cursor     int64
	upserts    map[string]WorkMessageArchiveUpsertResult
	messages   []WorkMessageArchiveMessage
	updatedSeq int64
}

func (s *fakeWorkMessageArchiveSyncStore) WorkMessageArchiveEnabledCorps(context.Context) ([]WorkMessageArchiveCorp, error) {
	return s.corps, nil
}

func (s *fakeWorkMessageArchiveSyncStore) WorkMessageArchiveCursor(context.Context, int) (int64, error) {
	return s.cursor, nil
}

func (s *fakeWorkMessageArchiveSyncStore) UpsertWorkMessageArchive(_ context.Context, _ int, message WorkMessageArchiveMessage) (WorkMessageArchiveUpsertResult, error) {
	s.messages = append(s.messages, message)
	if result, ok := s.upserts[message.MsgID]; ok {
		return result, nil
	}
	return WorkMessageArchiveUpsertResult{Inserted: true, Resolved: true}, nil
}

func (s *fakeWorkMessageArchiveSyncStore) UpdateWorkMessageArchiveCursor(_ context.Context, _ int, lastSeq int64) error {
	s.updatedSeq = lastSeq
	return nil
}

type fakeWorkMessageArchiveSyncClient struct {
	messages []WorkMessageArchiveMessage
	seq      int64
	limit    int
}

func (c *fakeWorkMessageArchiveSyncClient) FetchWorkMessageArchive(_ context.Context, _ WorkMessageArchiveCorp, seq int64, limit int) ([]WorkMessageArchiveMessage, error) {
	c.seq = seq
	c.limit = limit
	return c.messages, nil
}
