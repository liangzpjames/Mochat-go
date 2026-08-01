import assert from 'node:assert/strict';
import { test } from 'node:test';

import {
  validateFinalPhase32Manifest,
  validateFunctionMatrix,
  validatePhase32Manifest,
} from './check_phase3_2_dashboard_completion.mjs';

const phase32Routes = [
  '/index',
  '/chat/v2-all',
  '/ai-insight/v2/sensitive-word',
  '/customer/clue/default',
  '/customer/contact',
  '/customer/opportunity',
  '/customer/public-sea',
  '/customer/tags',
];

const matrixHeader = '| page | referenceFeature | decision | mochatEntry | frontend | api | permission | persistence | tests | evidence |';
const matrixDivider = '| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |';

function matrixRow(path, overrides = {}) {
  const row = {
    page: path,
    referenceFeature: '筛选客户',
    decision: '已对应',
    mochatEntry: '客户列表筛选区',
    frontend: 'CustomerFilterPanel',
    api: 'GET /api/customer',
    permission: 'customer:read',
    persistence: 'customer table',
    tests: 'customer-filter.test.ts',
    evidence: 'evidence/customer-filter.png',
    ...overrides,
  };
  return `| ${Object.values(row).join(' | ')} |`;
}

function functionMatrix(rows = phase32Routes.map((path) => matrixRow(path))) {
  return [matrixHeader, matrixDivider, ...rows].join('\n');
}

test('function matrix reports missing required columns exactly', () => {
  const markdown = '| page | referenceFeature | decision | mochatEntry | frontend | api | permission | persistence | tests |\n| --- | --- | --- | --- | --- | --- | --- | --- | --- |\n| /index | 概览 | 已对应 | 数据概览 | Dashboard | GET /api/index | index:read | dashboard | index.test |';

  assert.deepEqual(validateFunctionMatrix(markdown), ['missing function matrix column: evidence']);
});

test('function matrix rejects illegal decisions and empty merged evidence', () => {
  const markdown = functionMatrix([
    matrixRow('/index', { decision: '已完成' }),
    matrixRow('/chat/v2-all', { decision: '合理合并', evidence: '' }),
    ...phase32Routes.slice(2).map((path) => matrixRow(path)),
  ]);

  assert.deepEqual(validateFunctionMatrix(markdown), [
    'invalid function matrix decision: /index / 筛选客户: 已完成',
    'missing function matrix evidence: /chat/v2-all / 筛选客户',
  ]);
});

test('function matrix reports every missing Phase 3.2 page', () => {
  const markdown = functionMatrix([matrixRow('/index')]);

  assert.deepEqual(validateFunctionMatrix(markdown), phase32Routes.slice(1).map((path) => `missing Phase 3.2 function matrix page: ${path}`));
});

test('completed page cannot keep a fixture-backed function matrix row', () => {
  const manifest = {
    pages: phase32Routes.map((path) => ({
      path,
      implementation: 'native',
      backend: 'ready',
      acceptance: 'e2e-passed',
      evidence: { spec: 'docs/spec.md', acceptance: 'docs/acceptance.md' },
    })),
  };
  const markdown = functionMatrix([
    matrixRow('/index', { frontend: 'fixture: dashboard-summary.json' }),
    ...phase32Routes.slice(1).map((path) => matrixRow(path)),
  ]);

  assert.throws(
    () => validatePhase32Manifest(manifest, markdown),
    /e2e-passed page has unclosed function matrix items: \/index \/ 筛选客户: frontend uses fixture/,
  );
});

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

test('final Phase 3.2 gate rejects a route without browser acceptance', () => {
  const manifest = { pages: [{ path: '/index', implementation: 'native', backend: 'ready', acceptance: 'unit-passed', evidence: { spec: 'a', acceptance: 'b' } }] };
  assert.throws(() => validateFinalPhase32Manifest(manifest), /missing Phase 3\.2 route/);
});
