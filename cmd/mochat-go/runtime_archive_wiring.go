package main

import (
	"context"
	"errors"
	"log"

	"jiyi/mochat-go/internal/store"
)

func initializeRuntimeMySQLStore(target **store.MySQLStore, open func() (*store.MySQLStore, error)) error {
	if target == nil {
		return errors.New("runtime mysql target is required")
	}
	if *target != nil {
		return nil
	}
	if open == nil {
		return errors.New("runtime mysql initializer is required")
	}
	candidate, err := open()
	if err != nil {
		return err
	}
	if candidate == nil {
		return errors.New("runtime mysql initializer returned nil store")
	}
	*target = candidate
	return nil
}

type durableArchiveMediaBatchRunner interface {
	CleanupStaleAttempts(context.Context) (int, error)
	RunOne(context.Context) (bool, error)
}

func runDurableArchiveMediaBatch(ctx context.Context, runner durableArchiveMediaBatchRunner, limit int, logger *log.Logger) error {
	if logger == nil {
		logger = log.Default()
	}
	if _, err := runner.CleanupStaleAttempts(ctx); err != nil {
		logger.Print("go durable archive media attempt cleanup failed")
	}
	failed := false
	for index := 0; index < limit; index++ {
		worked, err := runner.RunOne(ctx)
		if err != nil {
			failed = true
			if !worked {
				break
			}
			continue
		}
		if !worked {
			break
		}
	}
	if failed {
		return errors.New("archive media batch completed with failed items")
	}
	return nil
}
