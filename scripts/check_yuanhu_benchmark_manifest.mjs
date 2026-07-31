import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

const allowedLevels = new Set(['P0', 'P1', 'P2']);
const requiredGroupIds = [
  'conversation',
  'risk-warning',
  'ai-insight',
  'marketing-tools',
  'scrm',
  'data-reports',
  'ai-settings',
  'company-settings',
];
const requiredPageFields = ['path', 'title', 'groupId', 'level', 'status', 'screenshotVersion'];
const allowedImplementations = new Set(['placeholder', 'demo', 'legacy-adapter', 'native']);
const allowedBackends = new Set(['missing', 'partial', 'ready']);
const allowedAcceptances = new Set(['not-started', 'unit-passed', 'integration-passed', 'e2e-passed']);
const allowedPhases = new Set(['3.1', '3.2', '3.3', '3.4', '3.5', '3.6']);
const allowedRisks = new Set(['low', 'medium', 'high']);

function isNonEmptyString(value) {
  return typeof value === 'string' && value.trim().length > 0;
}

export function validateManifest(manifest) {
  const errors = [];
  if (!manifest || typeof manifest !== 'object') {
    throw new Error('manifest must be an object');
  }
  if (!Array.isArray(manifest.groups)) errors.push('groups must be an array');
  if (!Array.isArray(manifest.pages)) errors.push('pages must be an array');
  if (errors.length > 0) throw new Error(errors.join('\n'));

  const groupIds = new Set();
  for (const group of manifest.groups) {
    if (!isNonEmptyString(group?.id)) {
      errors.push('group missing id');
      continue;
    }
    if (groupIds.has(group.id)) errors.push(`duplicate group id: ${group.id}`);
    groupIds.add(group.id);
    if (!isNonEmptyString(group.title)) errors.push(`group ${group.id} missing title`);
  }
  for (const groupId of requiredGroupIds) {
    if (!groupIds.has(groupId)) errors.push(`missing required group: ${groupId}`);
  }
  for (const groupId of groupIds) {
    if (!requiredGroupIds.includes(groupId)) errors.push(`unexpected group: ${groupId}`);
  }

  const pagePaths = new Set();
  for (const [index, page] of manifest.pages.entries()) {
    const label = `page ${index + 1}`;
    for (const field of requiredPageFields) {
      if (field === 'path') continue;
      if (field === 'groupId' && page?.path === '/index' && page?.groupId === null) continue;
      if (!isNonEmptyString(page?.[field])) errors.push(`${label} missing ${field}`);
    }
    if (!isNonEmptyString(page?.path)) {
      errors.push(`${label} missing path`);
      continue;
    }
    if (!page.path.startsWith('/')) errors.push(`${label} has invalid path: ${page.path}`);
    if (pagePaths.has(page.path)) errors.push(`duplicate page path: ${page.path}`);
    pagePaths.add(page.path);
    if (!allowedLevels.has(page.level)) errors.push(`${label} has invalid level: ${page.level}`);
    if (page.path === '/index' && page.groupId !== null) {
      errors.push('index page must have groupId: null');
    }
    if (page.path !== '/index' && !groupIds.has(page.groupId)) {
      errors.push(`${label} references missing group: ${page.groupId}`);
    }
    if (!allowedImplementations.has(page.implementation)) errors.push(`${label} has invalid implementation: ${page.implementation}`);
    if (!allowedBackends.has(page.backend)) errors.push(`${label} has invalid backend: ${page.backend}`);
    if (!allowedAcceptances.has(page.acceptance)) errors.push(`${label} has invalid acceptance: ${page.acceptance}`);
    if (!allowedPhases.has(page.phase)) errors.push(`${label} has invalid phase: ${page.phase}`);
    if (!isNonEmptyString(page.owner)) errors.push(`${label} missing owner`);
    if (!allowedRisks.has(page.risk)) errors.push(`${label} has invalid risk: ${page.risk}`);
    if (!Array.isArray(page.legacyRoutes)) errors.push(`${label} legacyRoutes must be an array`);
    if (!page.evidence || !isNonEmptyString(page.evidence.spec) || !isNonEmptyString(page.evidence.acceptance)) {
      errors.push(`${label} missing evidence paths`);
    }
  }
  if (!pagePaths.has('/index')) errors.push('missing required page: /index');
  if (errors.length > 0) throw new Error(errors.join('\n'));
}

export async function readManifest(manifestUrl = new URL('../web/apps/dashboard/src/benchmark/manifest.json', import.meta.url)) {
  return JSON.parse(await readFile(manifestUrl, 'utf8'));
}

async function main() {
  const manifest = await readManifest();
  validateManifest(manifest);
  console.log(`Yuanhu benchmark manifest passed (${manifest.pages.length} pages).`);
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  main().catch((error) => {
    console.error(`Yuanhu benchmark manifest failed: ${error.message}`);
    process.exitCode = 1;
  });
}
