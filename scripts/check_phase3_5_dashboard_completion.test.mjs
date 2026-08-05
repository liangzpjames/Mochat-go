import assert from 'node:assert/strict';
import { test } from 'node:test';

import {
  phase35TargetRoutes,
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

test('accepts exactly the nine completed Phase 3.5 routes', () => {
  assert.doesNotThrow(() => validatePhase35Manifest({ pages: phase35TargetRoutes.map(completePage) }));
});

test('rejects a missing target route', () => {
  assert.throws(
    () => validatePhase35Manifest({ pages: phase35TargetRoutes.slice(1).map(completePage) }),
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
  assert.throws(() => validatePhase35Manifest({ pages }), /incomplete routes \(8\/9\)/);
});
