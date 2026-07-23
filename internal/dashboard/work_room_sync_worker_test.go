package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"testing"
	"time"
)

func TestWorkRoomSyncEventSupportsUpdateApplyLegacyPayload(t *testing.T) {
	var event WorkRoomSyncEvent
	if err := json.Unmarshal([]byte(`[7]`), &event); err != nil {
		t.Fatal(err)
	}
	if event.CorpID != 7 || event.ChatID != "" {
		t.Fatalf("event = %+v", event)
	}
}

func TestWorkRoomSyncEventSupportsUpdateCallbackLegacyPayload(t *testing.T) {
	var event WorkRoomSyncEvent
	if err := json.Unmarshal([]byte(`[{"ToUserName":"ww-go","ChatId":"chat-1"}]`), &event); err != nil {
		t.Fatal(err)
	}
	if event.WXCorpID != "ww-go" || event.ChatID != "chat-1" || event.CorpID != 0 {
		t.Fatalf("event = %+v", event)
	}
}

func TestWorkRoomSyncWorkerFullSync(t *testing.T) {
	store := &fakeWorkRoomSyncWorkerStore{
		credential:     workRoomSyncCredentialFixture(),
		credentialOK:   true,
		tenantByCorpID: map[int]int{7: 11},
	}
	client := &fakeWorkRoomSyncWorkerClient{
		chats: []WorkRoomSyncGroupChat{
			{WXChatID: "chat-1", Status: 0},
			{WXChatID: "chat-2", Status: 1},
			{WXChatID: "chat-1", Status: 0},
		},
		rooms: map[string]WorkRoomSyncRoom{
			"chat-1": {WXChatID: "chat-1", Name: "客户群一", Owner: "go-owner", Members: []WorkRoomSyncMember{{WXUserID: "go-owner", Type: 1}}},
			"chat-2": {WXChatID: "chat-2", Name: "客户群二"},
		},
	}
	worker := NewWorkRoomSyncWorker(nil, store, client, log.New(io.Discard, "", 0))

	if err := worker.Process(context.Background(), WorkRoomSyncEvent{CorpID: 7, Source: "test"}); err != nil {
		t.Fatal(err)
	}
	if store.fullSyncCorpID != 7 || len(store.fullSyncRooms) != 2 {
		t.Fatalf("full sync corp=%d rooms=%+v", store.fullSyncCorpID, store.fullSyncRooms)
	}
	if len(client.detailIDs) != 2 || client.detailIDs[0] != "chat-1" || client.detailIDs[1] != "chat-2" {
		t.Fatalf("detail ids = %+v", client.detailIDs)
	}
	if store.fullSyncRooms[1].Status != 1 {
		t.Fatalf("room status = %+v", store.fullSyncRooms[1])
	}
}

func TestWorkRoomSyncWorkerCallbackSyncByWXCorpID(t *testing.T) {
	store := &fakeWorkRoomSyncWorkerStore{
		credentialByWX: map[string]RoomWelcomeCorpCredential{"ww-go": workRoomSyncCredentialFixture()},
		tenantByCorpID: map[int]int{7: 11},
	}
	client := &fakeWorkRoomSyncWorkerClient{
		chats: []WorkRoomSyncGroupChat{{WXChatID: "chat-1", Status: 1}},
		rooms: map[string]WorkRoomSyncRoom{"chat-1": {WXChatID: "chat-1", Name: "客户群一"}},
	}
	worker := NewWorkRoomSyncWorker(nil, store, client, log.New(io.Discard, "", 0))

	if err := worker.Process(context.Background(), WorkRoomSyncEvent{WXCorpID: "ww-go", ChatID: "chat-1", Source: "callback"}); err != nil {
		t.Fatal(err)
	}
	if store.singleSyncCorpID != 7 || store.singleSyncRoom.WXChatID != "chat-1" || store.singleSyncRoom.Status != 1 {
		t.Fatalf("single sync corp=%d room=%+v", store.singleSyncCorpID, store.singleSyncRoom)
	}
}

func TestWorkRoomSyncWorkerAcksSuccessfulDelivery(t *testing.T) {
	queue := &fakeWorkRoomSyncQueue{}
	store := &fakeWorkRoomSyncWorkerStore{credential: workRoomSyncCredentialFixture(), credentialOK: true, tenantByCorpID: map[int]int{7: 11}}
	client := &fakeWorkRoomSyncWorkerClient{}
	worker := NewWorkRoomSyncWorker(queue, store, client, log.New(io.Discard, "", 0))

	worker.handleDelivery(context.Background(), WorkRoomSyncDelivery{
		Event:    WorkRoomSyncEvent{CorpID: 7},
		Raw:      "raw",
		Attempts: 0,
	})
	if !queue.acked || queue.retried {
		t.Fatalf("queue acked=%v retried=%v", queue.acked, queue.retried)
	}
}

func TestWorkRoomSyncWorkerRetriesFailedDelivery(t *testing.T) {
	queue := &fakeWorkRoomSyncQueue{}
	store := &fakeWorkRoomSyncWorkerStore{credentialErr: fmt.Errorf("boom")}
	client := &fakeWorkRoomSyncWorkerClient{}
	worker := NewWorkRoomSyncWorker(queue, store, client, log.New(io.Discard, "", 0))

	worker.handleDelivery(context.Background(), WorkRoomSyncDelivery{
		Event:    WorkRoomSyncEvent{CorpID: 7},
		Raw:      "raw",
		Attempts: 0,
	})
	if queue.acked || !queue.retried || queue.retryReason == "" {
		t.Fatalf("queue acked=%v retried=%v reason=%q", queue.acked, queue.retried, queue.retryReason)
	}
}

func workRoomSyncCredentialFixture() RoomWelcomeCorpCredential {
	return RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"}
}

type fakeWorkRoomSyncWorkerStore struct {
	credential       RoomWelcomeCorpCredential
	credentialOK     bool
	credentialErr    error
	credentialByWX   map[string]RoomWelcomeCorpCredential
	fullSyncCorpID   int
	fullSyncRooms    []WorkRoomSyncRoom
	singleSyncCorpID int
	singleSyncRoom   WorkRoomSyncRoom
	syncErr          error
	tenantByCorpID   map[int]int
}

func (s *fakeWorkRoomSyncWorkerStore) TenantIDByCorpID(_ context.Context, corpID int) (int, error) {
	if s.tenantByCorpID == nil {
		return 0, nil
	}
	return s.tenantByCorpID[corpID], nil
}

func (s *fakeWorkRoomSyncWorkerStore) RoomWelcomeCorpCredentialByID(_ context.Context, _ int) (RoomWelcomeCorpCredential, bool, error) {
	return s.credential, s.credentialOK, s.credentialErr
}

func (s *fakeWorkRoomSyncWorkerStore) RoomWelcomeCorpCredentialByWXCorpID(_ context.Context, wxCorpID string) (RoomWelcomeCorpCredential, bool, error) {
	if s.credentialErr != nil {
		return RoomWelcomeCorpCredential{}, false, s.credentialErr
	}
	if s.credentialByWX == nil {
		return RoomWelcomeCorpCredential{}, false, nil
	}
	credential, ok := s.credentialByWX[wxCorpID]
	return credential, ok, nil
}

func (s *fakeWorkRoomSyncWorkerStore) SyncWorkRooms(_ context.Context, corpID int, rooms []WorkRoomSyncRoom) (WorkRoomSyncResult, error) {
	s.fullSyncCorpID = corpID
	s.fullSyncRooms = append([]WorkRoomSyncRoom{}, rooms...)
	return WorkRoomSyncResult{RoomsCreated: len(rooms)}, s.syncErr
}

func (s *fakeWorkRoomSyncWorkerStore) SyncWorkRoom(_ context.Context, corpID int, room WorkRoomSyncRoom) (WorkRoomSyncResult, error) {
	s.singleSyncCorpID = corpID
	s.singleSyncRoom = room
	return WorkRoomSyncResult{RoomsUpdated: 1}, s.syncErr
}

type fakeWorkRoomSyncWorkerClient struct {
	chats     []WorkRoomSyncGroupChat
	rooms     map[string]WorkRoomSyncRoom
	err       error
	detailIDs []string
}

func (c *fakeWorkRoomSyncWorkerClient) GroupChats(_ context.Context, _ RoomWelcomeCorpCredential) ([]WorkRoomSyncGroupChat, error) {
	if c.err != nil {
		return nil, c.err
	}
	return append([]WorkRoomSyncGroupChat{}, c.chats...), nil
}

func (c *fakeWorkRoomSyncWorkerClient) GroupChatDetail(_ context.Context, _ RoomWelcomeCorpCredential, wxChatID string) (WorkRoomSyncRoom, error) {
	c.detailIDs = append(c.detailIDs, wxChatID)
	if c.err != nil {
		return WorkRoomSyncRoom{}, c.err
	}
	return c.rooms[wxChatID], nil
}

type fakeWorkRoomSyncQueue struct {
	acked       bool
	retried     bool
	retryReason string
	deadLetter  bool
}

func (q *fakeWorkRoomSyncQueue) DequeueWorkRoomSync(_ context.Context, _ time.Duration) (WorkRoomSyncDelivery, bool, error) {
	return WorkRoomSyncDelivery{}, false, nil
}

func (q *fakeWorkRoomSyncQueue) AckWorkRoomSync(_ context.Context, _ WorkRoomSyncDelivery) error {
	q.acked = true
	return nil
}

func (q *fakeWorkRoomSyncQueue) RetryWorkRoomSync(_ context.Context, _ WorkRoomSyncDelivery, reason string, _ int) (bool, error) {
	q.retried = true
	q.retryReason = reason
	return q.deadLetter, nil
}

func (q *fakeWorkRoomSyncQueue) RecoverWorkRoomSyncProcessing(_ context.Context, _ time.Duration, _ int) (int, error) {
	return 0, nil
}
