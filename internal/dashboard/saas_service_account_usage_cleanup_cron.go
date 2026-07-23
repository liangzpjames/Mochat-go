package dashboard

import (
	"context"
	"log"
)

const SaaSServiceAccountUsageCleanupCronTaskName = "cron-saas-service-account-usage-cleanup"

type SaaSServiceAccountUsageCleanupCron struct {
	cleaner SaaSServiceAccountUsageCleaner
	limit   int
	logger  *log.Logger
}

func NewSaaSServiceAccountUsageCleanupCron(cleaner SaaSServiceAccountUsageCleaner, limit int, logger *log.Logger) *SaaSServiceAccountUsageCleanupCron {
	if limit <= 0 || limit > 1000000 {
		limit = 10000
	}
	if logger == nil {
		logger = log.Default()
	}
	return &SaaSServiceAccountUsageCleanupCron{cleaner: cleaner, limit: limit, logger: logger}
}

func (c *SaaSServiceAccountUsageCleanupCron) RunOnce(ctx context.Context) error {
	result, err := c.cleaner.CleanupSaaSServiceAccountUsage(ctx, c.limit)
	if err != nil {
		return err
	}
	c.logger.Printf("SaaS service account usage cleanup completed: retention_days=%d cutoff_date=%s eligible=%d protected=%d deleted=%d remaining=%d",
		result.RetentionDays, result.CutoffDate, result.EligibleRows, result.ProtectedRows, result.DeletedRows, result.RemainingRows)
	return nil
}
