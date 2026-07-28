import assert from 'node:assert/strict';
import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';

import { checkDocumentation } from './check_documentation_structure.mjs';

function createRoot() {
  return mkdtempSync(join(tmpdir(), 'mochat-docs-check-'));
}

function write(root, path, content = '# Test\n') {
  const target = join(root, path);
  mkdirSync(join(target, '..'), { recursive: true });
  writeFileSync(target, content);
}

test('requires the project progress entry and phase readmes', () => {
  const root = createRoot();
  mkdirSync(join(root, 'docs'), { recursive: true });

  const result = checkDocumentation(root);

  assert.deepEqual(result.missingRequired, [
    'docs/PROJECT_PROGRESS.zh-CN.md',
    'docs/phases/phase-pre0-standalone/README.md',
    'docs/phases/phase-0-go-foundation/README.md',
    'docs/phases/phase-1-frontend-foundation/README.md',
    'docs/phases/phase-2-frontend-migration/README.md',
  ]);
});

test('rejects duplicate and zero-byte evidence artifacts', () => {
  const root = createRoot();
  write(
    root,
    'docs/evidence/production/current/env 2.production-evidence',
    'duplicate\n',
  );
  write(
    root,
    'docs/evidence/production/current/production-evidence.log',
    '',
  );

  const result = checkDocumentation(root);

  assert.deepEqual(result.forbiddenArtifacts.sort(), [
    'docs/evidence/production/current/env 2.production-evidence',
    'docs/evidence/production/current/production-evidence.log',
  ]);
});

test('reports broken relative markdown links', () => {
  const root = createRoot();
  write(
    root,
    'docs/README.md',
    '[missing](phases/missing/README.md)\n',
  );

  const result = checkDocumentation(root);

  assert.deepEqual(result.brokenLinks, [{
    file: 'docs/README.md',
    target: 'phases/missing/README.md',
  }]);
});

test('accepts existing links and ignores external links and anchors', () => {
  const root = createRoot();
  write(root, 'docs/target.md');
  write(
    root,
    'docs/README.md',
    [
      '[target](target.md)',
      '[anchor](#section)',
      '[external](https://example.com)',
      '![inline](data:image/png;base64,AAAA)',
    ].join('\n'),
  );

  const result = checkDocumentation(root);

  assert.deepEqual(result.brokenLinks, []);
});
