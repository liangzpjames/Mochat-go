package main

import (
	"context"
	"log/slog"
	"time"
)

func backgroundTaskCleanup(
	cancelRoot context.CancelFunc,
	wait func(context.Context) error,
	timeout time.Duration,
	logger *slog.Logger,
) func() {
	return func() {
		if cancelRoot != nil {
			cancelRoot()
		}
		if wait == nil {
			return
		}
		waitCtx, cancelWait := context.WithTimeout(context.Background(), timeout)
		defer cancelWait()
		if err := wait(waitCtx); err != nil {
			if logger == nil {
				logger = slog.Default()
			}
			logger.Error("后台任务未在关闭期限内退出",
				"event", "background_tasks_drain_failed", "component", "runtime", "step", "shutdown",
				"result", "failed", "error_code", "BACKGROUND_TASKS_DRAIN_TIMEOUT")
		}
	}
}
