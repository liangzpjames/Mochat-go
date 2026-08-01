import { readFile } from 'node:fs/promises';
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

const completedImplementations = new Set(['native', 'legacy-adapter']);
const requiredEvidenceKeys = ['spec', 'acceptance'];
const allowedDecisions = new Set(['已对应', '合理合并', '不适用']);
const functionMatrixColumns = [
  'page',
  'referenceFeature',
  'decision',
  'mochatEntry',
  'frontend',
  'api',
  'permission',
  'persistence',
  'tests',
  'evidence',
];
const requiredClosureColumns = ['mochatEntry', 'frontend', 'api', 'permission', 'persistence', 'tests', 'evidence'];

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

  if (headerIndex === -1) return { columns: [], rows: [] };

  const columns = tableCells(lines[headerIndex]);
  const rows = [];
  for (let index = headerIndex + 1; index < lines.length; index += 1) {
    const line = lines[index];
    if (!line.trim().startsWith('|')) {
      if (rows.length > 0) break;
      continue;
    }
    if (isTableDivider(line)) continue;
    const cells = tableCells(line);
    if (cells.length !== columns.length) continue;
    rows.push(Object.fromEntries(columns.map((column, columnIndex) => [column, cells[columnIndex]])));
  }
  return { columns, rows };
}

function unclosedMatrixItems(markdown) {
  const { columns, rows } = parseFunctionMatrix(markdown);
  if (functionMatrixColumns.some((column) => !columns.includes(column))) return [];

  return rows.flatMap((row) => requiredClosureColumns.flatMap((column) => {
    const value = row[column] ?? '';
    if (!value.trim()) return [`${row.page} / ${row.referenceFeature}: ${column} is empty`];
    if (/fixture/i.test(value)) return [`${row.page} / ${row.referenceFeature}: ${column} uses fixture`];
    if (/(^|[：:\s])待|pending|not-started/i.test(value)) return [`${row.page} / ${row.referenceFeature}: ${column} is pending`];
    return [];
  }));
}

export function validateFunctionMatrix(markdown) {
  const { columns, rows } = parseFunctionMatrix(markdown);
  const missingColumns = functionMatrixColumns.filter((column) => !columns.includes(column));
  if (missingColumns.length > 0) return missingColumns.map((column) => `missing function matrix column: ${column}`);

  const errors = [];
  for (const row of rows) {
    const label = `${row.page} / ${row.referenceFeature}`;
    if (!allowedDecisions.has(row.decision)) {
      errors.push(`invalid function matrix decision: ${label}: ${row.decision}`);
      continue;
    }
    for (const column of ['page', 'referenceFeature', 'mochatEntry', 'frontend', 'api', 'permission', 'persistence', 'tests', 'evidence']) {
      if (!row[column]) errors.push(`missing function matrix ${column}: ${label}`);
    }
  }

  for (const path of phase32TargetRoutes) {
    if (!rows.some((row) => row.page === path)) errors.push(`missing Phase 3.2 function matrix page: ${path}`);
  }
  return errors;
}

export function validatePhase32Manifest(manifest, functionMatrixMarkdown = '') {
  const pages = new Map((manifest?.pages ?? []).map((page) => [page.path, page]));
  const errors = [];

  for (const path of phase32TargetRoutes) {
    const page = pages.get(path);
    if (!page) {
      errors.push(`missing Phase 3.2 route: ${path}`);
      continue;
    }
    const isComplete = page.backend === 'ready' && page.acceptance === 'e2e-passed';
    if (isComplete && !completedImplementations.has(page.implementation)) {
      errors.push(`completed page must be native or legacy-adapter: ${path}`);
    }
    if (isComplete && requiredEvidenceKeys.some((key) => typeof page.evidence?.[key] !== 'string' || page.evidence[key].length === 0)) {
      errors.push(`completed page missing evidence: ${path}`);
    }
  }

  if (functionMatrixMarkdown) {
    errors.push(...validateFunctionMatrix(functionMatrixMarkdown));
    const unclosedByPage = new Map();
    for (const item of unclosedMatrixItems(functionMatrixMarkdown)) {
      const [path] = item.split(' / ', 1);
      const existing = unclosedByPage.get(path) ?? [];
      existing.push(item);
      unclosedByPage.set(path, existing);
    }
    for (const path of phase32TargetRoutes) {
      if (pages.get(path)?.acceptance === 'e2e-passed') {
        for (const item of unclosedByPage.get(path) ?? []) {
          errors.push(`e2e-passed page has unclosed function matrix items: ${item}`);
        }
      }
    }
  }

  if (errors.length > 0) throw new Error(errors.join('\n'));
}

export function validateFinalPhase32Manifest(manifest, functionMatrixMarkdown = '') {
  validatePhase32Manifest(manifest, functionMatrixMarkdown);
  const pages = new Map((manifest?.pages ?? []).map((page) => [page.path, page]));
  const unclosed = unclosedMatrixItems(functionMatrixMarkdown);
  if (unclosed.length) throw new Error(`Phase 3.2 unclosed function matrix items (${unclosed.length}): ${unclosed.join(', ')}`);
  const incomplete = phase32TargetRoutes.filter((path) => {
    const page = pages.get(path);
    return page?.backend !== 'ready' || page?.acceptance !== 'e2e-passed' || !completedImplementations.has(page?.implementation);
  });
  if (incomplete.length) throw new Error(`Phase 3.2 incomplete routes (${phase32TargetRoutes.length - incomplete.length}/${phase32TargetRoutes.length}): ${incomplete.join(', ')}`);
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
  validateFinalPhase32Manifest(manifest, functionMatrix);
  console.log(`8/8 Phase 3.2 routes complete.`);
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  main().catch((error) => {
    console.error(`Phase 3.2 dashboard manifest failed: ${error.message}`);
    process.exitCode = 1;
  });
}
