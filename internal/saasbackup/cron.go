package saasbackup

import (
	"context"
	"log"
)

const CronTaskName = "cron-saas-backup"

type Cron struct {
	manager *Manager
	logger  *log.Logger
}

func NewCron(manager *Manager, logger *log.Logger) *Cron {
	if logger == nil {
		logger = log.Default()
	}
	return &Cron{manager: manager, logger: logger}
}

func (c *Cron) RunOnce(ctx context.Context) error {
	run, created, err := c.manager.CreateIfDue(ctx)
	if err != nil {
		return err
	}
	cleanup, resumed, err := c.manager.ResumeCleanup(ctx, Actor{})
	if err != nil {
		return err
	}
	if created {
		c.logger.Printf("SaaS backup cron finished: backup_no=%s size_bytes=%d verified=%s cleanup_resumed=%v cleanup_no=%s cleanup_status=%s", run.BackupNo, run.SizeBytes, run.VerificationStatus, resumed, cleanup.CleanupNo, cleanup.Status)
	} else {
		c.logger.Printf("SaaS backup cron finished: backup not due cleanup_resumed=%v cleanup_no=%s cleanup_status=%s", resumed, cleanup.CleanupNo, cleanup.Status)
	}
	return nil
}
