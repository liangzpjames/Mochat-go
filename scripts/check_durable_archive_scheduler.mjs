import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const sourcesByName = {
  runtimePlan: "cmd/mochat-go/archive_runtime.go",
  main: "cmd/mochat-go/main.go",
  runner: "internal/modules/providers/archive/durable_bridge.go",
  config: "internal/config/config.go",
  compose: "deploy/standalone/docker-compose.yml",
};

export async function inspectDurableArchiveScheduler(root) {
  const entries = await Promise.all(
    Object.entries(sourcesByName).map(async ([name, relativePath]) => [
      name,
      await readFile(path.join(root, relativePath), "utf8"),
    ]),
  );
  return inspectDurableArchiveSchedulerSources(Object.fromEntries(entries));
}

export function inspectDurableArchiveSchedulerSources(sources) {
  const findings = [];
  const runtimePlan = sources.runtimePlan ?? "";
  const main = sources.main ?? "";
  const runner = sources.runner ?? "";
  const config = sources.config ?? "";
  const compose = sources.compose ?? "";

  if (!/durableScheduler:\s*cfg\.EnableDurableWorkMessageArchive\s*&&\s*cfg\.EnableWorkMessageArchiveSyncCron/.test(runtimePlan)) {
    findings.push("automatic durable scheduling must require both durable and cron flags");
  }
  if (!/legacyScheduler:\s*!cfg\.EnableDurableWorkMessageArchive\s*&&\s*cfg\.EnableWorkMessageArchiveSyncCron/.test(runtimePlan)) {
    findings.push("legacy scheduling must remain exclusive to non-durable mode");
  }

  const workerStart = main.indexOf("if archivePlan.durableWorker {");
  const schedulerStart = main.indexOf("if archivePlan.durableScheduler {", workerStart);
  const workerBody = workerStart >= 0
    ? main.slice(workerStart, schedulerStart >= 0 ? schedulerStart : main.length)
    : "";
  if (
    !workerBody.includes("durableRunner.RunPendingOnce") ||
    workerBody.includes("durableRunner.EnqueueScheduledOnce") ||
    workerBody.includes("DurableArchiveBindings")
  ) {
    findings.push("durable worker must not discover or enqueue scheduled archive work");
  }
  if (!/if archivePlan\.durableScheduler \{[\s\S]*?cron-durable-work-message-archive-enqueue[\s\S]*?durableRunner\.EnqueueScheduledOnce/.test(main)) {
    findings.push("durable scheduler registration must be gated by archivePlan.durableScheduler");
  }
  if (!/if archivePlan\.legacyScheduler \{[\s\S]*?NewWorkMessageArchiveSyncCron/.test(main)) {
    findings.push("legacy scheduler registration must be gated by archivePlan.legacyScheduler");
  }

  const pendingStart = runner.indexOf("func (r *DurableBridgeRunner) RunPendingOnce");
  const enqueueStart = runner.indexOf("func (r *DurableBridgeRunner) EnqueueScheduledOnce", pendingStart);
  const pendingBody = pendingStart >= 0
    ? runner.slice(pendingStart, enqueueStart >= 0 ? enqueueStart : runner.length)
    : "";
  if (
    !pendingBody.includes("PendingDurableArchiveRuns") ||
    pendingBody.includes("DurableArchiveBindings")
  ) {
    findings.push("pending worker must only consume persisted queued or expired runs");
  }
  const enqueueBody = enqueueStart >= 0 ? runner.slice(enqueueStart) : "";
  for (const required of [
    "BusyDurableArchiveScopes",
    "DurableArchiveBindings",
    "if !hasNewMessage",
    "NewSyncService(r.store).Enqueue",
  ]) {
    if (!enqueueBody.includes(required)) {
      findings.push(`scheduled enqueuer is missing ${required}`);
    }
  }

  if (/cannot both be enabled|mutually exclusive/i.test(config)) {
    findings.push("durable worker and automatic scheduler flags must not be mutually exclusive");
  }
  if (!compose.includes("MOCHAT_GO_ENABLE_WORK_MESSAGE_ARCHIVE_SYNC_CRON:-0")) {
    findings.push("standalone automatic archive scheduling must default to disabled");
  }
  return findings;
}

async function main() {
  const findings = await inspectDurableArchiveScheduler(process.cwd());
  if (findings.length > 0) {
    for (const finding of findings) {
      console.error(`FAIL ${finding}`);
    }
    process.exitCode = 1;
    return;
  }
  console.log("durable archive scheduler contract passed");
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? fileURLToPath(import.meta.url)).href) {
  await main();
}
