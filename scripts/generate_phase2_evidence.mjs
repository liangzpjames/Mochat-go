import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, statSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

import { derivePhase2BuildAudit } from './phase2_build_audit.mjs';
import {
  currentPhase2Routes,
  expectedPhase2PlaywrightTitles,
  phase2Apps,
} from './phase2_current_routes.mjs';

const root = process.cwd();
const phase = join(root, 'docs/phases/phase-2-frontend-migration/evidence');
const screenshots = join(phase, 'screenshots');
const runFile = join(root, 'web/e2e/test-results/.last-run.json');
const reportFile = join(root, 'web/e2e/test-results/phase2-results.json');
const readJson = (path) => JSON.parse(readFileSync(path, 'utf8'));
const slug = (value) => value === '/' ? 'root' : value.slice(1).replaceAll('/', '__');
const sha256 = (path) => createHash('sha256').update(readFileSync(path)).digest('hex');

mkdirSync(phase, { recursive: true });
if (!existsSync(runFile)) throw new Error('Playwright result is missing; run the Phase 2 E2E suite first');
const playwrightRun = readJson(runFile);
if (playwrightRun.status !== 'passed' || playwrightRun.failedTests.length !== 0) {
  throw new Error('latest Playwright run did not pass');
}

const routesByApp = currentPhase2Routes(root);
const expectedTitles = expectedPhase2PlaywrightTitles(routesByApp);
const expectedTests = expectedTitles.length;
if (!existsSync(reportFile)) {
  throw new Error('Playwright JSON report is missing; run pnpm test:e2e:phase2-evidence first');
}
const reporterRun = readJson(reportFile);
const startedAtMs = Date.parse(reporterRun.stats.startTime);
if (
  reporterRun.errors.length !== 0
  || reporterRun.stats.expected !== expectedTests
  || reporterRun.stats.skipped !== 0
  || reporterRun.stats.unexpected !== 0
  || reporterRun.stats.flaky !== 0
  || reporterRun.suites.length !== 1
  || reporterRun.suites[0].file !== 'dashboard-migration.spec.ts'
  || reporterRun.suites[0].specs.length !== expectedTests
  || reporterRun.suites[0].specs.some((spec) => spec.ok !== true)
  || !Number.isFinite(startedAtMs)
) {
  throw new Error(`Playwright JSON report is not the complete passing Phase 2 suite (${expectedTests} tests required)`);
}
const testTitles = reporterRun.suites[0].specs.map((spec) => spec.title);
if (JSON.stringify(testTitles) !== JSON.stringify(expectedTitles)) {
  throw new Error('Playwright JSON report titles do not match the exact current Phase 2 suite');
}
if (statSync(runFile).mtimeMs < startedAtMs || statSync(reportFile).mtimeMs < startedAtMs) {
  throw new Error('Playwright result files predate the reported Phase 2 run');
}
const routeEvidence = existsSync(join(phase, 'route-evidence.json'))
  ? readJson(join(phase, 'route-evidence.json'))
  : {};
const rollbackRecords = existsSync(join(phase, 'rollback-records.json'))
  ? readJson(join(phase, 'rollback-records.json'))
  : {};
for (const value of Object.values(routeEvidence)) {
  value.currentReachable = false;
  value.evidenceStatus = 'historical';
  value.playwrightRun = null;
}
for (const value of Object.values(rollbackRecords)) value.currentReachable = false;
for (const app of phase2Apps) {
  const routes = routesByApp[app];
  for (const route of routes) {
    const key = `${app}:${route.path}`;
    const images = {};
    for (const viewport of ['desktop', 'mobile']) {
      const relative = `screenshots/${app}--${slug(route.path)}--${viewport}.png`;
      const path = join(phase, relative);
      if (!existsSync(path) || statSync(path).size === 0) throw new Error(`missing visual evidence: ${path}`);
      if (statSync(path).mtimeMs < startedAtMs) throw new Error(`stale visual evidence: ${path}`);
      images[`${viewport}Screenshot`] = relative;
      images[`${viewport}Sha256`] = sha256(path);
    }
    routeEvidence[key] = {
      target: 'react',
      currentReachable: true,
      evidenceStatus: 'current',
      playwrightRun: 'playwright-run.json',
      preservesQueryAndHash: true,
      ...images,
    };
    rollbackRecords[key] = {
      currentReachable: true,
      procedureRecorded: true,
      mechanismVerified: true,
      productionRestoreDrillExecuted: false,
      sourceBaselineCommit: '09a010ae27ade37b0e8f2c598ee60ec01b4ebbd9',
      procedure: `restore ${app}:${route.path} and its required legacy artifact from the source baseline in an isolated recovery branch, then rebuild and rerun all gates`,
      verification: 'manifest route isolation and static mount regression tests',
    };
  }
}

const buildAudit = derivePhase2BuildAudit(root);

writeFileSync(join(phase, 'playwright-run.json'), `${JSON.stringify({
  ...playwrightRun,
  suite: 'web/e2e/tests/dashboard-migration.spec.ts',
  source: 'playwright-json-reporter',
  expectedTests,
  actualTests: reporterRun.stats.expected,
  testTitles,
  startedAt: reporterRun.stats.startTime,
  durationMs: reporterRun.stats.duration,
  reportSha256: sha256(reportFile),
  recordedAt: new Date().toISOString(),
}, null, 2)}\n`);
writeFileSync(join(phase, 'route-evidence.json'), `${JSON.stringify(routeEvidence, null, 2)}\n`);
writeFileSync(join(phase, 'rollback-records.json'), `${JSON.stringify(rollbackRecords, null, 2)}\n`);
writeFileSync(join(phase, 'build-audit.json'), `${JSON.stringify(buildAudit, null, 2)}\n`);
console.log(`phase2 evidence indexed from passing Playwright run: routes=${expectedTests - 2}`);
