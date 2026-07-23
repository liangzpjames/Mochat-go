package dashboard

import (
	"context"
	"fmt"
	"log"
)

const SaaSAdminApprovalReminderCronTaskName = "cron-saas-admin-approval-reminder"

type SaaSAdminApprovalReminderCron struct {
	handler     *SaaSAdminHandler
	limit       int
	maxAttempts int
	logger      *log.Logger
}

func NewSaaSAdminApprovalReminderCron(handler *SaaSAdminHandler, limit, maxAttempts int, logger *log.Logger) *SaaSAdminApprovalReminderCron {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if maxAttempts <= 0 || maxAttempts > 20 {
		maxAttempts = 3
	}
	if logger == nil {
		logger = log.Default()
	}
	return &SaaSAdminApprovalReminderCron{handler: handler, limit: limit, maxAttempts: maxAttempts, logger: logger}
}

func (c *SaaSAdminApprovalReminderCron) RunOnce(ctx context.Context) error {
	if c == nil || c.handler == nil || c.handler.store == nil || c.handler.platformAdminTenantID <= 0 {
		return fmt.Errorf("SaaS approval reminder cron dependencies are not configured")
	}
	store, ok := c.handler.store.(SaaSAdminApprovalGovernanceStore)
	if !ok || store == nil {
		return fmt.Errorf("SaaS approval governance store is not configured")
	}
	result, err := store.CreateSaaSAdminApprovalReminders(ctx, SaaSAdminApprovalReminderCreate{
		Limit: c.limit, MaxAttempts: c.maxAttempts, ActorUserID: 0,
		ActorTenantID: c.handler.platformAdminTenantID, PlatformTenantID: c.handler.platformAdminTenantID,
	})
	if err != nil {
		return err
	}
	c.logger.Printf("SaaS approval reminder cron finished: scanned=%d enqueued=%d skipped=%d", result.Scanned, result.Enqueued, result.Skipped)
	return nil
}
