import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import test from 'node:test';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

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
  assert.match(source, /permission_audits/);
  assert.match(source, /tenant_corp_bindings/);
  assert.match(source, /dashboard_identities/);
  assert.match(source, /identity_activations/);
  assert.match(source, /tenant_provision_runs/);
  assert.match(source, /table-counts-before/);
  assert.match(source, /table-counts-after/);
  assert.match(source, /Assert-ExactCountDelta/);
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
  assert.match(source, /Assert-MachineCode[\s\S]*VERSION_CONFLICT/);
  assert.match(source, /dashboard\.company\.profile\.update/);
  assert.match(source, /dashboard\.company\.wecom_credentials\.rotate/);
  assert.match(source, /dashboard_permission_audits.*2|2.*dashboard_permission_audits/);
  assert.match(source, /mc_user.*0|0.*mc_user/);
  assert.match(source, /mc_corp.*0|0.*mc_corp/);
  assert.match(source, /tenant_corp_bindings.*0|0.*tenant_corp_bindings/);
  assert.match(source, /dashboard_identities.*0|0.*dashboard_identities/);
  assert.match(source, /identity_activations.*0|0.*identity_activations/);
  assert.match(source, /tenant_provision_runs.*0|0.*tenant_provision_runs/);
  assert.match(source, /Assert-AuditRecords|requestId.*resultVersion/);
  assert.doesNotMatch(source, /identity-single-corp-smoke-\$initialVersion/);
  assert.match(source, /CredentialSecretFile|MOCHAT_IDENTITY_SINGLE_CORP_CREDENTIAL_FILE/);
  assert.match(source, /credential.*metadata|configured|keyId|updatedAt/i);
  assert.match(source, /Assert.*(?:Secret|Credential)|forbidden.*(?:secret|credential)/i);
  assert.doesNotMatch(source, /\b(?:INSERT|UPDATE|DELETE|REPLACE|ALTER|DROP|TRUNCATE)\s+(?:INTO|FROM|TABLE|DATABASE|SET|VALUES|JOIN)\b/i);
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

test('PowerShell AST/structure keeps read-only and mutation branches separate and enforces full evidence', async () => {
  const source = await readSmokeScript();
  const scriptPath = fileURLToPath(scriptURL).replaceAll("'", "''");
  const parseCommand = `
$tokens = $null
$errors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile('${scriptPath}', [ref]$tokens, [ref]$errors)
if ($errors.Count -gt 0) { $errors | ForEach-Object { Write-Error $_.Message }; exit 1 }
$functions = @($ast.FindAll({ param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst] }, $true) | ForEach-Object {
  [pscustomobject]@{ name = $_.Name; body = $_.Body.Extent.Text }
})
$branches = @($ast.FindAll({ param($node) $node -is [System.Management.Automation.Language.IfStatementAst] }, $true) | ForEach-Object {
  $firstClause = $_.Clauses[0]
  [pscustomobject]@{
    condition = $firstClause.Item1.Extent.Text
    thenText = $firstClause.Item2.Extent.Text
    elseText = if ($null -ne $_.ElseClause) { $_.ElseClause.Extent.Text } else { '' }
  }
})
[pscustomobject]@{ functions = $functions; branches = $branches } | ConvertTo-Json -Depth 8 -Compress
`;
  const parsed = spawnSync('powershell.exe', ['-NoLogo', '-NoProfile', '-NonInteractive', '-Command', parseCommand], { encoding: 'utf8' });
  assert.equal(parsed.status, 0, parsed.stderr || parsed.stdout);
  const ast = JSON.parse(parsed.stdout.trim());
  const functions = [].concat(ast.functions ?? []);
  const branches = [].concat(ast.branches ?? []);
  const functionNames = functions.map((functionNode) => functionNode.name);
  assert.ok(functionNames.includes('Assert-ExactCountDelta'));
  assert.ok(functionNames.includes('Assert-AuditRecords'));
  assert.ok(functionNames.includes('Assert-MachineCode'));
  const countDeltaFunction = functions.find((functionNode) => functionNode.name === 'Assert-ExactCountDelta');
  const auditFunction = functions.find((functionNode) => functionNode.name === 'Assert-AuditRecords');
  assert.match(countDeltaFunction.body, /missing table[\s\S]*throw/);
  assert.match(countDeltaFunction.body, /throw[\s\S]*delta=/);
  assert.match(auditFunction.body, /matches\.Count\s*-ne\s*1[\s\S]*throw/);
  assert.match(auditFunction.body, /resultVersion[\s\S]*throw/);

  const readOnlyBranch = branches.find((branch) => /\$ReadOnly/.test(branch.condition) && /Invoke-ReadOnlyCheck/.test(branch.thenText));
  assert.ok(readOnlyBranch, 'PowerShell AST must expose a read-only branch');
  assert.doesNotMatch(readOnlyBranch.thenText, /company-profile-write|wecom-credential|\"PUT\"/i);
  assert.match(readOnlyBranch.elseText, /company-profile-write|wecom-credential|\"PUT\"/i);
  assert.match(source, /Assert-ExactCountDelta[\s\S]*dashboard_permission_audits[\s\S]*2/);
  assert.match(source, /Assert-AuditRecords[\s\S]*company-audits-after-write/);
  assert.match(source, /staleWrite[\s\S]*Assert-MachineCode[\s\S]*VERSION_CONFLICT/);
});
