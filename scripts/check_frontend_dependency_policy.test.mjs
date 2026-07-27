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
