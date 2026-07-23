package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jiyi/mochat-go/internal/taskrunner"
)

type AsyncFileUploadFile struct {
	SourcePath   string `json:"sourcePath"`
	TargetPath   string `json:"targetPath"`
	DeleteSource bool   `json:"deleteSource,omitempty"`
}

type AsyncFileUploadEvent struct {
	Files    []AsyncFileUploadFile `json:"files"`
	Source   string                `json:"source,omitempty"`
	TenantID int                   `json:"tenantId,omitempty"`
	CorpID   int                   `json:"corpId,omitempty"`
}

func (e *AsyncFileUploadEvent) UnmarshalJSON(raw []byte) error {
	type asyncFileUploadEventAlias AsyncFileUploadEvent
	var structured asyncFileUploadEventAlias
	if err := json.Unmarshal(raw, &structured); err == nil && structured.Files != nil {
		*e = AsyncFileUploadEvent(structured)
		return nil
	}

	var object struct {
		Files    json.RawMessage `json:"files"`
		Source   string          `json:"source"`
		TenantID int             `json:"tenantId"`
		CorpID   int             `json:"corpId"`
	}
	if err := json.Unmarshal(raw, &object); err == nil && len(object.Files) > 0 {
		files, err := asyncFileUploadFilesFromLegacyJSON(object.Files)
		if err != nil {
			return err
		}
		e.Files = files
		e.Source = object.Source
		e.TenantID = object.TenantID
		e.CorpID = object.CorpID
		return nil
	}

	files, err := asyncFileUploadFilesFromLegacyJSON(raw)
	if err != nil {
		return err
	}
	e.Files = files
	return nil
}

type AsyncFileUploadDelivery struct {
	Event    AsyncFileUploadEvent
	Raw      string
	Attempts int
}

type AsyncFileUploadWorkerQueue interface {
	DequeueAsyncFileUpload(ctx context.Context, timeout time.Duration) (AsyncFileUploadDelivery, bool, error)
	AckAsyncFileUpload(ctx context.Context, delivery AsyncFileUploadDelivery) error
	RetryAsyncFileUpload(ctx context.Context, delivery AsyncFileUploadDelivery, reason string, maxAttempts int) (bool, error)
	RecoverAsyncFileUploadProcessing(ctx context.Context, staleAfter time.Duration, maxAttempts int) (int, error)
}

type AsyncFileUploadWorker struct {
	queue             AsyncFileUploadWorkerQueue
	fileStorageRoot   string
	client            *http.Client
	pollTimeout       time.Duration
	maxAttempts       int
	processingTimeout time.Duration
	recoveryInterval  time.Duration
	logger            *log.Logger
	executionStore    any
	alertNotifier     SaaSAlertNotifier
}

func NewAsyncFileUploadWorker(queue AsyncFileUploadWorkerQueue, fileStorageRoot string, logger *log.Logger) *AsyncFileUploadWorker {
	if logger == nil {
		logger = log.Default()
	}
	fileStorageRoot = strings.TrimSpace(fileStorageRoot)
	if fileStorageRoot == "" {
		fileStorageRoot = defaultMediumFileStorageRoot
	}
	return &AsyncFileUploadWorker{
		queue:             queue,
		fileStorageRoot:   fileStorageRoot,
		client:            &http.Client{Timeout: 180 * time.Second},
		pollTimeout:       5 * time.Second,
		maxAttempts:       1,
		processingTimeout: 5 * time.Minute,
		recoveryInterval:  time.Minute,
		logger:            logger,
	}
}

func (w *AsyncFileUploadWorker) WithProcessingTimeout(timeout time.Duration) *AsyncFileUploadWorker {
	if timeout > 0 {
		w.processingTimeout = timeout
		w.recoveryInterval = timeout
		if w.recoveryInterval > time.Minute {
			w.recoveryInterval = time.Minute
		}
	}
	return w
}

func (w *AsyncFileUploadWorker) WithHTTPClient(client *http.Client) *AsyncFileUploadWorker {
	if client != nil {
		w.client = client
	}
	return w
}

func (w *AsyncFileUploadWorker) WithSaaSExecutionStore(store any) *AsyncFileUploadWorker {
	w.executionStore = store
	return w
}

func (w *AsyncFileUploadWorker) WithSaaSAlertNotifier(notifier SaaSAlertNotifier) *AsyncFileUploadWorker {
	w.alertNotifier = notifier
	return w
}

func (w *AsyncFileUploadWorker) Run(ctx context.Context) error {
	if w.queue == nil {
		return fmt.Errorf("async file upload worker dependencies are not configured")
	}
	nextRecovery := time.Now()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if !time.Now().Before(nextRecovery) {
			w.recoverProcessing(ctx)
			nextRecovery = time.Now().Add(w.recoveryInterval)
		}
		delivery, ok, err := w.queue.DequeueAsyncFileUpload(ctx, w.pollTimeout)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			w.logger.Printf("async file upload dequeue failed: %v", err)
			continue
		}
		if !ok {
			continue
		}
		w.handleDelivery(ctx, delivery)
	}
}

func (w *AsyncFileUploadWorker) recoverProcessing(ctx context.Context) {
	recovered, err := w.queue.RecoverAsyncFileUploadProcessing(ctx, w.processingTimeout, w.maxAttempts)
	if err != nil {
		w.logger.Printf("async file upload processing recovery failed: %v", err)
		return
	}
	if recovered > 0 {
		w.logger.Printf("async file upload recovered processing jobs: %d", recovered)
	}
}

func (w *AsyncFileUploadWorker) handleDelivery(ctx context.Context, delivery AsyncFileUploadDelivery) {
	tenantID := delivery.Event.TenantID
	if tenantID <= 0 && delivery.Event.CorpID > 0 {
		tenantID = tenantIDForQueueExecution(ctx, w.logger, w.executionStore, delivery.Event.CorpID)
	}
	if delivery.Event.TenantID <= 0 && tenantID > 0 {
		delivery.Event.TenantID = tenantID
	}
	ctx = WithSaaSAlertNotifier(ctx, w.alertNotifier)
	finishExecution := startQueueItemExecution(ctx, w.logger, QueueNameAsyncFileUpload, w.executionStore, tenantID)
	if err := w.Process(ctx, delivery.Event); err != nil {
		deadLettered, retryErr := w.queue.RetryAsyncFileUpload(ctx, delivery, err.Error(), w.maxAttempts)
		if retryErr != nil {
			finishExecution(taskrunner.StatusFailed, fmt.Errorf("%w; retry failed: %v", err, retryErr))
			w.logger.Printf("async file upload retry failed: source=%s files=%d err=%v retry_err=%v", delivery.Event.Source, len(delivery.Event.Files), err, retryErr)
			return
		}
		finishExecution(taskrunner.StatusFailed, err)
		if deadLettered {
			w.logger.Printf("async file upload moved to dead letter: source=%s files=%d attempts=%d err=%v", delivery.Event.Source, len(delivery.Event.Files), delivery.Attempts+1, err)
			return
		}
		w.logger.Printf("async file upload requeued: source=%s files=%d attempts=%d err=%v", delivery.Event.Source, len(delivery.Event.Files), delivery.Attempts+1, err)
		return
	}
	if err := w.queue.AckAsyncFileUpload(ctx, delivery); err != nil {
		finishExecution(taskrunner.StatusFailed, fmt.Errorf("ack failed: %w", err))
		w.logger.Printf("async file upload ack failed: source=%s files=%d err=%v", delivery.Event.Source, len(delivery.Event.Files), err)
		return
	}
	finishExecution(taskrunner.StatusSucceeded, nil)
}

func (w *AsyncFileUploadWorker) Process(ctx context.Context, event AsyncFileUploadEvent) error {
	if len(event.Files) == 0 {
		return fmt.Errorf("missing files")
	}
	root, err := filepath.Abs(w.fileStorageRoot)
	if err != nil {
		return err
	}
	if event.TenantID <= 0 && event.CorpID > 0 {
		event.TenantID = tenantIDForQueueExecution(ctx, w.logger, w.executionStore, event.CorpID)
	}
	for index, file := range event.Files {
		if err := w.processFile(ctx, root, event, file); err != nil {
			return fmt.Errorf("file %d: %w", index, err)
		}
	}
	return nil
}

func (w *AsyncFileUploadWorker) processFile(ctx context.Context, root string, event AsyncFileUploadEvent, file AsyncFileUploadFile) error {
	sourcePath := strings.TrimSpace(file.SourcePath)
	if sourcePath == "" {
		return nil
	}
	targetPath, relativePath, err := asyncFileUploadTargetPath(root, file.TargetPath)
	if err != nil {
		return err
	}
	if asyncFileUploadIsHTTPSource(sourcePath) {
		if err := w.copyHTTPSource(ctx, sourcePath, targetPath); err != nil {
			return err
		}
	} else {
		if err := w.copyLocalSource(sourcePath, targetPath); err != nil {
			return err
		}
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		return err
	}
	if err := w.recordStorageObject(ctx, event, file, relativePath, info.Size()); err != nil {
		_ = os.Remove(targetPath)
		return err
	}
	if !asyncFileUploadIsHTTPSource(sourcePath) && file.DeleteSource {
		if err := os.Remove(sourcePath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (w *AsyncFileUploadWorker) copyHTTPSource(ctx context.Context, sourceURL string, targetPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return err
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("GET %s returned %d", sourceURL, resp.StatusCode)
	}
	return asyncFileUploadWrite(targetPath, resp.Body)
}

func (w *AsyncFileUploadWorker) copyLocalSource(sourcePath string, targetPath string) error {
	input, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer input.Close()
	return asyncFileUploadWrite(targetPath, input)
}

func (w *AsyncFileUploadWorker) recordStorageObject(ctx context.Context, event AsyncFileUploadEvent, file AsyncFileUploadFile, relativePath string, sizeBytes int64) error {
	if event.TenantID <= 0 || sizeBytes <= 0 || w.executionStore == nil {
		return nil
	}
	if quotaStore, ok := w.executionStore.(commonUploadStorageQuotaStore); ok {
		status, err := quotaStore.SaaSStorageQuotaStatus(ctx, event.TenantID, sizeBytes)
		if err != nil {
			return err
		}
		if status.Exceeded() {
			return NewSaaSQuotaExceededError(status)
		}
	}
	recorder, ok := w.executionStore.(commonUploadStorageRecorder)
	if !ok {
		return nil
	}
	source := strings.TrimSpace(event.Source)
	if source == "" {
		source = "file_upload_queue"
	}
	if err := recorder.RecordCommonUploadStorageObject(ctx, CommonUploadStorageObject{
		TenantID:     event.TenantID,
		CorpID:       event.CorpID,
		Source:       source,
		OriginalName: asyncFileUploadOriginalName(file),
		RelativePath: relativePath,
		SizeBytes:    sizeBytes,
	}); err != nil {
		return err
	}
	if quotaStore, ok := w.executionStore.(commonUploadStorageQuotaStore); ok {
		if err := quotaStore.RefreshSaaSUsageCounter(ctx, event.TenantID, SaaSMetricStorage); err != nil {
			return err
		}
	}
	return nil
}

func asyncFileUploadTargetPath(root string, targetPath string) (string, string, error) {
	targetPath = strings.TrimSpace(targetPath)
	if !commonUploadSafeRelativePath(targetPath) {
		return "", "", fmt.Errorf("target path is unsafe")
	}
	relativePath := filepath.ToSlash(filepath.Clean(filepath.FromSlash(targetPath)))
	target := filepath.Join(root, filepath.FromSlash(relativePath))
	if target != root && !strings.HasPrefix(target, root+string(filepath.Separator)) {
		return "", "", fmt.Errorf("target path is outside storage root")
	}
	return target, relativePath, nil
}

func asyncFileUploadOriginalName(file AsyncFileUploadFile) string {
	sourcePath := strings.TrimSpace(file.SourcePath)
	if sourcePath != "" && !asyncFileUploadIsHTTPSource(sourcePath) {
		if name := filepath.Base(sourcePath); name != "." && name != string(filepath.Separator) {
			return name
		}
	}
	if name := filepath.Base(strings.TrimSpace(file.TargetPath)); name != "." && name != string(filepath.Separator) {
		return name
	}
	return ""
}

func asyncFileUploadWrite(targetPath string, input io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(targetPath), ".async-upload-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if err := tmp.Chmod(0644); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := io.Copy(tmp, input); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, targetPath)
}

func asyncFileUploadIsHTTPSource(sourcePath string) bool {
	sourcePath = strings.ToLower(strings.TrimSpace(sourcePath))
	return strings.HasPrefix(sourcePath, "http://") || strings.HasPrefix(sourcePath, "https://")
}

func asyncFileUploadFilesFromLegacyJSON(raw []byte) ([]AsyncFileUploadFile, error) {
	var rows [][]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	files := make([]AsyncFileUploadFile, 0, len(rows))
	for index, row := range rows {
		if len(row) < 2 {
			return nil, fmt.Errorf("file %d has fewer than 2 fields", index)
		}
		sourcePath, ok := asyncFileUploadLegacyString(row[0])
		if !ok {
			return nil, fmt.Errorf("file %d source must be a string", index)
		}
		targetPath, ok := asyncFileUploadLegacyString(row[1])
		if !ok {
			return nil, fmt.Errorf("file %d target must be a string", index)
		}
		file := AsyncFileUploadFile{SourcePath: sourcePath, TargetPath: targetPath}
		if len(row) > 2 {
			file.DeleteSource = asyncFileUploadLegacyBool(row[2])
		}
		files = append(files, file)
	}
	return files, nil
}

func asyncFileUploadLegacyString(value any) (string, bool) {
	if value == nil {
		return "", true
	}
	text, ok := value.(string)
	return text, ok
}

func asyncFileUploadLegacyBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case float64:
		return typed != 0
	case string:
		typed = strings.TrimSpace(strings.ToLower(typed))
		return typed == "1" || typed == "true" || typed == "yes"
	default:
		return false
	}
}
