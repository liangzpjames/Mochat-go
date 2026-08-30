import { spawnSync } from 'node:child_process';
import { rmSync } from 'node:fs';
import { resolve } from 'node:path';

import { currentPhase2Routes, phase2Apps } from './phase2_current_routes.mjs';

const root = process.cwd();
const routesByApp = currentPhase2Routes(root);
const slug = (value) => value === '/' ? 'root' : value.slice(1).replaceAll('/', '__');
for (const file of [
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

const args = [
  'pnpm',
  '--dir',
  'web/e2e',
  'exec',
  'playwright',
  'test',
  'tests/dashboard-migration.spec.ts',
  '--reporter=line,json',
];
const result = spawnSync('corepack', args, {
  cwd: root,
  encoding: 'utf8',
  env: {
    ...process.env,
    PLAYWRIGHT_JSON_OUTPUT_NAME: 'test-results/phase2-results.json',
  },
  shell: process.platform === 'win32',
  stdio: 'inherit',
});

if (result.error) throw result.error;
process.exitCode = result.status ?? 1;
