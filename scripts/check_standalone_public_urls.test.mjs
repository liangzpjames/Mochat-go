import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { existsSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const composeFile = join(repositoryRoot, 'deploy', 'standalone', 'docker-compose.yml');

function renderCompose(overrides = {}) {
  const temporaryDirectory = mkdtempSync(join(tmpdir(), 'mochat-standalone-public-urls-'));
  const saasMfaKeyFile = join(temporaryDirectory, 'saas-admin-mfa-key');
  const dashboardMfaKeyFile = join(temporaryDirectory, 'dashboard-mfa-key');
  const environment = { ...process.env };
  for (const name of [
    'MOCHAT_API_BASE_URL',
    'MOCHAT_DASHBOARD_BASE_URL',
    'MOCHAT_SIDEBAR_BASE_URL',
    'MOCHAT_OPERATION_BASE_URL',
    'MOCHAT_GO_PORT',
    'MOCHAT_SIDEBAR_PORT',
    'MOCHAT_OPERATION_PORT',
  ]) {
    delete environment[name];
  }
  writeFileSync(saasMfaKeyFile, '');
  writeFileSync(dashboardMfaKeyFile, '');

  try {
    const composeEnvironment = {
      ...environment,
      MOCHAT_SAAS_ADMIN_JWT_SECRET: 'test-saas-admin-jwt-secret',
      MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_FILE: saasMfaKeyFile,
      MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_ID: 'test-saas-admin-key',
      MOCHAT_DASHBOARD_JWT_SECRET: 'test-dashboard-jwt-secret',
      MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_FILE: dashboardMfaKeyFile,
      MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_ID: 'test-dashboard-key',
      ...overrides,
    };
    const composeArguments = ['compose', '-f', composeFile, '--profile', 'app', 'config'];
    execFileSync('docker', [...composeArguments, '--quiet'], {
      cwd: repositoryRoot,
      encoding: 'utf8',
      env: composeEnvironment,
    });
    const output = execFileSync(
      'docker',
      [...composeArguments, '--format', 'json'],
      {
        cwd: repositoryRoot,
        encoding: 'utf8',
        env: composeEnvironment,
      },
    );
    return JSON.parse(output);
  } finally {
    rmSync(temporaryDirectory, { recursive: true, force: true });
    assert.equal(existsSync(temporaryDirectory), false, 'temporary secret directory must be cleaned up');
  }
}

function appEnvironment(config) {
  return config.services.app.environment;
}

test('standalone defaults publish browser-reachable URLs for every frontend', () => {
  const environment = appEnvironment(renderCompose());

  assert.deepEqual(
    {
      api: environment.MOCHAT_API_BASE_URL,
      dashboard: environment.MOCHAT_DASHBOARD_BASE_URL,
      sidebar: environment.MOCHAT_SIDEBAR_BASE_URL,
      operation: environment.MOCHAT_OPERATION_BASE_URL,
    },
    {
      api: 'http://127.0.0.1:18080',
      dashboard: 'http://127.0.0.1:18080',
      sidebar: 'http://127.0.0.1:18081',
      operation: 'http://127.0.0.1:18082',
    },
  );
  assert.notEqual(environment.MOCHAT_SIDEBAR_BASE_URL, 'http://127.0.0.1:8081');
});

test('standalone public URLs follow overridden host ports', () => {
  const environment = appEnvironment(renderCompose({
    MOCHAT_GO_PORT: '28080',
    MOCHAT_SIDEBAR_PORT: '28081',
    MOCHAT_OPERATION_PORT: '28082',
  }));

  assert.deepEqual(
    {
      api: environment.MOCHAT_API_BASE_URL,
      dashboard: environment.MOCHAT_DASHBOARD_BASE_URL,
      sidebar: environment.MOCHAT_SIDEBAR_BASE_URL,
      operation: environment.MOCHAT_OPERATION_BASE_URL,
    },
    {
      api: 'http://127.0.0.1:28080',
      dashboard: 'http://127.0.0.1:28080',
      sidebar: 'http://127.0.0.1:28081',
      operation: 'http://127.0.0.1:28082',
    },
  );
});

test('explicit HTTPS public URL overrides the standalone sidebar default', () => {
  const environment = appEnvironment(renderCompose({
    MOCHAT_SIDEBAR_PORT: '28081',
    MOCHAT_SIDEBAR_BASE_URL: 'https://sidebar.example.test',
  }));

  assert.equal(environment.MOCHAT_SIDEBAR_BASE_URL, 'https://sidebar.example.test');
});
