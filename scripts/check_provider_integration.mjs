import { spawnSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const integrationPackages = [
  './internal/store',
  './internal/migration',
  './internal/archivesim',
  './cmd/mochat-archive-simulator',
];
const integrationPattern = 'TestApplyCleanupReapply|TestSimulationCLI|TestArchive(Source|Sync)';

export function buildProviderIntegrationArgs(verbose = false) {
  return [
    'test',
    ...(verbose ? ['-v'] : []),
    '-count=1',
    ...integrationPackages,
    '-run',
    integrationPattern,
  ];
}

export function integrationAvailability(env = process.env, required = false) {
  const dsn = String(env.MOCHAT_GO_MYSQL_INTEGRATION_DSN ?? '').trim();
  if (!dsn) {
    return required
      ? {
          required: true,
          ok: false,
          message: 'required-real blocked: MOCHAT_GO_MYSQL_INTEGRATION_DSN is not set',
        }
      : {
          required: false,
          ok: true,
          message: 'SKIP: MOCHAT_GO_MYSQL_INTEGRATION_DSN is not set; isolated MariaDB integration was not run',
        };
  }
  return {
    required,
    ok: true,
    message: required ? 'required-real: isolated MariaDB DSN is configured' : 'integration: isolated MariaDB DSN is configured',
  };
}

export function outputContainsSkip(output) {
  return /(^|\n).*\bSKIP\b/i.test(String(output));
}

export function runProviderIntegration({ root = process.cwd(), required = false, env = process.env } = {}) {
  const availability = integrationAvailability(env, required);
  process.stdout.write(`${availability.message}\n`);
  if (!availability.ok) return availability;

  const completed = spawnSync('go', buildProviderIntegrationArgs(true), {
    cwd: root,
    env,
    encoding: 'utf8',
    maxBuffer: 8 * 1024 * 1024,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  const output = `${String(completed.stdout ?? '')}${String(completed.stderr ?? '')}`;
  process.stdout.write(output);
  if (completed.error || completed.status !== 0) {
    return { ...availability, ok: false, output, exitCode: completed.status ?? 1, error: completed.error };
  }
  if (required && outputContainsSkip(output)) {
    return { ...availability, ok: false, output, message: 'required-real failed: go test output contains SKIP' };
  }
  return { ...availability, ok: true, output };
}

if (process.argv[1] && path.resolve(process.argv[1]) === path.resolve(fileURLToPath(import.meta.url))) {
  const required = process.argv.includes('--required-real');
  const result = runProviderIntegration({ required });
  if (!result.ok) process.exitCode = 1;
}
