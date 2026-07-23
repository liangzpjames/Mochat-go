package dashboard

import (
	"context"
	"fmt"
	"log"
)

const SaaSAdminSystemHealthCronTaskName = "cron-saas-admin-system-health"

type SaaSAdminSystemHealthCron struct {
	handler                  *SaaSAdminHandler
	failureWindowHours       int
	notificationStaleMinutes int
	maxAttempts              int
	logger                   *log.Logger
}

func NewSaaSAdminSystemHealthCron(handler *SaaSAdminHandler, failureWindowHours, notificationStaleMinutes, maxAttempts int, logger *log.Logger) *SaaSAdminSystemHealthCron {
	if failureWindowHours <= 0 || failureWindowHours > 720 {
		failureWindowHours = 24
	}
	if notificationStaleMinutes <= 0 || notificationStaleMinutes > 10080 {
		notificationStaleMinutes = 15
	}
	if maxAttempts <= 0 || maxAttempts > 20 {
		maxAttempts = 3
	}
	if logger == nil {
		logger = log.Default()
	}
	return &SaaSAdminSystemHealthCron{
		handler: handler, failureWindowHours: failureWindowHours, notificationStaleMinutes: notificationStaleMinutes,
		maxAttempts: maxAttempts, logger: logger,
	}
}

func (c *SaaSAdminSystemHealthCron) RunOnce(ctx context.Context) error {
	if c == nil || c.handler == nil || c.handler.store == nil || c.handler.platformAdminTenantID <= 0 {
		return fmt.Errorf("SaaS system health cron dependencies are not configured")
	}
	result, err := c.handler.RunSaaSAdminSystemHealthScan(ctx, SaaSAdminSystemHealthScanInput{
		TriggerType: SaaSAdminSystemHealthTriggerCron,
		Options: SaaSAdminSystemHealthOptions{
			FailureWindowHours: c.failureWindowHours, NotificationStaleMins: c.notificationStaleMinutes,
		},
		Notify: true, MaxAttempts: c.maxAttempts, ActorUserID: 0,
		ActorTenantID: c.handler.platformAdminTenantID, PlatformTenantID: c.handler.platformAdminTenantID,
	})
	if err != nil {
		return err
	}
	c.logger.Printf("SaaS system health cron finished: state=%s checks=%d issues=%d opened=%d reopened=%d recovered=%d notifications=%d",
		result.Scan.HealthState, result.Scan.CheckCount, result.Scan.IssueCount, result.OpenedCount, result.ReopenedCount,
		result.RecoveredCount, len(result.NotificationKeys))
	return nil
}
