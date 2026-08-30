import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

import * as phase2BuildAudit from './phase2_build_audit.mjs';
import {
  currentPhase2RouteCount,
  currentPhase2Routes,
  expectedPhase2PlaywrightTitles,
  phase2Apps,
} from './phase2_current_routes.mjs';

const readJson = (path) => JSON.parse(readFileSync(path, 'utf8'));

test('Phase 2 build provenance rejects stale dist and out-of-order Playwright runs', () => {
  assert.equal(typeof phase2BuildAudit.validatePhase2BuildProvenance, 'function');

  const marker = {
    schemaVersion: 1,
    command: 'corepack pnpm build',
    startedAt: '2026-08-30T08:00:00.000Z',
    completedAt: '2026-08-30T08:01:00.000Z',
    sourceFingerprint: 'a'.repeat(64),
  };
  const valid = {
    marker,
    currentSourceFingerprint: marker.sourceFingerprint,
    oldestOutputMtimeMs: Date.parse(marker.startedAt),
    playwrightStartedAtMs: Date.parse(marker.completedAt),
  };

  assert.doesNotThrow(() => phase2BuildAudit.validatePhase2BuildProvenance(valid));
  assert.throws(
    () => phase2BuildAudit.validatePhase2BuildProvenance({
      ...valid,
      oldestOutputMtimeMs: Date.parse(marker.startedAt) - 1,
    }),
    /predates the Phase 2 build/,
  );
  assert.throws(
    () => phase2BuildAudit.validatePhase2BuildProvenance({
      ...valid,
      currentSourceFingerprint: 'b'.repeat(64),
    }),
    /source fingerprint/,
  );
  assert.throws(
    () => phase2BuildAudit.validatePhase2BuildProvenance({
      ...valid,
      playwrightStartedAtMs: Date.parse(marker.completedAt) - 1,
    }),
    /started before the Phase 2 build completed/,
  );
});

test('Phase 2 runner keeps build provenance outside the Playwright output directory', () => {
  const runner = readFileSync('scripts/run_phase2_evidence_e2e.mjs', 'utf8');
  assert.doesNotMatch(runner, /test-results\/phase2-build\.json/);
  assert.match(runner, /\.tmp-phase2-evidence\/phase2-build\.json/);
  assert.ok(
    runner.indexOf("spawn(['pnpm', 'build'])") < runner.indexOf("'playwright'"),
    'the Phase 2 runner must build before starting Playwright',
  );
});

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
  assert.equal(typeof phase2BuildAudit.phase2BuildInputFingerprint, 'function');
  assert.equal(buildAudit.provenance.sourceFingerprint, phase2BuildAudit.phase2BuildInputFingerprint());
  assert.deepEqual(buildAudit.apps, phase2BuildAudit.derivePhase2BuildAudit());
  for (const app of phase2Apps) {
    assert.ok(buildAudit.apps[app].entryAssets.some((name) => name.endsWith('.js')), `${app} lacks a JavaScript entry`);
    assert.ok(buildAudit.apps[app].javascriptAssets.length > 0, `${app} lacks JavaScript assets`);
    assert.ok(buildAudit.apps[app].assets.length > 0, `${app} lacks hashed asset evidence`);
    for (const asset of buildAudit.apps[app].assets) {
      assert.ok(asset.bytes > 0, `${app}:${asset.name} is empty`);
      assert.match(asset.sha256, /^[a-f0-9]{64}$/, `${app}:${asset.name} lacks SHA-256`);
    }
  }
  assert.ok(buildAudit.apps.dashboard.lazyChunks.length > 0, 'Dashboard lazy chunks were not recorded');
  assert.deepEqual(buildAudit.apps.sidebar.lazyChunks, []);
  assert.deepEqual(buildAudit.apps.operation.lazyChunks, []);
});

test('Docker builds every shipped React application', () => {
  const dockerfile = readFileSync('Dockerfile', 'utf8');
  for (const app of ['dashboard', 'sidebar', 'operation', 'saas-admin']) {
    assert.match(dockerfile, new RegExp(`pnpm --filter @mochat/${app} build`));
  }
});
