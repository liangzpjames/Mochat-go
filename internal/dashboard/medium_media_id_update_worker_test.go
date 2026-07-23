package dashboard

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"jiyi/mochat-go/internal/taskrunner"
)

func TestMediumMediaIDUpdateEventUnmarshalObjectAndLegacyPayloads(t *testing.T) {
	var objectEvent MediumMediaIDUpdateEvent
	if err := json.Unmarshal([]byte(`{"corp_id":"7","medium_ids":["21",9,21],"source":" media_id_update "}`), &objectEvent); err != nil {
		t.Fatal(err)
	}
	if objectEvent.CorpID != 7 || !reflect.DeepEqual(objectEvent.MediumIDs, []int{21, 9, 21}) || objectEvent.Source != " media_id_update " {
		t.Fatalf("object event = %+v", objectEvent)
	}

	var legacyEvent MediumMediaIDUpdateEvent
	if err := json.Unmarshal([]byte(`[7,["21",9],"php.queue"]`), &legacyEvent); err != nil {
		t.Fatal(err)
	}
	if legacyEvent.CorpID != 7 || !reflect.DeepEqual(legacyEvent.MediumIDs, []int{21, 9}) || legacyEvent.Source != "php.queue" {
		t.Fatalf("legacy event = %+v", legacyEvent)
	}
}

func TestMediumMediaIDUpdateWorkerUploadsExpiredMedia(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "image"), 0o755); err != nil {
		t.Fatal(err)
	}
	localPath := filepath.Join(root, "image", "hello.txt")
	if err := os.WriteFile(localPath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2_000_000_000, 0)
	store := &fakeMediumMediaIDUpdateWorkerStore{
		credential: MediumCorpCredential{CorpID: 7, WXCorpID: "wx-corp", EmployeeSecret: "employee-secret"},
		corpFound:  true,
		items: map[int]MediumMediaUpdateItem{
			21: {
				ID:             21,
				Type:           2,
				MediaID:        "old-media-id",
				LastUploadTime: 1,
				Content:        map[string]any{"imagePath": "image/hello.txt"},
			},
		},
	}
	client := &fakeMediumMediaClient{mediaID: "worker-media-id"}
	worker := NewMediumMediaIDUpdateWorker(nil, store, client, root, log.Default())
	worker.now = func() time.Time { return now }

	if err := worker.Process(context.Background(), MediumMediaIDUpdateEvent{CorpID: 7, MediumIDs: []int{21, 21, 0}, Source: "test"}); err != nil {
		t.Fatal(err)
	}
	if client.calls != 1 || client.lastMediaType != "image" || client.lastFilePath != localPath {
		t.Fatalf("client calls=%d type=%q path=%q", client.calls, client.lastMediaType, client.lastFilePath)
	}
	if store.updatedMediumID != 21 || store.updatedMediaID != "worker-media-id" || store.updatedLastUploadTime != now.Unix() {
		t.Fatalf("update = id %d media %q time %d", store.updatedMediumID, store.updatedMediaID, store.updatedLastUploadTime)
	}
}

func TestMediumMediaIDUpdateWorkerRequiresCorpAndMediumIDs(t *testing.T) {
	worker := NewMediumMediaIDUpdateWorker(nil, &fakeMediumMediaIDUpdateWorkerStore{}, &fakeMediumMediaClient{}, t.TempDir(), log.Default())
	if err := worker.Process(context.Background(), MediumMediaIDUpdateEvent{MediumIDs: []int{21}}); err == nil {
		t.Fatalf("expected missing corp id error")
	}
	if err := worker.Process(context.Background(), MediumMediaIDUpdateEvent{CorpID: 7}); err == nil {
		t.Fatalf("expected missing medium ids error")
	}
}

func TestMediumMediaIDUpdateWorkerAcksSuccessfulDeliveryAndRecordsExecution(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "file"), 0o755); err != nil {
		t.Fatal(err)
	}
	localPath := filepath.Join(root, "file", "hello.txt")
	if err := os.WriteFile(localPath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	queue := &fakeMediumMediaIDUpdateWorkerQueue{}
	store := &fakeMediumMediaIDUpdateWorkerStore{
		tenantIDs:  map[int]int{7: 11},
		credential: MediumCorpCredential{CorpID: 7, WXCorpID: "wx-corp", EmployeeSecret: "employee-secret"},
		corpFound:  true,
		items: map[int]MediumMediaUpdateItem{
			21: {ID: 21, Type: 7, LastUploadTime: 1, Content: map[string]any{"filePath": "file/hello.txt"}},
		},
	}
	recorder := &fakeWorkerExecutionRecorder{}
	ctx := taskrunner.WithTaskRuntime(context.Background(), QueueNameMediumMediaIDUpdate, "run-media-1", recorder)
	worker := NewMediumMediaIDUpdateWorker(queue, store, &fakeMediumMediaClient{mediaID: "worker-file-media-id"}, root, log.Default())
	worker.now = func() time.Time { return time.Unix(2_000_000_000, 0) }

	worker.handleDelivery(ctx, MediumMediaIDUpdateDelivery{Raw: "raw-job", Event: MediumMediaIDUpdateEvent{CorpID: 7, MediumIDs: []int{21}}})

	if queue.ackedRaw != "raw-job" {
		t.Fatalf("acked raw = %q", queue.ackedRaw)
	}
	if queue.retryRaw != "" {
		t.Fatalf("unexpected retry raw = %q", queue.retryRaw)
	}
	running := recordedExecutionByStatus(t, recorder, QueueNameMediumMediaIDUpdate, taskrunner.StatusRunning)
	succeeded := recordedExecutionByStatus(t, recorder, QueueNameMediumMediaIDUpdate, taskrunner.StatusSucceeded)
	if running.ExecutionID == "" || running.TenantID != 11 || succeeded.TenantID != 11 || succeeded.ExecutionID != running.ExecutionID || succeeded.RunID != "run-media-1" || succeeded.StoppedAt == "" || succeeded.Error != "" {
		t.Fatalf("executions = %+v", recorder.executions)
	}
}

func TestMediumMediaIDUpdateWorkerRetriesFailedDelivery(t *testing.T) {
	queue := &fakeMediumMediaIDUpdateWorkerQueue{}
	worker := NewMediumMediaIDUpdateWorker(queue, &fakeMediumMediaIDUpdateWorkerStore{}, &fakeMediumMediaClient{}, t.TempDir(), log.Default())

	worker.handleDelivery(context.Background(), MediumMediaIDUpdateDelivery{Raw: "raw-job", Attempts: 1, Event: MediumMediaIDUpdateEvent{CorpID: 7, MediumIDs: []int{21}, Source: "test"}})

	if queue.ackedRaw != "" {
		t.Fatalf("unexpected ack raw = %q", queue.ackedRaw)
	}
	if queue.retryRaw != "raw-job" || queue.retryReason == "" || queue.retryMaxAttempts != 3 {
		t.Fatalf("retry = raw:%q reason:%q max:%d", queue.retryRaw, queue.retryReason, queue.retryMaxAttempts)
	}
}

type fakeMediumMediaIDUpdateWorkerStore struct {
	tenantIDs             map[int]int
	corpIDs               []int
	itemsByCorp           map[int][]MediumMediaUpdateItem
	items                 map[int]MediumMediaUpdateItem
	credential            MediumCorpCredential
	corpFound             bool
	updatedMediumID       int
	updatedMediaID        string
	updatedLastUploadTime int64
}

func (s *fakeMediumMediaIDUpdateWorkerStore) TenantIDByCorpID(_ context.Context, corpID int) (int, error) {
	return s.tenantIDs[corpID], nil
}

func (s *fakeMediumMediaIDUpdateWorkerStore) ActiveCorpIDs(context.Context) ([]int, error) {
	return append([]int{}, s.corpIDs...), nil
}

func (s *fakeMediumMediaIDUpdateWorkerStore) MediumMediaForUpdateByCorp(_ context.Context, corpID int, _ int64) ([]MediumMediaUpdateItem, error) {
	return append([]MediumMediaUpdateItem{}, s.itemsByCorp[corpID]...), nil
}

func (s *fakeMediumMediaIDUpdateWorkerStore) MediumCorpCredentialByID(context.Context, int) (MediumCorpCredential, bool, error) {
	return s.credential, s.corpFound, nil
}

func (s *fakeMediumMediaIDUpdateWorkerStore) MediumMediaForUpdateByID(_ context.Context, mediumID int) (MediumMediaUpdateItem, bool, error) {
	item, ok := s.items[mediumID]
	return item, ok, nil
}

func (s *fakeMediumMediaIDUpdateWorkerStore) UpdateMediumMediaID(_ context.Context, mediumID int, mediaID string, lastUploadTime int64) (bool, error) {
	s.updatedMediumID = mediumID
	s.updatedMediaID = mediaID
	s.updatedLastUploadTime = lastUploadTime
	return true, nil
}

type fakeMediumMediaIDUpdateWorkerQueue struct {
	ackedRaw         string
	retryRaw         string
	retryReason      string
	retryMaxAttempts int
	deadLettered     bool
}

func (q *fakeMediumMediaIDUpdateWorkerQueue) DequeueMediumMediaIDUpdate(_ context.Context, _ time.Duration) (MediumMediaIDUpdateDelivery, bool, error) {
	return MediumMediaIDUpdateDelivery{}, false, nil
}

func (q *fakeMediumMediaIDUpdateWorkerQueue) AckMediumMediaIDUpdate(_ context.Context, delivery MediumMediaIDUpdateDelivery) error {
	q.ackedRaw = delivery.Raw
	return nil
}

func (q *fakeMediumMediaIDUpdateWorkerQueue) RetryMediumMediaIDUpdate(_ context.Context, delivery MediumMediaIDUpdateDelivery, reason string, maxAttempts int) (bool, error) {
	q.retryRaw = delivery.Raw
	q.retryReason = reason
	q.retryMaxAttempts = maxAttempts
	return q.deadLettered, nil
}

func (q *fakeMediumMediaIDUpdateWorkerQueue) RecoverMediumMediaIDUpdateProcessing(_ context.Context, _ time.Duration, _ int) (int, error) {
	return 0, nil
}
