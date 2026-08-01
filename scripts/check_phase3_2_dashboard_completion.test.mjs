import assert from 'node:assert/strict';
import { test } from 'node:test';

import {
  validateCompletedPageSources,
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
const mapped = '\u5df2\u5bf9\u5e94';
const merged = '\u5408\u7406\u5408\u5e76';
const notApplicable = '\u4e0d\u9002\u7528';
const matrixColumns = [
  'page', 'referenceFeature', 'decision', 'decisionReason', 'alternativeEntry',
  'decisionVerification', 'mochatEntry', 'frontend', 'api', 'permission',
  'persistence', 'tests', 'evidence',
];
const matrixHeader = `| ${matrixColumns.join(' | ')} |`;
const matrixDivider = `| ${matrixColumns.map(() => '---').join(' | ')} |`;

function matrixRow(path, overrides = {}) {
  const row = {
    page: path,
    referenceFeature: 'filter customers',
    decision: mapped,
    decisionReason: '-',
    alternativeEntry: '-',
    decisionVerification: '-',
    mochatEntry: 'CustomerFilterPanel',
    frontend: 'web/apps/dashboard/src/benchmark/page-registry.tsx',
    api: 'GET /api/customer',
    permission: 'customer:read',
    persistence: 'customer table',
    tests: 'scripts/check_phase3_2_dashboard_completion.test.mjs',
    evidence: 'scripts/check_phase3_2_dashboard_completion.mjs',
    ...overrides,
  };
  return `| ${matrixColumns.map((column) => row[column]).join(' | ')} |`;
}

function functionMatrix(rows = phase32Routes.map((path) => matrixRow(path))) {
  return [matrixHeader, matrixDivider, ...rows].join('\n');
}

function completeManifest(overrides = {}) {
  return {
    pages: phase32Routes.map((path) => ({
      path,
      implementation: path === '/ai-insight/v2/sensitive-word' || path === '/customer/contact' ? 'legacy-adapter' : 'native',
      backend: 'ready',
      acceptance: 'e2e-passed',
      phase: '3.2',
      evidence: { spec: 'scripts/check_phase3_2_dashboard_completion.mjs', acceptance: 'scripts/check_phase3_2_dashboard_completion.test.mjs' },
      ...overrides[path],
    })),
  };
}

test('function matrix reports a malformed Markdown row column count', () => {
  const markdown = [
    matrixHeader,
    matrixDivider,
    '| /index | overview | mapped | - | - | - | entry | frontend | api | permission | persistence | tests |',
  ].join('\n');

  assert.ok(
    validateFunctionMatrix(markdown).includes('function matrix row 3 has 12 columns; expected 13'),
  );
});

test('function matrix rejects illegal decisions and requires merge decision records', () => {
  const errors = validateFunctionMatrix(functionMatrix([
    matrixRow('/index', { decision: 'complete' }),
    matrixRow('/chat/v2-all', {
      decision: merged,
      decisionReason: '',
      alternativeEntry: '',
      decisionVerification: '',
    }),
    ...phase32Routes.slice(2).map((path) => matrixRow(path)),
  ]));

  assert.ok(errors.includes('invalid function matrix decision: /index / filter customers: complete'));
  assert.ok(errors.includes('missing function matrix decisionReason: /chat/v2-all / filter customers'));
  assert.ok(errors.includes('missing function matrix alternativeEntry: /chat/v2-all / filter customers'));
  assert.ok(errors.includes('missing function matrix decisionVerification: /chat/v2-all / filter customers'));
});

test('function matrix rejects duplicate page and reference feature rows', () => {
  const errors = validateFunctionMatrix(functionMatrix([
    matrixRow('/index'),
    matrixRow('/index'),
    ...phase32Routes.slice(1).map((path) => matrixRow(path)),
  ]));

  assert.ok(errors.includes('duplicate function matrix page and referenceFeature: /index / filter customers'));
});

test('function matrix reports every missing Phase 3.2 page', () => {
  const errors = validateFunctionMatrix(functionMatrix([matrixRow('/index')]));

  assert.deepEqual(
    errors,
    phase32Routes.slice(1).map((path) => `missing Phase 3.2 function matrix page: ${path}`),
  );
});

test('function matrix rejects fixture-backed closure fields', () => {
  const errors = validateFunctionMatrix(functionMatrix([
    matrixRow('/index', { frontend: 'fixture: dashboard-summary.json' }),
    ...phase32Routes.slice(1).map((path) => matrixRow(path)),
  ]));

  assert.ok(errors.includes('/index / filter customers: frontend uses fixture'));
});

test('function matrix treats TODO, TBD, unfinished, and not started values as unclosed', () => {
  const errors = validateFunctionMatrix(functionMatrix([
    matrixRow('/index', { frontend: 'TODO: implement overview' }),
    matrixRow('/chat/v2-all', { api: 'TBD' }),
    matrixRow('/ai-insight/v2/sensitive-word', { permission: 'unfinished' }),
    matrixRow('/customer/clue/default', { persistence: 'not started' }),
    ...phase32Routes.slice(4).map((path) => matrixRow(path)),
  ]));

  assert.ok(errors.includes('/index / filter customers: frontend is pending'));
  assert.ok(errors.includes('/chat/v2-all / filter customers: api is pending'));
  assert.ok(errors.includes('/ai-insight/v2/sensitive-word / filter customers: permission is pending'));
  assert.ok(errors.includes('/customer/clue/default / filter customers: persistence is pending'));
});

test('closed function matrix items require existing repository tests and evidence files', () => {
  const errors = validateFunctionMatrix(functionMatrix([
    matrixRow('/index', { tests: 'missing-test.mjs', evidence: 'missing-evidence.md' }),
    ...phase32Routes.slice(1).map((path) => matrixRow(path)),
  ]));

  assert.ok(errors.includes('/index / filter customers: tests file does not exist: missing-test.mjs'));
  assert.ok(errors.includes('/index / filter customers: evidence file does not exist: missing-evidence.md'));
});

test('manifest rejects extra and duplicate Phase 3.2 routes', () => {
  const manifest = completeManifest();
  manifest.pages.push(
    { ...manifest.pages[0], path: '/extra', phase: '3.2' },
    { ...manifest.pages[1] },
  );

  assert.throws(
    () => validatePhase32Manifest(manifest, functionMatrix()),
    /unexpected Phase 3\.2 route: \/extra[\s\S]*duplicate Phase 3\.2 route: \/chat\/v2-all/,
  );
});

test('final gate requires a function matrix instead of accepting its default empty value', () => {
  assert.throws(
    () => validateFinalPhase32Manifest(completeManifest()),
    /missing required Phase 3\.2 function matrix/,
  );
});

test('completed-page source gate rejects actual target registrations backed by DemoPage', () => {
  assert.throws(
    () => validateFinalPhase32Manifest(completeManifest(), functionMatrix()),
    /completed page frontend registration uses DemoPage: \/customer\/contact/,
  );
});

test('completed-page source scan rejects PlaceholderPage and demo-fixtures for target routes', () => {
  const manifest = completeManifest();
  const source = `
    import { sensitiveWordDemo } from './demo-fixtures';
    const pages = {
      '/index': <PlaceholderPage title="overview" />,
      '/ai-insight/v2/sensitive-word': <SensitiveWordPage config={sensitiveWordDemo} />,
    };
  `;

  const errors = validateCompletedPageSources(manifest, source);
  assert.ok(errors.includes('completed page frontend registration uses PlaceholderPage: /index'));
  assert.ok(errors.includes('completed page frontend registration uses demo-fixtures: /ai-insight/v2/sensitive-word'));
});

test('final gate reports a unit-passed page as the missing browser acceptance after all eight routes are present', () => {
  const manifest = completeManifest({ '/index': { acceptance: 'unit-passed' } });

  assert.throws(
    () => validateFinalPhase32Manifest(manifest, functionMatrix()),
    /Phase 3\.2 incomplete routes \(7\/8\): \/index/,
  );
});

test('function matrix accepts every required target page and the documented not-applicable decision fields', () => {
  const errors = validateFunctionMatrix(functionMatrix([
    matrixRow('/index', {
      decision: notApplicable,
      decisionReason: 'not in reference product',
      alternativeEntry: 'CustomerFilterPanel',
      decisionVerification: 'scripts/check_phase3_2_dashboard_completion.test.mjs',
    }),
    ...phase32Routes.slice(1).map((path) => matrixRow(path)),
  ]));

  assert.deepEqual(errors, []);
});
