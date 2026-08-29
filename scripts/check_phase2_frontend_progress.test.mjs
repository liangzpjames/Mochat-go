import assert from 'node:assert/strict';
import test from 'node:test';

import { buildPhase2ProgressReport } from './check_phase2_frontend_progress.mjs';

test('builds a stable Phase 2 progress report independent of source row order', () => {
  const rows = [
    { app: 'operation', status: 'legacy' },
    { app: 'dashboard', status: 'react' },
    { app: 'sidebar', status: 'candidate' },
    { app: 'dashboard', status: 'blocked' },
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

test('does not treat an unknown status as migrated', () => {
  const result = buildPhase2ProgressReport([{ app: 'dashboard', status: 'invented' }]);
  assert.equal(result.total.react, 0);
  assert.equal(result.total.total, 1);
});
