package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"jiyi/mochat-go/internal/taskrunner"
)

type MediumMediaIDUpdateEvent struct {
	CorpID    int    `json:"corpId,omitempty"`
	MediumIDs []int  `json:"mediumIds,omitempty"`
	Source    string `json:"source,omitempty"`
}

func (e *MediumMediaIDUpdateEvent) UnmarshalJSON(raw []byte) error {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if strings.HasPrefix(trimmed, "{") {
		return e.unmarshalMediumMediaIDUpdateObject(raw)
	}
	return e.unmarshalMediumMediaIDUpdateLegacyArray(raw)
}

func (e *MediumMediaIDUpdateEvent) unmarshalMediumMediaIDUpdateObject(raw []byte) error {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	if rawValue, ok := firstJSONField(payload, "corpId", "corp_id"); ok {
		corpID, err := decodeMessageRemindInt(rawValue)
		if err != nil {
			return fmt.Errorf("corpId: %w", err)
		}
		e.CorpID = corpID
	}
	if rawValue, ok := firstJSONField(payload, "mediumIds", "medium_ids", "ids"); ok {
		mediumIDs, err := decodeMediumMediaIDUpdateIDs(rawValue)
		if err != nil {
			return fmt.Errorf("mediumIds: %w", err)
		}
		e.MediumIDs = mediumIDs
	}
	if err := decodeOptionalJSONField(payload, &e.Source, "source"); err != nil {
		return fmt.Errorf("source: %w", err)
	}
	return nil
}

func (e *MediumMediaIDUpdateEvent) unmarshalMediumMediaIDUpdateLegacyArray(raw []byte) error {
	var legacy []json.RawMessage
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return err
	}
	if len(legacy) == 0 {
		return nil
	}
	corpID, err := decodeMessageRemindInt(legacy[0])
	if err != nil {
		return fmt.Errorf("corpId: %w", err)
	}
	e.CorpID = corpID
	if len(legacy) > 1 && strings.TrimSpace(string(legacy[1])) != "null" {
		mediumIDs, err := decodeMediumMediaIDUpdateIDs(legacy[1])
		if err != nil {
			return fmt.Errorf("mediumIds: %w", err)
		}
		e.MediumIDs = mediumIDs
	}
	if len(legacy) > 2 && strings.TrimSpace(string(legacy[2])) != "null" {
		if err := json.Unmarshal(legacy[2], &e.Source); err != nil {
			return fmt.Errorf("source: %w", err)
		}
	}
	return nil
}

func decodeMediumMediaIDUpdateIDs(raw json.RawMessage) ([]int, error) {
	value, err := decodeMessageRemindAny(raw)
	if err != nil {
		return nil, err
	}
	switch typed := value.(type) {
	case []any:
		out := make([]int, 0, len(typed))
		for _, item := range typed {
			id, err := decodeWorkDepartmentListIntValue(item)
			if err != nil {
				return nil, err
			}
			if id > 0 {
				out = append(out, id)
			}
		}
		return out, nil
	default:
		id, err := decodeWorkDepartmentListIntValue(typed)
		if err != nil {
			return nil, err
		}
		if id <= 0 {
			return nil, nil
		}
		return []int{id}, nil
	}
}

type MediumMediaIDUpdateDelivery struct {
	Event    MediumMediaIDUpdateEvent
	Raw      string
	Attempts int
}

type MediumMediaIDUpdateWorkerQueue interface {
	DequeueMediumMediaIDUpdate(ctx context.Context, timeout time.Duration) (MediumMediaIDUpdateDelivery, bool, error)
	AckMediumMediaIDUpdate(ctx context.Context, delivery MediumMediaIDUpdateDelivery) error
	RetryMediumMediaIDUpdate(ctx context.Context, delivery MediumMediaIDUpdateDelivery, reason string, maxAttempts int) (bool, error)
	RecoverMediumMediaIDUpdateProcessing(ctx context.Context, staleAfter time.Duration, maxAttempts int) (int, error)
}

type MediumMediaIDUpdateWorkerStore interface {
	MediumMediaCronStore
	MediumMediaForUpdateByID(ctx context.Context, mediumID int) (MediumMediaUpdateItem, bool, error)
}

type MediumMediaIDUpdateWorker struct {
	queue             MediumMediaIDUpdateWorkerQueue
	store             MediumMediaIDUpdateWorkerStore
	client            MediumMediaClient
	fileStorageRoot   string
	pollTimeout       time.Duration
	maxAttempts       int
	processingTimeout time.Duration
	recoveryInterval  time.Duration
	alertNotifier     SaaSAlertNotifier
	logger            *log.Logger
	now               func() time.Time
}

func NewMediumMediaIDUpdateWorker(queue MediumMediaIDUpdateWorkerQueue, store MediumMediaIDUpdateWorkerStore, client MediumMediaClient, fileStorageRoot string, logger *log.Logger) *MediumMediaIDUpdateWorker {
	if logger == nil {
		logger = log.Default()
	}
	fileStorageRoot = strings.TrimSpace(fileStorageRoot)
	if fileStorageRoot == "" {
		fileStorageRoot = defaultMediumFileStorageRoot
	}
	return &MediumMediaIDUpdateWorker{
		queue:             queue,
		store:             store,
		client:            client,
		fileStorageRoot:   fileStorageRoot,
		pollTimeout:       5 * time.Second,
		maxAttempts:       3,
		processingTimeout: 5 * time.Minute,
		recoveryInterval:  time.Minute,
		logger:            logger,
	}
}

func (w *MediumMediaIDUpdateWorker) WithProcessingTimeout(timeout time.Duration) *MediumMediaIDUpdateWorker {
	if timeout > 0 {
		w.processingTimeout = timeout
		w.recoveryInterval = timeout
		if w.recoveryInterval > time.Minute {
			w.recoveryInterval = time.Minute
		}
	}
	return w
}

func (w *MediumMediaIDUpdateWorker) WithSaaSAlertNotifier(notifier SaaSAlertNotifier) *MediumMediaIDUpdateWorker {
	w.alertNotifier = notifier
	return w
}

func (w *MediumMediaIDUpdateWorker) Run(ctx context.Context) error {
	if w.queue == nil || w.store == nil || w.client == nil {
		return fmt.Errorf("medium media_id update worker dependencies are not configured")
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
		delivery, ok, err := w.queue.DequeueMediumMediaIDUpdate(ctx, w.pollTimeout)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			w.logger.Printf("medium media_id update dequeue failed: %v", err)
			continue
		}
		if !ok {
			continue
		}
		w.handleDelivery(ctx, delivery)
	}
}

func (w *MediumMediaIDUpdateWorker) recoverProcessing(ctx context.Context) {
	recovered, err := w.queue.RecoverMediumMediaIDUpdateProcessing(ctx, w.processingTimeout, w.maxAttempts)
	if err != nil {
		w.logger.Printf("medium media_id update processing recovery failed: %v", err)
		return
	}
	if recovered > 0 {
		w.logger.Printf("medium media_id update recovered processing jobs: %d", recovered)
	}
}

func (w *MediumMediaIDUpdateWorker) handleDelivery(ctx context.Context, delivery MediumMediaIDUpdateDelivery) {
	ctx = WithSaaSAlertNotifier(ctx, w.alertNotifier)
	tenantID := tenantIDForQueueExecution(ctx, w.logger, w.store, delivery.Event.CorpID)
	finishExecution := startQueueItemExecution(ctx, w.logger, QueueNameMediumMediaIDUpdate, w.store, tenantID)
	if err := w.Process(ctx, delivery.Event); err != nil {
		deadLettered, retryErr := w.queue.RetryMediumMediaIDUpdate(ctx, delivery, err.Error(), w.maxAttempts)
		if retryErr != nil {
			finishExecution(taskrunner.StatusFailed, fmt.Errorf("%w; retry failed: %v", err, retryErr))
			w.logger.Printf("medium media_id update retry failed: corp=%d medium_ids=%v source=%s err=%v retry_err=%v", delivery.Event.CorpID, delivery.Event.MediumIDs, delivery.Event.Source, err, retryErr)
			return
		}
		finishExecution(taskrunner.StatusFailed, err)
		if deadLettered {
			w.logger.Printf("medium media_id update moved to dead letter: corp=%d medium_ids=%v source=%s attempts=%d err=%v", delivery.Event.CorpID, delivery.Event.MediumIDs, delivery.Event.Source, delivery.Attempts+1, err)
			return
		}
		w.logger.Printf("medium media_id update requeued: corp=%d medium_ids=%v source=%s attempts=%d err=%v", delivery.Event.CorpID, delivery.Event.MediumIDs, delivery.Event.Source, delivery.Attempts+1, err)
		return
	}
	if err := w.queue.AckMediumMediaIDUpdate(ctx, delivery); err != nil {
		finishExecution(taskrunner.StatusFailed, fmt.Errorf("ack failed: %w", err))
		w.logger.Printf("medium media_id update ack failed: corp=%d medium_ids=%v source=%s err=%v", delivery.Event.CorpID, delivery.Event.MediumIDs, delivery.Event.Source, err)
		return
	}
	finishExecution(taskrunner.StatusSucceeded, nil)
}

func (w *MediumMediaIDUpdateWorker) Process(ctx context.Context, event MediumMediaIDUpdateEvent) error {
	if w.store == nil || w.client == nil {
		return fmt.Errorf("medium media_id update worker dependencies are not configured")
	}
	if event.CorpID <= 0 {
		return fmt.Errorf("missing corp id")
	}
	mediumIDs := uniqueQueueCorpIDs(event.MediumIDs)
	if len(mediumIDs) == 0 {
		return fmt.Errorf("missing medium ids")
	}
	credential, found, err := w.store.MediumCorpCredentialByID(ctx, event.CorpID)
	if err != nil {
		return err
	}
	if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.EmployeeSecret) == "" {
		return fmt.Errorf("corp credential missing")
	}
	now := time.Now()
	if w.now != nil {
		now = w.now()
	}
	cron := NewMediumMediaCron(w.store, w.client, w.fileStorageRoot, w.logger)
	cron.now = func() time.Time { return now }
	var firstErr error
	scanned := 0
	updated := 0
	skipped := 0
	for _, mediumID := range mediumIDs {
		item, found, err := w.store.MediumMediaForUpdateByID(ctx, mediumID)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("medium %d: %w", mediumID, err)
			}
			continue
		}
		if !found {
			skipped++
			continue
		}
		scanned++
		itemUpdated, err := cron.refreshItem(ctx, credential, item, now)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("medium %d: %w", mediumID, err)
			}
			continue
		}
		if itemUpdated {
			updated++
			continue
		}
		skipped++
	}
	w.logger.Printf("medium media_id update finished: corp=%d scanned=%d updated=%d skipped=%d", event.CorpID, scanned, updated, skipped)
	return firstErr
}
