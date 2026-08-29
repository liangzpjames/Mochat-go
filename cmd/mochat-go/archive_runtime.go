package main

import "jiyi/mochat-go/internal/config"

type archiveRuntimePlan struct {
	durableAPI       bool
	durableWorker    bool
	durableScheduler bool
	legacyScheduler  bool
}

func archiveRuntimePlanFor(cfg config.Config) archiveRuntimePlan {
	responsibilities := cfg.RuntimeRole.Responsibilities(
		cfg.EnableDurableWorkMessageArchive,
		cfg.EnableWorkMessageArchiveSyncCron,
	)
	return archiveRuntimePlan{
		durableAPI:       responsibilities.DurableArchiveAPI,
		durableWorker:    responsibilities.DurableArchiveWorkers,
		durableScheduler: cfg.EnableDurableWorkMessageArchive && cfg.EnableWorkMessageArchiveSyncCron && responsibilities.ArchiveEnqueuer,
		legacyScheduler:  !cfg.EnableDurableWorkMessageArchive && cfg.EnableWorkMessageArchiveSyncCron && responsibilities.Schedulers,
	}
}
