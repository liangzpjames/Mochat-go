import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const readJson = (path) => JSON.parse(readFileSync(path, 'utf8'));

test('all three application manifests contain React-only routes', () => {
  for (const app of ['dashboard', 'sidebar', 'operation']) {
    const routes = readJson(`web/apps/${app}/src/migration-routes.json`);
    assert.ok(routes.length > 0, `${app} manifest must not be empty`);
    assert.deepEqual(
      routes.filter((route) => route.target !== 'react'),
      [],
      `${app} retains a legacy route`,
    );
  }
});

test('every manifest route has Playwright and rollback evidence rows', () => {
  const evidence = readJson('docs/phases/phase-2-frontend-migration/evidence/route-evidence.json');
  const rollback = readJson('docs/phases/phase-2-frontend-migration/evidence/rollback-records.json');
  for (const app of ['dashboard', 'sidebar', 'operation']) {
    const routes = readJson(`web/apps/${app}/src/migration-routes.json`);
    for (const route of routes) {
      const key = `${app}:${route.path}`;
      assert.equal(evidence[key]?.playwrightRun, 'playwright-run.json', `${key} lacks Playwright evidence`);
      assert.match(evidence[key]?.desktopScreenshot ?? '', /\.png$/, `${key} lacks desktop evidence`);
      assert.match(evidence[key]?.mobileScreenshot ?? '', /\.png$/, `${key} lacks mobile evidence`);
      assert.equal(rollback[key]?.procedureRecorded, true, `${key} lacks rollback procedure`);
      assert.equal(rollback[key]?.productionRestoreDrillExecuted, false);
    }
  }
});

test('Docker builds every shipped React application', () => {
  const dockerfile = readFileSync('Dockerfile', 'utf8');
  for (const app of ['dashboard', 'sidebar', 'operation', 'saas-admin']) {
    assert.match(dockerfile, new RegExp(`pnpm --filter @mochat/${app} build`));
  }
});
