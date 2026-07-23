package dashboard

import (
	"context"
	"fmt"
	"log"
)

const SaaSAdminOperationQueueAssignmentReminderCronTaskName = "cron-saas-operation-queue-assignment-reminder"

type SaaSAdminOperationQueueAssignmentReminderCron struct {
	handler     *SaaSAdminHandler
	limit       int
	maxAttempts int
	logger      *log.Logger
}

type SaaSAdminOperationQueueAssignmentReminderCronResult struct {
	DueStateCount        int
	MatchedCount         int
	EligibleCount        int
	EnqueuedCount        int
	SkippedExistingCount int
	SkippedInvalidCount  int
	SkippedStatusCount   int
}

func NewSaaSAdminOperationQueueAssignmentReminderCron(handler *SaaSAdminHandler, limit int, maxAttempts int, logger *log.Logger) *SaaSAdminOperationQueueAssignmentReminderCron {
	if limit <= 0 {
		limit = 500
	}
	if limit > saasAdminListMaxLimit {
		limit = saasAdminListMaxLimit
	}
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	if maxAttempts > 20 {
		maxAttempts = 20
	}
	if logger == nil {
		logger = log.Default()
	}
	return &SaaSAdminOperationQueueAssignmentReminderCron{
		handler:     handler,
		limit:       limit,
		maxAttempts: maxAttempts,
		logger:      logger,
	}
}

func (c *SaaSAdminOperationQueueAssignmentReminderCron) RunOnce(ctx context.Context) error {
	if c == nil || c.handler == nil || c.handler.store == nil {
		return fmt.Errorf("SaaS operation queue assignment reminder cron dependencies are not configured")
	}
	result := SaaSAdminOperationQueueAssignmentReminderCronResult{}
	for _, dueState := range []string{
		SaaSAdminRiskFollowUpDueStateOverdue,
		SaaSAdminRiskFollowUpDueStateDueSoon,
	} {
		if err := ctx.Err(); err != nil {
			return err
		}
		notificationResult, err := c.handler.createOperationQueueAssignmentNotifications(ctx, SaaSAdminOperationQueueAssignmentNotifications{
			Options: SaaSAdminOperationQueueAssignmentOptions{
				DueState:    dueState,
				Limit:       c.limit,
				CurrentOnly: true,
			},
			Channel:       SaaSAlertNotificationChannelWebhook,
			MaxAttempts:   c.maxAttempts,
			Remark:        "自动运营待办认领到期提醒",
			ActorUserID:   0,
			ActorTenantID: c.handler.platformAdminTenantID,
		})
		if err != nil {
			return err
		}
		result.DueStateCount++
		result.MatchedCount += notificationResult.MatchedCount
		result.EligibleCount += notificationResult.EligibleCount
		result.EnqueuedCount += notificationResult.EnqueuedCount
		result.SkippedExistingCount += notificationResult.SkippedExistingCount
		result.SkippedInvalidCount += notificationResult.SkippedInvalidCount
		result.SkippedStatusCount += notificationResult.SkippedStatusCount
	}
	c.logger.Printf(
		"SaaS operation queue assignment reminder cron finished: due_states=%d matched=%d eligible=%d enqueued=%d skipped_existing=%d skipped_invalid=%d skipped_status=%d",
		result.DueStateCount,
		result.MatchedCount,
		result.EligibleCount,
		result.EnqueuedCount,
		result.SkippedExistingCount,
		result.SkippedInvalidCount,
		result.SkippedStatusCount,
	)
	return nil
}
