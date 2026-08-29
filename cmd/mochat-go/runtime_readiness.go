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

func migrationReadinessProbes(reader migrationStatusReader) []compatserver.ReadinessProbe {
	return []compatserver.ReadinessProbe{
		{
			Code: "migration_database_ahead",
			Check: func(ctx context.Context) error {
				statuses, err := reader.StatusReadOnly(ctx)
				if err != nil {
					return err
				}
				for _, item := range statuses {
					if item.State == "database_ahead" {
						return errors.New("database migration ledger is ahead of this binary")
					}
				}
				return nil
			},
		},
		{
			Code: "migration_current",
			Check: func(ctx context.Context) error {
				statuses, err := reader.StatusReadOnly(ctx)
				if err != nil {
					return err
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
