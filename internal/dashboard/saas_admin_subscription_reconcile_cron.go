package dashboard

import (
	"context"
	"fmt"
	"log"
)

const SaaSAdminSubscriptionReconcileCronTaskName = "cron-saas-subscription-reconcile"

type SaaSAdminSubscriptionReconcileCron struct {
	handler *SaaSAdminHandler
	limit   int
	logger  *log.Logger
}

func NewSaaSAdminSubscriptionReconcileCron(handler *SaaSAdminHandler, limit int, logger *log.Logger) *SaaSAdminSubscriptionReconcileCron {
	if limit <= 0 {
		limit = 500
	}
	if limit > saasAdminExportMaxLimit {
		limit = saasAdminExportMaxLimit
	}
	if logger == nil {
		logger = log.Default()
	}
	return &SaaSAdminSubscriptionReconcileCron{handler: handler, limit: limit, logger: logger}
}

func (c *SaaSAdminSubscriptionReconcileCron) RunOnce(ctx context.Context) error {
	if c == nil || c.handler == nil || c.handler.store == nil || c.handler.platformAdminTenantID <= 0 {
		return fmt.Errorf("SaaS subscription reconcile cron dependencies are not configured")
	}
	result, err := c.handler.store.ReconcileSaaSAdminSubscriptions(ctx, SaaSAdminSubscriptionReconcile{
		Limit:            c.limit,
		ExcludedTenantID: c.handler.platformAdminTenantID,
		ActorTenantID:    c.handler.platformAdminTenantID,
	})
	if err != nil {
		return err
	}
	c.logger.Printf(
		"SaaS subscription reconcile cron finished: scanned=%d due=%d changed=%d skipped=%d failed=%d",
		result.ScannedCount,
		result.ReconciliationDue,
		result.ChangedCount,
		result.SkippedCount,
		result.FailedCount,
	)
	if result.FailedCount > 0 {
		return fmt.Errorf("SaaS subscription reconcile failed for %d tenants", result.FailedCount)
	}
	return nil
}
