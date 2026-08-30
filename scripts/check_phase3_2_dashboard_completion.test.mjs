import assert from 'node:assert/strict';
import { mkdtempSync, readFileSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { basename, join } from 'node:path';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';

import {
  isRepositoryFile,
  validateCompletedPageSources,
  validatePhase32E2ECoverage,
  validateFinalPhase32Manifest,
  validateFunctionMatrix,
  validateNonBrowserPhase32Manifest,
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
const decisionRecordColumns = ['decisionReason', 'alternativeEntry', 'decisionVerification'];
const matrixHeader = `| ${matrixColumns.join(' | ')} |`;
const matrixDivider = `| ${matrixColumns.map(() => '---').join(' | ')} |`;

function matrixRow(path, overrides = {}, columns = matrixColumns) {
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
  return `| ${columns.map((column) => row[column] ?? '').join(' | ')} |`;
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

const phase32E2EContracts = [
  ['/index', 'query-export-refresh'],
  ['/chat/v2-all', 'filter-detail-refresh'],
  ['/ai-insight/v2/sensitive-word', 'create-word-refresh'],
  ['/customer/clue/default', 'create-lead-refresh'],
  ['/customer/contact', 'append-follow-up-refresh'],
  ['/customer/opportunity', 'advance-stage-refresh'],
  ['/customer/public-sea', 'claim-refresh'],
  ['/customer/tags', 'create-tag-refresh'],
];

function e2eContractSource(contracts = phase32E2EContracts) {
  return `const phase32PageContracts = [\n${contracts.map(([route, closure]) => `  { route: '${route}', closure: '${closure}' },`).join('\n')}\n] as const;`;
}

test('Phase 3.2 e2e contract requires the exact eight routes and key closures', () => {
  assert.deepEqual(validatePhase32E2ECoverage(e2eContractSource()), []);

  const missingRoute = validatePhase32E2ECoverage(e2eContractSource(phase32E2EContracts.slice(0, -1)));
  assert.ok(missingRoute.includes('missing Phase 3.2 e2e route: /customer/tags'));

  const wrongClosure = validatePhase32E2ECoverage(e2eContractSource(
    phase32E2EContracts.map(([route, closure]) => route === '/customer/contact'
      ? [route, 'view-only']
      : [route, closure]),
  ));
  assert.ok(wrongClosure.includes('invalid Phase 3.2 e2e closure: /customer/contact (received "view-only")'));

  const extraRoute = validatePhase32E2ECoverage(`${e2eContractSource()}\n{ route: '/customer/extra', closure: 'view-only' }`);
  assert.ok(extraRoute.includes('unexpected Phase 3.2 e2e route: /customer/extra'));
});

test('non-browser gate accepts integration evidence while final gate remains browser-blocked', () => {
  const manifest = completeManifest(Object.fromEntries(
    phase32Routes.map((path) => [path, { acceptance: 'integration-passed' }]),
  ));

  assert.doesNotThrow(() => validateNonBrowserPhase32Manifest(
    manifest,
    functionMatrix(),
    e2eContractSource(),
  ));
  assert.throws(
    () => validateFinalPhase32Manifest(manifest, functionMatrix()),
    /Phase 3\.2 incomplete routes \(0\/8\)/,
  );
});

test('Phase 3.2 browser assertions follow the current reachable page semantics', () => {
  const source = readFileSync(new URL('../web/e2e/tests/phase3-2-dashboard.spec.ts', import.meta.url), 'utf8');

  assert.match(source, /getByLabel\('开始日期'\)\.fill\('2026-07-01'\)/);
  assert.match(source, /getByLabel\('结束日期'\)\.fill\('2026-08-01'\)/);
  assert.match(source, /getByRole\('region', \{ name: '查询概览' \}\)/);
  assert.match(source, /getByRole\('heading', \{ name: '敏感词', exact: true \}\)/);
  assert.match(source, /getByLabel\('新敏感词'\)\.fill\('账号密码'\)/);
  assert.match(source, /getByLabel\('新词所属词组'\)\.selectOption\('1'\)/);
  assert.match(source, /getByRole\('button', \{ name: '保存敏感词' \}\)\.click\(\)/);
  assert.doesNotMatch(source, /高级范围筛选|heading'.*全局消息|敏感词管理|敏感词名称|敏感词分组/);
});

test('package gate validates integration evidence before running the full browser suite', () => {
  const packageJSON = JSON.parse(readFileSync(new URL('../package.json', import.meta.url), 'utf8'));
  const command = packageJSON.scripts['check:phase3-2-dashboard'];

  assert.match(command, /check_phase3_2_dashboard_completion\.mjs --non-browser/);
  assert.doesNotMatch(command, /check_phase3_2_dashboard_completion\.mjs --final/);
  assert.ok(
    command.indexOf('--non-browser') < command.indexOf('playwright test tests/phase3-2-dashboard.spec.ts'),
    'the browser suite must run after the non-browser contract gate',
  );
});

test('non-browser gate rejects pages without integration evidence', () => {
  const manifest = completeManifest(Object.fromEntries(
    phase32Routes.map((path) => [path, { acceptance: 'integration-passed' }]),
  ));
  manifest.pages[0].acceptance = 'unit-passed';

  assert.throws(
    () => validateNonBrowserPhase32Manifest(manifest, functionMatrix(), e2eContractSource()),
    /Phase 3\.2 non-browser incomplete routes \(7\/8\): \/index/,
  );
});

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

test('function matrix requires the exact 13-column header without extra columns', () => {
  const extraColumns = [...matrixColumns, 'extraColumn'];
  const errors = validateFunctionMatrix([
    `| ${extraColumns.join(' | ')} |`,
    `| ${extraColumns.map(() => '---').join(' | ')} |`,
    ...phase32Routes.map((path) => matrixRow(path, {}, extraColumns)),
  ].join('\n'));

  assert.deepEqual(errors, [
    `function matrix header must exactly match: ${matrixColumns.join(' | ')}`,
  ]);
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

test('function matrix rejects placeholder decision records for merge and not-applicable decisions', () => {
  const placeholders = ['-', '—', 'N/A', '无', '不适用', 'TODO', 'TBD', 'pending', '待确认'];
  const validDecisionRecords = {
    decisionReason: 'reference feature is intentionally consolidated',
    alternativeEntry: 'CustomerFilterPanel',
    decisionVerification: 'scripts/check_phase3_2_dashboard_completion.test.mjs',
  };
  const rows = [];
  const expectedErrors = [];

  for (const [decisionIndex, decision] of [merged, notApplicable].entries()) {
    for (const [placeholderIndex, placeholder] of placeholders.entries()) {
      for (const [fieldIndex, field] of decisionRecordColumns.entries()) {
        const page = phase32Routes[(decisionIndex * placeholders.length * decisionRecordColumns.length
          + placeholderIndex * decisionRecordColumns.length + fieldIndex) % phase32Routes.length];
        const referenceFeature = `${decisionIndex}-${placeholderIndex}-${fieldIndex}`;
        rows.push(matrixRow(page, {
          referenceFeature,
          decision,
          ...validDecisionRecords,
          [field]: placeholder,
        }));
        expectedErrors.push(`${page} / ${referenceFeature}: ${field} is placeholder`);
      }
    }
  }

  const errors = validateFunctionMatrix(functionMatrix(rows));

  assert.deepEqual(
    errors.filter((error) => error.endsWith('is placeholder')),
    expectedErrors,
  );
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

test('function matrix rejects rows outside the eight Phase 3.2 target routes', () => {
  const errors = validateFunctionMatrix(functionMatrix([
    ...phase32Routes.map((path) => matrixRow(path)),
    matrixRow('/customer/not-a-phase32-page'),
  ]));

  assert.ok(errors.includes('unexpected Phase 3.2 function matrix page: /customer/not-a-phase32-page'));
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

test('manifest requires every target route to explicitly declare phase 3.2', () => {
  const manifest = completeManifest({ '/index': { phase: '3.1' } });

  assert.throws(
    () => validatePhase32Manifest(manifest, functionMatrix()),
    /Phase 3\.2 target route must have phase "3\.2": \/index \(received "3\.1"\)/,
  );
});

test('final gate requires a function matrix instead of accepting its default empty value', () => {
  assert.throws(
    () => validateFinalPhase32Manifest(completeManifest()),
    /missing required Phase 3\.2 function matrix/,
  );
});

test('completed-page source gate accepts the actual eight real registrations', () => {
  assert.doesNotThrow(() => validateCompletedPageSources(completeManifest()));
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

test('completed-page source scan finds forbidden references in multiline target route registrations', () => {
  const manifest = completeManifest();
  const source = `
    import {
      sensitiveWordDemo,
    } from './demo-fixtures';
    const pages = {
      '/index': (
        <DemoPage
          title="overview"
        />
      ),
      '/chat/v2-all': (
        <PlaceholderPage
          title="chat"
        />
      ),
      '/ai-insight/v2/sensitive-word': (
        <SensitiveWordPage
          config={
            sensitiveWordDemo
          }
        />
      ),
    };
  `;

  const errors = validateCompletedPageSources(manifest, source);
  assert.ok(errors.includes('completed page frontend registration uses DemoPage: /index'));
  assert.ok(errors.includes('completed page frontend registration uses PlaceholderPage: /chat/v2-all'));
  assert.ok(errors.includes('completed page frontend registration uses demo-fixtures: /ai-insight/v2/sensitive-word'));
});

test('repository evidence paths must be relative ordinary files and resolve symlinks inside the repository', (t) => {
  const linkStem = `phase32-is-repository-file-${process.pid}`;
  const internalLink = join('scripts', `${linkStem}-internal`);
  const externalLink = join('scripts', `${linkStem}-external`);
  const externalDirectory = mkdtempSync(join(tmpdir(), 'phase32-is-repository-file-'));
  const externalFile = join(externalDirectory, 'outside.mjs');

  writeFileSync(externalFile, 'export {};\n');
  t.after(() => {
    rmSync(internalLink, { recursive: true, force: true });
    rmSync(externalLink, { recursive: true, force: true });
    rmSync(externalDirectory, { recursive: true, force: true });
  });
  symlinkSync(join(process.cwd(), 'scripts'), internalLink, 'junction');
  symlinkSync(externalDirectory, externalLink, 'junction');

  assert.equal(isRepositoryFile('scripts/check_phase3_2_dashboard_completion.test.mjs'), true);
  assert.equal(isRepositoryFile('scripts'), false);
  assert.equal(isRepositoryFile(fileURLToPath(import.meta.url)), false);
  assert.equal(isRepositoryFile('../package.json'), false);
  assert.equal(isRepositoryFile(join(internalLink, 'check_phase3_2_dashboard_completion.test.mjs')), true);
  assert.equal(isRepositoryFile(join(externalLink, basename(externalFile))), false);
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
