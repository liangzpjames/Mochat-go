import { realpathSync, readFileSync, statSync } from 'node:fs';
import { readFile } from 'node:fs/promises';
import { isAbsolute, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

export const phase32TargetRoutes = [
  '/index',
  '/chat/v2-all',
  '/ai-insight/v2/sensitive-word',
  '/customer/clue/default',
  '/customer/contact',
  '/customer/opportunity',
  '/customer/public-sea',
  '/customer/tags',
];

const repositoryRoot = fileURLToPath(new URL('../', import.meta.url));
const completedImplementations = new Set(['native', 'legacy-adapter']);
const requiredEvidenceKeys = ['spec', 'acceptance'];
const nonBrowserAcceptances = new Set(['integration-passed', 'e2e-passed']);
const expectedE2EClosures = new Map([
  ['/index', 'query-export-refresh'],
  ['/chat/v2-all', 'filter-detail-refresh'],
  ['/ai-insight/v2/sensitive-word', 'create-word-refresh'],
  ['/customer/clue/default', 'create-lead-refresh'],
  ['/customer/contact', 'append-follow-up-refresh'],
  ['/customer/opportunity', 'advance-stage-refresh'],
  ['/customer/public-sea', 'claim-refresh'],
  ['/customer/tags', 'create-tag-refresh'],
]);
const allowedDecisions = new Set(['\u5df2\u5bf9\u5e94', '\u5408\u7406\u5408\u5e76', '\u4e0d\u9002\u7528']);
const functionMatrixColumns = [
  'page',
  'referenceFeature',
  'decision',
  'decisionReason',
  'alternativeEntry',
  'decisionVerification',
  'mochatEntry',
  'frontend',
  'api',
  'permission',
  'persistence',
  'tests',
  'evidence',
];
const requiredClosureColumns = ['mochatEntry', 'frontend', 'api', 'permission', 'persistence', 'tests', 'evidence'];
const decisionRecordColumns = ['decisionReason', 'alternativeEntry', 'decisionVerification'];
const pendingPattern = /(?:\b(?:todo|tbd|pending|unfinished|not[\s-]*started)\b|\u5f85|\u672a\u5b8c\u6210)/iu;
const decisionRecordPlaceholderPattern = /^(?:-|—|n\/a|\u65e0|\u4e0d\u9002\u7528)$/iu;

function tableCells(line) {
  return line.trim().replace(/^\||\|$/g, '').split('|').map((cell) => cell.trim());
}

function isTableDivider(line) {
  return /^\s*\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)+\|?\s*$/.test(line);
}

function parseFunctionMatrix(markdown) {
  const lines = markdown.split(/\r?\n/);
  const headerIndex = lines.findIndex((line) => {
    const cells = tableCells(line);
    return cells.includes('page') && cells.includes('referenceFeature') && cells.includes('decision');
  });
  if (headerIndex === -1) return { columns: [], rows: [], errors: [] };

  const columns = tableCells(lines[headerIndex]);
  const rows = [];
  const errors = [];
  for (let index = headerIndex + 1; index < lines.length; index += 1) {
    const line = lines[index];
    if (!line.trim().startsWith('|')) {
      if (rows.length > 0 || errors.length > 0) break;
      continue;
    }
    if (isTableDivider(line)) continue;
    const cells = tableCells(line);
    if (cells.length !== columns.length) {
      errors.push(`function matrix row ${index + 1} has ${cells.length} columns; expected ${columns.length}`);
      continue;
    }
    rows.push({ line: index + 1, values: Object.fromEntries(columns.map((column, columnIndex) => [column, cells[columnIndex]])) });
  }
  return { columns, rows, errors };
}

function matrixLabel(row) {
  return `${row.values.page} / ${row.values.referenceFeature}`;
}

function isPending(value) {
  return pendingPattern.test(value);
}

function isDecisionRecordPlaceholder(value) {
  const normalizedValue = value.trim();
  return decisionRecordPlaceholderPattern.test(normalizedValue) || isPending(normalizedValue);
}

function isFixture(value) {
  return /fixture/i.test(value);
}

export function isRepositoryFile(value) {
  if (typeof value !== 'string' || !value || value.includes(';') || isAbsolute(value)) return false;
  const absolutePath = resolve(repositoryRoot, value);
  const pathFromRoot = relative(repositoryRoot, absolutePath);
  if (!pathFromRoot || pathFromRoot.startsWith('..') || isAbsolute(pathFromRoot)) return false;

  try {
    const resolvedPath = realpathSync(absolutePath);
    const resolvedPathFromRoot = relative(repositoryRoot, resolvedPath);
    return Boolean(resolvedPathFromRoot)
      && !resolvedPathFromRoot.startsWith('..')
      && !isAbsolute(resolvedPathFromRoot)
      && statSync(resolvedPath).isFile();
  } catch {
    return false;
  }
}

function unclosedMatrixItems(markdown) {
  const { columns, rows } = parseFunctionMatrix(markdown);
  if (functionMatrixColumns.some((column) => !columns.includes(column))) return [];
  return rows.flatMap((row) => requiredClosureColumns.flatMap((column) => {
    const value = row.values[column] ?? '';
    if (!value.trim()) return [`${matrixLabel(row)}: ${column} is empty`];
    if (isFixture(value)) return [`${matrixLabel(row)}: ${column} uses fixture`];
    if (isPending(value)) return [`${matrixLabel(row)}: ${column} is pending`];
    return [];
  }));
}

export function validateFunctionMatrix(markdown) {
  if (typeof markdown !== 'string') return ['missing required Phase 3.2 function matrix'];
  const { columns, rows, errors: parseErrors } = parseFunctionMatrix(markdown);
  const hasExactHeader = columns.length === functionMatrixColumns.length
    && columns.every((column, columnIndex) => column === functionMatrixColumns[columnIndex]);
  const errors = [...parseErrors];
  if (!hasExactHeader) {
    errors.push(`function matrix header must exactly match: ${functionMatrixColumns.join(' | ')}`);
  }
  const missingColumns = functionMatrixColumns.filter((column) => !columns.includes(column));
  if (missingColumns.length > 0) return [...errors, ...missingColumns.map((column) => `missing function matrix column: ${column}`)];

  const seenFeatures = new Set();
  for (const row of rows) {
    const label = matrixLabel(row);
    const featureKey = `${row.values.page}\u0000${row.values.referenceFeature}`;
    if (seenFeatures.has(featureKey)) errors.push(`duplicate function matrix page and referenceFeature: ${label}`);
    seenFeatures.add(featureKey);

    if (!phase32TargetRoutes.includes(row.values.page)) {
      errors.push(`unexpected Phase 3.2 function matrix page: ${row.values.page}`);
    }

    if (!allowedDecisions.has(row.values.decision)) {
      errors.push(`invalid function matrix decision: ${label}: ${row.values.decision}`);
    }
    for (const column of ['page', 'referenceFeature', 'decision', ...requiredClosureColumns]) {
      if (!row.values[column]) errors.push(`missing function matrix ${column}: ${label}`);
    }
    if (row.values.decision === '\u5408\u7406\u5408\u5e76' || row.values.decision === '\u4e0d\u9002\u7528') {
      for (const column of decisionRecordColumns) {
        if (!row.values[column]) errors.push(`missing function matrix ${column}: ${label}`);
        else if (isDecisionRecordPlaceholder(row.values[column])) errors.push(`${label}: ${column} is placeholder`);
      }
    }
    for (const column of requiredClosureColumns) {
      const value = row.values[column] ?? '';
      if (isFixture(value)) errors.push(`${label}: ${column} uses fixture`);
      if (isPending(value)) errors.push(`${label}: ${column} is pending`);
    }
    const isClosed = requiredClosureColumns.every((column) => {
      const value = row.values[column] ?? '';
      return value.trim() && !isFixture(value) && !isPending(value);
    });
    if (isClosed) {
      for (const column of ['tests', 'evidence']) {
        if (!isRepositoryFile(row.values[column])) {
          errors.push(`${label}: ${column} file does not exist: ${row.values[column]}`);
        }
      }
    }
  }

  for (const path of phase32TargetRoutes) {
    if (!rows.some((row) => row.values.page === path)) errors.push(`missing Phase 3.2 function matrix page: ${path}`);
  }
  return errors;
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function registrationExpressionEnd(source, expressionStart) {
  const openToClose = new Map([['(', ')'], ['[', ']'], ['{', '}']]);
  const closers = [];
  let quote = null;
  let escaped = false;
  for (let index = expressionStart; index < source.length; index += 1) {
    const character = source[index];
    const nextCharacter = source[index + 1];
    if (quote) {
      if (escaped) escaped = false;
      else if (character === '\\') escaped = true;
      else if (character === quote) quote = null;
      continue;
    }
    if (character === '/' && nextCharacter === '/') {
      const newline = source.indexOf('\n', index + 2);
      index = newline === -1 ? source.length : newline;
      continue;
    }
    if (character === '/' && nextCharacter === '*') {
      const commentEnd = source.indexOf('*/', index + 2);
      index = commentEnd === -1 ? source.length : commentEnd + 1;
      continue;
    }
    if (character === '\'' || character === '"' || character === '`') {
      quote = character;
      continue;
    }
    if (openToClose.has(character)) {
      closers.push(openToClose.get(character));
      continue;
    }
    if (character === closers.at(-1)) {
      closers.pop();
      continue;
    }
    if (closers.length === 0 && (character === ',' || character === '}')) return index;
  }
  return source.length;
}

function pageRegistrationBlocks(source, path) {
  const routePattern = new RegExp(`(['"])${escapeRegExp(path)}\\1\\s*:`, 'g');
  const blocks = [];
  for (const match of source.matchAll(routePattern)) {
    const expressionStart = match.index + match[0].length;
    blocks.push(source.slice(match.index, registrationExpressionEnd(source, expressionStart)));
  }
  return blocks;
}

export function validateCompletedPageSources(manifest, source = readFileSync(new URL('../web/apps/dashboard/src/benchmark/page-registry.tsx', import.meta.url), 'utf8')) {
  const fixtureSymbols = new Set();
  for (const fixtureImport of source.matchAll(/import\s+([^;]*?)\s+from\s*['"]\.\/demo-fixtures['"]/g)) {
    const bindings = fixtureImport[1];
    const namedBindings = bindings.match(/\{([\s\S]*?)\}/)?.[1] ?? '';
    for (const binding of namedBindings.split(',')) {
      const localName = binding.trim().replace(/^type\s+/, '').split(/\s+as\s+/).at(-1)?.trim();
      if (/^[A-Za-z_$][\w$]*$/.test(localName ?? '')) fixtureSymbols.add(localName);
    }
    const namespaceName = bindings.match(/\*\s+as\s+([A-Za-z_$][\w$]*)/)?.[1];
    if (namespaceName) fixtureSymbols.add(namespaceName);
  }
  const errors = new Set();
  for (const page of manifest?.pages ?? []) {
    if (!phase32TargetRoutes.includes(page.path) || page.backend !== 'ready' || !nonBrowserAcceptances.has(page.acceptance)) continue;
    const registrations = pageRegistrationBlocks(source, page.path);
    if (registrations.length === 0) {
      errors.add(`completed page missing frontend registration: ${page.path}`);
      continue;
    }
    for (const registration of registrations) {
      if (/\bDemoPage\b/.test(registration)) errors.add(`completed page frontend registration uses DemoPage: ${page.path}`);
      if (/\bPlaceholderPage\b/.test(registration)) errors.add(`completed page frontend registration uses PlaceholderPage: ${page.path}`);
      if ([...fixtureSymbols].some((symbol) => new RegExp(`\\b${symbol}\\b`).test(registration))) {
        errors.add(`completed page frontend registration uses demo-fixtures: ${page.path}`);
      }
    }
  }
  return [...errors];
}

export function validatePhase32Manifest(manifest, functionMatrixMarkdown) {
  const pages = manifest?.pages ?? [];
  const pagesByPath = new Map(pages.map((page) => [page.path, page]));
  const errors = [];
  const phasePages = pages.filter((page) => page?.phase === '3.2');
  const phaseRouteCounts = new Map();
  for (const page of phasePages) phaseRouteCounts.set(page.path, (phaseRouteCounts.get(page.path) ?? 0) + 1);
  for (const page of phasePages) {
    if (!phase32TargetRoutes.includes(page.path)) errors.push(`unexpected Phase 3.2 route: ${page.path}`);
  }
  for (const [path, count] of phaseRouteCounts) {
    if (count > 1) errors.push(`duplicate Phase 3.2 route: ${path}`);
  }

  for (const path of phase32TargetRoutes) {
    const page = pagesByPath.get(path);
    if (!page) {
      errors.push(`missing Phase 3.2 route: ${path}`);
      continue;
    }
    if (page.phase !== '3.2') {
      errors.push(`Phase 3.2 target route must have phase "3.2": ${path} (received ${JSON.stringify(page.phase)})`);
    }
    const isComplete = page.backend === 'ready' && page.acceptance === 'e2e-passed';
    if (isComplete && !completedImplementations.has(page.implementation)) {
      errors.push(`completed page must be native or legacy-adapter: ${path}`);
    }
    if (isComplete && requiredEvidenceKeys.some((key) => typeof page.evidence?.[key] !== 'string' || page.evidence[key].length === 0)) {
      errors.push(`completed page missing evidence: ${path}`);
    }
  }

  if (functionMatrixMarkdown !== undefined) {
    errors.push(...validateFunctionMatrix(functionMatrixMarkdown));
    const unclosedByPage = new Map();
    for (const item of unclosedMatrixItems(functionMatrixMarkdown)) {
      const [path] = item.split(' / ', 1);
      const existing = unclosedByPage.get(path) ?? [];
      existing.push(item);
      unclosedByPage.set(path, existing);
    }
    for (const path of phase32TargetRoutes) {
      if (pagesByPath.get(path)?.acceptance === 'e2e-passed') {
        for (const item of unclosedByPage.get(path) ?? []) errors.push(`e2e-passed page has unclosed function matrix items: ${item}`);
      }
    }
  }
  if (errors.length > 0) throw new Error(errors.join('\n'));
}

export function validatePhase32E2ECoverage(source) {
  const errors = [];
  const contracts = new Map();
  const routeCounts = new Map();
  const contractPattern = /\{\s*route:\s*['"]([^'"]+)['"]\s*,\s*closure:\s*['"]([^'"]+)['"]\s*\}/gu;
  for (const match of String(source ?? '').matchAll(contractPattern)) {
    const [, route, closure] = match;
    routeCounts.set(route, (routeCounts.get(route) ?? 0) + 1);
    contracts.set(route, closure);
    if (!phase32TargetRoutes.includes(route)) errors.push(`unexpected Phase 3.2 e2e route: ${route}`);
  }
  for (const [route, count] of routeCounts) {
    if (count > 1) errors.push(`duplicate Phase 3.2 e2e route: ${route}`);
  }
  for (const route of phase32TargetRoutes) {
    if (!contracts.has(route)) {
      errors.push(`missing Phase 3.2 e2e route: ${route}`);
      continue;
    }
    const closure = contracts.get(route);
    if (closure !== expectedE2EClosures.get(route)) {
      errors.push(`invalid Phase 3.2 e2e closure: ${route} (received ${JSON.stringify(closure)})`);
    }
  }
  return errors;
}

export function validateNonBrowserPhase32Manifest(manifest, functionMatrixMarkdown, e2eSource) {
  if (typeof functionMatrixMarkdown !== 'string' || !functionMatrixMarkdown.trim()) {
    throw new Error('missing required Phase 3.2 function matrix');
  }
  validatePhase32Manifest(manifest, functionMatrixMarkdown);
  const pagesByPath = new Map((manifest?.pages ?? []).map((page) => [page.path, page]));
  const errors = [
    ...validatePhase32E2ECoverage(e2eSource),
    ...validateCompletedPageSources(manifest),
    ...unclosedMatrixItems(functionMatrixMarkdown).map((item) => `Phase 3.2 unclosed function matrix item: ${item}`),
  ];
  const incomplete = phase32TargetRoutes.filter((path) => {
    const page = pagesByPath.get(path);
    return page?.backend !== 'ready'
      || !nonBrowserAcceptances.has(page?.acceptance)
      || !completedImplementations.has(page?.implementation);
  });
  if (incomplete.length) {
    errors.push(`Phase 3.2 non-browser incomplete routes (${phase32TargetRoutes.length - incomplete.length}/${phase32TargetRoutes.length}): ${incomplete.join(', ')}`);
  }
  for (const path of phase32TargetRoutes) {
    const page = pagesByPath.get(path);
    if (!nonBrowserAcceptances.has(page?.acceptance)) continue;
    for (const key of requiredEvidenceKeys) {
      const evidencePath = page?.evidence?.[key];
      if (!isRepositoryFile(evidencePath)) {
        errors.push(`Phase 3.2 non-browser evidence file does not exist: ${path} / ${key}: ${evidencePath ?? ''}`);
      }
    }
  }
  if (errors.length > 0) throw new Error(errors.join('\n'));
}

export function validateFinalPhase32Manifest(manifest, functionMatrixMarkdown) {
  if (typeof functionMatrixMarkdown !== 'string' || !functionMatrixMarkdown.trim()) {
    throw new Error('missing required Phase 3.2 function matrix');
  }
  validatePhase32Manifest(manifest, functionMatrixMarkdown);
  const pagesByPath = new Map((manifest?.pages ?? []).map((page) => [page.path, page]));
  const errors = [
    ...validateCompletedPageSources(manifest),
    ...unclosedMatrixItems(functionMatrixMarkdown).map((item) => `Phase 3.2 unclosed function matrix item: ${item}`),
  ];
  const incomplete = phase32TargetRoutes.filter((path) => {
    const page = pagesByPath.get(path);
    return page?.backend !== 'ready' || page?.acceptance !== 'e2e-passed' || !completedImplementations.has(page?.implementation);
  });
  if (incomplete.length) errors.push(`Phase 3.2 incomplete routes (${phase32TargetRoutes.length - incomplete.length}/${phase32TargetRoutes.length}): ${incomplete.join(', ')}`);
  if (errors.length > 0) throw new Error(errors.join('\n'));
}

export async function readManifest(manifestUrl = new URL('../web/apps/dashboard/src/benchmark/manifest.json', import.meta.url)) {
  return JSON.parse(await readFile(manifestUrl, 'utf8'));
}

export async function readFunctionMatrix(matrixUrl = new URL('../docs/phase/phase-3.2-dashboard-completion/reports/phase3.2-function-matrix.md', import.meta.url)) {
  return readFile(matrixUrl, 'utf8');
}

async function main() {
  const manifest = await readManifest();
  const functionMatrix = await readFunctionMatrix();
  if (process.argv.includes('--non-browser')) {
    const e2eSource = readFileSync(new URL('../web/e2e/tests/phase3-2-dashboard.spec.ts', import.meta.url), 'utf8');
    validateNonBrowserPhase32Manifest(manifest, functionMatrix, e2eSource);
    console.log(`8/8 Phase 3.2 routes passed the non-browser gate; browser acceptance remains separate.`);
    return;
  }
  validateFinalPhase32Manifest(manifest, functionMatrix);
  console.log(`8/8 Phase 3.2 routes complete.`);
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  main().catch((error) => {
    console.error(`Phase 3.2 dashboard manifest failed: ${error.message}`);
    process.exitCode = 1;
  });
}
