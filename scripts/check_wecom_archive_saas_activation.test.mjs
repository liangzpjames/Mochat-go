import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import { checkRepository } from './check_wecom_archive_saas_activation.mjs';

test('acceptance repository contract covers secrets, pipeline exclusivity, fixtures and isolated compose', async () => {
  const errors = await checkRepository(process.cwd());
  assert.deepEqual(errors, []);
});

test('acceptance compose isolates the SaaS and Dashboard MFA key identities', async () => {
  const errors = await checkRepository(process.cwd());
  assert.ok(!errors.includes('acceptance compose reuses one MFA key identity'), errors.join('\n'));
});

test('acceptance migrations install the archive simulation registry before durable archive sync', async () => {
  const init = await readFile('deploy/local-acceptance/init/099-apply-migrations.sh', 'utf8');
  const simulation = init.indexOf('0133_archive_simulation_registry.up.sql');
  const durable = init.indexOf('0138_archive_source_sync.up.sql');
  assert.ok(simulation >= 0, '0133 archive simulation registry migration is missing');
  assert.ok(durable >= 0, '0138 durable archive migration is missing');
  assert.ok(simulation < durable, '0133 must run before 0138');
});

test('acceptance seed exercises durable replay, formal integration transitions and activation services', async () => {
  const source = await readFile('cmd/mochat-archive-acceptance/main.go', 'utf8');
  assert.doesNotMatch(source, /if\s+!found\s*\{[\s\S]*?NewSyncService/, 'seed must not bypass Sync for an existing succeeded run');
  for (const contract of [
    'SaveCandidate',
    'VerifyCandidate',
    'Switch(ctx',
    'Rollback(ctx',
    'ProvisionDashboardTenant',
    'DashboardActivationStatus',
    'syncIdempotent',
  ]) {
    assert.ok(source.includes(contract), `acceptance CLI missing ${contract}`);
  }
});

test('acceptance runtime secures Windows secrets, rebuilds the CLI and runs a durable worker', async () => {
  const script = await readFile('scripts/run_wecom_archive_saas_activation_acceptance.ps1', 'utf8');
  for (const contract of ['/inheritance:r', '*S-1-5-11', '*S-1-5-32-545', "@('build', 'acceptance')", "'worker'"]) {
    assert.ok(script.includes(contract), `acceptance PowerShell missing ${contract}`);
  }
  const compose = await readFile('deploy/local-acceptance/docker-compose.yml', 'utf8');
  assert.match(compose, /\n  worker:\n[\s\S]*MOCHAT_GO_RUNTIME_ROLE:\s*worker[\s\S]*MOCHAT_GO_ENABLE_DURABLE_WORK_MESSAGE_ARCHIVE:\s*"1"/);
  assert.match(compose, /worker:[\s\S]*restart:\s*unless-stopped/);
});

test('acceptance cleanup scopes the durable run key and verifies orphan removal', async () => {
  const source = await readFile('cmd/mochat-archive-acceptance/main.go', 'utf8');
  assert.match(source, /DELETE FROM mochat_go_archive_sync_runs[^`]*idempotency_key=\?/);
  assert.ok(source.includes('verifyCleanupOrphans'), 'cleanup does not verify database and filesystem orphans');
});
