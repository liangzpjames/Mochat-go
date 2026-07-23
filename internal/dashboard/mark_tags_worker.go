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

type MarkTagsEvent struct {
	CorpID          int    `json:"corpId"`
	ContactID       int    `json:"contactId"`
	EmployeeID      int    `json:"employeeId"`
	TagIDs          []int  `json:"tagIds"`
	Source          string `json:"source,omitempty"`
	AutoTagID       int    `json:"autoTagId,omitempty"`
	AutoTagRecordID int    `json:"autoTagRecordId,omitempty"`
}

func (e *MarkTagsEvent) UnmarshalJSON(raw []byte) error {
	type markTagsEventAlias MarkTagsEvent
	var structured markTagsEventAlias
	if strings.HasPrefix(strings.TrimSpace(string(raw)), "{") {
		if err := json.Unmarshal(raw, &structured); err != nil {
			return err
		}
		*e = MarkTagsEvent(structured)
		return nil
	}

	var legacy []json.RawMessage
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return err
	}
	if len(legacy) < 4 {
		return fmt.Errorf("mark tags legacy payload requires 4 fields")
	}
	if err := json.Unmarshal(legacy[0], &e.CorpID); err != nil {
		return fmt.Errorf("corpId: %w", err)
	}
	if err := json.Unmarshal(legacy[1], &e.ContactID); err != nil {
		return fmt.Errorf("contactId: %w", err)
	}
	if err := json.Unmarshal(legacy[2], &e.EmployeeID); err != nil {
		return fmt.Errorf("employeeId: %w", err)
	}
	if err := json.Unmarshal(legacy[3], &e.TagIDs); err != nil {
		return fmt.Errorf("tagIds: %w", err)
	}
	return nil
}

type MarkTagsDelivery struct {
	Event    MarkTagsEvent
	Raw      string
	Attempts int
}

type MarkTagsApplyValues struct {
	CorpID     int
	ContactID  int
	EmployeeID int
	TagIDs     []int
}

type MarkTagsApplyResult struct {
	WXUserID         string
	WXExternalUserID string
	AddedWXTagIDs    []string
	AddedTagNames    []string
}

type MarkTagsWorkerQueue interface {
	DequeueMarkTags(ctx context.Context, timeout time.Duration) (MarkTagsDelivery, bool, error)
	AckMarkTags(ctx context.Context, delivery MarkTagsDelivery) error
	RetryMarkTags(ctx context.Context, delivery MarkTagsDelivery, reason string, maxAttempts int) (bool, error)
	RecoverMarkTagsProcessing(ctx context.Context, staleAfter time.Duration, maxAttempts int) (int, error)
}

type MarkTagsWorkerStore interface {
	RoomWelcomeCorpCredentialByID(ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error)
	ApplyWorkContactTags(ctx context.Context, values MarkTagsApplyValues) (MarkTagsApplyResult, bool, error)
	MarkAutoTagRecordApplied(ctx context.Context, recordID int, autoTagID int) error
}

type MarkTagsWorkerClient interface {
	MarkExternalContactTags(ctx context.Context, credential RoomWelcomeCorpCredential, payload WorkContactMarkTagsPayload) error
}

type MarkTagsWorker struct {
	queue             MarkTagsWorkerQueue
	store             MarkTagsWorkerStore
	client            MarkTagsWorkerClient
	pollTimeout       time.Duration
	maxAttempts       int
	processingTimeout time.Duration
	recoveryInterval  time.Duration
	alertNotifier     SaaSAlertNotifier
	logger            *log.Logger
}

func NewMarkTagsWorker(queue MarkTagsWorkerQueue, store MarkTagsWorkerStore, client MarkTagsWorkerClient, logger *log.Logger) *MarkTagsWorker {
	if logger == nil {
		logger = log.Default()
	}
	if client == nil {
		client = NewRoomWelcomeWeComClient(defaultWeComAPIBaseURL)
	}
	return &MarkTagsWorker{
		queue:             queue,
		store:             store,
		client:            client,
		pollTimeout:       5 * time.Second,
		maxAttempts:       3,
		processingTimeout: 5 * time.Minute,
		recoveryInterval:  time.Minute,
		logger:            logger,
	}
}

func (w *MarkTagsWorker) WithProcessingTimeout(timeout time.Duration) *MarkTagsWorker {
	if timeout > 0 {
		w.processingTimeout = timeout
		w.recoveryInterval = timeout
		if w.recoveryInterval > time.Minute {
			w.recoveryInterval = time.Minute
		}
	}
	return w
}

func (w *MarkTagsWorker) WithSaaSAlertNotifier(notifier SaaSAlertNotifier) *MarkTagsWorker {
	w.alertNotifier = notifier
	return w
}

func (w *MarkTagsWorker) Run(ctx context.Context) error {
	if w.queue == nil || w.store == nil || w.client == nil {
		return fmt.Errorf("mark tags worker dependencies are not configured")
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
		delivery, ok, err := w.queue.DequeueMarkTags(ctx, w.pollTimeout)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			w.logger.Printf("mark tags dequeue failed: %v", err)
			continue
		}
		if !ok {
			continue
		}
		w.handleDelivery(ctx, delivery)
	}
}

func (w *MarkTagsWorker) recoverProcessing(ctx context.Context) {
	recovered, err := w.queue.RecoverMarkTagsProcessing(ctx, w.processingTimeout, w.maxAttempts)
	if err != nil {
		w.logger.Printf("mark tags processing recovery failed: %v", err)
		return
	}
	if recovered > 0 {
		w.logger.Printf("mark tags recovered processing jobs: %d", recovered)
	}
}

func (w *MarkTagsWorker) handleDelivery(ctx context.Context, delivery MarkTagsDelivery) {
	ctx = WithSaaSAlertNotifier(ctx, w.alertNotifier)
	tenantID := tenantIDForQueueExecution(ctx, w.logger, w.store, delivery.Event.CorpID)
	finishExecution := startQueueItemExecution(ctx, w.logger, QueueNameMarkTags, w.store, tenantID)
	if err := w.Process(ctx, delivery.Event); err != nil {
		deadLettered, retryErr := w.queue.RetryMarkTags(ctx, delivery, err.Error(), w.maxAttempts)
		if retryErr != nil {
			finishExecution(taskrunner.StatusFailed, fmt.Errorf("%w; retry failed: %v", err, retryErr))
			w.logger.Printf("mark tags retry failed: corp_id=%d contact_id=%d employee_id=%d source=%s err=%v retry_err=%v", delivery.Event.CorpID, delivery.Event.ContactID, delivery.Event.EmployeeID, delivery.Event.Source, err, retryErr)
			return
		}
		finishExecution(taskrunner.StatusFailed, err)
		if deadLettered {
			w.logger.Printf("mark tags moved to dead letter: corp_id=%d contact_id=%d employee_id=%d source=%s attempts=%d err=%v", delivery.Event.CorpID, delivery.Event.ContactID, delivery.Event.EmployeeID, delivery.Event.Source, delivery.Attempts+1, err)
			return
		}
		w.logger.Printf("mark tags requeued: corp_id=%d contact_id=%d employee_id=%d source=%s attempts=%d err=%v", delivery.Event.CorpID, delivery.Event.ContactID, delivery.Event.EmployeeID, delivery.Event.Source, delivery.Attempts+1, err)
		return
	}
	if err := w.queue.AckMarkTags(ctx, delivery); err != nil {
		finishExecution(taskrunner.StatusFailed, fmt.Errorf("ack failed: %w", err))
		w.logger.Printf("mark tags ack failed: corp_id=%d contact_id=%d employee_id=%d source=%s err=%v", delivery.Event.CorpID, delivery.Event.ContactID, delivery.Event.EmployeeID, delivery.Event.Source, err)
		return
	}
	finishExecution(taskrunner.StatusSucceeded, nil)
}

func (w *MarkTagsWorker) Process(ctx context.Context, event MarkTagsEvent) error {
	if event.CorpID <= 0 {
		return fmt.Errorf("missing corp id")
	}
	if event.ContactID <= 0 {
		return fmt.Errorf("missing contact id")
	}
	if event.EmployeeID <= 0 {
		return fmt.Errorf("missing employee id")
	}
	tagIDs := uniquePositiveIntList(event.TagIDs)
	if len(tagIDs) == 0 {
		return nil
	}
	credential, found, err := w.store.RoomWelcomeCorpCredentialByID(ctx, event.CorpID)
	if err != nil {
		return err
	}
	if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.ContactSecret) == "" {
		return fmt.Errorf("corp %d contact credential is not configured", event.CorpID)
	}
	result, found, err := w.store.ApplyWorkContactTags(ctx, MarkTagsApplyValues{
		CorpID:     event.CorpID,
		ContactID:  event.ContactID,
		EmployeeID: event.EmployeeID,
		TagIDs:     tagIDs,
	})
	if err != nil {
		return err
	}
	if !found || len(result.AddedWXTagIDs) == 0 {
		if found && event.AutoTagRecordID > 0 {
			if err := w.store.MarkAutoTagRecordApplied(ctx, event.AutoTagRecordID, event.AutoTagID); err != nil {
				return err
			}
		}
		return nil
	}
	if event.AutoTagRecordID > 0 {
		if err := w.store.MarkAutoTagRecordApplied(ctx, event.AutoTagRecordID, event.AutoTagID); err != nil {
			return err
		}
	}
	if strings.TrimSpace(result.WXUserID) == "" || strings.TrimSpace(result.WXExternalUserID) == "" {
		return nil
	}
	if err := w.client.MarkExternalContactTags(ctx, credential, WorkContactMarkTagsPayload{
		UserID:         result.WXUserID,
		ExternalUserID: result.WXExternalUserID,
		AddTag:         result.AddedWXTagIDs,
	}); err != nil {
		w.logger.Printf("mark tags WeCom sync failed after local apply: corp_id=%d contact_id=%d employee_id=%d err=%v", event.CorpID, event.ContactID, event.EmployeeID, err)
	}
	return nil
}
