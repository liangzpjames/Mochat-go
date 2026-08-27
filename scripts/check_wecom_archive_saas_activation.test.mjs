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

test('acceptance migrations install tenant AI providers before the integration schema', async () => {
  const init = await readFile('deploy/local-acceptance/init/099-apply-migrations.sh', 'utf8');
  const aiProviders = init.indexOf('0164_saas_tenant_ai_provider.up.sql');
  const integrations = init.indexOf('0166_wecom_integration_and_archive_media.up.sql');
  assert.ok(aiProviders >= 0, '0164 SaaS tenant AI provider migration is missing');
  assert.ok(integrations >= 0, '0166 WeCom integration migration is missing');
  assert.ok(aiProviders < integrations, '0164 must run before 0166');
});

test('acceptance database records only successful migrations and applies capability ledger compatibility', async () => {
  const init = await readFile('deploy/local-acceptance/init/099-apply-migrations.sh', 'utf8');
  assert.match(init, /CREATE TABLE IF NOT EXISTS mochat_go_schema_migrations/);
  assert.match(init, /sha256sum/);
  assert.match(init, /apply_migration/);
  assert.ok(init.includes('0139_wecom_capability_ledger.up.sql'));
  assert.ok(init.includes('/local-acceptance-init/0139-parent-compat.sql'));
  assert.match(init, /INSERT INTO mochat_go_schema_migrations/);
});

test('acceptance database installs the complete non-controlled Dashboard schema', async () => {
  const init = await readFile('deploy/local-acceptance/init/099-apply-migrations.sh', 'utf8');
  for (const migration of [
    '0106_scrm_lead_parity.up.sql',
    '0119_phase35_orders_settings.up.sql',
    '0148_ai_conversation_insights.up.sql',
    '0167_tenant_wecom_mode.up.sql',
    '0168_company_archive_sync_rbac.up.sql',
  ]) {
    assert.ok(init.includes(migration), `acceptance init is missing ${migration}`);
  }
  assert.doesNotMatch(init, /\b0130_identity_realms_single_corp_backfill\.up\.sql\b/);
  assert.doesNotMatch(init, /\b0131_identity_realms_single_corp_cutover\.up\.sql\b/);
});

test('acceptance seed exercises durable replay, immutable integration rejection and activation services', async () => {
  const source = await readFile('cmd/mochat-archive-acceptance/main.go', 'utf8');
  assert.doesNotMatch(source, /if\s+!found\s*\{[\s\S]*?NewSyncService/, 'seed must not bypass Sync for an existing succeeded run');
  for (const contract of [
	'ErrWeComModeImmutable',
	'acceptance immutable current integration contract failed',
	'acceptance retired integration lifecycle did not fail closed',
    'ProvisionDashboardTenant',
    'DashboardActivationStatus',
    'syncIdempotent',
  ]) {
    assert.ok(source.includes(contract), `acceptance CLI missing ${contract}`);
  }
});

test('acceptance runtime secures Windows secrets, rebuilds the CLI and runs a durable all-in-one app', async () => {
  const script = await readFile('scripts/run_wecom_archive_saas_activation_acceptance.ps1', 'utf8');
  for (const contract of ['/inheritance:r', '*S-1-5-11', '*S-1-5-32-545', "@('build', 'acceptance')", "'app'"]) {
    assert.ok(script.includes(contract), `acceptance PowerShell missing ${contract}`);
  }
  const compose = await readFile('deploy/local-acceptance/docker-compose.yml', 'utf8');
  assert.match(compose, /\n  app:\n[\s\S]*MOCHAT_GO_RUNTIME_ROLE:\s*all[\s\S]*MOCHAT_GO_ENABLE_DURABLE_WORK_MESSAGE_ARCHIVE:\s*"1"/);
  assert.match(compose, /app:[\s\S]*restart:\s*unless-stopped/);
  assert.doesNotMatch(compose, /\n  worker:\n/);
});

test('every data action refreshes the acceptance image and proves an interrupted checkpoint takeover', async () => {
  const script = await readFile('scripts/run_wecom_archive_saas_activation_acceptance.ps1', 'utf8');
  for (const action of ['seed', 'verify', 'cleanup']) {
    const branch = script.match(new RegExp(`'${action}'\\s*\\{([\\s\\S]*?)(?=\\n\\s*'[^']+'\\s*\\{|\\n\\s*})`))?.[1] ?? '';
    assert.ok(branch.includes('Ensure-AcceptanceImage'), `${action} does not refresh the acceptance image`);
  }
  for (const contract of ['-defer-media', 'checkpoint', 'SIGKILL', 'partial', 'expire', 'recovered']) {
    assert.ok(script.includes(contract), `acceptance runtime does not prove ${contract}`);
  }
});

test('acceptance cleanup scopes the durable run key and verifies orphan removal', async () => {
  const source = await readFile('cmd/mochat-archive-acceptance/main.go', 'utf8');
  assert.match(source, /DELETE FROM mochat_go_archive_sync_runs[^`]*idempotency_key=\?/);
  assert.ok(source.includes('verifyCleanupOrphans'), 'cleanup does not verify database and filesystem orphans');
});

test('acceptance initializes the bootstrap SaaS password only through the formal HTTP challenge', async () => {
  const script = await readFile('scripts/run_wecom_archive_saas_activation_acceptance.ps1', 'utf8');
  for (const contract of ['/saas/auth/login', '/saas/auth/password', 'PASSWORD_CHANGE_REQUIRED', 'passwordChangeToken', 'Protect-RuntimePath']) {
    assert.ok(script.includes(contract), `acceptance SaaS password initialization missing ${contract}`);
  }
  assert.doesNotMatch(script, /UPDATE\s+mochat_go_saas_admin_users[\s\S]{0,200}must_rotate/i);
});
