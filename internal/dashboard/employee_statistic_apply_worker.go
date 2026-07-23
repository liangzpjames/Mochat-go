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

type EmployeeStatisticApplyEvent struct {
	CorpID   int    `json:"corpId,omitempty"`
	TenantID int    `json:"tenantId,omitempty"`
	Source   string `json:"source,omitempty"`
}

func (e *EmployeeStatisticApplyEvent) UnmarshalJSON(raw []byte) error {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" || trimmed == "[]" {
		return nil
	}
	if strings.HasPrefix(trimmed, "{") {
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
	var legacy []json.RawMessage
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return err
	}
	if len(legacy) > 0 && strings.TrimSpace(string(legacy[0])) != "null" {
		if err := json.Unmarshal(legacy[0], &e.Source); err != nil {
			return fmt.Errorf("source: %w", err)
		}
	}
	return nil
}

type EmployeeStatisticApplyDelivery struct {
	Event    EmployeeStatisticApplyEvent
	Raw      string
	Attempts int
}

type EmployeeStatisticApplyWorkerQueue interface {
	DequeueEmployeeStatisticApply(ctx context.Context, timeout time.Duration) (EmployeeStatisticApplyDelivery, bool, error)
	AckEmployeeStatisticApply(ctx context.Context, delivery EmployeeStatisticApplyDelivery) error
	RetryEmployeeStatisticApply(ctx context.Context, delivery EmployeeStatisticApplyDelivery, reason string, maxAttempts int) (bool, error)
	RecoverEmployeeStatisticApplyProcessing(ctx context.Context, staleAfter time.Duration, maxAttempts int) (int, error)
}

type EmployeeStatisticApplyWorker struct {
	queue             EmployeeStatisticApplyWorkerQueue
	cron              *EmployeeStatisticCron
	store             EmployeeStatisticStore
	pollTimeout       time.Duration
	maxAttempts       int
	processingTimeout time.Duration
	recoveryInterval  time.Duration
	alertNotifier     SaaSAlertNotifier
	logger            *log.Logger
}

func NewEmployeeStatisticApplyWorker(queue EmployeeStatisticApplyWorkerQueue, store EmployeeStatisticStore, cache EmployeeStatisticCache, client EmployeeStatisticClient, logger *log.Logger) *EmployeeStatisticApplyWorker {
	if logger == nil {
		logger = log.Default()
	}
	return &EmployeeStatisticApplyWorker{
		queue:             queue,
		cron:              NewEmployeeStatisticCron(store, cache, client, logger),
		store:             store,
		pollTimeout:       5 * time.Second,
		maxAttempts:       3,
		processingTimeout: 5 * time.Minute,
		recoveryInterval:  time.Minute,
		logger:            logger,
	}
}

func (w *EmployeeStatisticApplyWorker) WithProcessingTimeout(timeout time.Duration) *EmployeeStatisticApplyWorker {
	if timeout > 0 {
		w.processingTimeout = timeout
		w.recoveryInterval = timeout
		if w.recoveryInterval > time.Minute {
			w.recoveryInterval = time.Minute
		}
	}
	return w
}

func (w *EmployeeStatisticApplyWorker) WithSaaSAlertNotifier(notifier SaaSAlertNotifier) *EmployeeStatisticApplyWorker {
	w.alertNotifier = notifier
	return w
}

func (w *EmployeeStatisticApplyWorker) Run(ctx context.Context) error {
	if w.queue == nil || w.cron == nil {
		return fmt.Errorf("employee statistic apply worker dependencies are not configured")
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
		delivery, ok, err := w.queue.DequeueEmployeeStatisticApply(ctx, w.pollTimeout)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			w.logger.Printf("employee statistic apply dequeue failed: %v", err)
			continue
		}
		if !ok {
			continue
		}
		w.handleDelivery(ctx, delivery)
	}
}

func (w *EmployeeStatisticApplyWorker) recoverProcessing(ctx context.Context) {
	recovered, err := w.queue.RecoverEmployeeStatisticApplyProcessing(ctx, w.processingTimeout, w.maxAttempts)
	if err != nil {
		w.logger.Printf("employee statistic apply processing recovery failed: %v", err)
		return
	}
	if recovered > 0 {
		w.logger.Printf("employee statistic apply recovered processing jobs: %d", recovered)
	}
}

func (w *EmployeeStatisticApplyWorker) handleDelivery(ctx context.Context, delivery EmployeeStatisticApplyDelivery) {
	ctx = WithSaaSAlertNotifier(ctx, w.alertNotifier)
	tenantID := delivery.Event.TenantID
	if tenantID <= 0 && delivery.Event.CorpID > 0 {
		tenantID = tenantIDForQueueExecution(ctx, w.logger, w.store, delivery.Event.CorpID)
	}
	finishExecution := startQueueItemExecution(ctx, w.logger, QueueNameEmployeeStatisticApply, w.store, tenantID)
	if err := w.Process(ctx, delivery.Event); err != nil {
		deadLettered, retryErr := w.queue.RetryEmployeeStatisticApply(ctx, delivery, err.Error(), w.maxAttempts)
		if retryErr != nil {
			finishExecution(taskrunner.StatusFailed, fmt.Errorf("%w; retry failed: %v", err, retryErr))
			w.logger.Printf("employee statistic apply retry failed: source=%s err=%v retry_err=%v", delivery.Event.Source, err, retryErr)
			return
		}
		finishExecution(taskrunner.StatusFailed, err)
		if deadLettered {
			w.logger.Printf("employee statistic apply moved to dead letter: source=%s attempts=%d err=%v", delivery.Event.Source, delivery.Attempts+1, err)
			return
		}
		w.logger.Printf("employee statistic apply requeued: source=%s attempts=%d err=%v", delivery.Event.Source, delivery.Attempts+1, err)
		return
	}
	if err := w.queue.AckEmployeeStatisticApply(ctx, delivery); err != nil {
		finishExecution(taskrunner.StatusFailed, fmt.Errorf("ack failed: %w", err))
		w.logger.Printf("employee statistic apply ack failed: source=%s err=%v", delivery.Event.Source, err)
		return
	}
	finishExecution(taskrunner.StatusSucceeded, nil)
}

func (w *EmployeeStatisticApplyWorker) Process(ctx context.Context, event EmployeeStatisticApplyEvent) error {
	if w.cron == nil {
		return fmt.Errorf("employee statistic apply worker dependencies are not configured")
	}
	corpIDs, scoped, err := w.scopedCorpIDs(ctx, event)
	if err != nil {
		return err
	}
	if !scoped {
		return w.cron.RunOnce(ctx)
	}
	return w.cron.RunOnceForCorpIDs(ctx, corpIDs)
}

func (w *EmployeeStatisticApplyWorker) scopedCorpIDs(ctx context.Context, event EmployeeStatisticApplyEvent) ([]int, bool, error) {
	if event.CorpID > 0 {
		return []int{event.CorpID}, true, nil
	}
	if event.TenantID <= 0 {
		return nil, false, nil
	}
	resolver, ok := w.store.(interface {
		CorpIDsByTenant(ctx context.Context, tenantID int) ([]int, error)
	})
	if !ok {
		return nil, true, fmt.Errorf("employee statistic apply tenant scope is not configured")
	}
	corpIDs, err := resolver.CorpIDsByTenant(ctx, event.TenantID)
	return uniqueEmployeeApplyCorpIDs(corpIDs), true, err
}
