import assert from 'node:assert/strict';
import test from 'node:test';
import { readFile } from 'node:fs/promises';

const specURL = new URL('../web/e2e/tests/identity-single-corp.spec.ts', import.meta.url);

async function readSpec() {
  return readFile(specURL, 'utf8');
}

test('E2E fixture entry delegates validation and credential resolution to the formal validator', async () => {
  const source = await readSpec();

  assert.match(source, /from ['"]\.\.\/\.\.\/\.\.\/scripts\/validate_identity_single_corp_e2e_fixture\.mjs['"]/);
  assert.match(source, /\bvalidateFixture\(value\)/);
  assert.match(source, /\bresolveFixtureCredentials\(reference, process\.env\)/);
  assert.match(source, /tenantCorpBindings:\s*Array</);
  assert.match(source, /expectedSaasGovernance:\s*ExpectedSaasGovernance/);
  assert.doesNotMatch(source, /tenantCorpBindings\?\s*:/);
  assert.doesNotMatch(source, /const\s+required\s*=\s*\[/);
  assert.doesNotMatch(source, /Object\.entries\([^\n]*fixture/);
});

test('E2E entry keeps fail-closed realm and machine-code contracts explicit', async () => {
  const source = await readSpec();

  assert.match(source, /parseStoredToken\(value, realm\)/);
  assert.match(source, /readStorage\(page, saasTokenKey, 'saas'\)/);
  assert.match(source, /readStorage\(page, dashboardTokenKey, 'dashboard'\)/);
  assert.match(source, /machineCode\(deniedBody\)\)\.toBe\('TENANT_ACCESS_DENIED'\)/);
  assert.match(source, /machineCode\(ordinaryBody\)\)\.toBe\('DASHBOARD_PERMISSION_DENIED'\)/);
  assert.doesNotMatch(source, /TENANT_ACCESS_DENIED[^\n]*\|\|[^\n]*403/);
  assert.doesNotMatch(source, /DASHBOARD_PERMISSION_DENIED[^\n]*\|\|[^\n]*403/);
  assert.match(source, /machineCode\(body\)\s*!==\s*'CORP_CONFIGURATION_REQUIRED'/);
  assert.match(source, /test\.skip\(!liveBase \|\| !liveFixture,[\s\S]*SKIP:/);
});
