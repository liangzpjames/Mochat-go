package identitysecurity

import (
	"context"
	"log"
)

const CronTaskName = "cron-saas-identity-cleanup"

type Cron struct {
	manager *Manager
	limit   int
	logger  *log.Logger
}

func NewCron(manager *Manager, limit int, logger *log.Logger) *Cron {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	if logger == nil {
		logger = log.Default()
	}
	return &Cron{manager: manager, limit: limit, logger: logger}
}

func (c *Cron) RunOnce(ctx context.Context) error {
	result, err := c.manager.Cleanup(ctx, c.limit)
	if err != nil {
		return err
	}
	c.logger.Printf("SaaS identity cleanup completed: expired_challenges=%d expired_sessions=%d deleted_challenges=%d deleted_sessions=%d deleted_events=%d",
		result.ExpiredChallenges, result.ExpiredSessions, result.DeletedChallenges, result.DeletedSessions, result.DeletedEvents)
	return nil
}
