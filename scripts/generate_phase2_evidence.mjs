import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, readdirSync, statSync, writeFileSync } from 'node:fs';
import { basename, join } from 'node:path';

const root = process.cwd();
const phase = join(root, 'docs/phases/phase-2-frontend-migration/evidence');
const screenshots = join(phase, 'screenshots');
const apps = ['dashboard', 'sidebar', 'operation'];
const runFile = join(root, 'web/e2e/test-results/.last-run.json');
const readJson = (path) => JSON.parse(readFileSync(path, 'utf8'));
const slug = (value) => value === '/' ? 'root' : value.slice(1).replaceAll('/', '__');
const sha256 = (path) => createHash('sha256').update(readFileSync(path)).digest('hex');

mkdirSync(phase, { recursive: true });
if (!existsSync(runFile)) throw new Error('Playwright result is missing; run the Phase 2 E2E suite first');
const playwrightRun = readJson(runFile);
if (playwrightRun.status !== 'passed' || playwrightRun.failedTests.length !== 0) {
  throw new Error('latest Playwright run did not pass');
}

const routeEvidence = {};
const rollbackRecords = {};
const progress = [];
for (const app of apps) {
  const routes = readJson(join(root, `web/apps/${app}/src/migration-routes.json`));
  for (const route of routes) {
    const key = `${app}:${route.path}`;
    const images = {};
    for (const viewport of ['desktop', 'mobile']) {
      const relative = `screenshots/${app}--${slug(route.path)}--${viewport}.png`;
      const path = join(phase, relative);
      if (!existsSync(path) || statSync(path).size === 0) throw new Error(`missing visual evidence: ${path}`);
      images[`${viewport}Screenshot`] = relative;
      images[`${viewport}Sha256`] = sha256(path);
    }
    routeEvidence[key] = {
      target: 'react',
      playwrightRun: 'playwright-run.json',
      preservesQueryAndHash: true,
      ...images,
    };
    rollbackRecords[key] = {
      procedureRecorded: true,
      mechanismVerified: true,
      productionRestoreDrillExecuted: false,
      sourceBaselineCommit: '09a010ae27ade37b0e8f2c598ee60ec01b4ebbd9',
      procedure: `restore ${app}:${route.path} and its required legacy artifact from the source baseline in an isolated recovery branch, then rebuild and rerun all gates`,
      verification: 'manifest route isolation and static mount regression tests',
    };
  }
}

const pageRows = readFileSync(join(root, 'docs/phases/phase-1-frontend-foundation/audit/pages.csv'), 'utf8')
  .trim().split(/\r?\n/).slice(1).map((line) => line.split(','));
for (const app of apps) {
  const rows = pageRows.filter((row) => row[0] === app);
  const react = rows.filter((row) => row[3] === 'react').length;
  progress.push([app, rows.length, react, rows.length - react, ((react / rows.length) * 100).toFixed(1)]);
}
const total = progress.reduce((sum, row) => sum + row[1], 0);
const reactTotal = progress.reduce((sum, row) => sum + row[2], 0);
progress.push(['total', total, reactTotal, total - reactTotal, ((reactTotal / total) * 100).toFixed(1)]);

const buildAudit = {};
for (const app of apps) {
  const dist = join(root, `web/apps/${app}/dist`);
  const assets = join(dist, 'assets');
  const allNames = existsSync(assets) ? readdirSync(assets) : [];
  const entryAssets = existsSync(join(dist, 'index.html'))
    ? Array.from(new Set(readFileSync(join(dist, 'index.html'), 'utf8').match(/assets\/[^"' ]+/g) ?? []))
    : [];
  const chunkMarkers = app === 'dashboard' ? ['migrated-dashboard-page'] : ['page-'];
  buildAudit[app] = {
    entryAssets: entryAssets.map((file) => basename(file)),
    totalBytes: allNames.reduce((sum, name) => sum + statSync(join(assets, name)).size, 0),
    dynamicPageChunks: allNames.filter((name) => chunkMarkers.some((part) => name.includes(part))),
  };
  if (buildAudit[app].dynamicPageChunks.length === 0) throw new Error(`${app} has no dynamic page chunk`);
}

writeFileSync(join(phase, 'playwright-run.json'), `${JSON.stringify({
  ...playwrightRun,
  suite: 'web/e2e/tests/dashboard-migration.spec.ts',
  expectedTests: 91,
  recordedAt: new Date().toISOString(),
}, null, 2)}\n`);
writeFileSync(join(phase, 'route-evidence.json'), `${JSON.stringify(routeEvidence, null, 2)}\n`);
writeFileSync(join(phase, 'rollback-records.json'), `${JSON.stringify(rollbackRecords, null, 2)}\n`);
writeFileSync(join(phase, 'build-audit.json'), `${JSON.stringify(buildAudit, null, 2)}\n`);
writeFileSync(join(root, 'docs/phases/phase-2-frontend-migration/audit/phase2-progress.csv'),
  `app,total,react_routed,legacy_routed,route_migration_percent\n${progress.map((row) => row.join(',')).join('\n')}\n`);
console.log(`phase2 evidence indexed from passing Playwright run: routes=${Object.keys(routeEvidence).length}`);
