package dashboard

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestContactBatchSendScheduleCronSubmitsDueBatches(t *testing.T) {
	now := time.Date(2026, 7, 4, 10, 30, 0, 0, time.Local)
	store := &fakeContactBatchSendScheduleStore{
		dueIDs:          []int{7001},
		credentialFound: true,
		credential:      RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		batches: map[int]ContactMessageBatchSendItem{
			7001: {
				ID:         7001,
				CorpID:     7,
				SendWay:    2,
				SendStatus: 0,
				Content: []ContactMessageBatchSendContent{
					{MsgType: "text", Content: "scheduled hello"},
					{MsgType: "image", PicURL: "image/a.jpg"},
				},
			},
		},
		targets: []ContactMessageBatchSendSendTarget{{
			EmployeeID:      21,
			WXUserID:        "employee-wx",
			ExternalUserIDs: []string{"external-1", "external-2"},
		}},
	}
	client := &fakeContactMessageBatchSendClient{mediaID: "media-image", msgID: "msg-contact-scheduled"}
	cron := NewContactBatchSendScheduleCron(store, client, "/tmp/mochat-go-test", nil)
	cron.now = func() time.Time { return now }

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !store.dueNow.Equal(now) {
		t.Fatalf("due now = %s", store.dueNow)
	}
	if store.credentialCorpID != 7 {
		t.Fatalf("credential corp id = %d", store.credentialCorpID)
	}
	if client.uploadPath != filepath.Join("/tmp/mochat-go-test", "image/a.jpg") || len(client.submits) != 1 {
		t.Fatalf("upload=%q submits=%#v", client.uploadPath, client.submits)
	}
	submit := client.submits[0]
	if submit.Sender != "employee-wx" || len(submit.ExternalUserID) != 2 || submit.ExternalUserID[0] != "external-1" || submit.Content[1].MediaID != "media-image" {
		t.Fatalf("submit=%#v", submit)
	}
	if store.markedBatchID != 7001 || len(store.markedResults) != 1 || store.markedResults[0].MsgID != "msg-contact-scheduled" {
		t.Fatalf("marked batch=%d results=%#v", store.markedBatchID, store.markedResults)
	}
}

func TestRoomBatchSendScheduleCronSubmitsDueBatches(t *testing.T) {
	now := time.Date(2026, 7, 4, 11, 0, 0, 0, time.Local)
	store := &fakeRoomBatchSendScheduleStore{
		dueIDs:          []int{8001},
		credentialFound: true,
		credential:      RoomWelcomeCorpCredential{CorpID: 8, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		batches: map[int]RoomMessageBatchSendItem{
			8001: {
				ID:         8001,
				CorpID:     8,
				SendWay:    2,
				SendStatus: 0,
				Content: []ContactMessageBatchSendContent{
					{MsgType: "text", Content: "scheduled room hello"},
				},
			},
		},
		targets: []RoomMessageBatchSendTarget{{
			EmployeeID: 31,
			WXUserID:   "room-owner",
			ChatIDs:    []string{"chat-1", "chat-2"},
		}},
	}
	client := &fakeRoomMessageBatchSendClient{msgID: "msg-room-scheduled"}
	cron := NewRoomBatchSendScheduleCron(store, client, "/tmp/mochat-go-test", nil)
	cron.now = func() time.Time { return now }

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !store.dueNow.Equal(now) {
		t.Fatalf("due now = %s", store.dueNow)
	}
	if store.credentialCorpID != 8 {
		t.Fatalf("credential corp id = %d", store.credentialCorpID)
	}
	if len(client.submits) != 1 {
		t.Fatalf("submits=%#v", client.submits)
	}
	submit := client.submits[0]
	if submit.Sender != "room-owner" || len(submit.ChatIDs) != 2 || submit.ChatIDs[0] != "chat-1" {
		t.Fatalf("submit=%#v", submit)
	}
	if store.markedBatchID != 8001 || len(store.markedResults) != 1 || store.markedResults[0].MsgID != "msg-room-scheduled" {
		t.Fatalf("marked batch=%d results=%#v", store.markedBatchID, store.markedResults)
	}
}

func TestBatchSendScheduleCronsRequireDependencies(t *testing.T) {
	if err := NewContactBatchSendScheduleCron(nil, nil, "", nil).RunOnce(context.Background()); err == nil || err.Error() != "ContactMessageBatchSend scheduled cron dependencies are not configured" {
		t.Fatalf("contact err = %v", err)
	}
	if err := NewRoomBatchSendScheduleCron(nil, nil, "", nil).RunOnce(context.Background()); err == nil || err.Error() != "RoomMessageBatchSend scheduled cron dependencies are not configured" {
		t.Fatalf("room err = %v", err)
	}
}

type fakeContactBatchSendScheduleStore struct {
	dueIDs           []int
	dueNow           time.Time
	credential       RoomWelcomeCorpCredential
	credentialFound  bool
	credentialCorpID int
	batches          map[int]ContactMessageBatchSendItem
	targets          []ContactMessageBatchSendSendTarget
	markedBatchID    int
	markedResults    []ContactMessageBatchSendMessageResult
}

func (s *fakeContactBatchSendScheduleStore) DueContactMessageBatchSendIDs(_ context.Context, now time.Time) ([]int, error) {
	s.dueNow = now
	return append([]int{}, s.dueIDs...), nil
}

func (s *fakeContactBatchSendScheduleStore) RoomWelcomeCorpCredentialByID(_ context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error) {
	s.credentialCorpID = corpID
	return s.credential, s.credentialFound, nil
}

func (s *fakeContactBatchSendScheduleStore) ContactMessageBatchSendByID(_ context.Context, batchID int) (ContactMessageBatchSendItem, bool, error) {
	item, ok := s.batches[batchID]
	return item, ok, nil
}

func (s *fakeContactBatchSendScheduleStore) CreateContactMessageBatchSendTasks(_ context.Context, _ int) ([]ContactMessageBatchSendSendTarget, error) {
	return append([]ContactMessageBatchSendSendTarget{}, s.targets...), nil
}

func (s *fakeContactBatchSendScheduleStore) MarkContactMessageBatchSendSubmitted(_ context.Context, batchID int, results []ContactMessageBatchSendMessageResult) error {
	s.markedBatchID = batchID
	s.markedResults = append([]ContactMessageBatchSendMessageResult{}, results...)
	return nil
}

type fakeRoomBatchSendScheduleStore struct {
	dueIDs           []int
	dueNow           time.Time
	credential       RoomWelcomeCorpCredential
	credentialFound  bool
	credentialCorpID int
	batches          map[int]RoomMessageBatchSendItem
	targets          []RoomMessageBatchSendTarget
	markedBatchID    int
	markedResults    []RoomMessageBatchSendMessageResult
}

func (s *fakeRoomBatchSendScheduleStore) DueRoomMessageBatchSendIDs(_ context.Context, now time.Time) ([]int, error) {
	s.dueNow = now
	return append([]int{}, s.dueIDs...), nil
}

func (s *fakeRoomBatchSendScheduleStore) RoomWelcomeCorpCredentialByID(_ context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error) {
	s.credentialCorpID = corpID
	return s.credential, s.credentialFound, nil
}

func (s *fakeRoomBatchSendScheduleStore) RoomMessageBatchSendByID(_ context.Context, batchID int) (RoomMessageBatchSendItem, bool, error) {
	item, ok := s.batches[batchID]
	return item, ok, nil
}

func (s *fakeRoomBatchSendScheduleStore) CreateRoomMessageBatchSendTasks(_ context.Context, _ int) ([]RoomMessageBatchSendTarget, error) {
	return append([]RoomMessageBatchSendTarget{}, s.targets...), nil
}

func (s *fakeRoomBatchSendScheduleStore) MarkRoomMessageBatchSendSubmitted(_ context.Context, batchID int, results []RoomMessageBatchSendMessageResult) error {
	s.markedBatchID = batchID
	s.markedResults = append([]RoomMessageBatchSendMessageResult{}, results...)
	return nil
}
