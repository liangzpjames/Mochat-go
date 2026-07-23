package saasauditanchor

import (
	"context"
	"fmt"
	"log"
)

const CronTaskName = "cron-saas-admin-audit-anchor"

type Cron struct {
	manager *Manager
	limit   int
	logger  *log.Logger
}

func NewCron(manager *Manager, limit int, logger *log.Logger) *Cron {
	if limit <= 0 {
		limit = 100
	}
	if logger == nil {
		logger = log.Default()
	}
	return &Cron{manager: manager, limit: limit, logger: logger}
}

func (c *Cron) RunOnce(ctx context.Context) error {
	if c.manager == nil {
		return fmt.Errorf("SaaS audit anchor manager is not configured")
	}
	created, err := c.manager.Create(ctx, CreateOptions{Limit: c.limit, Source: TriggerCron})
	if err != nil {
		return err
	}
	verified, err := c.manager.Verify(ctx, VerifyOptions{Limit: c.limit, Source: TriggerCron})
	if err != nil {
		return err
	}
	c.logger.Printf("SaaS audit anchor completed: chains=%d created=%d existing=%d backfilled=%d exported=%d verify_passed=%d verify_failed=%d",
		created.ScannedChains, created.CreatedCheckpoints, created.ExistingCheckpoints, created.BackfilledCheckpoints, created.ExportedArtifacts,
		verified.PassedCheckpoints, verified.FailedCheckpoints)
	if verified.FailedCheckpoints > 0 {
		return fmt.Errorf("SaaS audit anchor verification detected %d failed checkpoints", verified.FailedCheckpoints)
	}
	return nil
}
