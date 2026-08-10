import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

test('smoke script is safe, bearer-authenticated, and Windows PowerShell compatible', async () => {
  const source = await readFile(new URL('./smoke_dashboard_page_rbac.ps1', import.meta.url), 'utf8');
  assert.match(source, /BaseUrl = "http:\/\/127\.0\.0\.1:18080"/);
  assert.match(source, /dashboard\/user\/auth/);
  assert.match(source, /Authorization/);
  assert.match(source, /COUNT\(\*\)/);
  assert.match(source, /volumes-before|volumes-after/);
  assert.match(source, /container-ids/);
  assert.match(source, /MutationJson/);
  assert.match(source, /if \(-not \$ReadOnly -and \$TargetUserId/);
  assert.match(source, /CrossTenantUserId/);
  assert.doesNotMatch(source, /\?\?/);
  assert.doesNotMatch(source, /down\s+-v|volume\s+rm|system\s+prune|volume\s+prune/i);
});
