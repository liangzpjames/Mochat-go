import assert from 'node:assert/strict';
import { test } from 'node:test';

import { validatePhase32Manifest } from './check_phase3_2_dashboard_completion.mjs';

test('Phase 3.2 requires exactly the eight target routes', () => {
  const manifest = {
    pages: [
      '/index',
      '/chat/v2-all',
      '/ai-insight/v2/sensitive-word',
      '/customer/clue/default',
      '/customer/contact',
      '/customer/opportunity',
      '/customer/public-sea',
      '/customer/tags',
    ].map((path) => ({
      path,
      implementation: path === '/index' || path === '/chat/v2-all' ? 'native' : 'legacy-adapter',
      backend: 'ready',
      acceptance: 'e2e-passed',
      phase: '3.2',
      owner: 'dashboard',
      risk: 'medium',
      legacyRoutes: [],
      evidence: { spec: 'docs/spec.md', acceptance: 'docs/acceptance.md' },
    })),
  };

  assert.doesNotThrow(() => validatePhase32Manifest(manifest));
});
