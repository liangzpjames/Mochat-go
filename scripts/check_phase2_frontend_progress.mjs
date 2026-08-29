import { readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

const apps = ['dashboard', 'sidebar', 'operation'];

export function buildPhase2ProgressReport(rows) {
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
  return { content, summaries, total };
}

export function readPhase2Inventory(source) {
  return readFileSync(source, 'utf8').trim().split(/\r?\n/).slice(1)
    .map((line) => {
      const [app, sourceFile, route, status, risk, batch] = line.split(',');
      return { app, sourceFile, route, status, risk, batch };
    });
}

function main() {
  const root = process.cwd();
  const source = resolve(root, 'docs/phases/phase-1-frontend-foundation/audit/pages.csv');
  const report = resolve(root, 'docs/phases/phase-2-frontend-migration/audit/phase2-progress.csv');
  const result = buildPhase2ProgressReport(readPhase2Inventory(source));
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
