import { existsSync, readFileSync, readdirSync } from 'node:fs';
import { dirname, join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const requiredDockerImages = [
  'node:24.20.0-alpine@sha256:e67514e5d0f6c46656005e1b693b2ec9d52e80b641307de684d4a015ba7a4eaf',
  'golang:1.26.7-alpine@sha256:28d89ee9cc0ff9fec75c82ca201e6bf7fdf9a679d4b7b24dfa04f2bb766bb468',
  'alpine:3.22.5@sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce',
];

const requiredActionSHAs = new Map([
  ['actions/checkout', '11d5960a326750d5838078e36cf38b85af677262'],
  ['actions/setup-go', '40f1582b2485089dde7abd97c1529aa768e1baff'],
  ['pnpm/action-setup', 'b906affcce14559ad1aafd4ab0e942779e9f58b1'],
  ['actions/setup-node', '49933ea5288caeca8642d1e84afbd3f7d6820020'],
  ['actions/cache', '0057852bfaa89a56745cba8c7296529d2fc39830'],
  ['anchore/sbom-action', 'e22c389904149dbc22b58101806040fa8d37a610'],
  ['actions/upload-artifact', 'ea165f8d65b6e75b540449e92b4886f43607fa02'],
]);

function compareVersion(left, right) {
  const a = left.split('.').map(Number);
  const b = right.split('.').map(Number);
  for (let index = 0; index < 3; index += 1) {
    if ((a[index] ?? 0) !== (b[index] ?? 0)) return (a[index] ?? 0) - (b[index] ?? 0);
  }
  return 0;
}

export function inspectSupplyChainPolicy(files) {
  const errors = [];
  if (!/^toolchain go1\.26\.7$/m.test(files['go.mod'] ?? '')) {
    errors.push('go.mod must pin toolchain go1.26.7');
  }

  for (const image of requiredDockerImages) {
    if (!(files.Dockerfile ?? '').includes(image)) errors.push(`Dockerfile must use multi-arch index digest ${image}`);
  }

  const workflow = files.workflow ?? '';
  for (const match of workflow.matchAll(/^\s*uses:\s*([^\s#]+)(?:\s*#.*)?$/gm)) {
    const separator = match[1].lastIndexOf('@');
    const action = match[1].slice(0, separator);
    const ref = match[1].slice(separator + 1);
    if (!/^[0-9a-f]{40}$/.test(ref)) errors.push(`${action} must use a full commit SHA`);
    const expected = requiredActionSHAs.get(action);
    if (expected && ref !== expected) errors.push(`${action} must use verified commit ${expected}`);
  }
  for (const [action, sha] of requiredActionSHAs) {
    if (!workflow.includes(`uses: ${action}@${sha}`)) errors.push(`workflow must include pinned ${action}@${sha}`);
  }
  if (!workflow.includes('golang.org/x/vuln/cmd/govulncheck@v1.7.0')) errors.push('workflow must run govulncheck v1.7.0');
  if (!workflow.includes('pnpm audit --audit-level high')) errors.push('workflow must fail on high pnpm advisories');
  if (!workflow.includes('-action inventory')) errors.push('workflow must consume the migration inventory');
  if (!workflow.includes('sha256sum mochat-go.spdx.json')) errors.push('workflow must checksum the SBOM');
  if (workflow.includes("0098_scrm_lead_foundation")) errors.push('workflow must not hard-code migration version 0098');

  for (const { path, json } of files.manifests ?? []) {
    for (const dependencies of [json.dependencies, json.devDependencies]) {
      if (dependencies?.['react-router'] && dependencies['react-router'] !== '7.18.2') {
        errors.push(`${path}: react-router must be 7.18.2`);
      }
      if (dependencies?.vitest && compareVersion(dependencies.vitest, '3.2.6') < 0) {
        errors.push(`${path}: vitest must be at least 3.2.6`);
      }
    }
  }

  for (const override of [
    'brace-expansion@1.1.16: 1.1.18',
    'brace-expansion@2.1.2: 2.1.4',
    'js-yaml@4.3.0: 4.3.1',
    'nanoid@3.3.16: 3.3.18',
  ]) {
    if (!(files.workspace ?? '').includes(override)) errors.push(`pnpm-workspace.yaml must override ${override}`);
  }
  return errors;
}

function workspaceManifests(root) {
  const manifests = [];
  for (const parent of ['web/apps', 'web/packages']) {
    const directory = join(root, parent);
    if (!existsSync(directory)) continue;
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const path = join(directory, entry.name, 'package.json');
      if (!entry.isDirectory() || !existsSync(path)) continue;
      manifests.push({ path: relative(root, path).replaceAll('\\', '/'), json: JSON.parse(readFileSync(path, 'utf8')) });
    }
  }
  return manifests;
}

export function checkSupplyChainPolicy(root) {
  return inspectSupplyChainPolicy({
    'go.mod': readFileSync(join(root, 'go.mod'), 'utf8'),
    Dockerfile: readFileSync(join(root, 'Dockerfile'), 'utf8'),
    workflow: readFileSync(join(root, '.github/workflows/mysql57-amd64.yml'), 'utf8'),
    manifests: workspaceManifests(root),
    workspace: readFileSync(join(root, 'pnpm-workspace.yaml'), 'utf8'),
  });
}

const currentPath = fileURLToPath(import.meta.url);
if (process.argv[1] && currentPath === fileURLToPath(new URL(`file:///${process.argv[1].replaceAll('\\', '/')}`))) {
  const root = join(dirname(currentPath), '..');
  const errors = checkSupplyChainPolicy(root);
  if (errors.length > 0) {
    for (const error of errors) console.error(`ERROR: ${error}`);
    process.exitCode = 1;
  } else {
    console.log('supply-chain policy check passed');
  }
}
