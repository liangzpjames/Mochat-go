import { execFileSync, spawnSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { dirname, relative, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const dashboardRoot = resolve(repositoryRoot, 'web/apps/dashboard');
const dashboardPrefix = 'web/apps/dashboard/';
const lintableExtensions = /\.(?:ts|tsx)$/i;

export function selectDashboardChangedLintFiles(changedFiles) {
  return [...new Set(changedFiles.map((file) => file.replaceAll('\\', '/').replace(/^\.\//, '')))]
    .filter((file) => file.startsWith(dashboardPrefix) && lintableExtensions.test(file))
    .sort();
}

function gitFiles(args) {
  const output = execFileSync('git', args, {
    cwd: repositoryRoot,
    encoding: 'utf8',
  });
  return output.split(/\r?\n/).filter(Boolean);
}

function changedFilesSince(base) {
  return [
    ...gitFiles(['diff', '--name-only', '--diff-filter=ACMR', `${base}..HEAD`, '--', dashboardPrefix]),
    ...gitFiles(['diff', '--name-only', '--diff-filter=ACMR', base, '--', dashboardPrefix]),
    ...gitFiles(['ls-files', '--others', '--exclude-standard', '--', dashboardPrefix]),
  ];
}

function main() {
  const base = process.env.PHASE34_LINT_BASE ?? '13cd9cc';
  const files = selectDashboardChangedLintFiles(changedFilesSince(base));
  if (files.length === 0) {
    console.log(`Phase 3.4 changed-files lint passed: no Dashboard TypeScript files changed since ${base}.`);
    return;
  }

  const relativeFiles = files.map((file) => file.slice(dashboardPrefix.length));
  console.log(`Phase 3.4 changed-files lint: ${relativeFiles.length} file(s) since ${base}.`);
  for (const file of relativeFiles) console.log(`  ${file}`);

  if (!existsSync(resolve(dashboardRoot, 'node_modules'))) {
    console.error(`Dashboard dependencies are missing under ${dashboardRoot}.`);
    process.exitCode = 1;
    return;
  }

  const pnpm = process.platform === 'win32' ? 'pnpm.cmd' : 'pnpm';
  const result = spawnSync(pnpm, ['exec', 'eslint', ...relativeFiles], {
    cwd: dashboardRoot,
    stdio: 'inherit',
    shell: process.platform === 'win32',
  });
  if (result.error) console.error(`Failed to run ${pnpm}: ${result.error.message}`);
  process.exitCode = result.status ?? 1;
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  main();
}
