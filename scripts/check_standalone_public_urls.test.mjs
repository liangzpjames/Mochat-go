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

test('standalone defaults publish browser-reachable URLs for Sidebar dependencies only', () => {
  const environment = appEnvironment(renderCompose());

  assert.deepEqual(
    {
      api: environment.MOCHAT_API_BASE_URL,
      sidebar: environment.MOCHAT_SIDEBAR_BASE_URL,
    },
    {
      api: 'http://127.0.0.1:18080',
      sidebar: 'http://127.0.0.1:18081',
    },
  );
  assert.equal(environment.MOCHAT_DASHBOARD_BASE_URL, undefined);
  assert.equal(environment.MOCHAT_OPERATION_BASE_URL, undefined);
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
      sidebar: environment.MOCHAT_SIDEBAR_BASE_URL,
    },
    {
      api: 'http://127.0.0.1:28080',
      sidebar: 'http://127.0.0.1:28081',
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

test('standalone forwards the WeCom archive sync runtime configuration', () => {
  const environment = appEnvironment(renderCompose({
    MOCHAT_GO_ENABLE_WORK_MESSAGE_ARCHIVE_SYNC_CRON: '1',
    MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_CRON_INTERVAL_SECONDS: '15',
    MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_CRON_RUN_ON_START: '1',
    MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_LIMIT: '50',
    MOCHAT_GO_WORK_MESSAGE_ARCHIVE_BRIDGE_BASE_URL: 'http://wecom-archive-demo:8080',
    MOCHAT_GO_WORK_MESSAGE_ARCHIVE_BRIDGE_TOKEN: 'test-bridge-token',
  }));

  assert.deepEqual(
    {
      enabled: environment.MOCHAT_GO_ENABLE_WORK_MESSAGE_ARCHIVE_SYNC_CRON,
      interval: environment.MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_CRON_INTERVAL_SECONDS,
      runOnStart: environment.MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_CRON_RUN_ON_START,
      limit: environment.MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_LIMIT,
      bridgeURL: environment.MOCHAT_GO_WORK_MESSAGE_ARCHIVE_BRIDGE_BASE_URL,
      bridgeToken: environment.MOCHAT_GO_WORK_MESSAGE_ARCHIVE_BRIDGE_TOKEN,
    },
    {
      enabled: '1',
      interval: '15',
      runOnStart: '1',
      limit: '50',
      bridgeURL: 'http://wecom-archive-demo:8080',
      bridgeToken: 'test-bridge-token',
    },
  );
});
