package dashboard

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"jiyi/mochat-go/internal/taskrunner"
)

type EmployeeApplyEvent struct {
	// BindingID is a TenantBindingID (the authoritative tenant_id binding
	// reference), never a client-selected corp_id.
	BindingID TenantBindingID `json:"bindingId"`
	Source    string          `json:"source,omitempty"`
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
	employeeApplyStore
}

type EmployeeApplyWorker struct {
	queue             EmployeeApplyWorkerQueue
	store             EmployeeApplyWorkerStore
	client            WorkEmployeeSyncClient
	pollTimeout       time.Duration
	maxAttempts       int
	processingTimeout time.Duration
	recoveryInterval  time.Duration
	alertNotifier     SaaSAlertNotifier
	logger            *log.Logger
}

func NewEmployeeApplyWorker(queue EmployeeApplyWorkerQueue, store EmployeeApplyWorkerStore, client WorkEmployeeSyncClient, logger *log.Logger) *EmployeeApplyWorker {
	if logger == nil {
		logger = log.Default()
	}
	return &EmployeeApplyWorker{
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
	tenantID, tenantErr := w.store.TenantIDByBindingID(ctx, int(delivery.Event.BindingID))
	if tenantErr != nil {
		tenantID = 0
	}
	finishExecution := startQueueItemExecution(ctx, w.logger, "employee-apply", w.store, tenantID)
	if err := w.Process(ctx, delivery.Event); err != nil {
		w.retryFailedDelivery(ctx, delivery, finishExecution, employeeSyncVersion(err))
		return
	}
	if err := w.queue.AckEmployeeApply(ctx, delivery); err != nil {
		finishExecution(taskrunner.StatusFailed, fmt.Errorf("QUEUE_ACK_FAILED"))
		w.logger.Printf("employee apply ack failed: binding_id=%d source=%s code=QUEUE_ACK_FAILED", delivery.Event.BindingID, delivery.Event.Source)
		return
	}
	finishExecution(taskrunner.StatusSucceeded, nil)
}

func employeeSyncVersion(err error) uint64 {
	var versioned *employeeSyncVersionError
	if errors.As(err, &versioned) {
		return versioned.Version
	}
	return 0
}

func (w *EmployeeApplyWorker) retryFailedDelivery(ctx context.Context, delivery EmployeeApplyDelivery, finishExecution func(string, error), credentialVersion uint64) {
	deadLettered, retryErr := w.queue.RetryEmployeeApply(ctx, delivery, "SYNC_FAILED", w.maxAttempts)
	if retryErr != nil {
		finishExecution(taskrunner.StatusFailed, fmt.Errorf("SYNC_FAILED"))
		w.logger.Printf("employee apply retry failed: binding_id=%d source=%s code=QUEUE_RETRY_FAILED", delivery.Event.BindingID, delivery.Event.Source)
		return
	}
	finishExecution(taskrunner.StatusFailed, fmt.Errorf("SYNC_FAILED"))
	if deadLettered {
		w.recordDeadLetterState(ctx, int(delivery.Event.BindingID), credentialVersion)
		w.logger.Printf("employee apply moved to dead letter: binding_id=%d source=%s attempts=%d code=SYNC_FAILED", delivery.Event.BindingID, delivery.Event.Source, delivery.Attempts+1)
		return
	}
	w.recordRequeuedState(ctx, int(delivery.Event.BindingID), credentialVersion)
	w.logger.Printf("employee apply requeued: binding_id=%d source=%s attempts=%d code=SYNC_FAILED", delivery.Event.BindingID, delivery.Event.Source, delivery.Attempts+1)
}

func (w *EmployeeApplyWorker) recordDeadLetterState(ctx context.Context, bindingID int, credentialVersion uint64) {
	if credentialVersion == 0 {
		return
	}
	if err := w.store.RecordCompanyEmployeeSyncFailureAtVersion(ctx, bindingID, credentialVersion); err != nil {
		w.logger.Printf("employee apply state reconcile failed: binding_id=%d code=SYNC_STATE_FAILED", bindingID)
	}
}

func (w *EmployeeApplyWorker) recordRequeuedState(ctx context.Context, bindingID int, credentialVersion uint64) {
	if credentialVersion == 0 {
		return
	}
	if err := w.store.MarkCompanyEmployeeSyncQueuedAtVersion(ctx, bindingID, credentialVersion, "SYNC_FAILED"); err != nil {
		w.logger.Printf("employee apply state reconcile failed: binding_id=%d code=SYNC_STATE_FAILED", bindingID)
	}
}

func (w *EmployeeApplyWorker) Process(ctx context.Context, event EmployeeApplyEvent) error {
	if w.store == nil || w.client == nil {
		return fmt.Errorf("employee apply worker dependencies are not configured")
	}
	if event.BindingID <= 0 {
		return fmt.Errorf("missing binding id")
	}
	if err := syncCompanyEmployeesForBinding(ctx, w.store, w.client, int(event.BindingID)); err != nil {
		return fmt.Errorf("binding %d: %w", event.BindingID, err)
	}
	return nil
}
