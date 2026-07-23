package dashboard

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAsyncFileUploadEventSupportsLegacyArrayPayload(t *testing.T) {
	var event AsyncFileUploadEvent
	err := json.Unmarshal([]byte(`[["/tmp/a.txt","queued/a.txt",1],["https://example.com/b.txt","queued/b.txt","true"]]`), &event)
	if err != nil {
		t.Fatal(err)
	}
	if len(event.Files) != 2 {
		t.Fatalf("files = %+v", event.Files)
	}
	if event.Files[0].SourcePath != "/tmp/a.txt" || event.Files[0].TargetPath != "queued/a.txt" || !event.Files[0].DeleteSource {
		t.Fatalf("first file = %+v", event.Files[0])
	}
	if !event.Files[1].DeleteSource {
		t.Fatalf("second file = %+v", event.Files[1])
	}
}

func TestAsyncFileUploadEventSupportsStructuredTenantPayload(t *testing.T) {
	var event AsyncFileUploadEvent
	err := json.Unmarshal([]byte(`{"tenantId":8,"corpId":17,"source":"file_upload_queue","files":[["/tmp/a.txt","queued/a.txt",1]]}`), &event)
	if err != nil {
		t.Fatal(err)
	}
	if event.TenantID != 8 || event.CorpID != 17 || event.Source != "file_upload_queue" {
		t.Fatalf("event metadata = %+v", event)
	}
	if len(event.Files) != 1 || event.Files[0].SourcePath != "/tmp/a.txt" || event.Files[0].TargetPath != "queued/a.txt" || !event.Files[0].DeleteSource {
		t.Fatalf("files = %+v", event.Files)
	}
}

func TestAsyncFileUploadWorkerCopiesLocalFileAndDeletesSource(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(sourcePath, []byte("local payload"), 0644); err != nil {
		t.Fatal(err)
	}

	worker := NewAsyncFileUploadWorker(nil, root, log.New(io.Discard, "", 0))
	err := worker.Process(context.Background(), AsyncFileUploadEvent{
		Files: []AsyncFileUploadFile{{
			SourcePath:   sourcePath,
			TargetPath:   "queued/local.txt",
			DeleteSource: true,
		}},
		Source: "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(root, "queued", "local.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "local payload" {
		t.Fatalf("target content = %q", string(raw))
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("source file still exists err=%v", err)
	}
}

func TestAsyncFileUploadWorkerCopiesHTTPFile(t *testing.T) {
	root := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("remote payload"))
	}))
	defer server.Close()

	worker := NewAsyncFileUploadWorker(nil, root, log.New(io.Discard, "", 0)).
		WithHTTPClient(server.Client())
	err := worker.Process(context.Background(), AsyncFileUploadEvent{
		Files: []AsyncFileUploadFile{{
			SourcePath: server.URL + "/asset.txt",
			TargetPath: "remote/asset.txt",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "remote", "asset.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "remote payload" {
		t.Fatalf("target content = %q", string(raw))
	}
}

func TestAsyncFileUploadWorkerRecordsSaaSStorageUsage(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(t.TempDir(), "source.txt")
	payload := []byte("tenant storage payload")
	if err := os.WriteFile(sourcePath, payload, 0644); err != nil {
		t.Fatal(err)
	}
	store := &fakeAsyncFileUploadStorageStore{
		quota: SaaSQuotaStatus{Metric: SaaSMetricStorage, TenantID: 8, Current: 0, Limit: 10},
	}
	worker := NewAsyncFileUploadWorker(nil, root, log.New(io.Discard, "", 0)).
		WithSaaSExecutionStore(store)

	err := worker.Process(context.Background(), AsyncFileUploadEvent{
		TenantID: 8,
		CorpID:   17,
		Source:   "file_upload_queue",
		Files: []AsyncFileUploadFile{{
			SourcePath:   sourcePath,
			TargetPath:   "tenant/copied.txt",
			DeleteSource: true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("source file still exists err=%v", err)
	}
	if store.quotaTenantID != 8 || store.quotaAdditionalBytes != int64(len(payload)) {
		t.Fatalf("quota tenant=%d additional=%d", store.quotaTenantID, store.quotaAdditionalBytes)
	}
	if store.record.TenantID != 8 || store.record.CorpID != 17 || store.record.Source != "file_upload_queue" {
		t.Fatalf("record owner = %+v", store.record)
	}
	if store.record.RelativePath != "tenant/copied.txt" || store.record.OriginalName != "source.txt" || store.record.SizeBytes != int64(len(payload)) {
		t.Fatalf("record file = %+v", store.record)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricStorage {
		t.Fatalf("refresh tenant=%d metric=%q", store.refreshTenantID, store.refreshMetric)
	}
}

func TestAsyncFileUploadWorkerRejectsSaaSStorageQuotaBeforeDeletingSource(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(sourcePath, []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	store := &fakeAsyncFileUploadStorageStore{
		quota: SaaSQuotaStatus{Metric: SaaSMetricStorage, TenantID: 8, Current: 1, Limit: 1, Additional: 1},
	}
	worker := NewAsyncFileUploadWorker(nil, root, log.New(io.Discard, "", 0)).
		WithSaaSExecutionStore(store)

	err := worker.Process(context.Background(), AsyncFileUploadEvent{
		TenantID: 8,
		Files: []AsyncFileUploadFile{{
			SourcePath:   sourcePath,
			TargetPath:   "tenant/over-limit.txt",
			DeleteSource: true,
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "套餐额度已达上限") {
		t.Fatalf("expected quota error, got %v", err)
	}
	if _, err := os.Stat(sourcePath); err != nil {
		t.Fatalf("source file should remain after quota rejection: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "tenant", "over-limit.txt")); !os.IsNotExist(err) {
		t.Fatalf("target should be removed after quota rejection err=%v", err)
	}
	if store.record.SizeBytes != 0 {
		t.Fatalf("unexpected storage record = %+v", store.record)
	}
}

func TestAsyncFileUploadWorkerRejectsUnsafeTargetPath(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(sourcePath, []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}

	worker := NewAsyncFileUploadWorker(nil, root, log.New(io.Discard, "", 0))
	err := worker.Process(context.Background(), AsyncFileUploadEvent{
		Files: []AsyncFileUploadFile{{
			SourcePath: sourcePath,
			TargetPath: "../escape.txt",
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("expected unsafe target error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "..", "escape.txt")); err == nil {
		t.Fatal("unexpected escaped file")
	}
}

func TestAsyncFileUploadWorkerAcksSuccessfulDelivery(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(sourcePath, []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	queue := &fakeAsyncFileUploadQueue{}
	worker := NewAsyncFileUploadWorker(queue, root, log.New(io.Discard, "", 0))

	worker.handleDelivery(context.Background(), AsyncFileUploadDelivery{
		Raw: "raw-job",
		Event: AsyncFileUploadEvent{Files: []AsyncFileUploadFile{{
			SourcePath: sourcePath,
			TargetPath: "ack/copied.txt",
		}}},
	})

	if queue.ackRaw != "raw-job" {
		t.Fatalf("ack raw = %q", queue.ackRaw)
	}
	if queue.retryRaw != "" {
		t.Fatalf("unexpected retry raw = %q", queue.retryRaw)
	}
}

func TestAsyncFileUploadWorkerRefreshesAsyncExecutionUsageForTenant(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(sourcePath, []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	queue := &fakeAsyncFileUploadQueue{}
	store := &fakeQueueExecutionQuotaStore{
		status: SaaSQuotaStatus{Metric: SaaSMetricAsyncExecutions, TenantID: 8, Current: 1, Limit: 10},
	}
	worker := NewAsyncFileUploadWorker(queue, root, log.New(io.Discard, "", 0)).
		WithSaaSExecutionStore(store)

	worker.handleDelivery(context.Background(), AsyncFileUploadDelivery{
		Raw: "raw-job",
		Event: AsyncFileUploadEvent{
			TenantID: 8,
			Files: []AsyncFileUploadFile{{
				SourcePath: sourcePath,
				TargetPath: "tenant/copied.txt",
			}},
		},
	})

	if queue.ackRaw != "raw-job" {
		t.Fatalf("ack raw = %q", queue.ackRaw)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricAsyncExecutions {
		t.Fatalf("refresh tenant=%d metric=%q", store.refreshTenantID, store.refreshMetric)
	}
}

func TestAsyncFileUploadWorkerResolvesTenantFromCorpForUsage(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(sourcePath, []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	queue := &fakeAsyncFileUploadQueue{}
	store := &fakeAsyncFileUploadExecutionStore{
		corpTenantID: 8,
	}
	store.status = SaaSQuotaStatus{Metric: SaaSMetricAsyncExecutions, TenantID: 8, Current: 1, Limit: 10}
	worker := NewAsyncFileUploadWorker(queue, root, log.New(io.Discard, "", 0)).
		WithSaaSExecutionStore(store)

	worker.handleDelivery(context.Background(), AsyncFileUploadDelivery{
		Raw: "raw-job",
		Event: AsyncFileUploadEvent{
			CorpID: 17,
			Files: []AsyncFileUploadFile{{
				SourcePath: sourcePath,
				TargetPath: "corp/copied.txt",
			}},
		},
	})

	if queue.ackRaw != "raw-job" {
		t.Fatalf("ack raw = %q", queue.ackRaw)
	}
	if store.resolvedCorpID != 17 {
		t.Fatalf("resolved corp = %d", store.resolvedCorpID)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricAsyncExecutions {
		t.Fatalf("refresh tenant=%d metric=%q", store.refreshTenantID, store.refreshMetric)
	}
}

func TestAsyncFileUploadWorkerMovesFailedDeliveryToDeadLetter(t *testing.T) {
	queue := &fakeAsyncFileUploadQueue{deadLettered: true}
	worker := NewAsyncFileUploadWorker(queue, t.TempDir(), log.New(io.Discard, "", 0))

	worker.handleDelivery(context.Background(), AsyncFileUploadDelivery{
		Raw: "raw-job",
		Event: AsyncFileUploadEvent{Files: []AsyncFileUploadFile{{
			SourcePath: filepath.Join(t.TempDir(), "missing.txt"),
			TargetPath: "missing/copied.txt",
		}}},
	})

	if queue.retryRaw != "raw-job" || queue.retryMaxAttempts != 1 {
		t.Fatalf("retry raw=%q max_attempts=%d", queue.retryRaw, queue.retryMaxAttempts)
	}
	if queue.ackRaw != "" {
		t.Fatalf("unexpected ack raw = %q", queue.ackRaw)
	}
	if queue.retryReason == "" {
		t.Fatal("missing retry reason")
	}
}

type fakeAsyncFileUploadQueue struct {
	ackRaw           string
	retryRaw         string
	retryReason      string
	retryMaxAttempts int
	deadLettered     bool
}

func (q *fakeAsyncFileUploadQueue) DequeueAsyncFileUpload(_ context.Context, _ time.Duration) (AsyncFileUploadDelivery, bool, error) {
	return AsyncFileUploadDelivery{}, false, nil
}

func (q *fakeAsyncFileUploadQueue) AckAsyncFileUpload(_ context.Context, delivery AsyncFileUploadDelivery) error {
	q.ackRaw = delivery.Raw
	return nil
}

func (q *fakeAsyncFileUploadQueue) RetryAsyncFileUpload(_ context.Context, delivery AsyncFileUploadDelivery, reason string, maxAttempts int) (bool, error) {
	q.retryRaw = delivery.Raw
	q.retryReason = reason
	q.retryMaxAttempts = maxAttempts
	return q.deadLettered, nil
}

func (q *fakeAsyncFileUploadQueue) RecoverAsyncFileUploadProcessing(_ context.Context, _ time.Duration, _ int) (int, error) {
	return 0, nil
}

type fakeAsyncFileUploadExecutionStore struct {
	fakeQueueExecutionQuotaStore
	corpTenantID   int
	resolvedCorpID int
}

func (s *fakeAsyncFileUploadExecutionStore) TenantIDByCorpID(_ context.Context, corpID int) (int, error) {
	s.resolvedCorpID = corpID
	return s.corpTenantID, nil
}

type fakeAsyncFileUploadStorageStore struct {
	quota                SaaSQuotaStatus
	quotaTenantID        int
	quotaAdditionalBytes int64
	record               CommonUploadStorageObject
	refreshTenantID      int
	refreshMetric        string
}

func (s *fakeAsyncFileUploadStorageStore) SaaSStorageQuotaStatus(_ context.Context, tenantID int, additionalBytes int64) (SaaSQuotaStatus, error) {
	s.quotaTenantID = tenantID
	s.quotaAdditionalBytes = additionalBytes
	if s.quota.Metric == "" {
		return SaaSQuotaStatus{Metric: SaaSMetricStorage, TenantID: tenantID}, nil
	}
	return s.quota, nil
}

func (s *fakeAsyncFileUploadStorageStore) RecordCommonUploadStorageObject(_ context.Context, object CommonUploadStorageObject) error {
	s.record = object
	return nil
}

func (s *fakeAsyncFileUploadStorageStore) RefreshSaaSUsageCounter(_ context.Context, tenantID int, metric string) error {
	s.refreshTenantID = tenantID
	s.refreshMetric = metric
	return nil
}
