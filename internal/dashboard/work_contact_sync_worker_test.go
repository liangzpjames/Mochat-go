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

func TestWorkContactSyncEventSupportsSyncContactApplyLegacyPayload(t *testing.T) {
	var event WorkContactSyncEvent
	raw := `[{"id":11,"wxUserId":"zhangsan"},7,"ww-go"]`
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatal(err)
	}
	if event.CorpID != 7 || event.WXCorpID != "ww-go" || event.Employee.ID != 11 || event.Employee.WXUserID != "zhangsan" {
		t.Fatalf("event = %+v", event)
	}
}

func TestWorkContactSyncEventSupportsAdminSynContactApplyLegacyPayload(t *testing.T) {
	var event WorkContactSyncEvent
	raw := `[[{"id":"12","wxUserId":"lisi"},{"id":11,"wx_user_id":"zhangsan"}],7]`
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatal(err)
	}
	if event.CorpID != 7 || len(event.Employees) != 2 || event.Employees[0].ID != 12 || event.Employees[1].WXUserID != "zhangsan" {
		t.Fatalf("event = %+v", event)
	}
}

func TestWorkContactSyncEventSupportsGroupPayloadExternalUserIDs(t *testing.T) {
	var event WorkContactSyncEvent
	raw := `[{"id":11,"wxUserId":"zhangsan"},7,"ww-go",["external-1","external-2"]]`
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatal(err)
	}
	if len(event.ExternalUserIDs) != 2 || event.ExternalUserIDs[0] != "external-1" || event.ExternalUserIDs[1] != "external-2" {
		t.Fatalf("event = %+v", event)
	}
}

func TestWorkContactSyncWorkerProcessesSingleEmployeeGroupPayload(t *testing.T) {
	store := &fakeWorkContactSyncWorkerStore{
		credential:   workContactSyncCredentialFixture(),
		credentialOK: true,
		tenantByCorpID: map[int]int{
			7: 11,
		},
	}
	client := &fakeWorkContactSyncWorkerClient{
		details: map[string]WorkContactSyncContact{
			"external-1": {
				WXExternalUserID: "external-1",
				Name:             "客户一",
				FollowUsers: []WorkContactSyncFollowUser{{
					UserID: "zhangsan",
					Tags:   []WorkContactSyncTag{{WXContactTagID: "tag-1", GroupName: "阶段", TagName: "高意向", Type: 1}},
				}},
			},
			"external-2": {
				WXExternalUserID: "external-2",
				Name:             "客户二",
				FollowUsers:      []WorkContactSyncFollowUser{{UserID: "zhangsan"}},
			},
		},
	}
	worker := NewWorkContactSyncWorker(nil, store, client, log.New(io.Discard, "", 0))

	if err := worker.Process(context.Background(), WorkContactSyncEvent{
		CorpID:          7,
		Employee:        WorkContactSyncEmployee{ID: 11, WXUserID: "zhangsan"},
		ExternalUserIDs: []string{"external-1", "external-2", "external-1"},
		Source:          "test",
	}); err != nil {
		t.Fatal(err)
	}
	if len(client.listUsers) != 0 {
		t.Fatalf("unexpected list calls = %+v", client.listUsers)
	}
	if len(client.detailIDs) != 2 || client.detailIDs[0] != "external-1" || client.detailIDs[1] != "external-2" {
		t.Fatalf("detail ids = %+v", client.detailIDs)
	}
	if store.syncedCorpID != 7 || len(store.syncedBundles) != 1 {
		t.Fatalf("sync corp=%d bundles=%+v", store.syncedCorpID, store.syncedBundles)
	}
	bundle := store.syncedBundles[0]
	if bundle.Employee.ID != 11 || len(bundle.ExternalUserIDs) != 2 || len(bundle.Contacts) != 2 || bundle.NoContact {
		t.Fatalf("bundle = %+v", bundle)
	}
}

func TestWorkContactSyncWorkerProcessesFullSyncAndNoContact(t *testing.T) {
	store := &fakeWorkContactSyncWorkerStore{
		credential:   workContactSyncCredentialFixture(),
		credentialOK: true,
		employees: []WorkContactSyncEmployee{
			{ID: 11, WXUserID: "zhangsan"},
			{ID: 12, WXUserID: "lisi"},
		},
	}
	client := &fakeWorkContactSyncWorkerClient{
		lists: map[string][]string{
			"zhangsan": {"external-1", "external-1"},
		},
		noContact: map[string]bool{"lisi": true},
		details: map[string]WorkContactSyncContact{
			"external-1": {WXExternalUserID: "external-1", FollowUsers: []WorkContactSyncFollowUser{{UserID: "zhangsan"}}},
		},
	}
	worker := NewWorkContactSyncWorker(nil, store, client, log.New(io.Discard, "", 0))

	if err := worker.Process(context.Background(), WorkContactSyncEvent{CorpID: 7}); err != nil {
		t.Fatal(err)
	}
	if len(client.listUsers) != 2 || client.listUsers[0] != "zhangsan" || client.listUsers[1] != "lisi" {
		t.Fatalf("list users = %+v", client.listUsers)
	}
	if len(store.syncedBundles) != 2 {
		t.Fatalf("bundles = %+v", store.syncedBundles)
	}
	if store.syncedBundles[0].NoContact || len(store.syncedBundles[0].Contacts) != 1 {
		t.Fatalf("first bundle = %+v", store.syncedBundles[0])
	}
	if !store.syncedBundles[1].NoContact || len(store.syncedBundles[1].Contacts) != 0 {
		t.Fatalf("second bundle = %+v", store.syncedBundles[1])
	}
}

func TestWorkContactSyncWorkerAcksSuccessfulDelivery(t *testing.T) {
	queue := &fakeWorkContactSyncQueue{}
	store := &fakeWorkContactSyncWorkerStore{credential: workContactSyncCredentialFixture(), credentialOK: true, employees: []WorkContactSyncEmployee{{ID: 11, WXUserID: "zhangsan"}}}
	client := &fakeWorkContactSyncWorkerClient{noContact: map[string]bool{"zhangsan": true}}
	worker := NewWorkContactSyncWorker(queue, store, client, log.New(io.Discard, "", 0))

	worker.handleDelivery(context.Background(), WorkContactSyncDelivery{
		Event: WorkContactSyncEvent{CorpID: 7},
		Raw:   "raw",
	})
	if !queue.acked || queue.retried {
		t.Fatalf("queue acked=%v retried=%v", queue.acked, queue.retried)
	}
}

func TestWorkContactSyncWorkerRetriesFailedDelivery(t *testing.T) {
	queue := &fakeWorkContactSyncQueue{}
	store := &fakeWorkContactSyncWorkerStore{credentialErr: fmt.Errorf("boom")}
	worker := NewWorkContactSyncWorker(queue, store, &fakeWorkContactSyncWorkerClient{}, log.New(io.Discard, "", 0))

	worker.handleDelivery(context.Background(), WorkContactSyncDelivery{
		Event: WorkContactSyncEvent{CorpID: 7},
		Raw:   "raw",
	})
	if queue.acked || !queue.retried || queue.retryReason == "" {
		t.Fatalf("queue acked=%v retried=%v reason=%q", queue.acked, queue.retried, queue.retryReason)
	}
}

func workContactSyncCredentialFixture() RoomWelcomeCorpCredential {
	return RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"}
}

type fakeWorkContactSyncWorkerStore struct {
	credential     RoomWelcomeCorpCredential
	credentialOK   bool
	credentialErr  error
	credentialByWX map[string]RoomWelcomeCorpCredential
	employees      []WorkContactSyncEmployee
	syncedCorpID   int
	syncedBundles  []WorkContactSyncEmployeeContacts
	syncErr        error
	tenantByCorpID map[int]int
}

func (s *fakeWorkContactSyncWorkerStore) TenantIDByCorpID(_ context.Context, corpID int) (int, error) {
	if s.tenantByCorpID == nil {
		return 0, nil
	}
	return s.tenantByCorpID[corpID], nil
}

func (s *fakeWorkContactSyncWorkerStore) RoomWelcomeCorpCredentialByID(_ context.Context, _ int) (RoomWelcomeCorpCredential, bool, error) {
	return s.credential, s.credentialOK, s.credentialErr
}

func (s *fakeWorkContactSyncWorkerStore) RoomWelcomeCorpCredentialByWXCorpID(_ context.Context, wxCorpID string) (RoomWelcomeCorpCredential, bool, error) {
	if s.credentialErr != nil {
		return RoomWelcomeCorpCredential{}, false, s.credentialErr
	}
	if s.credentialByWX == nil {
		return RoomWelcomeCorpCredential{}, false, nil
	}
	credential, ok := s.credentialByWX[wxCorpID]
	return credential, ok, nil
}

func (s *fakeWorkContactSyncWorkerStore) WorkContactSyncEmployees(_ context.Context, _ int) ([]WorkContactSyncEmployee, error) {
	return append([]WorkContactSyncEmployee{}, s.employees...), nil
}

func (s *fakeWorkContactSyncWorkerStore) SyncWorkContacts(_ context.Context, corpID int, bundles []WorkContactSyncEmployeeContacts) (WorkContactSyncResult, error) {
	s.syncedCorpID = corpID
	s.syncedBundles = append([]WorkContactSyncEmployeeContacts{}, bundles...)
	return WorkContactSyncResult{ContactsUpdated: len(bundles)}, s.syncErr
}

type fakeWorkContactSyncWorkerClient struct {
	lists     map[string][]string
	noContact map[string]bool
	details   map[string]WorkContactSyncContact
	listUsers []string
	detailIDs []string
	err       error
}

func (c *fakeWorkContactSyncWorkerClient) ExternalContactList(_ context.Context, _ RoomWelcomeCorpCredential, wxUserID string) ([]string, bool, error) {
	c.listUsers = append(c.listUsers, wxUserID)
	if c.err != nil {
		return nil, false, c.err
	}
	return c.lists[wxUserID], c.noContact[wxUserID], nil
}

func (c *fakeWorkContactSyncWorkerClient) ExternalContactDetail(_ context.Context, _ RoomWelcomeCorpCredential, wxExternalUserID string) (WorkContactSyncContact, error) {
	c.detailIDs = append(c.detailIDs, wxExternalUserID)
	if c.err != nil {
		return WorkContactSyncContact{}, c.err
	}
	return c.details[wxExternalUserID], nil
}

type fakeWorkContactSyncQueue struct {
	acked       bool
	retried     bool
	retryReason string
	deadLetter  bool
}

func (q *fakeWorkContactSyncQueue) DequeueWorkContactSync(_ context.Context, _ time.Duration) (WorkContactSyncDelivery, bool, error) {
	return WorkContactSyncDelivery{}, false, nil
}

func (q *fakeWorkContactSyncQueue) AckWorkContactSync(_ context.Context, _ WorkContactSyncDelivery) error {
	q.acked = true
	return nil
}

func (q *fakeWorkContactSyncQueue) RetryWorkContactSync(_ context.Context, _ WorkContactSyncDelivery, reason string, _ int) (bool, error) {
	q.retried = true
	q.retryReason = reason
	return q.deadLetter, nil
}

func (q *fakeWorkContactSyncQueue) RecoverWorkContactSyncProcessing(_ context.Context, _ time.Duration, _ int) (int, error) {
	return 0, nil
}
