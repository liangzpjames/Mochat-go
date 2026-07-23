package dashboard

import (
	"context"
	"fmt"
	"log"
)

const SaaSPaymentDunningCronTaskName = "cron-saas-payment-dunning"

type SaaSPaymentDunningCron struct {
	store                   SaaSPaymentDunningStore
	limit                   int
	retryDelaySeconds       int
	notificationMaxAttempts int
	actorTenantID           int
	logger                  *log.Logger
}

func NewSaaSPaymentDunningCron(
	store SaaSPaymentDunningStore,
	limit int,
	retryDelaySeconds int,
	notificationMaxAttempts int,
	actorTenantID int,
	logger *log.Logger,
) *SaaSPaymentDunningCron {
	if limit <= 0 {
		limit = 100
	}
	if limit > saasAdminExportMaxLimit {
		limit = saasAdminExportMaxLimit
	}
	if retryDelaySeconds < 60 {
		retryDelaySeconds = 86400
	}
	if notificationMaxAttempts <= 0 {
		notificationMaxAttempts = 3
	}
	if logger == nil {
		logger = log.Default()
	}
	return &SaaSPaymentDunningCron{
		store: store, limit: limit, retryDelaySeconds: retryDelaySeconds,
		notificationMaxAttempts: notificationMaxAttempts, actorTenantID: actorTenantID, logger: logger,
	}
}

func (c *SaaSPaymentDunningCron) RunOnce(ctx context.Context) error {
	if c == nil || c.store == nil || c.actorTenantID <= 0 {
		return fmt.Errorf("SaaS payment dunning cron dependencies are not configured")
	}
	result, err := c.store.ProcessSaaSPaymentDunning(ctx, SaaSPaymentDunningOptions{
		Limit: c.limit, RetryDelaySeconds: c.retryDelaySeconds,
		NotificationMaxAttempts: c.notificationMaxAttempts, ActorTenantID: c.actorTenantID,
	})
	if err != nil {
		return err
	}
	c.logger.Printf(
		"SaaS payment dunning cron finished: matched=%d enqueued=%d exhausted=%d skipped=%d failed=%d",
		result.MatchedCount, result.EnqueuedCount, result.ExhaustedCount, result.SkippedCount, result.FailedCount,
	)
	if result.FailedCount > 0 {
		return fmt.Errorf("SaaS payment dunning failed for %d orders", result.FailedCount)
	}
	return nil
}
