import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import {
  assertPhase32TestsExecuted,
  createContainerName,
  phase32ScenarioTests,
  validateCurrentPhase32Tests,
} from './check_phase3_2_mysql_integration.mjs';

const source = readFileSync(new URL('./check_phase3_2_mysql_integration.mjs', import.meta.url), 'utf8');
const leadFixtureSource = readFileSync(new URL('../internal/modules/scrm/adapters/mysql/lead_repository_integration_test.go', import.meta.url), 'utf8');
const opportunityFixtureSource = readFileSync(new URL('../internal/modules/scrm/adapters/mysql/opportunity_repository_integration_test.go', import.meta.url), 'utf8');
const corpDataFixtureSource = readFileSync(new URL('../internal/store/corp_data_test.go', import.meta.url), 'utf8');

test('Phase 3.2 MySQL runner uses an ephemeral container without Compose or Docker volumes', () => {
  assert.doesNotMatch(source, /docker[\s\S]*compose|composeArgs|down[^\n]*-v/);
  assert.doesNotMatch(source, /['"](?:-v|--volume)['"]/);
  assert.match(source, /['"]run['"]/);
  assert.match(source, /['"]--rm['"]/);
  assert.match(source, /['"]--name['"]/);
  assert.match(source, /127\.0\.0\.1::3306/);
  assert.doesNotMatch(source, /--mount|type=bind|docker-entrypoint-initdb|phase32SnapshotFiles/);
});

test('Phase 3.2 MySQL runner executes current-registry integration scenarios and always cleans up', () => {
  assert.match(source, /['"]-tags=integration['"]/);
  assert.match(source, /MOCHAT_GO_MYSQL_INTEGRATION_DSN:\s*adminDSN/);
  assert.match(source, /['"]stop['"]/);
  assert.match(source, /['"]rm['"],\s*['"]-f['"]/);
});

test('Phase 3.2 container names are unique and scoped', () => {
  const first = createContainerName(123, 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa');
  const second = createContainerName(123, 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb');
  assert.equal(first, 'mochat-go-phase32-mysql-123-aaaaaaaaaaaa');
  assert.notEqual(first, second);
});

test('Phase 3.2 scenario fixtures use the current production migration registry', () => {
  assert.match(leadFixtureSource, /integrationtestdb\.NewIsolated/);
  assert.match(leadFixtureSource, /testharness\.NewControlledEvidence/);
  assert.match(leadFixtureSource, /testharness\.ApplyLatest/);
  assert.match(opportunityFixtureSource, /mysqlIntegrationDB\(t\)/);
  assert.match(corpDataFixtureSource, /newCurrentStoreIntegrationDB\(t\)/);
  assert.equal(phase32ScenarioTests.length, 9);
  assert.doesNotThrow(() => validateCurrentPhase32Tests(process.cwd()));
});

test('Phase 3.2 runner fails closed unless go test JSON proves every pinned test ran and passed', () => {
  const events = phase32ScenarioTests.flatMap((item) => [
    JSON.stringify({ Action: 'run', Package: item.package, Test: item.name }),
    JSON.stringify({ Action: 'pass', Package: item.package, Test: item.name }),
  ]).join('\n');
  assert.doesNotThrow(() => assertPhase32TestsExecuted(events));
  assert.throws(
    () => assertPhase32TestsExecuted(events.split('\n').slice(0, -2).join('\n')),
    /did not run and pass/,
  );
});
