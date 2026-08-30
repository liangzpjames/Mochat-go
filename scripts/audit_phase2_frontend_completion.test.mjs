import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

import { derivePhase2BuildAudit } from './phase2_build_audit.mjs';
import {
  currentPhase2RouteCount,
  currentPhase2Routes,
  expectedPhase2PlaywrightTitles,
  phase2Apps,
} from './phase2_current_routes.mjs';

const readJson = (path) => JSON.parse(readFileSync(path, 'utf8'));

test('all three application manifests contain React-only routes', () => {
  for (const app of ['dashboard', 'sidebar', 'operation']) {
    const routes = readJson(`web/apps/${app}/src/migration-routes.json`);
    assert.ok(routes.length > 0, `${app} manifest must not be empty`);
    assert.deepEqual(
      routes.filter((route) => route.target !== 'react'),
      [],
      `${app} retains a legacy route`,
    );
  }
});

test('recorded Playwright test count follows the current route manifests', () => {
  const routeCount = currentPhase2RouteCount(currentPhase2Routes());
  const playwrightRun = readJson('docs/phases/phase-2-frontend-migration/evidence/playwright-run.json');

  assert.equal(playwrightRun.expectedTests, routeCount + 2);
  assert.equal(playwrightRun.actualTests, routeCount + 2);
  assert.deepEqual(playwrightRun.testTitles, expectedPhase2PlaywrightTitles(currentPhase2Routes()));
  assert.match(playwrightRun.reportSha256, /^[a-f0-9]{64}$/);
});

test('current reachable Phase 2 routes follow the Dashboard security manifest', () => {
  const routes = currentPhase2Routes();

  assert.deepEqual(routes.dashboard.map((route) => route.path), ['/company-setting/website']);
  assert.equal(routes.sidebar.length, 12);
  assert.equal(routes.operation.length, 10);
});

test('every current route has Playwright and rollback evidence rows', () => {
  const evidence = readJson('docs/phases/phase-2-frontend-migration/evidence/route-evidence.json');
  const rollback = readJson('docs/phases/phase-2-frontend-migration/evidence/rollback-records.json');
  const routesByApp = currentPhase2Routes();
  const currentKeys = new Set();
  for (const app of phase2Apps) {
    const routes = routesByApp[app];
    for (const route of routes) {
      const key = `${app}:${route.path}`;
      currentKeys.add(key);
      assert.equal(evidence[key]?.currentReachable, true, `${key} is not marked current`);
      assert.equal(evidence[key]?.evidenceStatus, 'current', `${key} has stale evidence status`);
      assert.equal(evidence[key]?.playwrightRun, 'playwright-run.json', `${key} lacks Playwright evidence`);
      assert.match(evidence[key]?.desktopScreenshot ?? '', /\.png$/, `${key} lacks desktop evidence`);
      assert.match(evidence[key]?.mobileScreenshot ?? '', /\.png$/, `${key} lacks mobile evidence`);
      assert.equal(rollback[key]?.procedureRecorded, true, `${key} lacks rollback procedure`);
      assert.equal(rollback[key]?.productionRestoreDrillExecuted, false);
    }
  }
  for (const [key, value] of Object.entries(evidence)) {
    if (currentKeys.has(key)) continue;
    assert.equal(value.currentReachable, false, `${key} historical evidence is marked current`);
    assert.equal(value.evidenceStatus, 'historical', `${key} historical evidence status is missing`);
    assert.equal(value.playwrightRun, null, `${key} historical evidence references the current Playwright run`);
  }
});

test('build evidence hashes shipped assets without inventing lazy chunks', () => {
  const buildAudit = readJson('docs/phases/phase-2-frontend-migration/evidence/build-audit.json');
  assert.deepEqual(buildAudit, derivePhase2BuildAudit());
  for (const app of phase2Apps) {
    assert.ok(buildAudit[app].entryAssets.some((name) => name.endsWith('.js')), `${app} lacks a JavaScript entry`);
    assert.ok(buildAudit[app].javascriptAssets.length > 0, `${app} lacks JavaScript assets`);
    assert.ok(buildAudit[app].assets.length > 0, `${app} lacks hashed asset evidence`);
    for (const asset of buildAudit[app].assets) {
      assert.ok(asset.bytes > 0, `${app}:${asset.name} is empty`);
      assert.match(asset.sha256, /^[a-f0-9]{64}$/, `${app}:${asset.name} lacks SHA-256`);
    }
  }
  assert.ok(buildAudit.dashboard.lazyChunks.length > 0, 'Dashboard lazy chunks were not recorded');
  assert.deepEqual(buildAudit.sidebar.lazyChunks, []);
  assert.deepEqual(buildAudit.operation.lazyChunks, []);
});

test('Docker builds every shipped React application', () => {
  const dockerfile = readFileSync('Dockerfile', 'utf8');
  for (const app of ['dashboard', 'sidebar', 'operation', 'saas-admin']) {
    assert.match(dockerfile, new RegExp(`pnpm --filter @mochat/${app} build`));
  }
});
