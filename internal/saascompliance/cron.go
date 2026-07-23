package saascompliance

import (
	"context"
	"log"
)

const CronTaskName = "cron-saas-compliance"

type Cron struct {
	manager *Manager
	actor   Actor
	limit   int
	logger  *log.Logger
}

func NewCron(manager *Manager, actor Actor, limit int, logger *log.Logger) *Cron {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	if logger == nil {
		logger = log.Default()
	}
	return &Cron{manager: manager, actor: actor, limit: limit, logger: logger}
}

func (c *Cron) RunOnce(ctx context.Context) error {
	result, err := c.manager.ProcessPending(ctx, c.limit, c.actor)
	if err != nil {
		return err
	}
	c.logger.Printf("SaaS compliance cron finished: exports=%d export_deletions=%d erasures=%d artifacts_deleted=%d",
		result.ExportsProcessed, result.ExportDeletionsProcessed, result.ErasuresProcessed, result.ArtifactsDeleted)
	return nil
}
