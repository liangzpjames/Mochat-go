import assert from 'node:assert/strict';
import test from 'node:test';

import {
  buildProviderIntegrationArgs,
  integrationAvailability,
  outputContainsSkip,
} from './check_provider_integration.mjs';

test('required-real integration command includes archive, migration, simulator, and CLI contracts', () => {
  const args = buildProviderIntegrationArgs(true);
  assert.deepEqual(args.slice(0, 2), ['test', '-v']);
  assert.ok(args.includes('./internal/store'));
  assert.ok(args.includes('./internal/migration'));
  assert.ok(args.includes('./internal/archivesim'));
  assert.ok(args.includes('./cmd/mochat-archive-simulator'));
  assert.ok(args.some((value) => value.includes('TestApplyCleanupReapply|TestSimulationCLI')));
});

test('ordinary integration gate reports an explicit skip without DSN', () => {
  const result = integrationAvailability({});
  assert.equal(result.required, false);
  assert.match(result.message, /SKIP.*MOCHAT_GO_MYSQL_INTEGRATION_DSN/);
});

test('required-real integration gate rejects missing DSN and any test skip output', () => {
  const result = integrationAvailability({}, true);
  assert.equal(result.required, true);
  assert.equal(result.ok, false);
  assert.match(result.message, /required-real/i);
  assert.equal(outputContainsSkip('--- SKIP: isolated MariaDB DSN'), true);
  assert.equal(outputContainsSkip('PASS: no skipped tests'), false);
});
