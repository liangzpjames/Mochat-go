package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcapability"
)

// P0-3 fake HTTP full-chain evidence. These tests run the REAL
// RoomWelcomeWeComClient (gettoken -> add_msg_template -> get_groupmsg_task ->
// get_groupmsg_send_result) against a local httptest.Server. No real WeCom
// endpoint is ever called. They lock the official contract: chat_type=single,
// external_userid/sender/text fields, msgid as a response-only field, cursor
// pagination completeness, and the submit error classification that decides
// reconcile vs terminal.

type fakeContactBatchHTTPRuntimeStore struct {
	payload    ContactMessageBatchSendMessagePayload
	credential RoomWelcomeCorpCredential
}

func (s *fakeContactBatchHTTPRuntimeStore) ContactBatchDispatchPayload(context.Context, dashboardprincipal.DashboardPrincipal, wecomcapability.Dispatch) (ContactMessageBatchSendMessagePayload, error) {
	return s.payload, nil
}

func (s *fakeContactBatchHTTPRuntimeStore) ContactBatchDispatchCredential(context.Context, dashboardprincipal.DashboardPrincipal) (RoomWelcomeCorpCredential, bool, error) {
	return s.credential, true, nil
}

type contactBatchHTTPChain struct {
	server       *httptest.Server
	tokenCalls   atomic.Int32
	submitBodies []map[string]any
	taskBodies   []map[string]any
	resultBodies []map[string]any
	submitResp   func() (int, string)
	taskPages    func(cursor string) (int, string)
	resultPages  func(userID, cursor string) (int, string)
}

func newContactBatchHTTPChain(t *testing.T) *contactBatchHTTPChain {
	t.Helper()
	chain := &contactBatchHTTPChain{
		submitResp: func() (int, string) { return http.StatusOK, `{"errcode":0,"errmsg":"ok","msgid":"contact-msg-1"}` },
		taskPages: func(cursor string) (int, string) {
			return http.StatusOK, `{"errcode":0,"errmsg":"ok","task_list":[{"userid":"sender-1","status":1,"send_time":1720000000}],"next_cursor":""}`
		},
		resultPages: func(userID, cursor string) (int, string) {
			return http.StatusOK, `{"errcode":0,"errmsg":"ok","send_list":[{"userid":"sender-1","external_userid":"external-1","status":1,"send_time":1720000000}],"next_cursor":""}`
		},
	}
	chain.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/cgi-bin/gettoken":
			chain.tokenCalls.Add(1)
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","access_token":"token-1","expires_in":7200}`))
		case "/cgi-bin/externalcontact/add_msg_template":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			chain.submitBodies = append(chain.submitBodies, body)
			status, resp := chain.submitResp()
			w.WriteHeader(status)
			_, _ = w.Write([]byte(resp))
		case "/cgi-bin/externalcontact/get_groupmsg_task":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			chain.taskBodies = append(chain.taskBodies, body)
			status, resp := chain.taskPages(cursorOf(body))
			w.WriteHeader(status)
			_, _ = w.Write([]byte(resp))
		case "/cgi-bin/externalcontact/get_groupmsg_send_result":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			chain.resultBodies = append(chain.resultBodies, body)
			status, resp := chain.resultPages(userIDOf(body), cursorOf(body))
			w.WriteHeader(status)
			_, _ = w.Write([]byte(resp))
		default:
			t.Fatalf("unexpected path=%s", r.URL.Path)
		}
	}))
	t.Cleanup(chain.server.Close)
	return chain
}

func cursorOf(body map[string]any) string {
	cursor, _ := body["cursor"].(string)
	return cursor
}

func userIDOf(body map[string]any) string {
	userID, _ := body["userid"].(string)
	return userID
}

func contactBatchHTTPChainPrincipal() wecomcapability.DispatchPrincipal {
	return wecomcapability.DispatchPrincipal{TenantID: 1, CorpID: 7, UserID: 9, AuthVersion: 4}
}

func contactBatchHTTPChainDispatch() wecomcapability.Dispatch {
	return wecomcapability.Dispatch{DispatchKind: string(wecomcapability.DispatchKindContactBatch), TargetID: "contact_batch:701:employee:101:chunk:1"}
}

// TestContactBatchHTTPChainSubmitLocksOfficialRequestAndResponseMessageID
// locks the add_msg_template contract: chat_type=single, sender,
// external_userid list, text payload; msgid must NOT be sent as a request
// field (it is a response-only field) and must surface as ProviderMessageID.
func TestContactBatchHTTPChainSubmitLocksOfficialRequestAndResponseMessageID(t *testing.T) {
	chain := newContactBatchHTTPChain(t)
	store := &fakeContactBatchHTTPRuntimeStore{
		payload: ContactMessageBatchSendMessagePayload{
			Sender:         "employee-1",
			ExternalUserID: []string{"external-1", "external-2"},
			Content:        []ContactMessageBatchSendContent{{MsgType: "text", Content: "hello"}},
		},
		credential: RoomWelcomeCorpCredential{WXCorpID: "ww-test", ContactSecret: "contact-secret"},
	}
	sender := &ContactBatchDispatchSender{store: store, client: NewRoomWelcomeWeComClient(chain.server.URL)}
	result, err := sender.Submit(context.Background(), wecomcapability.DispatchSubmitRequest{
		Principal: contactBatchHTTPChainPrincipal(), Capability: wecomcapability.ContactBatchSend, Dispatch: contactBatchHTTPChainDispatch(),
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if result.ProviderMessageID != "contact-msg-1" || !result.Submitted {
		t.Fatalf("result=%#v", result)
	}
	if chain.tokenCalls.Load() != 1 {
		t.Fatalf("gettoken calls=%d, want 1", chain.tokenCalls.Load())
	}
	if len(chain.submitBodies) != 1 {
		t.Fatalf("submit bodies=%d", len(chain.submitBodies))
	}
	body := chain.submitBodies[0]
	if body["chat_type"] != "single" {
		t.Fatalf("chat_type=%v", body["chat_type"])
	}
	if body["sender"] != "employee-1" {
		t.Fatalf("sender=%v", body["sender"])
	}
	externalIDs, ok := body["external_userid"].([]any)
	if !ok || len(externalIDs) != 2 || externalIDs[0] != "external-1" || externalIDs[1] != "external-2" {
		t.Fatalf("external_userid=%#v", body["external_userid"])
	}
	text, ok := body["text"].(map[string]any)
	if !ok || text["content"] != "hello" {
		t.Fatalf("text=%#v", body["text"])
	}
	if _, present := body["msgid"]; present {
		t.Fatal("msgid must be a response-only field and must not be sent as a request idempotency field")
	}
	// Second submit must reuse the cached token, not call gettoken again.
	if _, err := sender.Submit(context.Background(), wecomcapability.DispatchSubmitRequest{
		Principal: contactBatchHTTPChainPrincipal(), Capability: wecomcapability.ContactBatchSend, Dispatch: contactBatchHTTPChainDispatch(),
	}); err != nil {
		t.Fatalf("second submit: %v", err)
	}
	if chain.tokenCalls.Load() != 1 {
		t.Fatalf("gettoken calls after second submit=%d, want 1 (token cache)", chain.tokenCalls.Load())
	}
}

// TestContactBatchHTTPChainPollWalksAllTaskAndResultPages locks cursor
// pagination completeness for both get_groupmsg_task and
// get_groupmsg_send_result and verifies every result target is aggregated.
func TestContactBatchHTTPChainPollWalksAllTaskAndResultPages(t *testing.T) {
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
				return http.StatusOK, `{"errcode":0,"errmsg":"ok","send_list":[{"userid":"sender-1","external_userid":"external-a","status":1}],"next_cursor":"result-1-2"}`
			}
			return http.StatusOK, `{"errcode":0,"errmsg":"ok","send_list":[{"userid":"sender-1","external_userid":"external-b","status":1}],"next_cursor":""}`
		case "sender-2":
			return http.StatusOK, `{"errcode":0,"errmsg":"ok","send_list":[{"userid":"sender-2","external_userid":"external-c","status":1}],"next_cursor":""}`
		default:
			t.Fatalf("unexpected result userID=%q", userID)
			return http.StatusInternalServerError, `{}`
		}
	}
	poller := &ContactBatchDispatchPoller{store: &fakeContactBatchHTTPRuntimeStore{credential: RoomWelcomeCorpCredential{WXCorpID: "ww-test", ContactSecret: "contact-secret"}}, client: NewRoomWelcomeWeComClient(chain.server.URL)}
	result, err := poller.Poll(context.Background(), wecomcapability.DispatchPollRequest{
		Principal: contactBatchHTTPChainPrincipal(), Capability: wecomcapability.ContactBatchSend,
		Dispatch: wecomcapability.Dispatch{DispatchKind: string(wecomcapability.DispatchKindContactBatch), TargetID: "contact_batch:701:employee:101:chunk:1", ProviderMessageID: "contact-msg-1"},
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
	for _, want := range []string{"101:external-a", "101:external-b", "101:external-c"} {
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

// TestContactBatchHTTPChainPollDoesNotTerminateWhileAnyTaskIsPending locks
// that a not-sent task keeps the poll non-terminal (never an early terminal).
func TestContactBatchHTTPChainPollDoesNotTerminateWhileAnyTaskIsPending(t *testing.T) {
	chain := newContactBatchHTTPChain(t)
	chain.taskPages = func(cursor string) (int, string) {
		return http.StatusOK, `{"errcode":0,"errmsg":"ok","task_list":[{"userid":"sender-1","status":1},{"userid":"sender-2","status":0}],"next_cursor":""}`
	}
	chain.resultPages = func(userID, cursor string) (int, string) {
		return http.StatusOK, `{"errcode":0,"errmsg":"ok","send_list":[{"userid":"sender-1","external_userid":"external-1","status":1}],"next_cursor":""}`
	}
	poller := &ContactBatchDispatchPoller{store: &fakeContactBatchHTTPRuntimeStore{credential: RoomWelcomeCorpCredential{WXCorpID: "ww-test", ContactSecret: "contact-secret"}}, client: NewRoomWelcomeWeComClient(chain.server.URL)}
	result, err := poller.Poll(context.Background(), wecomcapability.DispatchPollRequest{
		Principal: contactBatchHTTPChainPrincipal(), Capability: wecomcapability.ContactBatchSend,
		Dispatch: wecomcapability.Dispatch{DispatchKind: string(wecomcapability.DispatchKindContactBatch), TargetID: "contact_batch:701:employee:101:chunk:1", ProviderMessageID: "contact-msg-1"},
	})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if result.Terminal {
		t.Fatalf("poll terminated with a pending task: %#v", result)
	}
}

// TestContactBatchHTTPChainPollDeduplicatesRepeatedResultPages locks that
// repeated send_result pages are idempotent and that a failure observation
// wins over an earlier success for the same {employeeID}:{externalID} target.
func TestContactBatchHTTPChainPollDeduplicatesRepeatedResultPages(t *testing.T) {
	chain := newContactBatchHTTPChain(t)
	chain.taskPages = func(cursor string) (int, string) {
		return http.StatusOK, `{"errcode":0,"errmsg":"ok","task_list":[{"userid":"sender-1","status":1}],"next_cursor":""}`
	}
	chain.resultPages = func(userID, cursor string) (int, string) {
		return http.StatusOK, `{"errcode":0,"errmsg":"ok","send_list":[{"userid":"sender-1","external_userid":"external-shared","status":1},{"userid":"sender-1","external_userid":"external-shared","status":2}],"next_cursor":""}`
	}
	poller := &ContactBatchDispatchPoller{store: &fakeContactBatchHTTPRuntimeStore{credential: RoomWelcomeCorpCredential{WXCorpID: "ww-test", ContactSecret: "contact-secret"}}, client: NewRoomWelcomeWeComClient(chain.server.URL)}
	result, err := poller.Poll(context.Background(), wecomcapability.DispatchPollRequest{
		Principal: contactBatchHTTPChainPrincipal(), Capability: wecomcapability.ContactBatchSend,
		Dispatch: wecomcapability.Dispatch{DispatchKind: string(wecomcapability.DispatchKindContactBatch), TargetID: "contact_batch:701:employee:101:chunk:1", ProviderMessageID: "contact-msg-1"},
	})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(result.Results) != 1 || result.Results[0].TargetID != "101:external-shared" || result.Results[0].Status != wecomcapability.DispatchFailed {
		t.Fatalf("dedup result=%#v", result.Results)
	}
}

// TestContactBatchHTTPChainSubmitErrorClassification locks that ambiguous
// submit outcomes (5xx, 429, timeout) classify into reconcile-class provider
// errors while 401/403/contract errors classify terminal.
func TestContactBatchHTTPChainSubmitErrorClassification(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		wantCode   string
		wantCat    wecomcapability.DispatchProviderErrorCategory
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
			sender := &ContactBatchDispatchSender{
				store: &fakeContactBatchHTTPRuntimeStore{
					payload:    ContactMessageBatchSendMessagePayload{Sender: "employee-1", ExternalUserID: []string{"external-1"}, Content: []ContactMessageBatchSendContent{{MsgType: "text", Content: "hello"}}},
					credential: RoomWelcomeCorpCredential{WXCorpID: "ww-test", ContactSecret: "contact-secret"},
				},
				client: NewRoomWelcomeWeComClient(chain.server.URL),
			}
			_, err := sender.Submit(context.Background(), wecomcapability.DispatchSubmitRequest{
				Principal: contactBatchHTTPChainPrincipal(), Capability: wecomcapability.ContactBatchSend, Dispatch: contactBatchHTTPChainDispatch(),
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

// TestContactBatchHTTPChainSubmitTimeoutClassifiesReconcile locks that a
// context deadline on submit classifies as timeout (ambiguous acceptance)
// instead of a terminal failure, so the runner enters reconcile and never
// blindly resends.
func TestContactBatchHTTPChainSubmitTimeoutClassifiesReconcile(t *testing.T) {
	chain := newContactBatchHTTPChain(t)
	gate := make(chan struct{})
	chain.submitResp = func() (int, string) {
		<-gate
		return http.StatusOK, `{"errcode":0,"errmsg":"ok","msgid":"contact-msg-1"}`
	}
	sender := &ContactBatchDispatchSender{
		store: &fakeContactBatchHTTPRuntimeStore{
			payload:    ContactMessageBatchSendMessagePayload{Sender: "employee-1", ExternalUserID: []string{"external-1"}, Content: []ContactMessageBatchSendContent{{MsgType: "text", Content: "hello"}}},
			credential: RoomWelcomeCorpCredential{WXCorpID: "ww-test", ContactSecret: "contact-secret"},
		},
		client: NewRoomWelcomeWeComClient(chain.server.URL),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	defer close(gate)
	_, err := sender.Submit(ctx, wecomcapability.DispatchSubmitRequest{
		Principal: contactBatchHTTPChainPrincipal(), Capability: wecomcapability.ContactBatchSend, Dispatch: contactBatchHTTPChainDispatch(),
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
