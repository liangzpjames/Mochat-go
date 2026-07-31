import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';

import { validateManifest } from './check_yuanhu_benchmark_manifest.mjs';

const manifestPath = new URL('../web/apps/dashboard/src/benchmark/manifest.json', import.meta.url);
const expectedGroups = [
  'conversation',
  'risk-warning',
  'ai-insight',
  'marketing-tools',
  'scrm',
  'data-reports',
  'ai-settings',
  'company-settings',
];

async function loadManifest() {
  return JSON.parse(await readFile(manifestPath, 'utf8'));
}

test('the Yuanhu manifest records the eight observed navigation groups and index page', async () => {
  const manifest = await loadManifest();

  assert.deepEqual(manifest.groups.map((group) => group.id), expectedGroups);
  assert.deepEqual(manifest.pages.find((page) => page.path === '/index')?.groupId, null);
});

test('the Yuanhu manifest uses unique paths and valid implementation levels', async () => {
  const manifest = await loadManifest();

  assert.doesNotThrow(() => validateManifest(manifest));
  for (const page of manifest.pages) {
    assert.match(page.path, /^\//);
    assert.match(page.level, /^P[012]$/);
  }
});

test('the manifest validator rejects duplicate routes', async () => {
  const manifest = await loadManifest();
  const duplicate = structuredClone(manifest);
  duplicate.pages.push({ ...duplicate.pages[0] });

  assert.throws(() => validateManifest(duplicate), /duplicate page path: \/index/);
});

test('the manifest validator rejects an index page assigned to a navigation group', async () => {
  const manifest = await loadManifest();
  const invalidIndexGroup = structuredClone(manifest);
  invalidIndexGroup.pages.find((page) => page.path === '/index').groupId = 'conversation';

  assert.throws(() => validateManifest(invalidIndexGroup), /index page must have groupId: null/);
});

test('the manifest records Phase 3.2 governance fields for every page', async () => {
  const manifest = await loadManifest();

  for (const page of manifest.pages) {
    assert.match(page.implementation, /^(placeholder|demo|legacy-adapter|native)$/);
    assert.match(page.backend, /^(missing|partial|ready)$/);
    assert.match(page.acceptance, /^(not-started|unit-passed|integration-passed|e2e-passed)$/);
    assert.match(page.phase, /^3\.[1-6]$/);
    assert.match(page.owner, /^[-a-z]+$/);
    assert.match(page.risk, /^(low|medium|high)$/);
    assert.ok(Array.isArray(page.legacyRoutes));
    assert.match(page.evidence.spec, /^docs\//);
    assert.match(page.evidence.acceptance, /^docs\//);
  }
});

test('the Phase 3.2 gate rejects demo pages marked as complete', async () => {
  const { validatePhase32Manifest } = await import('./check_phase3_2_dashboard_completion.mjs');
  const manifest = await loadManifest();
  const invalid = structuredClone(manifest);
  const page = invalid.pages.find((candidate) => candidate.path === '/customer/contact');
  page.implementation = 'demo';
  page.backend = 'ready';
  page.acceptance = 'e2e-passed';

  assert.throws(() => validatePhase32Manifest(invalid), /completed page must be native or legacy-adapter/);
});
