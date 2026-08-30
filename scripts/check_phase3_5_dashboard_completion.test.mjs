import assert from 'node:assert/strict';
import { test } from 'node:test';

import {
  phase35TargetRoutes,
  readManifest,
  validatePhase35Manifest,
} from './check_phase3_5_dashboard_completion.mjs';

const completePage = (path) => ({
  path,
  phase: '3.5',
  implementation: 'native',
  backend: 'ready',
  acceptance: 'integration-passed',
  evidence: { spec: 'spec.md', acceptance: 'acceptance.md' },
});

test('accepts exactly the ten completed Phase 3.5 routes', () => {
  assert.doesNotThrow(() => validatePhase35Manifest({ pages: phase35TargetRoutes.map(completePage) }));
});

test('rejects a missing target route', () => {
  assert.throws(
    () => validatePhase35Manifest({
      pages: phase35TargetRoutes.filter((path) => path !== '/customer/friends').map(completePage),
    }),
    /missing Phase 3\.5 route: \/customer\/friends/,
  );
});

test('rejects an out-of-scope Phase 3.5 route', () => {
  const pages = [...phase35TargetRoutes.map(completePage), completePage('/enterprise/settings')];
  assert.throws(() => validatePhase35Manifest({ pages }), /unexpected Phase 3\.5 route/);
});

for (const implementation of ['demo', 'placeholder']) {
  test(`rejects ${implementation} as a completed implementation`, () => {
    const pages = phase35TargetRoutes.map(completePage);
    pages[0].implementation = implementation;
    assert.throws(() => validatePhase35Manifest({ pages }), /must be native or legacy-adapter/);
  });
}

test('rejects a route that is not actually complete', () => {
  const pages = phase35TargetRoutes.map(completePage);
  pages[0].backend = 'partial';
  pages[0].acceptance = 'not-started';
  assert.throws(() => validatePhase35Manifest({ pages }), /incomplete routes \(9\/10\)/);
});

test('production manifest assigns Phase 3.5 to its ten routes only', async () => {
  const manifest = await readManifest();
  assert.deepEqual(
    manifest.pages.filter((page) => page.phase === '3.5').map((page) => page.path).sort(),
    [...phase35TargetRoutes].sort(),
  );
  assert.equal(manifest.pages.find((page) => page.path === '/chat/file-audio')?.phase, '3.5');
});
