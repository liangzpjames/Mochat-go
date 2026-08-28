import assert from "node:assert/strict";
import test from "node:test";

import {
  inspectDurableArchiveScheduler,
  inspectDurableArchiveSchedulerSources,
} from "./check_durable_archive_scheduler.mjs";

test("durable worker and automatic scheduler stay independently gated", async () => {
  const findings = await inspectDurableArchiveScheduler(process.cwd());
  assert.deepEqual(findings, []);
});

test("contract rejects worker-side discovery and unconditional scheduler registration", () => {
  const findings = inspectDurableArchiveSchedulerSources({
    runtimePlan: `durableScheduler: durableEnabled && automaticEnabled`,
    main: `
      if archivePlan.durableWorker {
        runner.RunPendingOnce(ctx)
        runner.EnqueueScheduledOnce(ctx)
      }
      scheduler.Register("cron-durable-work-message-archive-enqueue")
      if archivePlan.legacyScheduler {}
    `,
    runner: `
      func (r *DurableBridgeRunner) RunPendingOnce(ctx context.Context) error {
        r.store.DurableArchiveBindings(ctx)
      }
      func (r *DurableBridgeRunner) EnqueueScheduledOnce(ctx context.Context) error {
        r.store.BusyDurableArchiveScopes(ctx)
        r.store.DurableArchiveBindings(ctx)
        if !hasNewMessage { continue }
        NewSyncService(r.store).Enqueue(ctx, source, request)
      }
    `,
    config: "",
    compose: "MOCHAT_GO_ENABLE_WORK_MESSAGE_ARCHIVE_SYNC_CRON: ${MOCHAT_GO_ENABLE_WORK_MESSAGE_ARCHIVE_SYNC_CRON:-0}",
  });
  assert.ok(findings.includes("durable worker must not discover or enqueue scheduled archive work"));
  assert.ok(findings.includes("durable scheduler registration must be gated by archivePlan.durableScheduler"));
});
