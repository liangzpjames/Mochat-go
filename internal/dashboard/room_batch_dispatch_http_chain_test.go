package dashboard

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcapability"
)

// P0-3 fake HTTP full-chain evidence for the ROOM batch dispatch path. These
// tests run the REAL RoomWelcomeWeComClient against a local httptest.Server
// and lock the official room contract: chat_type=group + chat_id_list (no
// external_userid, no msgid as a request field), msgid as response-only,
// get_groupmsg_task/get_groupmsg_send_result cursor pagination, and the same
// submit error classification shared with the contact runner.

type fakeRoomBatchHTTPRuntimeStore struct {
	payload    RoomMessageBatchSendMessagePayload
	credential RoomWelcomeCorpCredential
}

func (s *fakeRoomBatchHTTPRuntimeStore) RoomBatchDispatchPayload(context.Context, dashboardprincipal.DashboardPrincipal, wecomcapability.Dispatch) (RoomMessageBatchSendMessagePayload, error) {
	return s.payload, nil
}

func (s *fakeRoomBatchHTTPRuntimeStore) RoomBatchDispatchCredential(context.Context, dashboardprincipal.DashboardPrincipal) (RoomWelcomeCorpCredential, bool, error) {
	return s.credential, true, nil
}

func roomBatchHTTPChainDispatch() wecomcapability.Dispatch {
	return wecomcapability.Dispatch{DispatchKind: string(wecomcapability.DispatchKindRoomBatch), TargetID: "room_batch:701:owner:101:chunk:1"}
}

// TestRoomBatchHTTPChainSubmitLocksOfficialRequestAndResponseMessageID locks
// the add_msg_template room contract: chat_type=group, sender, chat_id_list
// (and NO external_userid), text payload; msgid must NOT be sent as a request
// field and must surface as ProviderMessageID.
func TestRoomBatchHTTPChainSubmitLocksOfficialRequestAndResponseMessageID(t *testing.T) {
	chain := newContactBatchHTTPChain(t)
	chain.submitResp = func() (int, string) { return http.StatusOK, `{"errcode":0,"errmsg":"ok","msgid":"room-msg-1"}` }
	store := &fakeRoomBatchHTTPRuntimeStore{
		payload: RoomMessageBatchSendMessagePayload{
			Sender:  "employee-1",
			ChatIDs: []string{"chat-1", "chat-2"},
			Content: []ContactMessageBatchSendContent{{MsgType: "text", Content: "hello"}},
		},
		credential: RoomWelcomeCorpCredential{WXCorpID: "ww-test", ContactSecret: "contact-secret"},
	}
	sender := &RoomBatchDispatchSender{store: store, client: NewRoomWelcomeWeComClient(chain.server.URL)}
	result, err := sender.Submit(context.Background(), wecomcapability.DispatchSubmitRequest{
		Principal: contactBatchHTTPChainPrincipal(), Capability: wecomcapability.RoomBatchSend, Dispatch: roomBatchHTTPChainDispatch(),
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if result.ProviderMessageID != "room-msg-1" || !result.Submitted {
		t.Fatalf("result=%#v", result)
	}
	if chain.tokenCalls.Load() != 1 {
		t.Fatalf("gettoken calls=%d, want 1", chain.tokenCalls.Load())
	}
	if len(chain.submitBodies) != 1 {
		t.Fatalf("submit bodies=%d", len(chain.submitBodies))
	}
	body := chain.submitBodies[0]
	if body["chat_type"] != "group" {
		t.Fatalf("chat_type=%v", body["chat_type"])
	}
	if body["sender"] != "employee-1" {
		t.Fatalf("sender=%v", body["sender"])
	}
	chatIDs, ok := body["chat_id_list"].([]any)
	if !ok || len(chatIDs) != 2 || chatIDs[0] != "chat-1" || chatIDs[1] != "chat-2" {
		t.Fatalf("chat_id_list=%#v", body["chat_id_list"])
	}
	text, ok := body["text"].(map[string]any)
	if !ok || text["content"] != "hello" {
		t.Fatalf("text=%#v", body["text"])
	}
	if _, present := body["external_userid"]; present {
		t.Fatal("room payload must not contain external_userid")
	}
	if _, present := body["msgid"]; present {
		t.Fatal("msgid must be a response-only field and must not be sent as a request idempotency field")
	}
	// Second submit must reuse the cached token, not call gettoken again.
	if _, err := sender.Submit(context.Background(), wecomcapability.DispatchSubmitRequest{
		Principal: contactBatchHTTPChainPrincipal(), Capability: wecomcapability.RoomBatchSend, Dispatch: roomBatchHTTPChainDispatch(),
	}); err != nil {
		t.Fatalf("second submit: %v", err)
	}
	if chain.tokenCalls.Load() != 1 {
		t.Fatalf("gettoken calls after second submit=%d, want 1 (token cache)", chain.tokenCalls.Load())
	}
}

// TestRoomBatchHTTPChainPollWalksAllTaskAndResultPages locks cursor
// pagination completeness for both get_groupmsg_task and
// get_groupmsg_send_result and verifies every room target is aggregated as
// {ownerEmployeeID}:{chatID}.
func TestRoomBatchHTTPChainPollWalksAllTaskAndResultPages(t *testing.T) {
	chain := newContactBatchHTTPChain(t)
	chain.taskPages = func(cursor string) (int, string) {
		switch cursor {
		case "":
			return http.StatusOK, `{"errcode":0,"errmsg":"ok","task_list":[{"userid":"sender-1","status":1}],"next_cursor":"task-2"}`
		case "task-2":
			return http.StatusOK, `{"errcode":0,"errmsg":"ok","task_list":[{"userid":"sender-2","status":1}],"next_cursor":""}`
		default:
			t.Fatalf("unexpected task cursor=%q", cursor)
			return http.StatusInternalServerError, `{}`
		}
	}
	chain.resultPages = func(userID, cursor string) (int, string) {
		switch userID {
		case "sender-1":
			if cursor == "" {
				return http.StatusOK, `{"errcode":0,"errmsg":"ok","send_list":[{"userid":"sender-1","chat_id":"chat-a","status":1}],"next_cursor":"result-1-2"}`
			}
			return http.StatusOK, `{"errcode":0,"errmsg":"ok","send_list":[{"userid":"sender-1","chat_id":"chat-b","status":1}],"next_cursor":""}`
		case "sender-2":
			return http.StatusOK, `{"errcode":0,"errmsg":"ok","send_list":[{"userid":"sender-2","chat_id":"chat-c","status":1}],"next_cursor":""}`
		default:
			t.Fatalf("unexpected result userID=%q", userID)
			return http.StatusInternalServerError, `{}`
		}
	}
	poller := &RoomBatchDispatchPoller{store: &fakeRoomBatchHTTPRuntimeStore{credential: RoomWelcomeCorpCredential{WXCorpID: "ww-test", ContactSecret: "contact-secret"}}, client: NewRoomWelcomeWeComClient(chain.server.URL)}
	result, err := poller.Poll(context.Background(), wecomcapability.DispatchPollRequest{
		Principal: contactBatchHTTPChainPrincipal(), Capability: wecomcapability.RoomBatchSend,
		Dispatch: wecomcapability.Dispatch{DispatchKind: string(wecomcapability.DispatchKindRoomBatch), TargetID: "room_batch:701:owner:101:chunk:1", ProviderMessageID: "room-msg-1"},
	})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if !result.Terminal || len(result.Results) != 3 {
		t.Fatalf("terminal=%v results=%#v", result.Terminal, result.Results)
	}
	got := map[string]bool{}
	for _, item := range result.Results {
		got[item.TargetID] = true
		if item.Status != wecomcapability.DispatchSucceeded {
			t.Fatalf("result=%#v", item)
		}
	}
	for _, want := range []string{"101:chat-a", "101:chat-b", "101:chat-c"} {
		if !got[want] {
			t.Fatalf("missing aggregated target %q in %#v", want, result.Results)
		}
	}
	if len(chain.taskBodies) != 2 {
		t.Fatalf("task pages=%d, want 2 (cursor walk)", len(chain.taskBodies))
	}
	if len(chain.resultBodies) != 3 {
		t.Fatalf("result pages=%d, want 3", len(chain.resultBodies))
	}
}

// TestRoomBatchHTTPChainPollDoesNotTerminateWhileAnyTaskIsPending locks that
// a not-sent task keeps the poll non-terminal (never an early terminal).
func TestRoomBatchHTTPChainPollDoesNotTerminateWhileAnyTaskIsPending(t *testing.T) {
	chain := newContactBatchHTTPChain(t)
	chain.taskPages = func(cursor string) (int, string) {
		return http.StatusOK, `{"errcode":0,"errmsg":"ok","task_list":[{"userid":"sender-1","status":1},{"userid":"sender-2","status":0}],"next_cursor":""}`
	}
	chain.resultPages = func(userID, cursor string) (int, string) {
		return http.StatusOK, `{"errcode":0,"errmsg":"ok","send_list":[{"userid":"sender-1","chat_id":"chat-1","status":1}],"next_cursor":""}`
	}
	poller := &RoomBatchDispatchPoller{store: &fakeRoomBatchHTTPRuntimeStore{credential: RoomWelcomeCorpCredential{WXCorpID: "ww-test", ContactSecret: "contact-secret"}}, client: NewRoomWelcomeWeComClient(chain.server.URL)}
	result, err := poller.Poll(context.Background(), wecomcapability.DispatchPollRequest{
		Principal: contactBatchHTTPChainPrincipal(), Capability: wecomcapability.RoomBatchSend,
		Dispatch: wecomcapability.Dispatch{DispatchKind: string(wecomcapability.DispatchKindRoomBatch), TargetID: "room_batch:701:owner:101:chunk:1", ProviderMessageID: "room-msg-1"},
	})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if result.Terminal {
		t.Fatalf("poll terminated with a pending task: %#v", result)
	}
}

// TestRoomBatchHTTPChainPollDeduplicatesRepeatedResultPages locks that
// repeated send_result pages are idempotent and that a failure observation
// wins over an earlier success for the same {ownerEmployeeID}:{chatID} target.
func TestRoomBatchHTTPChainPollDeduplicatesRepeatedResultPages(t *testing.T) {
	chain := newContactBatchHTTPChain(t)
	chain.taskPages = func(cursor string) (int, string) {
		return http.StatusOK, `{"errcode":0,"errmsg":"ok","task_list":[{"userid":"sender-1","status":1}],"next_cursor":""}`
	}
	chain.resultPages = func(userID, cursor string) (int, string) {
		return http.StatusOK, `{"errcode":0,"errmsg":"ok","send_list":[{"userid":"sender-1","chat_id":"chat-shared","status":1},{"userid":"sender-1","chat_id":"chat-shared","status":2}],"next_cursor":""}`
	}
	poller := &RoomBatchDispatchPoller{store: &fakeRoomBatchHTTPRuntimeStore{credential: RoomWelcomeCorpCredential{WXCorpID: "ww-test", ContactSecret: "contact-secret"}}, client: NewRoomWelcomeWeComClient(chain.server.URL)}
	result, err := poller.Poll(context.Background(), wecomcapability.DispatchPollRequest{
		Principal: contactBatchHTTPChainPrincipal(), Capability: wecomcapability.RoomBatchSend,
		Dispatch: wecomcapability.Dispatch{DispatchKind: string(wecomcapability.DispatchKindRoomBatch), TargetID: "room_batch:701:owner:101:chunk:1", ProviderMessageID: "room-msg-1"},
	})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(result.Results) != 1 || result.Results[0].TargetID != "101:chat-shared" || result.Results[0].Status != wecomcapability.DispatchFailed {
		t.Fatalf("dedup result=%#v", result.Results)
	}
}

// TestRoomBatchHTTPChainSubmitErrorClassification locks the shared
// classification: ambiguous submit outcomes (5xx, 429, timeout) classify into
// reconcile-class provider errors while 401/403/contract errors classify
// terminal.
func TestRoomBatchHTTPChainSubmitErrorClassification(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		body     string
		wantCode string
		wantCat  wecomcapability.DispatchProviderErrorCategory
	}{
		{"server 500", http.StatusInternalServerError, `{}`, "wecom.http_500", wecomcapability.DispatchErrorServer},
		{"rate limit 429", http.StatusTooManyRequests, `{}`, "wecom.http_429", wecomcapability.DispatchErrorRateLimit},
		{"unauthorized 401", http.StatusUnauthorized, `{}`, "wecom.http_401", wecomcapability.DispatchErrorUnauthorized},
		{"forbidden 403", http.StatusForbidden, `{}`, "wecom.http_403", wecomcapability.DispatchErrorForbidden},
		{"invalid credential API error", http.StatusOK, `{"errcode":40001,"errmsg":"invalid credential"}`, "wecom.http_401", wecomcapability.DispatchErrorUnauthorized},
		{"expired token API error", http.StatusOK, `{"errcode":40014,"errmsg":"invalid access_token"}`, "wecom.http_401", wecomcapability.DispatchErrorUnauthorized},
		{"generic contract error", http.StatusOK, `{"errcode":40003,"errmsg":"unsupported"}`, "wecom.contract_error", wecomcapability.DispatchErrorContract},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			chain := newContactBatchHTTPChain(t)
			chain.submitResp = func() (int, string) { return item.status, item.body }
			sender := &RoomBatchDispatchSender{
				store: &fakeRoomBatchHTTPRuntimeStore{
					payload:    RoomMessageBatchSendMessagePayload{Sender: "employee-1", ChatIDs: []string{"chat-1"}, Content: []ContactMessageBatchSendContent{{MsgType: "text", Content: "hello"}}},
					credential: RoomWelcomeCorpCredential{WXCorpID: "ww-test", ContactSecret: "contact-secret"},
				},
				client: NewRoomWelcomeWeComClient(chain.server.URL),
			}
			_, err := sender.Submit(context.Background(), wecomcapability.DispatchSubmitRequest{
				Principal: contactBatchHTTPChainPrincipal(), Capability: wecomcapability.RoomBatchSend, Dispatch: roomBatchHTTPChainDispatch(),
			})
			if err == nil {
				t.Fatal("submit unexpectedly succeeded")
			}
			var providerErr *wecomcapability.DispatchProviderError
			if !errors.As(err, &providerErr) {
				t.Fatalf("error is not a DispatchProviderError: %v", err)
			}
			if providerErr.Category != item.wantCat || providerErr.Code != item.wantCode {
				t.Fatalf("category=%s code=%s, want category=%s code=%s", providerErr.Category, providerErr.Code, item.wantCat, item.wantCode)
			}
		})
	}
}

// TestRoomBatchHTTPChainSubmitTimeoutClassifiesReconcile locks that a context
// deadline on the room submit classifies as timeout (ambiguous acceptance)
// instead of a terminal failure.
func TestRoomBatchHTTPChainSubmitTimeoutClassifiesReconcile(t *testing.T) {
	chain := newContactBatchHTTPChain(t)
	gate := make(chan struct{})
	chain.submitResp = func() (int, string) {
		<-gate
		return http.StatusOK, `{"errcode":0,"errmsg":"ok","msgid":"room-msg-1"}`
	}
	sender := &RoomBatchDispatchSender{
		store: &fakeRoomBatchHTTPRuntimeStore{
			payload:    RoomMessageBatchSendMessagePayload{Sender: "employee-1", ChatIDs: []string{"chat-1"}, Content: []ContactMessageBatchSendContent{{MsgType: "text", Content: "hello"}}},
			credential: RoomWelcomeCorpCredential{WXCorpID: "ww-test", ContactSecret: "contact-secret"},
		},
		client: NewRoomWelcomeWeComClient(chain.server.URL),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	defer close(gate)
	_, err := sender.Submit(ctx, wecomcapability.DispatchSubmitRequest{
		Principal: contactBatchHTTPChainPrincipal(), Capability: wecomcapability.RoomBatchSend, Dispatch: roomBatchHTTPChainDispatch(),
	})
	if err == nil {
		t.Fatal("submit unexpectedly succeeded under timeout")
	}
	var providerErr *wecomcapability.DispatchProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("error is not a DispatchProviderError: %v", err)
	}
	if providerErr.Category != wecomcapability.DispatchErrorTimeout {
		t.Fatalf("category=%s, want timeout (reconcile class)", providerErr.Category)
	}
}
