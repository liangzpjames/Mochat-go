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

export function validatePhase32Manifest(manifest) {
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

  if (errors.length > 0) throw new Error(errors.join('\n'));
}

export async function readManifest(manifestUrl = new URL('../web/apps/dashboard/src/benchmark/manifest.json', import.meta.url)) {
  return JSON.parse(await readFile(manifestUrl, 'utf8'));
}

async function main() {
  const manifest = await readManifest();
  validatePhase32Manifest(manifest);
  console.log(`Phase 3.2 dashboard manifest passed (${phase32TargetRoutes.length} target routes).`);
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  main().catch((error) => {
    console.error(`Phase 3.2 dashboard manifest failed: ${error.message}`);
    process.exitCode = 1;
  });
}
