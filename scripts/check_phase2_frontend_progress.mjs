import { readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

const apps = ['dashboard', 'sidebar', 'operation'];
const statuses = new Set(['react', 'legacy', 'candidate', 'blocked']);

function validateRows(rows) {
  const seen = new Set();
  return rows.map((row, index) => {
    if (!apps.includes(row.app)) throw new Error(`phase2-progress: unknown app ${JSON.stringify(row.app)} at row ${index + 1}`);
    if (!statuses.has(row.status)) throw new Error(`phase2-progress: unknown status ${JSON.stringify(row.status)} for ${row.app} ${row.route}`);
    if (typeof row.route !== 'string' || !row.route.startsWith('/')) throw new Error(`phase2-progress: invalid route for ${row.app} at row ${index + 1}`);
    const key = `${row.app} ${row.route}`;
    if (seen.has(key)) throw new Error(`phase2-progress: duplicate route ${key}`);
    seen.add(key);
    return { app: row.app, route: row.route, status: row.status };
  }).sort((left, right) => apps.indexOf(left.app) - apps.indexOf(right.app) || left.route.localeCompare(right.route));
}

export function buildPhase2ProgressReport(inputRows) {
  const rows = validateRows(inputRows);
  const summaries = apps.map((app) => {
    const scoped = rows.filter((row) => row.app === app);
    const count = (status) => scoped.filter((row) => row.status === status).length;
    const react = count('react');
    return {
      app, total: scoped.length, react, legacy: count('legacy'), candidate: count('candidate'),
      blocked: count('blocked'), percent: scoped.length === 0 ? '0.0' : (react * 100 / scoped.length).toFixed(1),
    };
  });
  const total = {
    app: 'total',
    total: summaries.reduce((sum, row) => sum + row.total, 0),
    react: summaries.reduce((sum, row) => sum + row.react, 0),
    legacy: summaries.reduce((sum, row) => sum + row.legacy, 0),
    candidate: summaries.reduce((sum, row) => sum + row.candidate, 0),
    blocked: summaries.reduce((sum, row) => sum + row.blocked, 0),
  };
  total.percent = total.total === 0 ? '0.0' : (total.react * 100 / total.total).toFixed(1);
  const content = 'app,total,react,legacy,candidate,blocked,percent\n' + [...summaries, total]
    .map((row) => `${row.app},${row.total},${row.react},${row.legacy},${row.candidate},${row.blocked},${row.percent}`)
    .join('\n') + '\n';
  return { content, summaries, total, rows };
}

export function readPhase2Inventory(root = process.cwd()) {
  return apps.flatMap((app) => {
    const source = resolve(root, 'web', 'apps', app, 'src', 'migration-routes.json');
    let manifest;
    try { manifest = JSON.parse(readFileSync(source, 'utf8')); }
    catch (error) { throw new Error(`phase2-progress: cannot parse ${source}: ${error.message}`); }
    if (!Array.isArray(manifest)) throw new Error(`phase2-progress: ${source} must contain a JSON array`);
    return manifest.map((entry, index) => {
      if (!entry || typeof entry !== 'object' || Array.isArray(entry)) throw new Error(`phase2-progress: invalid entry ${app}[${index}]`);
      if ('app' in entry && entry.app !== app) throw new Error(`phase2-progress: app mismatch ${app}[${index}] declares ${JSON.stringify(entry.app)}`);
      return { app, route: entry.path, status: entry.target };
    });
  });
}

function main() {
  const root = process.cwd();
  const report = resolve(root, 'docs/phases/phase-2-frontend-migration/audit/phase2-progress.csv');
  const result = buildPhase2ProgressReport(readPhase2Inventory(root));
  if (process.argv.includes('--refresh')) writeFileSync(report, result.content);
  const actual = readFileSync(report, 'utf8').replaceAll('\r\n', '\n');
  if (actual !== result.content) {
    console.error('phase2-progress: report is stale; run pnpm refresh:phase2-progress');
    process.exitCode = 1;
    return;
  }
  console.log(`phase2-progress: ${result.total.react}/${result.total.total} React (${result.total.percent}%), ${result.total.legacy} legacy`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) main();
