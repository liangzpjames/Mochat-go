package dashboard

import (
	"context"
	"log"
)

const SaaSServiceAccountUsageAlertCronTaskName = "cron-saas-service-account-usage-alert"

type SaaSServiceAccountUsageAlertCron struct {
	evaluator SaaSServiceAccountUsageAlertEvaluator
	limit     int
	logger    *log.Logger
}

func NewSaaSServiceAccountUsageAlertCron(evaluator SaaSServiceAccountUsageAlertEvaluator, limit int, logger *log.Logger) *SaaSServiceAccountUsageAlertCron {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if logger == nil {
		logger = log.Default()
	}
	return &SaaSServiceAccountUsageAlertCron{evaluator: evaluator, limit: limit, logger: logger}
}

func (c *SaaSServiceAccountUsageAlertCron) RunOnce(ctx context.Context) error {
	result, err := c.evaluator.EvaluateSaaSServiceAccountUsageAlerts(ctx, SaaSServiceAccountUsageAlertEvaluateOptions{Limit: c.limit, Source: "cron"})
	if err != nil {
		return err
	}
	c.logger.Printf("SaaS service account usage alert evaluation completed: scanned=%d eligible=%d disabled=%d usage_warnings=%d rejection_warnings=%d resolved=%d notifications=%d notifications_closed=%d",
		result.ScannedAccounts, result.EligibleAccounts, result.DisabledPolicies, result.UsageWarningAccounts,
		result.RejectionWarningAccounts, result.ResolvedAlerts, result.NotificationsQueued, result.NotificationsClosed)
	return nil
}
