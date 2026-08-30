import { execFileSync, spawnSync } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

const rootPassword = 'mochat_root';

export const phase32ScenarioTests = Object.freeze([
    { package: 'jiyi/mochat-go/internal/modules/scrm/adapters/mysql', source: 'internal/modules/scrm/adapters/mysql/lead_repository_integration_test.go', name: 'TestLeadRepositoryTenantIsolation' },
    { package: 'jiyi/mochat-go/internal/modules/scrm/adapters/mysql', source: 'internal/modules/scrm/adapters/mysql/lead_repository_integration_test.go', name: 'TestLeadRepositoryCreateOrGetIsIdempotent' },
    { package: 'jiyi/mochat-go/internal/modules/scrm/adapters/mysql', source: 'internal/modules/scrm/adapters/mysql/lead_repository_integration_test.go', name: 'TestLeadRepositoryAllowsSameBusinessKeyAcrossTenants' },
    { package: 'jiyi/mochat-go/internal/modules/scrm/adapters/mysql', source: 'internal/modules/scrm/adapters/mysql/lead_repository_integration_test.go', name: 'TestLeadRepositoryListUsesStableCreatedAtAndIDOrder' },
    { package: 'jiyi/mochat-go/internal/modules/scrm/adapters/mysql', source: 'internal/modules/scrm/adapters/mysql/lead_repository_integration_test.go', name: 'TestLeadRepositoryListIncludesMaximumMySQLTimestampOnFirstPage' },
    { package: 'jiyi/mochat-go/internal/modules/scrm/adapters/mysql', source: 'internal/modules/scrm/adapters/mysql/lead_repository_integration_test.go', name: 'TestLeadRepositoryConcurrentCreateProducesOneRow' },
    { package: 'jiyi/mochat-go/internal/modules/scrm/adapters/mysql', source: 'internal/modules/scrm/adapters/mysql/lead_repository_integration_test.go', name: 'TestIntegrationRepositoryPreservesOtherRowsAndCleansOwnTenant' },
    { package: 'jiyi/mochat-go/internal/modules/scrm/adapters/mysql', source: 'internal/modules/scrm/adapters/mysql/customer_tag_integration_test.go', name: 'TestCustomerTagMariaDBCatalogIsolationVersionsAndIdempotency' },
    { package: 'jiyi/mochat-go/internal/modules/scrm/adapters/mysql', source: 'internal/modules/scrm/adapters/mysql/opportunity_repository_integration_test.go', name: 'TestOpportunityRepositoryListUsesPersistedStageID' },
    { package: 'jiyi/mochat-go/internal/store', source: 'internal/store/corp_data_test.go', name: 'TestIntegrationCorpDataScopedQueriesExecute' },
]);

const run = (root, file, args, env = process.env) => execFileSync(file, args, {
  cwd: root,
  env,
  stdio: 'inherit',
});

const capture = (root, file, args) => execFileSync(file, args, {
  cwd: root,
  env: process.env,
  encoding: 'utf8',
});

const captureResult = (root, file, args, env) => spawnSync(file, args, {
  cwd: root,
  env,
  encoding: 'utf8',
  maxBuffer: 64 * 1024 * 1024,
});

const pause = (milliseconds) => {
  Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, milliseconds);
};

export const createContainerName = (pid = process.pid, uuid = randomUUID()) =>
  `mochat-go-phase32-mysql-${pid}-${uuid.replaceAll('-', '').slice(0, 12)}`;

export const validateCurrentPhase32Tests = (root) => {
  const missing = phase32ScenarioTests.filter((item) => {
    const current = readFileSync(join(root, item.source), 'utf8');
    return !new RegExp(`^func ${item.name}\\(`, 'm').test(current);
  });
  if (missing.length > 0) {
    throw new Error(`Phase 3.2 scenario tests missing from current tree: ${missing.map((item) => item.name).join(', ')}`);
  }
};

export const assertPhase32TestsExecuted = (jsonOutput) => {
  const runs = new Set();
  const passes = new Set();
  for (const line of jsonOutput.split(/\r?\n/)) {
    if (line.trim() === '') continue;
    let event;
    try { event = JSON.parse(line); } catch { continue; }
    if (!event.Package || !event.Test || event.Test.includes('/')) continue;
    const key = `${event.Package}\0${event.Test}`;
    if (event.Action === 'run') runs.add(key);
    if (event.Action === 'pass') passes.add(key);
  }
  const missing = phase32ScenarioTests.filter((item) => {
    const key = `${item.package}\0${item.name}`;
    return !runs.has(key) || !passes.has(key);
  });
  if (missing.length > 0) {
    throw new Error(`Phase 3.2 scenario tests did not run and pass: ${missing.map((item) => item.name).join(', ')}`);
  }
};

const cleanupContainer = (root, containerName) => {
  const stop = captureResult(root, 'docker', ['stop', '--timeout', '10', containerName], process.env);
  const stopOutput = `${stop.stdout ?? ''}${stop.stderr ?? ''}`;
  if (stop.status === 0 || /No such container/i.test(stopOutput)) return;
  process.stderr.write(stopOutput);
  const remove = captureResult(root, 'docker', ['rm', '-f', containerName], process.env);
  const removeOutput = `${remove.stdout ?? ''}${remove.stderr ?? ''}`;
  if (remove.status !== 0 && !/No such container/i.test(removeOutput)) process.stderr.write(removeOutput);
};

export const main = () => {
  const root = process.cwd();
  const containerName = createContainerName();
  let containerStarted = false;

  try {
    validateCurrentPhase32Tests(root);

    run(root, 'docker', [
      'run', '--rm', '--detach',
      '--name', containerName,
      '--env', `MARIADB_ROOT_PASSWORD=${rootPassword}`,
      '--publish', '127.0.0.1::3306',
      'mariadb:10.6',
      '--character-set-server=utf8mb4',
      '--collation-server=utf8mb4_unicode_ci',
    ]);
    containerStarted = true;

    let healthy = false;
    for (let attempt = 0; attempt < 90; attempt += 1) {
      try {
        run(root, 'docker', [
          'exec', containerName,
          'mariadb-admin', 'ping', '-h', '127.0.0.1', '-uroot', `-p${rootPassword}`, '--silent',
        ]);
        healthy = true;
        break;
      } catch {
        pause(2000);
      }
    }
    if (!healthy) {
      try { run(root, 'docker', ['logs', '--tail', '100', containerName]); } catch {}
      throw new Error('Phase 3.2 MariaDB did not become ready');
    }

    const portOutput = capture(root, 'docker', ['port', containerName, '3306/tcp']).trim();
    const portMatch = /127\.0\.0\.1:(\d+)/.exec(portOutput);
    if (!portMatch) throw new Error(`Cannot resolve Phase 3.2 MariaDB host port from: ${portOutput}`);
    const adminDSN = `root:${rootPassword}@tcp(127.0.0.1:${portMatch[1]})/mysql?parseTime=true&multiStatements=true&loc=Local`;

    const testPattern = `^(?:${phase32ScenarioTests.map((item) => item.name).join('|')})$`;
    const testResult = captureResult(root, 'go', [
      'test', '-json', '-tags=integration', '-count=1', `-run=${testPattern}`,
      './internal/modules/scrm/adapters/mysql', './internal/store',
    ], {
      ...process.env,
      MOCHAT_GO_MYSQL_INTEGRATION_DSN: adminDSN,
    });
    process.stdout.write(testResult.stdout ?? '');
    process.stderr.write(testResult.stderr ?? '');
    if (testResult.status !== 0) throw new Error(`Phase 3.2 go test failed with exit code ${testResult.status}`);
    assertPhase32TestsExecuted(testResult.stdout ?? '');
    console.log('Phase 3.2 isolated MariaDB integration passed (10/10 scenarios, current production migration registry).');
  } finally {
    if (containerStarted) {
      cleanupContainer(root, containerName);
    }
  }
};

const entrypoint = process.argv[1] ? pathToFileURL(resolve(process.argv[1])).href : '';
if (import.meta.url === entrypoint) main();
