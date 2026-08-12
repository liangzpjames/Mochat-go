import assert from 'node:assert/strict';
import test from 'node:test';
import { readFile } from 'node:fs/promises';

const scriptURL = new URL('./smoke_identity_single_corp.ps1', import.meta.url);

async function readSmokeScript() {
  return readFile(scriptURL, 'utf8');
}

test('PowerShell smoke exposes explicit mutually exclusive read-only and fixture-write modes', async () => {
  const source = await readSmokeScript();

  assert.match(source, /\[switch\]\$ReadOnly/);
  assert.match(source, /\[switch\]\$ExerciseFixtureWrites/);
  assert.match(source, /\$ReadOnly\s*-and\s*\$ExerciseFixtureWrites/);
  assert.match(source, /-not\s+\$ReadOnly\s*-and\s*-not\s+\$ExerciseFixtureWrites/);
  assert.match(source, /BaseUrl\s*=\s*"http:\/\/localhost:18080"/);
  assert.match(source, /ComposeFile[^\r\n]*docker-compose\.yml/);
  assert.match(source, /mochat-go-desktop/);
});

test('PowerShell smoke reads database credentials inside mysql and proves exact counts and data retention', async () => {
  const source = await readSmokeScript();

  assert.match(source, /MARIADB_USER/);
  assert.match(source, /MARIADB_PASSWORD/);
  assert.match(source, /MARIADB_DATABASE/);
  assert.match(source, /MYSQL_PWD/);
  assert.doesNotMatch(source, /-p["']\$MARIADB_PASSWORD/);
  assert.match(source, /COUNT\(\*\)/);
  assert.match(source, /table-counts-before/);
  assert.match(source, /table-counts-after/);
  assert.match(source, /read-only smoke changed table counts/i);
  assert.match(source, /container-ids-before|containerIdsBefore/);
  assert.match(source, /container-ids-after|containerIdsAfter/);
  assert.match(source, /mysql/);
  assert.match(source, /redis/);
  assert.match(source, /Mountpoint/);
  assert.match(source, /volumes-before/);
  assert.match(source, /volumes-after/);
  assert.match(source, /four .*volumes|expectedVolumes/i);
});

test('full smoke uses protected APIs for cross-tenant, stale-version, credential metadata, and audit checks', async () => {
  const source = await readSmokeScript();

  assert.match(source, /dashboard\/access\/users/);
  assert.match(source, /dashboard\/company\/profile/);
  assert.match(source, /dashboard\/company\/wecom-credentials/);
  assert.match(source, /dashboard\/company\/audits/);
  assert.match(source, /expectedVersion/);
  assert.match(source, /409/);
  assert.match(source, /CredentialSecretFile|MOCHAT_IDENTITY_SINGLE_CORP_CREDENTIAL_FILE/);
  assert.match(source, /credential.*metadata|configured|keyId|updatedAt/i);
  assert.match(source, /Assert.*(?:Secret|Credential)|forbidden.*(?:secret|credential)/i);
  assert.doesNotMatch(source, /\b(?:INSERT|UPDATE|DELETE|REPLACE|ALTER|DROP|TRUNCATE)\b/i);
});

test('PowerShell smoke has no destructive Docker operations or raw command-line credentials', async () => {
  const source = await readSmokeScript();
  const parameterBlock = source.match(/param\([\s\S]*?\n\)/i)?.[0] ?? '';

  assert.doesNotMatch(source, /docker\s+compose[^\r\n]*\bdown\b/i);
  assert.doesNotMatch(source, /docker\s+volume\s+rm/i);
  assert.doesNotMatch(source, /docker\s+system\s+prune/i);
  assert.doesNotMatch(source, /docker\s+volume\s+prune/i);
  assert.doesNotMatch(source, /(?:mysql|redis)[^\r\n]*(?:recreate|up\s+-d|down|rm)/i);
  assert.doesNotMatch(parameterBlock, /\[string\]\$(?:Password|Jwt|Token|Secret)\b/i);
  assert.doesNotMatch(parameterBlock, /\b(?:password|jwt|token)\s*=/i);
  assert.doesNotMatch(source, /Write-(?:Json|Output|Host)[^\r\n]*(?:password|jwt|token|secret|credentialPayload)/i);
  assert.doesNotMatch(source, /\$response\.Content\s*\|\s*(?:Set-Content|Write-|ConvertTo-Json)/i);
});
