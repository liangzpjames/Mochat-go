import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const helper = readFileSync('scripts/lib/migration_inventory_smoke.sh', 'utf8');
const wrappers = ['scripts/smoke_schema_migrate.sh', 'scripts/smoke_mysql57_schema_migrate.sh']
  .map((file) => [file, readFileSync(file, 'utf8')]);

test('both database smoke entrypoints consume the shared runtime inventory lifecycle', () => {
  for (const [file, source] of wrappers) {
    assert.match(source, /migration_inventory_smoke\.sh/, file);
    assert.match(source, /run_migration_inventory_smoke/, file);
    assert.doesNotMatch(source, /0098_scrm_lead_foundation/, file);
    assert.doesNotMatch(source, /down\s+-v|down\s+--volumes/, file);
  }
});

test('inventory lifecycle derives count first latest checksum and kind without a fixed latest version', () => {
  for (const token of ['-action inventory', 'INVENTORY_COUNT', 'INVENTORY_FIRST', 'INVENTORY_LATEST', 'INVENTORY_LATEST_CHECKSUM', 'INVENTORY_LATEST_KIND']) {
    assert.ok(helper.includes(token), `missing ${token}`);
  }
  assert.doesNotMatch(helper, /0098_scrm_lead_foundation/);
  assert.match(helper, /NR\s*!=\s*\(version \+ 0\)/);
});

test('inventory lifecycle verifies full ledger status latest rollback-reapply baseline and checksum drift', () => {
  for (const token of ['-action status', '-action rollback', 'rolled_back', '-action apply', '-action baseline', 'checksum drift', 'mochat_go_schema_migrations']) {
    assert.ok(helper.includes(token), `missing ${token}`);
  }
});
