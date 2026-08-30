package main

import (
	"fmt"
	"testing"

	appruntime "jiyi/mochat-go/internal/app/runtime"
	"jiyi/mochat-go/internal/config"
)

func TestArchiveRuntimePlanSeparatesArchiveResponsibilitiesByRole(t *testing.T) {
	for _, role := range []appruntime.Role{appruntime.RoleAll, appruntime.RoleAPI, appruntime.RoleWorker, appruntime.RoleScheduler} {
		for _, durable := range []bool{false, true} {
			for _, scheduled := range []bool{false, true} {
				t.Run(fmt.Sprintf("role=%s/durable=%t/scheduled=%t", role, durable, scheduled), func(t *testing.T) {
					plan := archiveRuntimePlanFor(config.Config{
						RuntimeRole:                      role,
						EnableDurableWorkMessageArchive:  durable,
						EnableWorkMessageArchiveSyncCron: scheduled,
					})
					wantAPI := (role == appruntime.RoleAll || role == appruntime.RoleAPI) && durable
					wantWorker := (role == appruntime.RoleAll || role == appruntime.RoleWorker) && durable
					wantDurableScheduler := (role == appruntime.RoleAll || role == appruntime.RoleScheduler) && durable && scheduled
					wantLegacyScheduler := (role == appruntime.RoleAll || role == appruntime.RoleScheduler) && !durable && scheduled

					if plan.durableAPI != wantAPI || plan.durableWorker != wantWorker || plan.durableScheduler != wantDurableScheduler || plan.legacyScheduler != wantLegacyScheduler {
						t.Fatalf("plan=%#v want api=%t worker=%t durable_scheduler=%t legacy_scheduler=%t", plan, wantAPI, wantWorker, wantDurableScheduler, wantLegacyScheduler)
					}
					if plan.durableScheduler && plan.legacyScheduler {
						t.Fatalf("duplicate automatic schedulers in plan=%#v", plan)
					}
				})
			}
		}
	}
}
