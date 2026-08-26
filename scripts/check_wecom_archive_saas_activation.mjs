import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const required = [
  ['internal/modules/providers/archive/durable_bridge.go', ['ValidateArchivePipelineFlags', 'mutually exclusive']],
  ['internal/modules/providers/archive/bridge_source.go', ['sanitizeSDKIdentifiers', 'sdkfileid']],
  ['internal/testfixtures/archivesource/fixture.go', ['MOCHAT-LOCAL-ACCEPTANCE-20260827', 'MediaMissing', 'MediaCorrupt']],
  ['cmd/mochat-archive-acceptance/main.go', ['cleanupStatements', 'safeDatasetObjectPath', 'production']],
  ['web/apps/dashboard/src/features/auth/activation-page.tsx', ['location.hash', "window.history.replaceState({}, '', '/activate')"]],
  ['deploy/local-acceptance/docker-compose.yml', ['name: mochat-wecom-acceptance-20260827', '19080:8080', '19091:9091', 'MOCHAT_BOOTSTRAP_SAAS_ADMIN_PASSWORD_FILE', 'mochat-local-acceptance-admin']],
  ['deploy/local-acceptance/init/099-apply-migrations.sh', ['0104_scrm_opportunity_owner', '0127_dashboard_page_rbac', '0129_identity_realms_single_corp_schema', '0133_archive_simulation_registry', '0138_archive_source_sync', '0143_group_conversation_workspace', '0166_wecom_integration_and_archive_media']],
  ['scripts/run_wecom_archive_saas_activation_acceptance.ps1', ['mochat-wecom-acceptance-20260827', 'MOCHAT-LOCAL-ACCEPTANCE-20260827', '[Security.Cryptography.RandomNumberGenerator]::Create()', 'dashboard-acceptance-password']],
];

export async function checkRepository(root) {
  const errors = [];
  for (const [relative, fragments] of required) {
    let body;
    try {
      body = await readFile(path.join(root, relative), 'utf8');
    } catch {
      errors.push(`missing ${relative}`);
      continue;
    }
    for (const fragment of fragments) {
      if (!body.includes(fragment)) errors.push(`${relative} missing ${fragment}`);
    }
  }

  for (const relative of [
    'web/apps/saas-admin/src/lib/api.ts',
    'web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts',
  ]) {
    const body = await readFile(path.join(root, relative), 'utf8');
    for (const forbidden of ['credentialCiphertext', 'sdkFileId']) {
      if (body.includes(forbidden)) errors.push(`${relative} exposes ${forbidden}`);
    }
  }

  const activation = await readFile(path.join(root, 'web/apps/dashboard/src/features/auth/activation-page.tsx'), 'utf8');
  const clearIndex = activation.indexOf("window.history.replaceState({}, '', '/activate')");
  const statusIndex = activation.indexOf('activationToken={activationToken}');
  if (clearIndex < 0 || statusIndex < 0 || clearIndex > statusIndex) {
    errors.push('activation token is not cleared before status lookup');
  }

  const compose = await readFile(path.join(root, 'deploy/local-acceptance/docker-compose.yml'), 'utf8');
  const saasKeyId = compose.match(/MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_ID:\s*([^\r\n]+)/)?.[1]?.trim();
  const dashboardKeyId = compose.match(/MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_ID:\s*([^\r\n]+)/)?.[1]?.trim();
  if (!saasKeyId || !dashboardKeyId || saasKeyId === dashboardKeyId) {
    errors.push('acceptance compose reuses one MFA key identity');
  }
  const migrations = await readFile(path.join(root, 'deploy/local-acceptance/init/099-apply-migrations.sh'), 'utf8');
  const simulationMigration = migrations.indexOf('0133_archive_simulation_registry.up.sql');
  const durableMigration = migrations.indexOf('0138_archive_source_sync.up.sql');
  if (simulationMigration < 0 || durableMigration < 0 || simulationMigration > durableMigration) {
    errors.push('acceptance migrations do not install 0133 before 0138');
  }
  return errors;
}

async function main() {
  const root = process.cwd();
  const errors = await checkRepository(root);
  if (errors.length) {
    for (const error of errors) process.stderr.write(`FAIL ${error}\n`);
    process.exitCode = 1;
    return;
  }
  process.stdout.write('PASS WeCom archive/SaaS activation static acceptance contract\n');
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  await main();
}
