import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

export const phase35TargetRoutes = [
  '/customer/friends',
  '/customer/group',
  '/customer/order',
  '/customer/settings',
  '/data/customer',
  '/data/employee',
  '/data/conversion',
  '/data/behavior',
  '/data/report',
];

const completedImplementations = new Set(['native', 'legacy-adapter']);
const completedAcceptances = new Set(['integration-passed', 'e2e-passed']);

export function validatePhase35Manifest(manifest) {
  const pages = manifest?.pages ?? [];
  const phasePages = pages.filter((page) => page?.phase === '3.5');
  const errors = [];
  const counts = new Map();

  for (const page of phasePages) {
    counts.set(page.path, (counts.get(page.path) ?? 0) + 1);
    if (!phase35TargetRoutes.includes(page.path)) errors.push(`unexpected Phase 3.5 route: ${page.path}`);
  }
  for (const [path, count] of counts) {
    if (count > 1) errors.push(`duplicate Phase 3.5 route: ${path}`);
  }

  const incomplete = [];
  for (const path of phase35TargetRoutes) {
    const page = phasePages.find((candidate) => candidate.path === path);
    if (!page) {
      errors.push(`missing Phase 3.5 route: ${path}`);
      incomplete.push(path);
      continue;
    }
    const complete = page.backend === 'ready' && completedAcceptances.has(page.acceptance);
    if (complete && !completedImplementations.has(page.implementation)) {
      errors.push(`completed page must be native or legacy-adapter: ${path}`);
    }
    if (!complete || !completedImplementations.has(page.implementation)) incomplete.push(path);
    if (complete && (!page.evidence?.spec || !page.evidence?.acceptance)) {
      errors.push(`completed page missing evidence: ${path}`);
    }
  }
  if (incomplete.length) {
    errors.push(`Phase 3.5 incomplete routes (${phase35TargetRoutes.length - incomplete.length}/${phase35TargetRoutes.length}): ${incomplete.join(', ')}`);
  }
  if (errors.length) throw new Error(errors.join('\n'));
}

export async function readManifest(url = new URL('../web/apps/dashboard/src/benchmark/manifest.json', import.meta.url)) {
  return JSON.parse(await readFile(url, 'utf8'));
}

async function main() {
  validatePhase35Manifest(await readManifest());
  console.log('9/9 Phase 3.5 routes passed the implementation gate.');
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  main().catch((error) => {
    console.error(`Phase 3.5 dashboard manifest failed: ${error.message}`);
    process.exitCode = 1;
  });
}
