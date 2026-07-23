package dashboard

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestContactTransferStateCronRefreshesNextLog(t *testing.T) {
	store := &fakeContactTransferStateStore{
		log: ContactTransferStateLog{
			ID:                 12,
			CorpID:             7,
			ContactID:          "external-31",
			HandoverEmployeeID: "handover-user",
			TakeoverEmployeeID: "takeover-user",
		},
		found:      true,
		credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wx-corp", ContactSecret: "contact-secret"},
		corpFound:  true,
	}
	cache := &fakeContactTransferStateCache{logID: 10}
	client := &fakeContactTransferStateClient{
		results: []ContactTransferStateResult{
			{ExternalUserID: "external-other", Status: 2},
			{ExternalUserID: "external-31", Status: 3},
		},
	}
	cron := NewContactTransferStateCron(store, cache, client, nil)

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.afterID != 10 || cache.setLogID != 12 {
		t.Fatalf("cursor after=%d set=%d", store.afterID, cache.setLogID)
	}
	if client.handoverUserID != "handover-user" || client.takeoverUserID != "takeover-user" {
		t.Fatalf("client handover=%q takeover=%q", client.handoverUserID, client.takeoverUserID)
	}
	if store.updatedLogID != 12 || store.updatedState != 3 {
		t.Fatalf("updated log=%d state=%d", store.updatedLogID, store.updatedState)
	}
	if cache.ttl != ContactTransferStateCursorTTL {
		t.Fatalf("ttl = %s", cache.ttl)
	}
}

func TestContactTransferStateCronResetsCursorWhenNoLog(t *testing.T) {
	store := &fakeContactTransferStateStore{}
	cache := &fakeContactTransferStateCache{logID: 12}
	client := &fakeContactTransferStateClient{}
	cron := NewContactTransferStateCron(store, cache, client, nil)

	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if cache.setLogID != 0 {
		t.Fatalf("set log id = %d", cache.setLogID)
	}
}

func TestContactTransferStateCronRequiresDependencies(t *testing.T) {
	cron := NewContactTransferStateCron(nil, nil, nil, nil)
	err := cron.RunOnce(context.Background())
	if err == nil || !strings.Contains(err.Error(), "dependencies are not configured") {
		t.Fatalf("error = %v", err)
	}
}

type fakeContactTransferStateStore struct {
	log          ContactTransferStateLog
	found        bool
	afterID      int
	credential   RoomWelcomeCorpCredential
	corpFound    bool
	updatedLogID int
	updatedState int
}

func (s *fakeContactTransferStateStore) NextContactTransferStateLog(_ context.Context, afterID int) (ContactTransferStateLog, bool, error) {
	s.afterID = afterID
	return s.log, s.found, nil
}

func (s *fakeContactTransferStateStore) UpdateContactTransferLogState(_ context.Context, logID int, state int) (bool, error) {
	s.updatedLogID = logID
	s.updatedState = state
	return true, nil
}

func (s *fakeContactTransferStateStore) RoomWelcomeCorpCredentialByID(context.Context, int) (RoomWelcomeCorpCredential, bool, error) {
	return s.credential, s.corpFound, nil
}

type fakeContactTransferStateCache struct {
	logID    int
	setLogID int
	ttl      time.Duration
}

func (c *fakeContactTransferStateCache) ContactTransferStateLogID(context.Context) (int, error) {
	return c.logID, nil
}

func (c *fakeContactTransferStateCache) SetContactTransferStateLogID(_ context.Context, logID int, ttl time.Duration) error {
	c.setLogID = logID
	c.ttl = ttl
	return nil
}

type fakeContactTransferStateClient struct {
	results        []ContactTransferStateResult
	handoverUserID string
	takeoverUserID string
}

func (c *fakeContactTransferStateClient) TransferResult(_ context.Context, _ RoomWelcomeCorpCredential, handoverUserID string, takeoverUserID string) ([]ContactTransferStateResult, error) {
	c.handoverUserID = handoverUserID
	c.takeoverUserID = takeoverUserID
	return append([]ContactTransferStateResult{}, c.results...), nil
}
