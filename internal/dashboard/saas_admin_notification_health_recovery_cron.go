package dashboard

import (
	"context"
	"fmt"
	"log"
)

const SaaSAdminNotificationHealthRecoveryCronTaskName = "cron-saas-notification-health-recovery"

type SaaSAdminNotificationHealthRecoveryCron struct {
	handler      *SaaSAdminHandler
	windowHours  int
	staleMinutes int
	logger       *log.Logger
}

func NewSaaSAdminNotificationHealthRecoveryCron(handler *SaaSAdminHandler, windowHours, staleMinutes int, logger *log.Logger) *SaaSAdminNotificationHealthRecoveryCron {
	if windowHours <= 0 {
		windowHours = saasAdminNotificationHealthDefaultWindowHours
	}
	if windowHours > saasAdminNotificationHealthMaxWindowHours {
		windowHours = saasAdminNotificationHealthMaxWindowHours
	}
	if staleMinutes <= 0 {
		staleMinutes = saasAdminNotificationHealthDefaultStaleMinutes
	}
	if staleMinutes > saasAdminNotificationHealthMaxStaleMinutes {
		staleMinutes = saasAdminNotificationHealthMaxStaleMinutes
	}
	if logger == nil {
		logger = log.Default()
	}
	return &SaaSAdminNotificationHealthRecoveryCron{
		handler:      handler,
		windowHours:  windowHours,
		staleMinutes: staleMinutes,
		logger:       logger,
	}
}

func (c *SaaSAdminNotificationHealthRecoveryCron) RunOnce(ctx context.Context) error {
	if c == nil || c.handler == nil || c.handler.store == nil || c.handler.platformAdminTenantID <= 0 {
		return fmt.Errorf("SaaS notification health recovery cron dependencies are not configured")
	}
	result, err := c.handler.recoverNotificationHealthAssignments(ctx, SaaSAdminNotificationHealthRecovery{
		Options: SaaSAdminNotificationHealthOptions{
			Channel:      SaaSAlertNotificationChannelWebhook,
			State:        SaaSAdminNotificationHealthStateAll,
			WindowHours:  c.windowHours,
			StaleMinutes: c.staleMinutes,
			Limit:        saasAdminExportMaxLimit,
		},
		Remark:        "自动通知健康恢复结案",
		ActorUserID:   0,
		ActorTenantID: c.handler.platformAdminTenantID,
	})
	if err != nil {
		return err
	}
	c.logger.Printf(
		"SaaS notification health recovery cron finished: matched=%d active=%d recovered=%d closed=%d already_closed=%d unhealthy=%d no_data=%d no_delivery_evidence=%d missing_tenant=%d",
		result.MatchedAssignmentCount,
		result.ActiveAssignmentCount,
		result.RecoveredTenantCount,
		result.ClosedCount,
		result.AlreadyClosedCount,
		result.UnhealthyCount,
		result.NoDataCount,
		result.NoDeliveryEvidenceCount,
		result.MissingTenantCount,
	)
	return nil
}
