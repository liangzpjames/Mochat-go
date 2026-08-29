import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';

import { buildPhase2ProgressReport, readPhase2Inventory } from './check_phase2_frontend_progress.mjs';

function fixture(t, manifests) {
  const root = mkdtempSync(join(tmpdir(), 'phase2-progress-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  for (const app of ['dashboard', 'sidebar', 'operation']) {
    const directory = join(root, 'web', 'apps', app, 'src');
    mkdirSync(directory, { recursive: true });
    writeFileSync(join(directory, 'migration-routes.json'), JSON.stringify(manifests[app] ?? []));
  }
  return root;
}

test('builds a stable Phase 2 progress report independent of source row order', () => {
  const rows = [
    { app: 'operation', route: '/legacy', status: 'legacy' },
    { app: 'dashboard', route: '/react', status: 'react' },
    { app: 'sidebar', route: '/candidate', status: 'candidate' },
    { app: 'dashboard', route: '/blocked', status: 'blocked' },
  ];
  const expected = [
    'app,total,react,legacy,candidate,blocked,percent',
    'dashboard,2,1,0,0,1,50.0',
    'sidebar,1,0,0,1,0,0.0',
    'operation,1,0,1,0,0,0.0',
    'total,4,1,1,1,1,25.0',
    '',
  ].join('\n');
  assert.equal(buildPhase2ProgressReport(rows).content, expected);
  assert.equal(buildPhase2ProgressReport([...rows].reverse()).content, expected);
});

test('reads all three live migration manifests and additions/removals change the report', (t) => {
  const root = fixture(t, {
    dashboard: [{ path: '/a', target: 'react' }],
    sidebar: [{ path: '/b', target: 'legacy' }],
    operation: [{ path: '/c', target: 'candidate' }],
  });
  const before = buildPhase2ProgressReport(readPhase2Inventory(root));
  assert.equal(before.total.total, 3);
  writeFileSync(join(root, 'web', 'apps', 'dashboard', 'src', 'migration-routes.json'), JSON.stringify([{ path: '/added', target: 'blocked' }, { path: '/a', target: 'react' }]));
  const added = buildPhase2ProgressReport(readPhase2Inventory(root));
  assert.equal(added.total.total, 4);
  assert.notEqual(added.content, before.content);
  writeFileSync(join(root, 'web', 'apps', 'sidebar', 'src', 'migration-routes.json'), '[]');
  assert.equal(buildPhase2ProgressReport(readPhase2Inventory(root)).total.total, 3);
});

test('rejects unknown status, unknown app and duplicate routes', () => {
  assert.throws(() => buildPhase2ProgressReport([{ app: 'dashboard', route: '/a', status: 'invented' }]), /unknown status/);
  assert.throws(() => buildPhase2ProgressReport([{ app: 'saas-admin', route: '/a', status: 'react' }]), /unknown app/);
  assert.throws(() => buildPhase2ProgressReport([
    { app: 'dashboard', route: '/a', status: 'react' },
    { app: 'dashboard', route: '/a', status: 'legacy' },
  ]), /duplicate route/);
});

test('rejects a manifest entry that lies about its owning app', (t) => {
  const root = fixture(t, { dashboard: [{ app: 'sidebar', path: '/a', target: 'react' }] });
  assert.throws(() => readPhase2Inventory(root), /app mismatch/);
});
