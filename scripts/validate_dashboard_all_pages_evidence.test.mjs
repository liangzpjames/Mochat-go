import assert from 'node:assert/strict';
import { mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';

import { validateDashboardAllPagesEvidence, validateDashboardAllPagesEvidenceFiles } from './validate_dashboard_all_pages_evidence.mjs';

const routes = Array.from({ length: 53 }, (_, index) => `/page-${index + 1}`);
const actions = routes.map((route) => ({ route, action: `button:${route}` }));
const evidence = routes.map((route) => ({
  route,
  action: `button:${route}`,
  titleVisible: true,
  responseStatuses: [{ method: 'GET', url: `/dashboard/read${route}`, status: 200 }],
  unexpectedResponses: [],
  consoleErrors: [],
  pageErrors: [],
  screenshot: `screenshots/${route.slice(1)}.png`,
  returnedToIndex: true,
  userNameVisible: true,
  corpNameVisible: true,
}));

function input(overrides = {}) {
  return { manifestRoutes: routes, actionRegistry: actions, evidence, ...overrides };
}

function changedEvidence(index, change) {
  return evidence.map((item, itemIndex) => itemIndex === index ? { ...item, ...change } : item);
}

test('accepts complete 53-page evidence', () => {
  assert.deepEqual(validateDashboardAllPagesEvidence(input()), { pageCount: 53, expectedResponseCount: 0 });
});

test('rejects 52 pages and duplicate routes', () => {
  assert.throws(() => validateDashboardAllPagesEvidence(input({ manifestRoutes: routes.slice(0, 52) })), /exactly 53/);
  assert.throws(() => validateDashboardAllPagesEvidence(input({ manifestRoutes: [...routes.slice(0, 52), routes[0]] })), /duplicate manifest route/);
});

test('rejects a missing or empty safe action', () => {
  assert.throws(() => validateDashboardAllPagesEvidence(input({ actionRegistry: actions.slice(0, 52) })), /action registry/);
  assert.throws(() => validateDashboardAllPagesEvidence(input({ actionRegistry: actions.map((item, index) => index === 3 ? { ...item, action: '' } : item) })), /safe action/);
  for (const action of ['goto:/index', 'wait:networkidle', 'input:keyword']) {
    assert.throws(() => validateDashboardAllPagesEvidence(input({ actionRegistry: actions.map((item, index) => index === 3 ? { ...item, action } : item) })), /clickable button or link/);
  }
});

test('rejects missing screenshots', () => {
  assert.throws(() => validateDashboardAllPagesEvidence(input({ evidence: changedEvidence(4, { screenshot: '' }) })), /screenshot/);
});

test('rejects every 401 response', () => {
  const unauthorized = changedEvidence(5, { responseStatuses: [{ method: 'GET', url: '/dashboard/private', status: 401, machineCode: 'UNAUTHORIZED' }] });
  assert.throws(() => validateDashboardAllPagesEvidence(input({ evidence: unauthorized })), /unexpected response.*401/);
  assert.throws(() => validateDashboardAllPagesEvidence(input({ evidence: unauthorized, expectedResponses: [{ route: routes[5], method: 'GET', url: '/dashboard/private', status: 401, machineCode: 'UNAUTHORIZED' }] })), /unexpected response.*401/);
});

test('rejects an authorized page permission 403', () => {
  assert.throws(() => validateDashboardAllPagesEvidence(input({ evidence: changedEvidence(6, { responseStatuses: [{ method: 'GET', url: '/dashboard/private', status: 403, machineCode: 'DASHBOARD_PERMISSION_DENIED' }] }) })), /unexpected response.*403/);
});

test('permits only an exact URL status and machine-code exception', () => {
  const denied = changedEvidence(7, { responseStatuses: [{ method: 'POST', url: '/dashboard/provider', status: 503, machineCode: 'PROVIDER_UNAVAILABLE' }] });
  assert.doesNotThrow(() => validateDashboardAllPagesEvidence(input({ evidence: denied, expectedResponses: [{ route: routes[7], method: 'POST', url: '/dashboard/provider', status: 503, machineCode: 'PROVIDER_UNAVAILABLE' }] })));
  assert.throws(() => validateDashboardAllPagesEvidence(input({ evidence: denied, expectedResponses: [{ route: routes[8], method: 'POST', url: '/dashboard/provider', status: 503, machineCode: 'PROVIDER_UNAVAILABLE' }] })), /unexpected response.*503/);
  assert.throws(() => validateDashboardAllPagesEvidence(input({ evidence: denied, expectedResponses: [{ route: routes[7], method: 'GET', url: '/dashboard/provider', status: 503, machineCode: 'PROVIDER_UNAVAILABLE' }] })), /unexpected response.*503/);
  assert.throws(() => validateDashboardAllPagesEvidence(input({ evidence: denied, expectedResponses: [{ route: routes[7], method: 'POST', url: '/dashboard/provider', status: 503, machineCode: 'WRONG' }] })), /unexpected response.*503/);
});

test('rejects 404 and 5xx responses', () => {
  assert.throws(() => validateDashboardAllPagesEvidence(input({ evidence: changedEvidence(8, { responseStatuses: [{ method: 'GET', url: '/dashboard/missing', status: 404 }] }) })), /unexpected response.*404/);
  assert.throws(() => validateDashboardAllPagesEvidence(input({ evidence: changedEvidence(8, { responseStatuses: [{ method: 'GET', url: '/dashboard/broken', status: 500 }] }) })), /unexpected response.*500/);
});

test('rejects console errors and page errors', () => {
  assert.throws(() => validateDashboardAllPagesEvidence(input({ evidence: changedEvidence(9, { consoleErrors: ['boom'] }) })), /console errors/);
  assert.throws(() => validateDashboardAllPagesEvidence(input({ evidence: changedEvidence(9, { pageErrors: ['boom'] }) })), /page errors/);
});

test('rejects pages that cannot return to index or lose the user or corp name', () => {
  assert.throws(() => validateDashboardAllPagesEvidence(input({ evidence: changedEvidence(10, { returnedToIndex: false }) })), /return to \/index/);
  assert.throws(() => validateDashboardAllPagesEvidence(input({ evidence: changedEvidence(10, { userNameVisible: false }) })), /user name/);
  assert.throws(() => validateDashboardAllPagesEvidence(input({ evidence: changedEvidence(10, { corpNameVisible: false }) })), /corp name/);
});

test('rejects sensitive nested request and response fields in evidence', () => {
  for (const field of ['body', 'headers', 'token', 'authorization', 'cookie', 'set-cookie', 'password', 'secret']) {
    assert.throws(() => validateDashboardAllPagesEvidence(input({ evidence: changedEvidence(11, { request: { response: { [field]: 'sensitive' } } }) })), /forbidden in evidence/);
  }
  assert.throws(() => validateDashboardAllPagesEvidence(input({ evidence: changedEvidence(11, { responseStatuses: [{ method: 'GET', url: '/dashboard/private?token=sensitive', status: 200 }] }) })), /pathname only/);
});

test('requires every screenshot to exist and be non-empty under an injected evidence root', async () => {
  const evidenceRoot = await mkdtemp(join(tmpdir(), 'dashboard-evidence-'));
  try {
    await mkdir(join(evidenceRoot, 'screenshots'));
    for (const page of evidence) await writeFile(join(evidenceRoot, page.screenshot), 'png');
    await assert.doesNotReject(() => validateDashboardAllPagesEvidenceFiles(input(), { evidenceRoot }));
    await writeFile(join(evidenceRoot, evidence[12].screenshot), '');
    await assert.rejects(() => validateDashboardAllPagesEvidenceFiles(input(), { evidenceRoot }), /empty/);
    await rm(join(evidenceRoot, evidence[12].screenshot));
    await assert.rejects(() => validateDashboardAllPagesEvidenceFiles(input(), { evidenceRoot }), /missing/);
  } finally {
    await rm(evidenceRoot, { recursive: true, force: true });
  }
});
