import assert from 'node:assert/strict';
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import test from 'node:test';

const repositoryRoot = join(dirname(fileURLToPath(import.meta.url)), '..');
const policyModuleUrl = pathToFileURL(join(repositoryRoot, 'scripts', 'check_frontend_dependency_policy.mjs')).href;

function write(root, relativePath, content) {
  const target = join(root, relativePath);
  const parent = dirname(target);
  mkdirSync(parent, { recursive: true });
  writeFileSync(target, content);
}

function createFixture() {
  const root = mkdtempSync(join(tmpdir(), 'mochat-dependency-policy-'));
  const dashboard = join(root, 'web', 'apps', 'dashboard');
  const config = join(root, 'web', 'packages', 'config');
  for (const directory of [dashboard, config]) {
    mkdirSync(directory, { recursive: true });
  }

  write(root, 'package.json', JSON.stringify({
    name: 'mochat-go-workspace',
    private: true,
    packageManager: 'pnpm@11.17.0',
  }, null, 2));
  write(root, 'pnpm-workspace.yaml', `packages:\n  - web/apps/*\n  - web/packages/*\nautoInstallPeers: false\ndedupePeerDependents: true\nengineStrict: true\npreferWorkspacePackages: true\nsaveExact: true\nstrictPeerDependencies: true\n`);
  write(root, 'web/apps/dashboard/package.json', JSON.stringify({
    name: '@mochat/dashboard',
    private: true,
    dependencies: {
      '@mochat/config': 'workspace:*',
      react: '19.2.8',
      'react-dom': '19.2.8',
    },
  }, null, 2));
  write(root, 'web/packages/config/package.json', JSON.stringify({
    name: '@mochat/config',
    private: true,
  }, null, 2));
  write(root, 'pnpm-lock.yaml', `lockfileVersion: '9.0'\n\nimporters:\n\n  .: {}\n\n  web/apps/dashboard:\n    dependencies:\n      '@mochat/config':\n        specifier: workspace:*\n        version: link:../../packages/config\n      react:\n        specifier: 19.2.8\n        version: 19.2.8\n      react-dom:\n        specifier: 19.2.8\n        version: 19.2.8\n\n  web/packages/config: {}\n`);
  return root;
}

async function check(root) {
  const { checkDependencyPolicy } = await import(`${policyModuleUrl}?fixture=${Date.now()}-${Math.random()}`);
  return checkDependencyPolicy(root);
}

function withFixture(callback) {
  const root = createFixture();
  return callback(root).finally(() => rmSync(root, { recursive: true, force: true }));
}

test('rejects mismatched React and React DOM versions', async () => withFixture(async (root) => {
  write(root, 'web/apps/dashboard/package.json', JSON.stringify({
    name: '@mochat/dashboard',
    private: true,
    dependencies: { react: '19.2.8', 'react-dom': '19.2.7' },
  }, null, 2));

  const result = await check(root);
  assert.equal(result.ok, false);
  assert.match(result.errors.join('\n'), /react and react-dom must use the same exact version/);
}));

test('rejects app references to internal packages without workspace star', async () => withFixture(async (root) => {
  write(root, 'web/apps/dashboard/package.json', JSON.stringify({
    name: '@mochat/dashboard',
    private: true,
    dependencies: { '@mochat/config': '1.0.0', react: '19.2.8', 'react-dom': '19.2.8' },
  }, null, 2));

  const result = await check(root);
  assert.equal(result.ok, false);
  assert.match(result.errors.join('\n'), /internal dependency @mochat\/config must use workspace:\*/);
}));

test('rejects shared packages that depend on applications', async () => withFixture(async (root) => {
  write(root, 'web/packages/config/package.json', JSON.stringify({
    name: '@mochat/config',
    private: true,
    dependencies: { '@mochat/dashboard': 'workspace:*' },
  }, null, 2));

  const result = await check(root);
  assert.equal(result.ok, false);
  assert.match(result.errors.join('\n'), /shared package @mochat\/config must not depend on app @mochat\/dashboard/);
}));

test('rejects a root manifest without packageManager', async () => withFixture(async (root) => {
  write(root, 'package.json', JSON.stringify({ name: 'mochat-go-workspace', private: true }, null, 2));

  const result = await check(root);
  assert.equal(result.ok, false);
  assert.match(result.errors.join('\n'), /root package.json must declare packageManager/);
}));

test('rejects lockfile specifier drift', async () => withFixture(async (root) => {
  const lockfile = join(root, 'pnpm-lock.yaml');
  writeFileSync(lockfile, `lockfileVersion: '9.0'\n\nimporters:\n\n  .: {}\n\n  web/apps/dashboard:\n    dependencies:\n      '@mochat/config':\n        specifier: workspace:*\n        version: link:../../packages/config\n      react:\n        specifier: 19.2.7\n        version: 19.2.7\n      react-dom:\n        specifier: 19.2.8\n        version: 19.2.8\n\n  web/packages/config: {}\n`);

  const result = await check(root);
  assert.equal(result.ok, false);
  assert.match(result.errors.join('\n'), /pnpm-lock.yaml drift: web\/apps\/dashboard dependency react has specifier 19\.2\.7, expected 19\.2\.8/);
}));

test('allows a peer dependency absent from an importer when autoInstallPeers is disabled', async () => withFixture(async (root) => {
  write(root, 'web/apps/dashboard/package.json', JSON.stringify({
    name: '@mochat/dashboard',
    private: true,
    peerDependencies: { typescript: '5.9.3' },
  }, null, 2));
  writeFileSync(join(root, 'pnpm-lock.yaml'), `lockfileVersion: '9.0'\n\nimporters:\n\n  .: {}\n\n  web/apps/dashboard: {}\n\n  web/packages/config: {}\n`);

  const result = await check(root);
  assert.equal(result.ok, true, result.errors.join('\n'));
}));

test('rejects dependency type moves between manifest and lockfile', async () => withFixture(async (root) => {
  write(root, 'web/apps/dashboard/package.json', JSON.stringify({
    name: '@mochat/dashboard',
    private: true,
    devDependencies: { vite: '8.1.5' },
  }, null, 2));
  writeFileSync(join(root, 'pnpm-lock.yaml'), `lockfileVersion: '9.0'\n\nimporters:\n\n  .: {}\n\n  web/apps/dashboard:\n    dependencies:\n      vite:\n        specifier: 8.1.5\n        version: 8.1.5\n\n  web/packages/config: {}\n`);

  const result = await check(root);
  assert.equal(result.ok, false);
  assert.match(result.errors.join('\n'), /dependency vite is recorded under dependencies, expected devDependencies/);
}));

test('rejects stale lockfile-only importer entries', async () => withFixture(async (root) => {
  const lockfile = join(root, 'pnpm-lock.yaml');
  writeFileSync(lockfile, `lockfileVersion: '9.0'\n\nimporters:\n\n  .: {}\n\n  web/apps/dashboard:\n    dependencies:\n      '@mochat/config':\n        specifier: workspace:*\n        version: link:../../packages/config\n      react:\n        specifier: 19.2.8\n        version: 19.2.8\n      react-dom:\n        specifier: 19.2.8\n        version: 19.2.8\n      vite:\n        specifier: 8.1.5\n        version: 8.1.5\n\n  web/packages/config: {}\n`);

  const result = await check(root);
  assert.equal(result.ok, false);
  assert.match(result.errors.join('\n'), /stale dependency vite under dependencies/);
}));

test('normalizes quoted lockfile specifiers', async () => withFixture(async (root) => {
  const lockfile = join(root, 'pnpm-lock.yaml');
  writeFileSync(lockfile, `lockfileVersion: '9.0'\n\nimporters:\n\n  .: {}\n\n  web/apps/dashboard:\n    dependencies:\n      '@mochat/config':\n        specifier: 'workspace:*'\n        version: link:../../packages/config\n      react:\n        specifier: '19.2.8'\n        version: 19.2.8\n      react-dom:\n        specifier: \"19.2.8\"\n        version: 19.2.8\n\n  web/packages/config: {}\n`);

  const result = await check(root);
  assert.equal(result.ok, true, result.errors.join('\n'));
}));

test('rejects conflicting duplicate internal dependency declarations', async () => withFixture(async (root) => {
  write(root, 'web/apps/dashboard/package.json', JSON.stringify({
    name: '@mochat/dashboard',
    private: true,
    dependencies: { '@mochat/config': 'workspace:*' },
    devDependencies: { '@mochat/config': '1.0.0' },
  }, null, 2));

  const result = await check(root);
  assert.equal(result.ok, false);
  assert.match(result.errors.join('\n'), /declares @mochat\/config in both dependencies and devDependencies/);
  assert.match(result.errors.join('\n'), /internal dependency @mochat\/config must use workspace:\*/);
}));

test('ignores nested manifests outside direct workspace package children', async () => withFixture(async (root) => {
  write(root, 'web/packages/config/examples/not-a-workspace/package.json', JSON.stringify({
    name: '@mochat/not-a-workspace',
    dependencies: { react: '18.3.1' },
  }, null, 2));

  const result = await check(root);
  assert.equal(result.ok, true, result.errors.join('\n'));
}));
