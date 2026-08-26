import assert from 'node:assert/strict';
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
