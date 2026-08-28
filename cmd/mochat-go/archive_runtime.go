package main

import "jiyi/mochat-go/internal/config"

type archiveRuntimePlan struct {
	durableWorker    bool
	durableScheduler bool
	legacyScheduler  bool
}

func archiveRuntimePlanFor(cfg config.Config) archiveRuntimePlan {
	return archiveRuntimePlan{
		durableWorker:    cfg.EnableDurableWorkMessageArchive,
		durableScheduler: cfg.EnableDurableWorkMessageArchive && cfg.EnableWorkMessageArchiveSyncCron,
		legacyScheduler:  !cfg.EnableDurableWorkMessageArchive && cfg.EnableWorkMessageArchiveSyncCron,
	}
}
