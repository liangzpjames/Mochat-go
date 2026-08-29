import test from 'node:test';
import assert from 'node:assert/strict';

import { eslintInvocation, partitionLintFiles, selectDashboardChangedLintFiles } from './check_phase34_lint.mjs';

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

test('partitions every lint target into stable command-sized batches without omission', () => {
  const files = Array.from({ length: 17 }, (_, index) => `src/feature-${String(index).padStart(2, '0')}.tsx`);
  const batches = partitionLintFiles(files, { maxFiles: 4, maxCommandCharacters: 70 });

  assert.deepEqual(batches.flat(), files);
  assert.ok(batches.every((batch) => batch.length <= 4));
  assert.ok(batches.every((batch) => batch.reduce((total, file) => total + file.length + 1, 0) <= 70));
});

test('rejects an individual lint target that cannot fit the command budget', () => {
  const longFile = `src/${'nested/'.repeat(20)}page.tsx`;
  assert.throws(() => partitionLintFiles([longFile], { maxFiles: 8, maxCommandCharacters: 30 }), /exceeds command budget/);
});

test('default Windows-safe batches use direct Node execution and stay below 7000 characters', () => {
  const files = Array.from({ length: 300 }, (_, index) => `src/features/phase34/${String(index).padStart(3, '0')}-${'nested-'.repeat(6)}page.tsx`);
  const batches = partitionLintFiles(files);
  assert.deepEqual(batches.flat(), files);
  for (const batch of batches) {
    const invocation = eslintInvocation(batch);
    assert.equal(invocation.command, process.execPath);
    assert.equal(invocation.options.shell, false);
    assert.ok([invocation.command, ...invocation.args].join(' ').length <= 7000);
  }
});
