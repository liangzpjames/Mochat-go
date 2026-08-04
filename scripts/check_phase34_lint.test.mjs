import test from 'node:test';
import assert from 'node:assert/strict';

import { selectDashboardChangedLintFiles } from './check_phase34_lint.mjs';

test('selects changed Dashboard TypeScript files and excludes non-lintable paths', () => {
  assert.deepEqual(
    selectDashboardChangedLintFiles([
      'web/apps/dashboard/src/features/phase34/acquisition-pages.tsx',
      'web/apps/dashboard/src/features/phase34/acquisition-pages.tsx',
      'web/apps/dashboard/src/features/phase34/material-management/material-selector.test.tsx',
      'web/apps/dashboard/src/styles/dashboard-scroll-layout.test.ts',
      'web/apps/dashboard/src/benchmark/manifest.json',
      'web/apps/dashboard/src/styles/index.css',
      'web/apps/dashboard/src/features/phase34/README.md',
      'internal/dashboard/phase34_provider.go',
      'web/saas-admin/src/app.tsx',
    ]),
    [
      'web/apps/dashboard/src/features/phase34/acquisition-pages.tsx',
      'web/apps/dashboard/src/features/phase34/material-management/material-selector.test.tsx',
      'web/apps/dashboard/src/styles/dashboard-scroll-layout.test.ts',
    ],
  );
});

test('accepts slash-normalized paths from Git on Windows', () => {
  assert.deepEqual(
    selectDashboardChangedLintFiles([
      './web/apps/dashboard/src/features/phase34/conversion-pages.tsx',
      'web/apps/dashboard/src/features/phase34/content-reach-pages.ts',
    ]),
    [
      'web/apps/dashboard/src/features/phase34/content-reach-pages.ts',
      'web/apps/dashboard/src/features/phase34/conversion-pages.tsx',
    ],
  );
});
