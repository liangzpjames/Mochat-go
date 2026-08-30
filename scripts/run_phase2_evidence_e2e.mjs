import { spawnSync } from 'node:child_process';
import { mkdirSync, rmSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';

import {
  phase2BuildInputFingerprint,
  phase2BuildOutputMtimeRange,
  validatePhase2BuildProvenance,
} from './phase2_build_audit.mjs';
import { currentPhase2Routes, phase2Apps } from './phase2_current_routes.mjs';

const root = process.cwd();
const routesByApp = currentPhase2Routes(root);
const slug = (value) => value === '/' ? 'root' : value.slice(1).replaceAll('/', '__');
for (const file of [
  '.tmp-phase2-evidence/phase2-build.json',
  'web/e2e/test-results/.last-run.json',
  'web/e2e/test-results/phase2-results.json',
]) rmSync(resolve(root, file), { force: true });
for (const app of phase2Apps) {
  for (const route of routesByApp[app]) {
    for (const viewport of ['desktop', 'mobile']) {
      rmSync(resolve(
        root,
        'docs/phases/phase-2-frontend-migration/evidence/screenshots',
        `${app}--${slug(route.path)}--${viewport}.png`,
      ), { force: true });
    }
  }
}

const spawn = (args, env = process.env) => spawnSync('corepack', args, {
  cwd: root,
  encoding: 'utf8',
  env,
  shell: process.platform === 'win32',
  stdio: 'inherit',
});

const sourceFingerprint = phase2BuildInputFingerprint(root);
const buildStartedAt = new Date().toISOString();
const buildResult = spawn(['pnpm', 'build']);
if (buildResult.error) throw buildResult.error;
if (buildResult.status !== 0) {
  process.exitCode = buildResult.status ?? 1;
} else {
  const buildCompletedAt = new Date().toISOString();
  const currentSourceFingerprint = phase2BuildInputFingerprint(root);
  if (sourceFingerprint !== currentSourceFingerprint) {
    throw new Error('Phase 2 frontend source changed while the production build was running');
  }
  const marker = {
    schemaVersion: 1,
    command: 'corepack pnpm build',
    startedAt: buildStartedAt,
    completedAt: buildCompletedAt,
    sourceFingerprint,
  };
  validatePhase2BuildProvenance({
    marker,
    currentSourceFingerprint,
    oldestOutputMtimeMs: phase2BuildOutputMtimeRange(root).oldest,
  });
  const markerFile = resolve(root, '.tmp-phase2-evidence/phase2-build.json');
  mkdirSync(resolve(root, '.tmp-phase2-evidence'), { recursive: true });
  writeFileSync(markerFile, `${JSON.stringify(marker, null, 2)}\n`);

  const playwrightResult = spawn([
    'pnpm',
    '--dir',
    'web/e2e',
    'exec',
    'playwright',
    'test',
    'tests/dashboard-migration.spec.ts',
    '--reporter=line,json',
  ], {
    ...process.env,
    PLAYWRIGHT_JSON_OUTPUT_NAME: 'test-results/phase2-results.json',
  });
  if (playwrightResult.error) throw playwrightResult.error;
  process.exitCode = playwrightResult.status ?? 1;
}
