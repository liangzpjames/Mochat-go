package main

import (
	"testing"

	"jiyi/mochat-go/internal/config"
)

func TestArchiveRuntimePlanSeparatesDurableWorkerAndAutomaticScheduling(t *testing.T) {
	tests := []struct {
		name                                string
		durable, scheduled                  bool
		wantWorker, wantDurable, wantLegacy bool
	}{
		{name: "all disabled"},
		{name: "legacy scheduled", scheduled: true, wantLegacy: true},
		{name: "durable worker only", durable: true, wantWorker: true},
		{name: "durable scheduled", durable: true, scheduled: true, wantWorker: true, wantDurable: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := archiveRuntimePlanFor(config.Config{
				EnableDurableWorkMessageArchive:  test.durable,
				EnableWorkMessageArchiveSyncCron: test.scheduled,
			})
			if plan.durableWorker != test.wantWorker || plan.durableScheduler != test.wantDurable || plan.legacyScheduler != test.wantLegacy {
				t.Fatalf("plan=%#v want worker=%t durable_scheduler=%t legacy_scheduler=%t", plan, test.wantWorker, test.wantDurable, test.wantLegacy)
			}
			if plan.durableScheduler && plan.legacyScheduler {
				t.Fatalf("duplicate automatic schedulers in plan=%#v", plan)
			}
		})
	}
}
