import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

import {
  isAcceptedState,
  parseCsv,
  validateMatrix,
} from './check_phase2_1_frontend_completion.mjs';

const manifests = {
  dashboard: [{ path: '/example' }],
  sidebar: [{ path: '/contact' }],
  operation: [{ path: '/lottery' }],
};

const completeRow = (overrides = {}) => ({
  app: 'dashboard',
  feature: 'example',
  legacy_sources: 'web/legacy/dashboard/src/views/example.vue',
  routes: '/example',
  react_module: 'web/apps/dashboard/src/features/example/page.tsx',
  go_apis: 'GET /dashboard/example/index',
  implementation: 'functional',
  read: 'passed',
  write: 'passed',
  permissions: 'passed',
  fake_external: 'not-applicable:no external platform',
  component_tests: 'passed',
  browser: 'passed',
  evidence: 'docs/example.md',
  ...overrides,
});

test('accepts passed and explained not-applicable capability states', () => {
  assert.equal(isAcceptedState('passed'), true);
  assert.equal(isAcceptedState('not-applicable:read-only route'), true);
  assert.equal(isAcceptedState('not-applicable:'), false);
  assert.equal(isAcceptedState('missing'), false);
});

test('rejects placeholder implementations and missing capabilities', () => {
  const errors = validateMatrix({
    manifests: { dashboard: manifests.dashboard },
    rows: [
      completeRow({
        feature: '',
        implementation: 'placeholder',
        read: 'missing',
        write: 'missing',
        browser: 'missing',
      }),
    ],
    fileExists: () => true,
  });

  assert.ok(errors.some((error) => error.includes('feature')));
  assert.ok(errors.some((error) => error.includes('placeholder')));
  assert.ok(errors.some((error) => error.includes('read=missing')));
  assert.ok(errors.some((error) => error.includes('browser=missing')));
});

test('requires every manifest route exactly once', () => {
  const duplicate = validateMatrix({
    manifests: { dashboard: manifests.dashboard },
    rows: [completeRow(), completeRow({ feature: 'duplicate' })],
    fileExists: () => true,
  });
  assert.ok(duplicate.some((error) => error.includes('appears 2 times')));

  const missing = validateMatrix({
    manifests: { dashboard: manifests.dashboard },
    rows: [],
    fileExists: () => true,
  });
  assert.ok(missing.some((error) => error.includes('is missing')));
});

test('rejects generic migration modules and missing referenced files', () => {
  const errors = validateMatrix({
    manifests: { dashboard: manifests.dashboard },
    rows: [
      completeRow({
        react_module: 'web/apps/dashboard/src/pages/migrated-dashboard-page.tsx',
      }),
    ],
    fileExists: () => false,
  });

  assert.ok(errors.some((error) => error.includes('generic migration page')));
  assert.ok(errors.some((error) => error.includes('does not exist')));
});

test('accepts complete rows across applications', () => {
  const rows = [
    completeRow(),
    completeRow({
      app: 'sidebar',
      feature: 'contact',
      routes: '/contact',
      legacy_sources: 'web/legacy/sidebar/src/views/contact/index.vue',
      react_module: 'web/apps/sidebar/src/features/contact/page.tsx',
      go_apis: 'GET /sidebar/workContact/detail',
    }),
    completeRow({
      app: 'operation',
      feature: 'lottery',
      routes: '/lottery',
      legacy_sources: 'web/legacy/operation/src/views/lottery/index.vue',
      react_module: 'web/apps/operation/src/features/lottery/page.tsx',
      go_apis: 'GET /operation/lottery/openUserInfo',
    }),
  ];

  assert.deepEqual(validateMatrix({
    manifests,
    rows,
    fileExists: () => true,
  }), []);
});

test('production Phase 2.1 contract uses the unique current company-profile route', () => {
  const dashboardManifest = JSON.parse(readFileSync('web/apps/dashboard/src/migration-routes.json', 'utf8'));
  const matrix = parseCsv(readFileSync('docs/phases/phase-2.1-functional-frontend-migration/functional-matrix.csv', 'utf8'));
  const manifestPaths = new Set(dashboardManifest.map((route) => route.path));
  const matrixRoutes = new Set(matrix.flatMap((row) => String(row.routes).split(';')));

  assert.equal(manifestPaths.has('/company-setting/website'), true);
  assert.equal(matrixRoutes.has('/company-setting/website'), true);
  for (const stale of ['/corp/index', '/corpData/index']) {
    assert.equal(manifestPaths.has(stale), false);
    assert.equal(matrixRoutes.has(stale), false);
  }
});
