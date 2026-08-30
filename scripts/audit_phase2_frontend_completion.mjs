import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { existsSync, readFileSync, statSync } from 'node:fs';

import { derivePhase2BuildAudit } from './phase2_build_audit.mjs';
import {
  currentPhase2Routes,
  expectedPhase2PlaywrightTitles,
  phase2Apps,
} from './phase2_current_routes.mjs';

const readJson = (path) => JSON.parse(readFileSync(path, 'utf8'));
const evidence = readJson('docs/phases/phase-2-frontend-migration/evidence/route-evidence.json');
const rollback = readJson('docs/phases/phase-2-frontend-migration/evidence/rollback-records.json');
const playwrightRun = readJson('docs/phases/phase-2-frontend-migration/evidence/playwright-run.json');
const buildAudit = readJson('docs/phases/phase-2-frontend-migration/evidence/build-audit.json');
const sha256 = (path) => createHash('sha256').update(readFileSync(path)).digest('hex');
assert.equal(playwrightRun.status, 'passed', 'recorded Playwright run did not pass');
assert.deepEqual(playwrightRun.failedTests, [], 'recorded Playwright run has failures');
let routeCount = 0;
const routesByApp = currentPhase2Routes();
const currentKeys = new Set();

for (const app of phase2Apps) {
  const routes = routesByApp[app];
  assert.ok(routes.length > 0, `${app} manifest is empty`);
  assert.equal(routes.some((route) => route.target !== 'react'), false, `${app} retains legacy targets`);
  for (const route of routes) {
    routeCount += 1;
    const key = `${app}:${route.path}`;
    currentKeys.add(key);
    assert.equal(evidence[key]?.currentReachable, true, `${key} is not marked current`);
    assert.equal(evidence[key]?.evidenceStatus, 'current', `${key} has stale evidence status`);
    assert.equal(evidence[key]?.playwrightRun, 'playwright-run.json', `${key} lacks Playwright evidence`);
    assert.equal(rollback[key]?.currentReachable, true, `${key} rollback is not marked current`);
    assert.equal(rollback[key]?.procedureRecorded, true, `${key} lacks rollback procedure`);
    for (const property of ['desktopScreenshot', 'mobileScreenshot']) {
      const path = `docs/phases/phase-2-frontend-migration/evidence/${evidence[key][property]}`;
      assert.ok(existsSync(path) && statSync(path).size > 0, `${key} lacks ${property}`);
      const hashProperty = property.replace('Screenshot', 'Sha256');
      assert.match(evidence[key][hashProperty] ?? '', /^[a-f0-9]{64}$/, `${key} lacks screenshot hash`);
      assert.equal(sha256(path), evidence[key][hashProperty], `${key} has stale ${property} hash`);
    }
  }
}
const expectedTitles = expectedPhase2PlaywrightTitles(routesByApp);
assert.equal(playwrightRun.expectedTests, expectedTitles.length, 'recorded Playwright test count is stale');
assert.equal(playwrightRun.actualTests, expectedTitles.length, 'recorded Playwright run was filtered');
assert.deepEqual(playwrightRun.testTitles, expectedTitles, 'recorded Playwright title inventory is stale');
assert.match(playwrightRun.reportSha256, /^[a-f0-9]{64}$/, 'recorded Playwright JSON report lacks SHA-256');
assert.deepEqual(buildAudit, derivePhase2BuildAudit(), 'recorded production build audit is stale');
for (const [key, value] of Object.entries(evidence)) {
  if (currentKeys.has(key)) continue;
  assert.equal(value.currentReachable, false, `${key} historical evidence is marked current`);
  assert.equal(value.evidenceStatus, 'historical', `${key} historical evidence status is missing`);
  assert.equal(value.playwrightRun, null, `${key} historical evidence references the current Playwright run`);
}
for (const [key, value] of Object.entries(rollback)) {
  if (currentKeys.has(key)) continue;
  assert.equal(value.currentReachable, false, `${key} historical rollback is marked current`);
}

for (const retired of ['web/dashboard/dist', 'web/sidebar/dist', 'web/operation/dist']) {
  assert.equal(existsSync(retired), false, `${retired} legacy build artifact still exists`);
}

const runtimeFiles = [
  'Dockerfile',
  'internal/config/config.go',
  'cmd/mochat-go/main.go',
  'scripts/frontend_check.sh',
  'scripts/frontend_check.ps1',
];
for (const file of runtimeFiles) {
  const source = readFileSync(file, 'utf8');
  assert.doesNotMatch(source, /web\/(?:dashboard|sidebar|operation)\/dist/, `${file} references a retired dist`);
}

console.log(`phase2 frontend completion: routes=${routeCount} legacy_targets=0 screenshots=${routeCount * 2}`);
