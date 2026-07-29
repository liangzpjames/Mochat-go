import { readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';

const root = process.cwd();
const source = resolve(root, 'docs/phases/phase-1-frontend-foundation/audit/pages.csv');
const report = resolve(root, 'docs/phases/phase-2-frontend-migration/audit/phase2-progress.csv');
const rows = readFileSync(source, 'utf8').trim().split(/\r?\n/).slice(1)
  .map((line) => {
    const [app, sourceFile, route, status, risk, batch] = line.split(',');
    return { app, sourceFile, route, status, risk, batch };
  });
const apps = ['dashboard', 'sidebar', 'operation'];
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
const header = 'app,total,react,legacy,candidate,blocked,percent\n';
const expected = header + [...summaries, total]
  .map((row) => `${row.app},${row.total},${row.react},${row.legacy},${row.candidate},${row.blocked},${row.percent}`)
  .join('\n') + '\n';

if (process.argv.includes('--refresh')) writeFileSync(report, expected);
const actual = readFileSync(report, 'utf8').replaceAll('\r\n', '\n');
if (actual !== expected) {
  console.error('phase2-progress: report is stale; run npm run refresh:phase2-progress');
  process.exitCode = 1;
} else {
  console.log(`phase2-progress: ${total.react}/${total.total} React (${total.percent}%), ${total.legacy} legacy`);
}
