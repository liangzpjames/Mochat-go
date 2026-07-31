import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';

import { validateManifest } from './check_yuanhu_benchmark_manifest.mjs';

const manifestPath = new URL('../docs/benchmark/yuanhu/manifest.json', import.meta.url);
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
