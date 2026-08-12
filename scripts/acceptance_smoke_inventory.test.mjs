import assert from 'node:assert/strict';
import { readFile, readdir, stat } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import test from 'node:test';

const root = path.resolve(fileURLToPath(new URL('.', import.meta.url)), '..');

test('restored smoke inventory covers every script and keeps retired contracts out of active acceptance', async () => {
  const inventory = JSON.parse(await readFile(path.join(root, 'scripts', 'acceptance_smoke_inventory.json'), 'utf8'));
  const deferred = inventory.deferredHistorical;
  assert.ok(Array.isArray(deferred));
  assert.ok(deferred.length >= 60);
  assert.match(inventory.deferredReason, /旧 bootstrap|corp\/login|保留/);
  assert.equal(new Set(deferred).size, deferred.length);

  const existing = (await readdir(path.join(root, 'scripts')))
    .filter((name) => /^smoke_.*\.sh$/.test(name))
    .sort();
  const acceptance = await readFile(path.join(root, 'scripts', 'standalone_acceptance.sh'), 'utf8');
  const active = [...acceptance.matchAll(/^\s*run_step\s+"[^"]+"\s+\.\/scripts\/(smoke_[A-Za-z0-9_]+\.sh)/gm)]
    .map((match) => match[1]);
  assert.equal(new Set(active).size, active.length);
  assert.deepEqual([...new Set([...active, ...deferred])].sort(), existing);

  for (const name of deferred) {
    const body = await readFile(path.join(root, 'scripts', name), 'utf8');
    assert.match(body, /^#!\/usr\/bin\/env bash/);
    assert.ok(body.length > 200, `${name} must remain a real smoke script`);
  }
  for (const name of active) {
    const body = await readFile(path.join(root, 'scripts', name), 'utf8');
    assert.doesNotMatch(body, /mochat-bootstrap|\/security\/login|\/dashboard\/corp\/|\s-secret\b/);
  }
  await stat(path.join(root, 'scripts', 'smoke_saas_backup_recovery.sh'));
  await stat(path.join(root, 'scripts', 'smoke_saas_admin_access_rbac.sh'));
  await stat(path.join(root, 'scripts', 'smoke_channel_code_dashboard.sh'));
  await stat(path.join(root, 'scripts', 'smoke_room_calendar_dashboard.sh'));
  for (const name of [
    'smoke_saas_backup_recovery.sh',
    'smoke_saas_billing_invoices.sh',
    'smoke_saas_compliance_lifecycle.sh',
    'smoke_saas_payment_collection.sh',
    'smoke_saas_admin_access_rbac.sh',
  ]) {
    const body = await readFile(path.join(root, 'scripts', name), 'utf8');
    assert.match(body, /\/dashboard\/saas(?:Admin|Billing)\//, name + ' must retain SaaS business coverage');
  }
  for (const name of [
    'smoke_channel_code_dashboard.sh',
    'smoke_room_calendar_dashboard.sh',
    'smoke_sop_dashboard.sh',
    'smoke_work_fission_dashboard.sh',
  ]) {
    const body = await readFile(path.join(root, 'scripts', name), 'utf8');
    assert.match(body, /\/dashboard\/(?:channelCode|roomCalendar|contactSop|roomSop|workFission)\//i, name + ' must retain Dashboard business coverage');
  }
});
