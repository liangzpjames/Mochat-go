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

type WorkDepartmentListEvent struct {
	CorpIDs  []int  `json:"corpIds,omitempty"`
	UserID   int    `json:"userId,omitempty"`
	TenantID int    `json:"tenantId,omitempty"`
	Source   string `json:"source,omitempty"`
}

func (e *WorkDepartmentListEvent) UnmarshalJSON(raw []byte) error {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if strings.HasPrefix(trimmed, "{") {
		return e.unmarshalWorkDepartmentListObject(raw)
	}
	return e.unmarshalWorkDepartmentListLegacyArray(raw)
}

func (e *WorkDepartmentListEvent) unmarshalWorkDepartmentListObject(raw []byte) error {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	if rawValue, ok := firstJSONField(payload, "corpIds", "corp_ids"); ok {
		corpIDs, err := decodeWorkDepartmentListCorpIDs(rawValue)
		if err != nil {
			return fmt.Errorf("corpIds: %w", err)
		}
		e.CorpIDs = corpIDs
	}
	if rawValue, ok := firstJSONField(payload, "userId", "user_id"); ok {
		userID, err := decodeMessageRemindInt(rawValue)
		if err != nil {
			return fmt.Errorf("userId: %w", err)
		}
		e.UserID = userID
	}
	if rawValue, ok := firstJSONField(payload, "tenantId", "tenant_id"); ok {
		tenantID, err := decodeMessageRemindInt(rawValue)
		if err != nil {
			return fmt.Errorf("tenantId: %w", err)
		}
		e.TenantID = tenantID
	}
	if err := decodeOptionalJSONField(payload, &e.Source, "source"); err != nil {
		return fmt.Errorf("source: %w", err)
	}
	return nil
}

func (e *WorkDepartmentListEvent) unmarshalWorkDepartmentListLegacyArray(raw []byte) error {
	var legacy []json.RawMessage
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return err
	}
	if len(legacy) == 0 {
		return nil
	}
	if workDepartmentListAllScalars(legacy) {
		corpIDs, err := decodeWorkDepartmentListCorpIDs(raw)
		if err != nil {
			return fmt.Errorf("corpIds: %w", err)
		}
		e.CorpIDs = corpIDs
		return nil
	}
	corpIDs, err := decodeWorkDepartmentListCorpIDs(legacy[0])
	if err != nil {
		return fmt.Errorf("corpIds: %w", err)
	}
	e.CorpIDs = corpIDs
	if len(legacy) > 1 && strings.TrimSpace(string(legacy[1])) != "null" {
		userID, err := decodeMessageRemindInt(legacy[1])
		if err != nil {
			return fmt.Errorf("userId: %w", err)
		}
		e.UserID = userID
	}
	if len(legacy) > 2 && strings.TrimSpace(string(legacy[2])) != "null" {
		if err := json.Unmarshal(legacy[2], &e.Source); err != nil {
			return fmt.Errorf("source: %w", err)
		}
	}
	return nil
}

func decodeWorkDepartmentListCorpIDs(raw json.RawMessage) ([]int, error) {
	value, err := decodeMessageRemindAny(raw)
	if err != nil {
		return nil, err
	}
	switch typed := value.(type) {
	case []any:
		out := make([]int, 0, len(typed))
		for _, item := range typed {
			corpID, err := decodeWorkDepartmentListIntValue(item)
			if err != nil {
				return nil, err
			}
			if corpID > 0 {
				out = append(out, corpID)
			}
		}
		return out, nil
	default:
		corpID, err := decodeWorkDepartmentListIntValue(typed)
		if err != nil {
			return nil, err
		}
		if corpID <= 0 {
			return nil, nil
		}
		return []int{corpID}, nil
	}
}

func decodeWorkDepartmentListIntValue(value any) (int, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	return decodeMessageRemindInt(raw)
}

func workDepartmentListAllScalars(values []json.RawMessage) bool {
	for _, value := range values {
		trimmed := strings.TrimSpace(string(value))
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			return false
		}
	}
	return true
}

type WorkDepartmentListDelivery struct {
	Event    WorkDepartmentListEvent
	Raw      string
	Attempts int
}

type WorkDepartmentListWorkerQueue interface {
	DequeueWorkDepartmentList(ctx context.Context, timeout time.Duration) (WorkDepartmentListDelivery, bool, error)
	AckWorkDepartmentList(ctx context.Context, delivery WorkDepartmentListDelivery) error
	RetryWorkDepartmentList(ctx context.Context, delivery WorkDepartmentListDelivery, reason string, maxAttempts int) (bool, error)
	RecoverWorkDepartmentListProcessing(ctx context.Context, staleAfter time.Duration, maxAttempts int) (int, error)
}

type WorkDepartmentListWorkerStore interface {
	workEmployeeSyncStore
	CorpIDsByUser(ctx context.Context, userID int) ([]int, error)
	CorpIDsByTenant(ctx context.Context, tenantID int) ([]int, error)
}

type WorkDepartmentListWorker struct {
	queue             WorkDepartmentListWorkerQueue
	store             WorkDepartmentListWorkerStore
	client            WorkEmployeeSyncClient
	passwordKey       string
	pollTimeout       time.Duration
	maxAttempts       int
	processingTimeout time.Duration
	recoveryInterval  time.Duration
	alertNotifier     SaaSAlertNotifier
	logger            *log.Logger
}

func NewWorkDepartmentListWorker(queue WorkDepartmentListWorkerQueue, store WorkDepartmentListWorkerStore, client WorkEmployeeSyncClient, passwordKey string, logger *log.Logger) *WorkDepartmentListWorker {
	if logger == nil {
		logger = log.Default()
	}
	return &WorkDepartmentListWorker{
		queue:             queue,
		store:             store,
		client:            client,
		passwordKey:       passwordKey,
		pollTimeout:       5 * time.Second,
		maxAttempts:       3,
		processingTimeout: 5 * time.Minute,
		recoveryInterval:  time.Minute,
		logger:            logger,
	}
}

func (w *WorkDepartmentListWorker) WithProcessingTimeout(timeout time.Duration) *WorkDepartmentListWorker {
	if timeout > 0 {
		w.processingTimeout = timeout
		w.recoveryInterval = timeout
		if w.recoveryInterval > time.Minute {
			w.recoveryInterval = time.Minute
		}
	}
	return w
}

func (w *WorkDepartmentListWorker) WithSaaSAlertNotifier(notifier SaaSAlertNotifier) *WorkDepartmentListWorker {
	w.alertNotifier = notifier
	return w
}

func (w *WorkDepartmentListWorker) Run(ctx context.Context) error {
	if w.queue == nil || w.store == nil || w.client == nil {
		return fmt.Errorf("work department list worker dependencies are not configured")
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
		delivery, ok, err := w.queue.DequeueWorkDepartmentList(ctx, w.pollTimeout)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			w.logger.Printf("work department list dequeue failed: %v", err)
			continue
		}
		if !ok {
			continue
		}
		w.handleDelivery(ctx, delivery)
	}
}

func (w *WorkDepartmentListWorker) recoverProcessing(ctx context.Context) {
	recovered, err := w.queue.RecoverWorkDepartmentListProcessing(ctx, w.processingTimeout, w.maxAttempts)
	if err != nil {
		w.logger.Printf("work department list processing recovery failed: %v", err)
		return
	}
	if recovered > 0 {
		w.logger.Printf("work department list recovered processing jobs: %d", recovered)
	}
}

func (w *WorkDepartmentListWorker) handleDelivery(ctx context.Context, delivery WorkDepartmentListDelivery) {
	ctx = WithSaaSAlertNotifier(ctx, w.alertNotifier)
	corpIDs, _ := w.corpIDs(ctx, delivery.Event)
	tenantID := tenantIDForQueueExecution(ctx, w.logger, w.store, firstPositiveInt(corpIDs))
	finishExecution := startQueueItemExecution(ctx, w.logger, QueueNameWorkDepartmentList, w.store, tenantID)
	if err := w.Process(ctx, delivery.Event); err != nil {
		deadLettered, retryErr := w.queue.RetryWorkDepartmentList(ctx, delivery, err.Error(), w.maxAttempts)
		if retryErr != nil {
			finishExecution(taskrunner.StatusFailed, fmt.Errorf("%w; retry failed: %v", err, retryErr))
			w.logger.Printf("work department list retry failed: corp_ids=%v user_id=%d tenant_id=%d source=%s err=%v retry_err=%v", delivery.Event.CorpIDs, delivery.Event.UserID, delivery.Event.TenantID, delivery.Event.Source, err, retryErr)
			return
		}
		finishExecution(taskrunner.StatusFailed, err)
		if deadLettered {
			w.logger.Printf("work department list moved to dead letter: corp_ids=%v user_id=%d tenant_id=%d source=%s attempts=%d err=%v", delivery.Event.CorpIDs, delivery.Event.UserID, delivery.Event.TenantID, delivery.Event.Source, delivery.Attempts+1, err)
			return
		}
		w.logger.Printf("work department list requeued: corp_ids=%v user_id=%d tenant_id=%d source=%s attempts=%d err=%v", delivery.Event.CorpIDs, delivery.Event.UserID, delivery.Event.TenantID, delivery.Event.Source, delivery.Attempts+1, err)
		return
	}
	if err := w.queue.AckWorkDepartmentList(ctx, delivery); err != nil {
		finishExecution(taskrunner.StatusFailed, fmt.Errorf("ack failed: %w", err))
		w.logger.Printf("work department list ack failed: corp_ids=%v user_id=%d tenant_id=%d source=%s err=%v", delivery.Event.CorpIDs, delivery.Event.UserID, delivery.Event.TenantID, delivery.Event.Source, err)
		return
	}
	finishExecution(taskrunner.StatusSucceeded, nil)
}

func (w *WorkDepartmentListWorker) Process(ctx context.Context, event WorkDepartmentListEvent) error {
	if w.store == nil || w.client == nil {
		return fmt.Errorf("work department list worker dependencies are not configured")
	}
	corpIDs, err := w.corpIDs(ctx, event)
	if err != nil {
		return err
	}
	if len(corpIDs) == 0 {
		return fmt.Errorf("missing corp ids")
	}
	for _, corpID := range corpIDs {
		if err := syncWorkEmployeesForCorp(ctx, w.store, w.client, w.passwordKey, corpID); err != nil {
			return fmt.Errorf("corp %d: %w", corpID, err)
		}
	}
	return nil
}

func (w *WorkDepartmentListWorker) corpIDs(ctx context.Context, event WorkDepartmentListEvent) ([]int, error) {
	if corpIDs := uniqueEmployeeApplyCorpIDs(event.CorpIDs); len(corpIDs) > 0 {
		return corpIDs, nil
	}
	if event.UserID > 0 {
		corpIDs, err := w.store.CorpIDsByUser(ctx, event.UserID)
		return uniqueEmployeeApplyCorpIDs(corpIDs), err
	}
	if event.TenantID > 0 {
		corpIDs, err := w.store.CorpIDsByTenant(ctx, event.TenantID)
		return uniqueEmployeeApplyCorpIDs(corpIDs), err
	}
	return nil, nil
}
