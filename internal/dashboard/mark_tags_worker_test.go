package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"reflect"
	"testing"
	"time"
)

func TestMarkTagsEventSupportsLegacyArrayPayload(t *testing.T) {
	var event MarkTagsEvent
	if err := json.Unmarshal([]byte(`[7,101,3,[9,2]]`), &event); err != nil {
		t.Fatal(err)
	}
	if event.CorpID != 7 || event.ContactID != 101 || event.EmployeeID != 3 || !reflect.DeepEqual(event.TagIDs, []int{9, 2}) {
		t.Fatalf("event = %+v", event)
	}
}

func TestMarkTagsWorkerAppliesTagsAndSyncsWeCom(t *testing.T) {
	store := &fakeMarkTagsWorkerStore{
		credential: MarkTagsWorkerCredentialFixture(),
		result: MarkTagsApplyResult{
			WXUserID:         "go-employee",
			WXExternalUserID: "external-user",
			AddedWXTagIDs:    []string{"et1", "et2"},
			AddedTagNames:    []string{"重点", "成交"},
		},
	}
	client := &fakeMarkTagsWorkerClient{}
	worker := NewMarkTagsWorker(nil, store, client, log.New(io.Discard, "", 0))

	err := worker.Process(context.Background(), MarkTagsEvent{
		CorpID:     7,
		ContactID:  101,
		EmployeeID: 3,
		TagIDs:     []int{2, 0, 9, 2},
		Source:     "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	if store.applied.CorpID != 7 || store.applied.ContactID != 101 || store.applied.EmployeeID != 3 || !reflect.DeepEqual(store.applied.TagIDs, []int{2, 9}) {
		t.Fatalf("applied = %+v", store.applied)
	}
	if client.payload.UserID != "go-employee" || client.payload.ExternalUserID != "external-user" || !reflect.DeepEqual(client.payload.AddTag, []string{"et1", "et2"}) {
		t.Fatalf("payload = %+v", client.payload)
	}
}

func TestMarkTagsWorkerSkipsEmptyTags(t *testing.T) {
	store := &fakeMarkTagsWorkerStore{}
	client := &fakeMarkTagsWorkerClient{}
	worker := NewMarkTagsWorker(nil, store, client, log.New(io.Discard, "", 0))

	if err := worker.Process(context.Background(), MarkTagsEvent{CorpID: 7, ContactID: 101, EmployeeID: 3}); err != nil {
		t.Fatal(err)
	}
	if store.applied.CorpID != 0 {
		t.Fatalf("unexpected apply = %+v", store.applied)
	}
	if client.payload.UserID != "" {
		t.Fatalf("unexpected client payload = %+v", client.payload)
	}
}

func TestMarkTagsWorkerAcksSuccessfulDelivery(t *testing.T) {
	queue := &fakeMarkTagsQueue{}
	store := &fakeMarkTagsWorkerStore{credential: MarkTagsWorkerCredentialFixture()}
	worker := NewMarkTagsWorker(queue, store, &fakeMarkTagsWorkerClient{}, log.New(io.Discard, "", 0))

	worker.handleDelivery(context.Background(), MarkTagsDelivery{
		Raw: "raw-job",
		Event: MarkTagsEvent{
			CorpID:     7,
			ContactID:  101,
			EmployeeID: 3,
			TagIDs:     []int{2},
		},
	})

	if queue.ackRaw != "raw-job" {
		t.Fatalf("ack raw = %q", queue.ackRaw)
	}
	if queue.retryRaw != "" {
		t.Fatalf("unexpected retry raw = %q", queue.retryRaw)
	}
}

func TestMarkTagsWorkerRetriesFailedDelivery(t *testing.T) {
	queue := &fakeMarkTagsQueue{}
	store := &fakeMarkTagsWorkerStore{credentialErr: fmt.Errorf("boom")}
	worker := NewMarkTagsWorker(queue, store, &fakeMarkTagsWorkerClient{}, log.New(io.Discard, "", 0))

	worker.handleDelivery(context.Background(), MarkTagsDelivery{
		Raw:      "raw-job",
		Attempts: 1,
		Event: MarkTagsEvent{
			CorpID:     7,
			ContactID:  101,
			EmployeeID: 3,
			TagIDs:     []int{2},
		},
	})

	if queue.retryRaw != "raw-job" || queue.retryMaxAttempts != 3 {
		t.Fatalf("retry raw=%q max_attempts=%d", queue.retryRaw, queue.retryMaxAttempts)
	}
	if queue.ackRaw != "" {
		t.Fatalf("unexpected ack raw = %q", queue.ackRaw)
	}
}

func MarkTagsWorkerCredentialFixture() RoomWelcomeCorpCredential {
	return RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"}
}

type fakeMarkTagsWorkerStore struct {
	credential    RoomWelcomeCorpCredential
	credentialErr error
	applied       MarkTagsApplyValues
	result        MarkTagsApplyResult
	markedRecord  int
}

func (s *fakeMarkTagsWorkerStore) TenantIDByCorpID(_ context.Context, corpID int) (int, error) {
	return corpID + 1000, nil
}

func (s *fakeMarkTagsWorkerStore) RoomWelcomeCorpCredentialByID(_ context.Context, _ int) (RoomWelcomeCorpCredential, bool, error) {
	if s.credentialErr != nil {
		return RoomWelcomeCorpCredential{}, false, s.credentialErr
	}
	if s.credential.CorpID == 0 {
		return RoomWelcomeCorpCredential{}, false, nil
	}
	return s.credential, true, nil
}

func (s *fakeMarkTagsWorkerStore) ApplyWorkContactTags(_ context.Context, values MarkTagsApplyValues) (MarkTagsApplyResult, bool, error) {
	s.applied = values
	return s.result, true, nil
}

func (s *fakeMarkTagsWorkerStore) MarkAutoTagRecordApplied(_ context.Context, recordID int, _ int) error {
	s.markedRecord = recordID
	return nil
}

type fakeMarkTagsWorkerClient struct {
	payload WorkContactMarkTagsPayload
	err     error
}

func (c *fakeMarkTagsWorkerClient) MarkExternalContactTags(_ context.Context, _ RoomWelcomeCorpCredential, payload WorkContactMarkTagsPayload) error {
	c.payload = payload
	return c.err
}

type fakeMarkTagsQueue struct {
	ackRaw           string
	retryRaw         string
	retryReason      string
	retryMaxAttempts int
}

func (q *fakeMarkTagsQueue) DequeueMarkTags(_ context.Context, _ time.Duration) (MarkTagsDelivery, bool, error) {
	return MarkTagsDelivery{}, false, nil
}

func (q *fakeMarkTagsQueue) AckMarkTags(_ context.Context, delivery MarkTagsDelivery) error {
	q.ackRaw = delivery.Raw
	return nil
}

func (q *fakeMarkTagsQueue) RetryMarkTags(_ context.Context, delivery MarkTagsDelivery, reason string, maxAttempts int) (bool, error) {
	q.retryRaw = delivery.Raw
	q.retryReason = reason
	q.retryMaxAttempts = maxAttempts
	return false, nil
}

func (q *fakeMarkTagsQueue) RecoverMarkTagsProcessing(_ context.Context, _ time.Duration, _ int) (int, error) {
	return 0, nil
}
