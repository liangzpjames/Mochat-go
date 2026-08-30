import assert from 'node:assert/strict';
import test from 'node:test';

import { inspectSupplyChainPolicy } from './check_supply_chain_policy.mjs';

const safeFiles = {
  'go.mod': 'module example\n\ngo 1.26\n\ntoolchain go1.26.7\n',
  Dockerfile: [
    'FROM node:24.20.0-alpine@sha256:e67514e5d0f6c46656005e1b693b2ec9d52e80b641307de684d4a015ba7a4eaf',
    'FROM golang:1.26.7-alpine@sha256:28d89ee9cc0ff9fec75c82ca201e6bf7fdf9a679d4b7b24dfa04f2bb766bb468',
    'FROM alpine:3.22.5@sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce',
  ].join('\n'),
  workflow: [
    'uses: actions/checkout@11d5960a326750d5838078e36cf38b85af677262',
    'uses: actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff',
    'uses: pnpm/action-setup@b906affcce14559ad1aafd4ab0e942779e9f58b1',
    'uses: actions/setup-node@49933ea5288caeca8642d1e84afbd3f7d6820020',
    'uses: actions/cache@0057852bfaa89a56745cba8c7296529d2fc39830',
    'uses: anchore/sbom-action@e22c389904149dbc22b58101806040fa8d37a610',
    'uses: actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02',
    'go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...',
    'pnpm audit --audit-level high',
    'go run ./cmd/mochat-migrate -project-root . -action inventory',
    'sha256sum mochat-go.spdx.json',
  ].join('\n'),
  manifests: [
    { path: 'web/apps/dashboard/package.json', json: { dependencies: { 'react-router': '7.18.2' }, devDependencies: { vitest: '3.2.6' } } },
    { path: 'web/packages/config/package.json', json: { devDependencies: { vitest: '3.2.6' } } },
  ],
  workspace: [
    "  brace-expansion@1.1.16: 1.1.18",
    "  brace-expansion@2.1.2: 2.1.4",
    "  js-yaml@4.3.0: 4.3.1",
    "  nanoid@3.3.16: 3.3.18",
  ].join('\n'),
};

test('accepts pinned toolchains, images, actions and remediated frontend dependencies', () => {
  assert.deepEqual(inspectSupplyChainPolicy(safeFiles), []);
});

test('rejects floating actions, old dependency floors and hard-coded migration versions', () => {
  const errors = inspectSupplyChainPolicy({
    ...safeFiles,
    workflow: `${safeFiles.workflow}\nuses: actions/checkout@v4\nSELECT version = '0098_scrm_lead_foundation'`,
    manifests: [{ path: 'web/apps/dashboard/package.json', json: { dependencies: { 'react-router': '7.18.1' }, devDependencies: { vitest: '2.1.9' } } }],
  });

  assert.match(errors.join('\n'), /full commit SHA/);
  assert.match(errors.join('\n'), /must not hard-code migration version 0098/);
  assert.match(errors.join('\n'), /react-router must be 7\.18\.2/);
  assert.match(errors.join('\n'), /vitest must be at least 3\.2\.6/);
});
