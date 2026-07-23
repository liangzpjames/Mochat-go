package dashboard

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"jiyi/mochat-go/internal/taskrunner"
)

type EmployeeApplyEvent struct {
	CorpIDs []int  `json:"corpIds"`
	UserID  int    `json:"userId,omitempty"`
	Source  string `json:"source,omitempty"`
}

type EmployeeApplyDelivery struct {
	Event    EmployeeApplyEvent
	Raw      string
	Attempts int
}

type EmployeeApplyWorkerQueue interface {
	DequeueEmployeeApply(ctx context.Context, timeout time.Duration) (EmployeeApplyDelivery, bool, error)
	AckEmployeeApply(ctx context.Context, delivery EmployeeApplyDelivery) error
	RetryEmployeeApply(ctx context.Context, delivery EmployeeApplyDelivery, reason string, maxAttempts int) (bool, error)
	RecoverEmployeeApplyProcessing(ctx context.Context, staleAfter time.Duration, maxAttempts int) (int, error)
}

type EmployeeApplyWorkerStore interface {
	WorkEmployeeSyncCredentials(ctx context.Context, corpIDs []int) ([]WorkEmployeeSyncCredential, error)
	SyncWorkEmployees(ctx context.Context, credential WorkEmployeeSyncCredential, departments []WorkEmployeeSyncDepartment, employees []WorkEmployeeSyncEmployee, followUserIDs []string, defaultPasswordHash string) (WorkEmployeeSyncResult, error)
}

type EmployeeApplyWorker struct {
	queue             EmployeeApplyWorkerQueue
	store             EmployeeApplyWorkerStore
	client            WorkEmployeeSyncClient
	passwordKey       string
	pollTimeout       time.Duration
	maxAttempts       int
	processingTimeout time.Duration
	recoveryInterval  time.Duration
	alertNotifier     SaaSAlertNotifier
	logger            *log.Logger
}

func NewEmployeeApplyWorker(queue EmployeeApplyWorkerQueue, store EmployeeApplyWorkerStore, client WorkEmployeeSyncClient, passwordKey string, logger *log.Logger) *EmployeeApplyWorker {
	if logger == nil {
		logger = log.Default()
	}
	return &EmployeeApplyWorker{
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

func (w *EmployeeApplyWorker) WithProcessingTimeout(timeout time.Duration) *EmployeeApplyWorker {
	if timeout > 0 {
		w.processingTimeout = timeout
		w.recoveryInterval = timeout
		if w.recoveryInterval > time.Minute {
			w.recoveryInterval = time.Minute
		}
	}
	return w
}

func (w *EmployeeApplyWorker) WithSaaSAlertNotifier(notifier SaaSAlertNotifier) *EmployeeApplyWorker {
	w.alertNotifier = notifier
	return w
}

func (w *EmployeeApplyWorker) Run(ctx context.Context) error {
	if w.queue == nil || w.store == nil || w.client == nil {
		return fmt.Errorf("employee apply worker dependencies are not configured")
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
		delivery, ok, err := w.queue.DequeueEmployeeApply(ctx, w.pollTimeout)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			w.logger.Printf("employee apply dequeue failed: %v", err)
			continue
		}
		if !ok {
			continue
		}
		w.handleDelivery(ctx, delivery)
	}
}

func (w *EmployeeApplyWorker) recoverProcessing(ctx context.Context) {
	recovered, err := w.queue.RecoverEmployeeApplyProcessing(ctx, w.processingTimeout, w.maxAttempts)
	if err != nil {
		w.logger.Printf("employee apply processing recovery failed: %v", err)
		return
	}
	if recovered > 0 {
		w.logger.Printf("employee apply recovered processing jobs: %d", recovered)
	}
}

func (w *EmployeeApplyWorker) handleDelivery(ctx context.Context, delivery EmployeeApplyDelivery) {
	ctx = WithSaaSAlertNotifier(ctx, w.alertNotifier)
	tenantID := tenantIDForQueueExecution(ctx, w.logger, w.store, firstPositiveInt(delivery.Event.CorpIDs))
	finishExecution := startQueueItemExecution(ctx, w.logger, "employee-apply", w.store, tenantID)
	if err := w.Process(ctx, delivery.Event); err != nil {
		deadLettered, retryErr := w.queue.RetryEmployeeApply(ctx, delivery, err.Error(), w.maxAttempts)
		if retryErr != nil {
			finishExecution(taskrunner.StatusFailed, fmt.Errorf("%w; retry failed: %v", err, retryErr))
			w.logger.Printf("employee apply retry failed: corp_ids=%v source=%s err=%v retry_err=%v", delivery.Event.CorpIDs, delivery.Event.Source, err, retryErr)
			return
		}
		finishExecution(taskrunner.StatusFailed, err)
		if deadLettered {
			w.logger.Printf("employee apply moved to dead letter: corp_ids=%v source=%s attempts=%d err=%v", delivery.Event.CorpIDs, delivery.Event.Source, delivery.Attempts+1, err)
			return
		}
		w.logger.Printf("employee apply requeued: corp_ids=%v source=%s attempts=%d err=%v", delivery.Event.CorpIDs, delivery.Event.Source, delivery.Attempts+1, err)
		return
	}
	if err := w.queue.AckEmployeeApply(ctx, delivery); err != nil {
		finishExecution(taskrunner.StatusFailed, fmt.Errorf("ack failed: %w", err))
		w.logger.Printf("employee apply ack failed: corp_ids=%v source=%s err=%v", delivery.Event.CorpIDs, delivery.Event.Source, err)
		return
	}
	finishExecution(taskrunner.StatusSucceeded, nil)
}

func (w *EmployeeApplyWorker) Process(ctx context.Context, event EmployeeApplyEvent) error {
	if w.store == nil || w.client == nil {
		return fmt.Errorf("employee apply worker dependencies are not configured")
	}
	corpIDs := uniqueEmployeeApplyCorpIDs(event.CorpIDs)
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

func uniqueEmployeeApplyCorpIDs(values []int) []int {
	seen := map[int]struct{}{}
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}
