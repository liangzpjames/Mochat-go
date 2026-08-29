package main

import (
	"context"
	"errors"
	"time"

	"jiyi/mochat-go/internal/migration"
	compatserver "jiyi/mochat-go/internal/server"
	"jiyi/mochat-go/internal/taskrunner"
)

type migrationStatusReader interface {
	StatusReadOnly(context.Context) ([]migration.StatusItem, error)
}

type redisPinger interface {
	Ping(context.Context) error
}

func migrationReadinessProbes(reader migrationStatusReader) []compatserver.ReadinessProbe {
	return []compatserver.ReadinessProbe{
		{
			Code: "migration_current",
			Check: func(ctx context.Context) error {
				statuses, err := reader.StatusReadOnly(ctx)
				if err != nil {
					return err
				}
				for _, item := range statuses {
					if item.State == "database_ahead" {
						return compatserver.NewReadinessFailure("migration_database_ahead")
					}
				}
				for _, item := range statuses {
					if item.State != "applied" {
						return errors.New("migration ledger is not current")
					}
				}
				return nil
			},
		},
	}
}

func redisReadinessProbe(redis redisPinger) compatserver.ReadinessProbe {
	return compatserver.ReadinessProbe{
		Code: "redis_connection",
		Check: func(ctx context.Context) error {
			if redis == nil {
				return errors.New("redis dependency is not configured")
			}
			return redis.Ping(ctx)
		},
	}
}

func backgroundTasksReadinessProbe(snapshots func() []taskrunner.Snapshot) compatserver.ReadinessProbe {
	return compatserver.ReadinessProbe{
		Code: "background_tasks",
		Check: func(context.Context) error {
			if snapshots == nil || !taskrunner.RequiredTasksReady(snapshots(), time.Now()) {
				return errors.New("required background task health threshold exceeded")
			}
			return nil
		},
	}
}
