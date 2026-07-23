package dashboard

import (
	"context"
	"fmt"
	"log"
	"time"
)

type SaaSAlertNotificationDispatchCron struct {
	store      SaaSAlertNotificationStore
	notifier   SaaSAlertNotifier
	limit      int
	retryDelay time.Duration
	logger     *log.Logger
}

func NewSaaSAlertNotificationDispatchCron(store SaaSAlertNotificationStore, notifier SaaSAlertNotifier, limit int, retryDelay time.Duration, logger *log.Logger) *SaaSAlertNotificationDispatchCron {
	if logger == nil {
		logger = log.Default()
	}
	if limit <= 0 {
		limit = 100
	}
	if retryDelay < 0 {
		retryDelay = 0
	}
	return &SaaSAlertNotificationDispatchCron{
		store:      store,
		notifier:   notifier,
		limit:      limit,
		retryDelay: retryDelay,
		logger:     logger,
	}
}

func (c *SaaSAlertNotificationDispatchCron) RunOnce(ctx context.Context) error {
	if c.store == nil || c.notifier == nil {
		return fmt.Errorf("SaaS alert notification dispatch cron dependencies are not configured")
	}
	result, err := DispatchDueSaaSAlertNotifications(ctx, c.store, c.notifier, c.limit, c.retryDelay)
	if err != nil {
		return err
	}
	c.logger.Printf(
		"SaaS alert notification dispatch cron finished: scanned=%d delivered=%d deferred=%d suppressed=%d failed=%d dead=%d",
		result.Scanned,
		result.Delivered,
		result.Deferred,
		result.Suppressed,
		result.Failed,
		result.Dead,
	)
	return nil
}
