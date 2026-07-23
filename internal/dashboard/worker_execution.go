package dashboard

import (
	"context"
	"log"
	"time"

	"jiyi/mochat-go/internal/taskrunner"
)

type queueExecutionTenantResolver interface {
	TenantIDByCorpID(ctx context.Context, corpID int) (int, error)
}

func startQueueItemExecution(ctx context.Context, logger *log.Logger, fallbackTaskName string, store any, tenantID int) func(status string, err error) {
	startedAt := time.Now()
	taskName := taskrunner.RuntimeTaskName(ctx, fallbackTaskName)
	execution := taskrunner.ExecutionSnapshot{
		ExecutionID: taskrunner.NewExecutionID(taskName, taskrunner.ExecutionKindQueueItem, startedAt),
		TenantID:    tenantID,
		TaskName:    taskName,
		RunID:       taskrunner.RuntimeRunID(ctx),
		Kind:        taskrunner.ExecutionKindQueueItem,
		Status:      taskrunner.StatusRunning,
		StartedAt:   startedAt.Format(time.RFC3339),
	}
	taskrunner.RecordTaskExecution(ctx, logger, execution)
	return func(status string, err error) {
		execution.Status = status
		execution.StoppedAt = time.Now().Format(time.RFC3339)
		if err != nil {
			execution.Error = err.Error()
		} else {
			execution.Error = ""
		}
		taskrunner.RecordTaskExecution(ctx, logger, execution)
		refreshAsyncExecutionUsage(ctx, logger, store, tenantID)
	}
}

func refreshAsyncExecutionUsage(ctx context.Context, logger *log.Logger, store any, tenantID int) {
	if tenantID <= 0 || store == nil {
		return
	}
	quotaStore, ok := store.(SaaSQuotaStore)
	if !ok {
		return
	}
	if err := quotaStore.RefreshSaaSUsageCounter(ctx, tenantID, SaaSMetricAsyncExecutions); err != nil {
		if logger == nil {
			logger = log.Default()
		}
		logger.Printf("refresh async execution SaaS usage failed: tenant=%d err=%v", tenantID, err)
		return
	}
	status, err := quotaStore.SaaSQuotaStatus(ctx, tenantID, SaaSMetricAsyncExecutions, 0)
	if err != nil {
		if logger == nil {
			logger = log.Default()
		}
		logger.Printf("inspect async execution SaaS quota failed: tenant=%d err=%v", tenantID, err)
		return
	}
	if status.Exceeded() {
		if logger == nil {
			logger = log.Default()
		}
		logger.Printf("async execution SaaS quota exceeded: tenant=%d current=%d limit=%d", tenantID, status.Current, status.Limit)
		recordAsyncExecutionQuotaAlert(ctx, logger, store, status)
	}
}

func recordAsyncExecutionQuotaAlert(ctx context.Context, logger *log.Logger, store any, status SaaSQuotaStatus) {
	alertStore, ok := store.(SaaSAlertStore)
	if !ok {
		return
	}
	alert := SaaSQuotaAlert{
		Status:    status,
		AlertType: SaaSAlertTypeQuotaExceeded,
		Severity:  SaaSAlertSeverityWarning,
		PeriodKey: SaaSAlertPeriodLifetime,
		Source:    "worker.queue_item",
		Message:   saasQuotaExceededMessage(status),
		Context: map[string]any{
			"nonBlocking": true,
		},
	}
	if err := alertStore.RecordSaaSQuotaAlert(ctx, alert); err != nil {
		if logger == nil {
			logger = log.Default()
		}
		logger.Printf("record async execution SaaS quota alert failed: tenant=%d current=%d limit=%d err=%v", status.TenantID, status.Current, status.Limit, err)
		return
	}
	notifier, ok := saasAlertNotifierFromContext(ctx)
	if !ok {
		return
	}
	if err := notifier.NotifySaaSQuotaAlert(ctx, alert); err != nil {
		if logger == nil {
			logger = log.Default()
		}
		logger.Printf("notify async execution SaaS quota alert failed: tenant=%d current=%d limit=%d err=%v", status.TenantID, status.Current, status.Limit, err)
	}
}

func tenantIDForQueueExecution(ctx context.Context, logger *log.Logger, store any, corpID int) int {
	if corpID <= 0 || store == nil {
		return 0
	}
	resolver, ok := store.(queueExecutionTenantResolver)
	if !ok {
		return 0
	}
	tenantID, err := resolver.TenantIDByCorpID(ctx, corpID)
	if err != nil {
		if logger == nil {
			logger = log.Default()
		}
		logger.Printf("resolve queue execution tenant failed: corp=%d err=%v", corpID, err)
		return 0
	}
	return tenantID
}

func firstPositiveInt(values []int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
