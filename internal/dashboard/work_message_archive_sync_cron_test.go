package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
	cron := NewWorkMessageArchiveSyncCron(store, client, slog.New(slog.NewTextHandler(io.Discard, nil))).WithLimit(2)

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

func TestWorkMessageArchiveSyncCronKeepsEmptyPollSilent(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	cron := NewWorkMessageArchiveSyncCron(&fakeWorkMessageArchiveSyncStore{}, &fakeWorkMessageArchiveSyncClient{}, logger)
	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if logs.Len() != 0 {
		t.Fatalf("empty poll logged: %s", logs.String())
	}
}

func TestWorkMessageArchiveSyncCronReportsEnabledCorpWithMissingConfiguration(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	store := &fakeWorkMessageArchiveSyncStore{corps: []WorkMessageArchiveCorp{{
		CorpID: 9, WXCorpID: "ww-missing", ChatSecret: "", RSAPublicKey: "public-value", RSAPrivateKey: "private-value",
	}}}
	client := &fakeWorkMessageArchiveSyncClient{}
	err := NewWorkMessageArchiveSyncCron(store, client, logger).RunOnce(context.Background())
	if err == nil || !strings.Contains(err.Error(), "configuration is incomplete") {
		t.Fatalf("missing configuration error = %v", err)
	}
	text := logs.String()
	for _, required := range []string{`"event":"archive_sync_failed"`, `"corp_id":9`, `"items_failed":1`} {
		if !strings.Contains(text, required) {
			t.Fatalf("missing %q: %s", required, text)
		}
	}
	for _, forbidden := range []string{"ww-missing", "public-value", "private-value"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("configuration log leaked %q: %s", forbidden, text)
		}
	}
	if len(client.corpIDs) != 0 {
		t.Fatalf("client called for invalid configuration: %v", client.corpIDs)
	}
}

func TestWorkMessageArchiveSyncCronLogsNonEmptyAndFailureOutcomes(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	store := &fakeWorkMessageArchiveSyncStore{corps: []WorkMessageArchiveCorp{{
		CorpID: 7, WXCorpID: "ww-go", ChatSecret: "archive-secret", RSAPublicKey: "archive-public", RSAPrivateKey: "archive-private",
	}}}
	client := &fakeWorkMessageArchiveSyncClient{messages: []WorkMessageArchiveMessage{{Seq: 1, MsgID: "message-1"}}}
	if err := NewWorkMessageArchiveSyncCron(store, client, logger).RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), "archive_sync_completed") || !strings.Contains(logs.String(), `"messages_fetched":1`) {
		t.Fatalf("non-empty log = %s", logs.String())
	}

	logs.Reset()
	store.corpsErr = context.DeadlineExceeded
	if err := NewWorkMessageArchiveSyncCron(store, client, logger).RunOnce(context.Background()); err == nil {
		t.Fatal("expected archive sync failure")
	}
	if !strings.Contains(logs.String(), "archive_sync_failed") || !strings.Contains(logs.String(), `"level":"ERROR"`) {
		t.Fatalf("failure log = %s", logs.String())
	}
}

func TestWorkMessageArchiveSyncCronThrottlesRepeatedFailuresAndLogsRecovery(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	store := &fakeWorkMessageArchiveSyncStore{corpsErr: context.DeadlineExceeded}
	cron := NewWorkMessageArchiveSyncCron(store, &fakeWorkMessageArchiveSyncClient{}, logger)

	for index := 0; index < 3; index++ {
		if err := cron.RunOnce(context.Background()); err == nil {
			t.Fatal("expected archive sync failure")
		}
	}
	if count := strings.Count(logs.String(), `"event":"archive_sync_failed"`); count != 1 {
		t.Fatalf("failure log count = %d logs=%s", count, logs.String())
	}
	if !strings.Contains(logs.String(), `"retry_count":1`) {
		t.Fatalf("failure log missing retry count: %s", logs.String())
	}

	store.corpsErr = nil
	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(logs.String(), `"event":"archive_sync_recovered"`); count != 1 {
		t.Fatalf("recovery log count = %d logs=%s", count, logs.String())
	}
}

func TestWorkMessageArchiveManualSyncDoesNotSharePeriodicFailureState(t *testing.T) {
	t.Run("manual failure always logs", func(t *testing.T) {
		var logs bytes.Buffer
		store := &fakeWorkMessageArchiveSyncStore{corpsErr: context.DeadlineExceeded}
		cron := NewWorkMessageArchiveSyncCron(store, &fakeWorkMessageArchiveSyncClient{}, slog.New(slog.NewJSONHandler(&logs, nil)))
		if err := cron.RunOnce(context.Background()); err == nil {
			t.Fatal("expected periodic failure")
		}
		store.corpsErr = nil
		store.corps = []WorkMessageArchiveCorp{{CorpID: 9, WXCorpID: "ww-manual", ChatSecret: ""}}
		if err := cron.RunCorp(context.Background(), 9); err == nil {
			t.Fatal("expected manual failure")
		}
		if count := strings.Count(logs.String(), `"event":"archive_sync_failed"`); count != 2 {
			t.Fatalf("manual failure was throttled by periodic state: count=%d logs=%s", count, logs.String())
		}
	})

	t.Run("manual success does not recover periodic state", func(t *testing.T) {
		var logs bytes.Buffer
		store := &fakeWorkMessageArchiveSyncStore{corpsErr: context.DeadlineExceeded}
		cron := NewWorkMessageArchiveSyncCron(store, &fakeWorkMessageArchiveSyncClient{}, slog.New(slog.NewJSONHandler(&logs, nil)))
		if err := cron.RunOnce(context.Background()); err == nil {
			t.Fatal("expected periodic failure")
		}
		store.corpsErr = nil
		store.corps = []WorkMessageArchiveCorp{{
			CorpID: 9, WXCorpID: "ww-manual", ChatSecret: "secret", RSAPublicKey: "public", RSAPrivateKey: "private",
		}}
		if err := cron.RunCorp(context.Background(), 9); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(logs.String(), `"event":"archive_sync_recovered"`) {
			t.Fatalf("manual success cleared periodic failure state: %s", logs.String())
		}
		if err := cron.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if count := strings.Count(logs.String(), `"event":"archive_sync_recovered"`); count != 1 {
			t.Fatalf("periodic recovery count=%d logs=%s", count, logs.String())
		}
	})
}

func TestWorkMessageArchiveBridgeErrorsDoNotContainResponseOrMessageBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("secret response body"))
	}))
	defer server.Close()
	client := NewWorkMessageArchiveBridgeClient(server.URL, "bridge-secret")
	_, err := client.FetchWorkMessageArchive(context.Background(), WorkMessageArchiveCorp{CorpID: 7, WXCorpID: "ww-test"}, 0, 10)
	if err == nil || !strings.Contains(err.Error(), "HTTP 502") || strings.Contains(err.Error(), "secret response body") {
		t.Fatalf("bridge error = %v", err)
	}
	_, err = parseWorkMessageArchiveBridgeMessage(json.RawMessage(`{"text":{"content":"private conversation"}}`))
	if err == nil || strings.Contains(err.Error(), "private conversation") || strings.Contains(err.Error(), `"text"`) {
		t.Fatalf("parse error leaked message: %v", err)
	}
}

func TestWorkMessageArchiveSyncCronRunCorpOnlyFetchesRequestedCorp(t *testing.T) {
	store := &fakeWorkMessageArchiveSyncStore{corps: []WorkMessageArchiveCorp{
		{CorpID: 4, WXCorpID: "ww-live", ChatSecret: "secret-4", RSAPublicKey: "public-4", RSAPrivateKey: "private-4"},
		{CorpID: 5, WXCorpID: "ww-other", ChatSecret: "secret-5", RSAPublicKey: "public-5", RSAPrivateKey: "private-5"},
	}}
	client := &fakeWorkMessageArchiveSyncClient{}
	cron := NewWorkMessageArchiveSyncCron(store, client, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := cron.RunCorp(context.Background(), 4); err != nil {
		t.Fatal(err)
	}
	if len(client.corpIDs) != 1 || client.corpIDs[0] != 4 {
		t.Fatalf("synced corps=%v, want [4]", client.corpIDs)
	}
}

func TestWorkMessageArchiveSyncCronSerializesEventAndPeriodicPulls(t *testing.T) {
	store := &fakeWorkMessageArchiveSyncStore{corps: []WorkMessageArchiveCorp{{
		CorpID: 4, WXCorpID: "ww-live", ChatSecret: "secret-4", RSAPublicKey: "public-4", RSAPrivateKey: "private-4",
	}}}
	client := &fakeWorkMessageArchiveSyncClient{delay: 50 * time.Millisecond}
	cron := NewWorkMessageArchiveSyncCron(store, client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	start := make(chan struct{})
	errs := make(chan error, 2)
	go func() {
		<-start
		errs <- cron.RunOnce(context.Background())
	}()
	go func() {
		<-start
		errs <- cron.RunCorp(context.Background(), 4)
	}()
	close(start)
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if client.maxConcurrent != 1 {
		t.Fatalf("max concurrent archive pulls=%d, want 1", client.maxConcurrent)
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
	corpsErr   error
}

func (s *fakeWorkMessageArchiveSyncStore) WorkMessageArchiveEnabledCorps(context.Context) ([]WorkMessageArchiveCorp, error) {
	return s.corps, s.corpsErr
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
	mu            sync.Mutex
	messages      []WorkMessageArchiveMessage
	seq           int64
	limit         int
	corpIDs       []int
	delay         time.Duration
	concurrent    int
	maxConcurrent int
}

func (c *fakeWorkMessageArchiveSyncClient) FetchWorkMessageArchive(_ context.Context, corp WorkMessageArchiveCorp, seq int64, limit int) ([]WorkMessageArchiveMessage, error) {
	c.mu.Lock()
	c.seq = seq
	c.limit = limit
	c.corpIDs = append(c.corpIDs, corp.CorpID)
	c.concurrent++
	if c.concurrent > c.maxConcurrent {
		c.maxConcurrent = c.concurrent
	}
	c.mu.Unlock()
	if c.delay > 0 {
		time.Sleep(c.delay)
	}
	c.mu.Lock()
	c.concurrent--
	c.mu.Unlock()
	return c.messages, nil
}
