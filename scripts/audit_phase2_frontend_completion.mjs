import assert from 'node:assert/strict';
import { existsSync, readFileSync, statSync } from 'node:fs';

const readJson = (path) => JSON.parse(readFileSync(path, 'utf8'));
const evidence = readJson('docs/phases/phase-2-frontend-migration/evidence/route-evidence.json');
const rollback = readJson('docs/phases/phase-2-frontend-migration/evidence/rollback-records.json');
const playwrightRun = readJson('docs/phases/phase-2-frontend-migration/evidence/playwright-run.json');
assert.equal(playwrightRun.status, 'passed', 'recorded Playwright run did not pass');
assert.deepEqual(playwrightRun.failedTests, [], 'recorded Playwright run has failures');
let routeCount = 0;

for (const app of ['dashboard', 'sidebar', 'operation']) {
  const routes = readJson(`web/apps/${app}/src/migration-routes.json`);
  assert.ok(routes.length > 0, `${app} manifest is empty`);
  assert.equal(routes.some((route) => route.target !== 'react'), false, `${app} retains legacy targets`);
  for (const route of routes) {
    routeCount += 1;
    const key = `${app}:${route.path}`;
    assert.equal(evidence[key]?.playwrightRun, 'playwright-run.json', `${key} lacks Playwright evidence`);
    assert.equal(rollback[key]?.procedureRecorded, true, `${key} lacks rollback procedure`);
    for (const property of ['desktopScreenshot', 'mobileScreenshot']) {
      const path = `docs/phases/phase-2-frontend-migration/evidence/${evidence[key][property]}`;
      assert.ok(existsSync(path) && statSync(path).size > 0, `${key} lacks ${property}`);
      const hashProperty = property.replace('Screenshot', 'Sha256');
      assert.match(evidence[key][hashProperty] ?? '', /^[a-f0-9]{64}$/, `${key} lacks screenshot hash`);
    }
  }
  assert.ok(existsSync(`web/apps/${app}/dist/index.html`), `${app} production build is missing`);
}

for (const retired of ['web/dashboard/dist', 'web/sidebar/dist', 'web/operation/dist']) {
  assert.equal(existsSync(retired), false, `${retired} legacy build artifact still exists`);
}

const runtimeFiles = [
  'Dockerfile',
  'internal/config/config.go',
  'cmd/mochat-go/main.go',
  'scripts/frontend_check.sh',
  'scripts/frontend_check.ps1',
];
for (const file of runtimeFiles) {
  const source = readFileSync(file, 'utf8');
  assert.doesNotMatch(source, /web\/(?:dashboard|sidebar|operation)\/dist/, `${file} references a retired dist`);
}

console.log(`phase2 frontend completion: routes=${routeCount} legacy_targets=0 screenshots=${routeCount * 2}`);
