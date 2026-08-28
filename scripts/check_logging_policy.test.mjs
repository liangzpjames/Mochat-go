import assert from 'node:assert/strict';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

import { checkLoggingPolicy } from './check_logging_policy.mjs';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

test('logging policy rejects noisy and sensitive regressions', () => {
  const result = checkLoggingPolicy(repoRoot);
  assert.deepEqual(result.violations, []);
});
