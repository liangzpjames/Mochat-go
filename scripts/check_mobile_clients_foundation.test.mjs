import assert from 'node:assert/strict';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import test from 'node:test';

import { auditMobileClientsFoundation } from './check_mobile_clients_foundation.mjs';

const sidebarRoutes = [
  '/',
  '/auth',
  '/codeAuth',
  '/contact',
  '/contact/editDetail',
  '/contact/remark',
  '/contact/settingTag',
  '/contactBatchAdd',
  '/contactSop',
  '/login',
  '/medium',
  '/roomSop',
];

const operationRoutes = [
  '/',
  '/explain',
  '/lottery',
  '/roomClockIn',
  '/roomFission',
  '/fissionSpeed',
  '/roomInfinitePull',
  '/shopCode',
  '/workFission',
  '/speed',
];

function write(root, relativePath, source) {
  const filePath = join(root, relativePath);
  mkdirSync(dirname(filePath), { recursive: true });
  writeFileSync(filePath, source, 'utf8');
}

function manifest(paths) {
  return `${JSON.stringify(paths.map((path) => ({
    path,
    target: 'react',
    auth: false,
    corpContext: false,
    permission: null,
  })), null, 2)}\n`;
}

function registry(name, paths) {
  const definitions = paths
    .map((path, index) => `  ${JSON.stringify(path)}: { moduleKey: ${JSON.stringify(`${name}-${index}`)} },`)
    .join('\n');
  return `const routeDefinitions = {\n${definitions}\n} as const;\nexport const ${name}RouteRegistry = routeDefinitions;\n`;
}

function routeCases(name, paths) {
  return `const ${name}Cases = [\n${paths.map((path) => `  { path: ${JSON.stringify(path)}, title: ${JSON.stringify(`${name}-${path}`)}, expectsAction: ${name === 'sidebar' && path === '/login'}${name === 'operation' && path === '/workFission' ? ", query: '?id=17'" : ''} },`).join('\n')}\n] as const;`;
}

function validE2E() {
  return `
import { expect, test } from '@playwright/test';
${routeCases('sidebar', sidebarRoutes)}
${routeCases('operation', operationRoutes)}
const viewports = [
  { name: 'mobile-390', width: 390, height: 844 },
  { name: 'desktop-1280', width: 1280, height: 900 },
] as const;
const browserContract = {
  fixtures: {
    contact: '**/sidebar/workContact/detail?*',
    openUserInfo: '**/operation/openUserInfo/workFission?*',
    taskData: '**/operation/workFission/taskData?*',
  },
  minimumActionHeight: 44,
} as const;
const rawContactData = { id: 1, name: 'Fixture Contact', avatar: null, corpId: 2 };
const rawTaskData = {
  invite_count: 2,
  differ_count: 1,
  end_time: 4102444800,
  task: [{ count: 3, status: 0, receive_status: 0, gift_type: 1, gift_url: '/reward' }],
};
const rawWorkFissionParticipant = {
  openid: 'openid-fixture', unionid: 'union-fixture-session', nickname: 'Fixture Participant', headimgurl: '',
};
function rawGoEnvelope(data, requestId) {
  return JSON.stringify({ code: 200, msg: 'ok', data, requestId });
}
async function installRawGoFixtures(page) {
  const audit = {
    consoleErrors: [], pageErrors: [], requestFailures: [], unexpectedResponses: [], unexpectedRequests: [],
    participantData: rawWorkFissionParticipant, workFissionParticipantRequests: [], workFissionRequests: [],
    workFissionEvidenceChecks: 0,
  };
  page.on('console', (message) => { if (message.type() === 'error') audit.consoleErrors.push(message.text()); });
  page.on('pageerror', (error) => audit.pageErrors.push(error.message));
  page.on('requestfailed', (request) => audit.requestFailures.push(request.url()));
  page.on('response', (response) => { if (response.status() >= 400) audit.unexpectedResponses.push(response.url()); });
  await page.route('**/sidebar/**', async (route) => { audit.unexpectedRequests.push(route.request().url()); await route.fulfill({ status: 500 }); });
  await page.route('**/operation/**', async (route) => { audit.unexpectedRequests.push(route.request().url()); await route.fulfill({ status: 500 }); });
  await page.route(browserContract.fixtures.contact, async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: rawGoEnvelope(rawContactData, 'contact-request') });
  });
  await page.route(browserContract.fixtures.taskData, async (route) => {
    audit.workFissionRequests.push(route.request().url());
    await route.fulfill({ status: 200, contentType: 'application/json', body: rawGoEnvelope(rawTaskData, 'task-request') });
  });
  await page.route(browserContract.fixtures.openUserInfo, async (route) => {
    audit.workFissionParticipantRequests.push(route.request().url());
    await route.fulfill({ status: 200, contentType: 'application/json', body: rawGoEnvelope(audit.participantData, 'participant-request') });
  });
  return audit;
}
async function assertVisiblePage(page, routeCase, expectsAction) {
  await expect(page.getByRole('heading', { name: routeCase.title })).toBeVisible();
  const pageShape = await page.evaluate(() => ({
    clientWidth: document.documentElement.clientWidth,
    scrollWidth: document.documentElement.scrollWidth,
  }));
  expect(pageShape.scrollWidth).toBeLessThanOrEqual(pageShape.clientWidth);
  const controls = page.locator('a.primary:visible, button.retry:visible');
  if (expectsAction) expect(await controls.count()).toBeGreaterThanOrEqual(1);
  for (let index = 0; index < await controls.count(); index += 1) {
    const box = await controls.nth(index).boundingBox();
    expect(box?.height ?? 0).toBeGreaterThanOrEqual(browserContract.minimumActionHeight);
  }
}
function assertCleanAudit(audit) {
  expect(audit.consoleErrors).toEqual([]);
  expect(audit.pageErrors).toEqual([]);
  expect(audit.requestFailures).toEqual([]);
  expect(audit.unexpectedResponses).toEqual([]);
  expect(audit.unexpectedRequests).toEqual([]);
}
async function assertStableCleanAudit(page, audit): Promise<void> {
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(0);
  assertCleanAudit(audit);
}
for (const viewport of viewports) {
  for (const routeCase of sidebarCases) {
    test(\`${'${viewport.name}'} Sidebar ${'${routeCase.path}'}\`, async ({ page }) => {
      const audit = await installRawGoFixtures(page);
      await page.setViewportSize(viewport);
      await page.goto(routeCase.path + ('query' in routeCase ? routeCase.query : ''));
      await assertVisiblePage(page, routeCase, routeCase.expectsAction);
      await assertStableCleanAudit(page, audit);
    });
  }
  for (const routeCase of operationCases) {
    test(\`${'${viewport.name}'} Operation ${'${routeCase.path}'}\`, async ({ page }) => {
      const audit = await installRawGoFixtures(page);
      await page.setViewportSize(viewport);
      await page.goto(routeCase.path + ('query' in routeCase ? routeCase.query : ''));
      await assertVisiblePage(page, routeCase, routeCase.expectsAction);
      if (routeCase.path === '/workFission') {
        audit.workFissionEvidenceChecks += 1;
        expect(new URL(audit.workFissionParticipantRequests[0]).searchParams.get('id')).toBe('17');
        expect(new URL(audit.workFissionRequests[0]).searchParams.get('union_id')).toBe('union-fixture-session');
        audit.participantData = [];
        await page.goto('/workFission?id=17&union_id=attacker-controlled');
        await expect(page.getByRole('link', { name: '重新授权' })).toHaveAttribute('href', '/auth/workFission?id=17&target=%2FworkFission%3Fid%3D17%26union_id%3Dattacker-controlled');
        expect(audit.workFissionEvidenceChecks).toBe(1);
      }
      await assertStableCleanAudit(page, audit);
    });
  }
}
test('unknown paths show 404 and do not render home content', async ({ page }) => {
  const audit = await installRawGoFixtures(page);
  await page.goto('/sidebar-app/not-a-sidebar-page');
  await expect(page.getByRole('heading', { name: '页面不存在' })).toBeVisible();
  await expect(page.getByRole('heading', { name: '客户侧边栏' })).toHaveCount(0);
  await assertStableCleanAudit(page, audit);
  await page.goto('/operation-app/not-an-operation-page');
  await expect(page.getByRole('heading', { name: '页面不存在' })).toBeVisible();
  await expect(page.getByRole('heading', { name: '营销活动中心' })).toHaveCount(0);
  await assertStableCleanAudit(page, audit);
});
`;
}

function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'mobile-clients-foundation-'));
  t.after(() => rmSync(root, { force: true, recursive: true }));

  write(root, 'package.json', `${JSON.stringify({
    scripts: {
      'check:mobile-clients-foundation': 'node scripts/check_mobile_clients_foundation.mjs && node --test scripts/check_mobile_clients_foundation.test.mjs',
    },
  }, null, 2)}\n`);
  write(root, 'web/e2e/package.json', `${JSON.stringify({
    scripts: {
      'test:mobile-clients-foundation': 'playwright test tests/mobile-clients-foundation.spec.ts --workers=1',
    },
  }, null, 2)}\n`);
  write(root, 'web/apps/sidebar/src/migration-routes.json', manifest(sidebarRoutes));
  write(root, 'web/apps/operation/src/migration-routes.json', manifest(operationRoutes));
  write(root, 'web/apps/sidebar/src/routes/registry.tsx', registry('sidebar', sidebarRoutes));
  write(root, 'web/apps/operation/src/routes/registry.tsx', registry('operation', operationRoutes));
  write(root, 'web/apps/sidebar/src/app/sidebar-router.tsx', "const routes = [{ path: '*', element: <SidebarNotFoundPage /> }];\n");
  write(root, 'web/apps/operation/src/app/operation-router.tsx', "const routes = [{ path: '*', element: <OperationNotFoundPage /> }];\n");
  write(root, 'web/apps/sidebar/src/features/contact/contact-page.tsx', "export function ContactPage() { return <main>客户资料</main>; }\n");
  write(root, 'web/apps/sidebar/src/auth/sidebar-session.ts', `
const SIDEBAR_SESSION_STORAGE_KEY = 'mochat_sidebar_session_v1';
export function createCookieSidebarSessionAdapter(cookies) {
  return { write: () => cookies.set('token=value; Path=/sidebar-app; SameSite=Lax') };
}
export function createSessionStorageSidebarSessionAdapter(storage) {
  return {
    read: () => storage.getItem(SIDEBAR_SESSION_STORAGE_KEY),
    write: (value) => storage.setItem(SIDEBAR_SESSION_STORAGE_KEY, value),
    clear: () => storage.removeItem(SIDEBAR_SESSION_STORAGE_KEY),
  };
}
`);
  write(root, 'web/apps/sidebar/src/auth/sidebar-session.test.ts', `
test('root callback uses Sidebar sessionStorage and malformed values fail closed', () => {
  createSessionStorageSidebarSessionAdapter(window.sessionStorage);
  expect(completeSidebarAuthCallback(rootCallback)).toBeTruthy();
  expect(readSidebarSession(rootAdapter)).toEqual({ token: null, agentId: null });
});
`);
  write(root, 'web/apps/sidebar/src/main.tsx', `
const basename = sidebarBasename(window.location.pathname);
const session = basename === '/sidebar-app'
  ? createCookieSidebarSessionAdapter(documentCookieAdapter, secure)
  : createSessionStorageSidebarSessionAdapter(window.sessionStorage);
`);
  write(root, 'web/apps/operation/src/features/work-fission/work-fission-page.tsx', "export function WorkFissionPage() { return <main>任务宝活动</main>; }\n");
  write(root, 'web/e2e/tests/mobile-clients-foundation.spec.ts', validE2E());
  return root;
}

function expectDefect(root, pattern) {
  assert.throws(() => auditMobileClientsFoundation(root), pattern);
}

test('accepts a source-derived 12 plus 10 route foundation', (t) => {
  const result = auditMobileClientsFoundation(fixture(t));
  assert.deepEqual(result, {
    sidebarRoutes: 12,
    operationRoutes: 10,
    directFetch: 0,
    dashboardSessionReferences: 0,
    mojibakeMarkers: 0,
    fakeBusinessOutcomes: 0,
    mobileViewportCases: 22,
  });
});

test('rejects an 11-route Sidebar manifest independently', (t) => {
  const root = fixture(t);
  write(root, 'web/apps/sidebar/src/migration-routes.json', manifest(sidebarRoutes.slice(0, 11)));
  expectDefect(root, /Sidebar manifest.*12.*found 11/i);
});

test('rejects a 9-route Operation manifest independently', (t) => {
  const root = fixture(t);
  write(root, 'web/apps/operation/src/migration-routes.json', manifest(operationRoutes.slice(0, 9)));
  expectDefect(root, /Operation manifest.*10.*found 9/i);
});

test('rejects a registry route absent from its manifest independently', (t) => {
  const root = fixture(t);
  write(root, 'web/apps/sidebar/src/routes/registry.tsx', registry('sidebar', [...sidebarRoutes, '/ghost']));
  expectDefect(root, /Sidebar registry route.*\/ghost.*absent from manifest/i);
});

test('rejects direct fetch in production page source independently', (t) => {
  const root = fixture(t);
  write(root, 'web/apps/sidebar/src/features/contact/contact-page.tsx', "export const load = () => fetch('/sidebar/workContact/detail');\n");
  expectDefect(root, /direct fetch.*contact-page\.tsx/i);
});

test('scans only production source for direct fetch', (t) => {
  const root = fixture(t);
  write(root, 'web/apps/sidebar/src/features/contact/contact-page.test.tsx', "test('mock', () => fetch('/fixture'));\n");
  assert.equal(auditMobileClientsFoundation(root).directFetch, 0);
});

test('rejects a Dashboard storage key in Sidebar independently', (t) => {
  const root = fixture(t);
  write(root, 'web/apps/sidebar/src/session.ts', "localStorage.getItem('mochat_dashboard_token');\n");
  expectDefect(root, /Dashboard session reference.*sidebar.*session\.ts/i);
});

test('rejects a Dashboard storage key in Operation independently', (t) => {
  const root = fixture(t);
  write(root, 'web/apps/operation/src/session.ts', "sessionStorage.getItem('mochat_dashboard_corp_id');\n");
  expectDefect(root, /Dashboard session reference.*operation.*session\.ts/i);
});

test('rejects a root-scoped Sidebar token cookie independently', (t) => {
  const root = fixture(t);
  write(
    root,
    'web/apps/sidebar/src/auth/sidebar-session.ts',
    readFileSync(join(root, 'web/apps/sidebar/src/auth/sidebar-session.ts'), 'utf8')
      .replace('Path=/sidebar-app', 'Path=/'),
  );
  expectDefect(root, /Sidebar token cookie.*Path=\//i);
});

test('rejects a missing independent-root sessionStorage adapter independently', (t) => {
  const root = fixture(t);
  write(
    root,
    'web/apps/sidebar/src/auth/sidebar-session.ts',
    readFileSync(join(root, 'web/apps/sidebar/src/auth/sidebar-session.ts'), 'utf8')
      .replaceAll('createSessionStorageSidebarSessionAdapter', 'missingRootAdapter'),
  );
  expectDefect(root, /independent-root Sidebar sessionStorage adapter/i);
});

test('rejects missing root adapter fail-closed tests independently', (t) => {
  const root = fixture(t);
  write(root, 'web/apps/sidebar/src/auth/sidebar-session.test.ts', "test('cookie only', () => {});\n");
  expectDefect(root, /root Sidebar sessionStorage.*test/i);
});

test('rejects mojibake markers independently', (t) => {
  const root = fixture(t);
  write(root, 'web/apps/sidebar/src/broken.tsx', "export const title = '椤甸潰涓嶅瓨鍦?';\n");
  expectDefect(root, /mojibake.*broken\.tsx/i);
});

test('rejects a fake success outcome independently', (t) => {
  const root = fixture(t);
  write(root, 'web/apps/sidebar/src/fake.tsx', "export const message = '操作已完成';\n");
  expectDefect(root, /fake business outcome.*fake\.tsx/i);
});

test('rejects a random prize independently', (t) => {
  const root = fixture(t);
  write(root, 'web/apps/operation/src/random.ts', "export const prize = prizes[Math.floor(Math.random() * prizes.length)];\n");
  expectDefect(root, /fake business outcome.*random\.ts/i);
});

test('rejects a fixed progress result independently', (t) => {
  const root = fixture(t);
  write(root, 'web/apps/operation/src/progress.tsx', "export const progress = 66;\n");
  expectDefect(root, /fake business outcome.*progress\.tsx/i);
});

test('rejects a missing 390 by 844 browser viewport independently', (t) => {
  const root = fixture(t);
  write(
    root,
    'web/e2e/tests/mobile-clients-foundation.spec.ts',
    validE2E().replace("{ name: 'mobile-390', width: 390, height: 844 },", ''),
  );
  expectDefect(root, /390.*844.*viewport/i);
});

test('rejects a missing contact raw Go envelope independently', (t) => {
  const root = fixture(t);
  write(
    root,
    'web/e2e/tests/mobile-clients-foundation.spec.ts',
    validE2E().replace("body: rawGoEnvelope(rawContactData, 'contact-request')", "body: JSON.stringify({ contact: rawContactData })"),
  );
  expectDefect(root, /contact.*raw Go envelope/i);
});

test('rejects a missing taskData raw Go envelope independently', (t) => {
  const root = fixture(t);
  write(
    root,
    'web/e2e/tests/mobile-clients-foundation.spec.ts',
    validE2E().replace("body: rawGoEnvelope(rawTaskData, 'task-request')", "body: JSON.stringify({ taskData: rawTaskData })"),
  );
  expectDefect(root, /taskData.*raw Go envelope/i);
});

test('rejects a missing openUserInfo raw Go envelope independently', (t) => {
  const root = fixture(t);
  write(
    root,
    'web/e2e/tests/mobile-clients-foundation.spec.ts',
    validE2E().replace("body: rawGoEnvelope(audit.participantData, 'participant-request')", "body: JSON.stringify({ participant: audit.participantData })"),
  );
  expectDefect(root, /openUserInfo.*raw Go envelope/i);
});

test('rejects the legacy union_id work-fission entry independently', (t) => {
  const root = fixture(t);
  write(
    root,
    'web/e2e/tests/mobile-clients-foundation.spec.ts',
    validE2E().replace("query: '?id=17'", "query: '?union_id=url-user&fission_id=17'"),
  );
  expectDefect(root, /workFission.*positive id.*entry/i);
});

test('rejects a work-fission fixture without session-derived taskData identity proof', (t) => {
  const root = fixture(t);
  write(
    root,
    'web/e2e/tests/mobile-clients-foundation.spec.ts',
    validE2E().replace("expect(new URL(audit.workFissionRequests[0]).searchParams.get('union_id')).toBe('union-fixture-session');", ''),
  );
  expectDefect(root, /taskData.*session.*unionid/i);
});

test('rejects a work-fission fixture without raw empty-session OAuth proof', (t) => {
  const root = fixture(t);
  write(
    root,
    'web/e2e/tests/mobile-clients-foundation.spec.ts',
    validE2E().replace('        audit.participantData = [];\n', ''),
  );
  expectDefect(root, /empty.*participant.*OAuth/i);
});

test('rejects work-fission evidence outside the real Operation branch', (t) => {
  const root = fixture(t);
  write(
    root,
    'web/e2e/tests/mobile-clients-foundation.spec.ts',
    validE2E().replace("if (routeCase.path === '/workFission')", 'if (false)'),
  );
  expectDefect(root, /Operation workFission evidence branch/i);
});

test('rejects a work-fission branch without an execution count', (t) => {
  const root = fixture(t);
  write(
    root,
    'web/e2e/tests/mobile-clients-foundation.spec.ts',
    validE2E().replace('        audit.workFissionEvidenceChecks += 1;\n', ''),
  );
  expectDefect(root, /workFission evidence execution count/i);
});

test('rejects a missing clean-audit event collector independently', (t) => {
  const root = fixture(t);
  write(
    root,
    'web/e2e/tests/mobile-clients-foundation.spec.ts',
    validE2E().replace("  page.on('requestfailed', (request) => audit.requestFailures.push(request.url()));\n", ''),
  );
  expectDefect(root, /requestfailed.*audit/i);
});

test('rejects a missing clean-audit final assertion independently', (t) => {
  const root = fixture(t);
  write(
    root,
    'web/e2e/tests/mobile-clients-foundation.spec.ts',
    validE2E().replace('  expect(audit.unexpectedRequests).toEqual([]);\n', ''),
  );
  expectDefect(root, /unexpected request.*assertion/i);
});

test('rejects a missing 4xx and 5xx response collector independently', (t) => {
  const root = fixture(t);
  write(
    root,
    'web/e2e/tests/mobile-clients-foundation.spec.ts',
    validE2E().replace('if (response.status() >= 400)', 'if (false)'),
  );
  expectDefect(root, /400.*response audit/i);
});

test('rejects a missing overflow assertion independently', (t) => {
  const root = fixture(t);
  write(
    root,
    'web/e2e/tests/mobile-clients-foundation.spec.ts',
    validE2E().replace('  expect(pageShape.scrollWidth).toBeLessThanOrEqual(pageShape.clientWidth);\n', ''),
  );
  expectDefect(root, /scrollWidth.*clientWidth/i);
});

test('rejects a missing mandatory 44px action assertion independently', (t) => {
  const root = fixture(t);
  write(
    root,
    'web/e2e/tests/mobile-clients-foundation.spec.ts',
    validE2E().replace('  if (expectsAction) expect(await controls.count()).toBeGreaterThanOrEqual(1);\n', ''),
  );
  expectDefect(root, /expectsAction.*control count/i);
});

test('rejects a route matrix without an applicable action case independently', (t) => {
  const root = fixture(t);
  write(
    root,
    'web/e2e/tests/mobile-clients-foundation.spec.ts',
    validE2E().replaceAll('expectsAction: true', 'expectsAction: false'),
  );
  expectDefect(root, /login.*expectsAction.*true/i);
});

test('rejects an only-mobile route loop independently', (t) => {
  const root = fixture(t);
  write(
    root,
    'web/e2e/tests/mobile-clients-foundation.spec.ts',
    validE2E().replace('for (const viewport of viewports)', 'for (const viewport of [viewports[0]]'),
  );
  expectDefect(root, /viewport loop.*all viewports/i);
});

test('rejects an unstable audit assertion independently', (t) => {
  const root = fixture(t);
  write(
    root,
    'web/e2e/tests/mobile-clients-foundation.spec.ts',
    validE2E().replace("  await page.waitForLoadState('networkidle');\n", ''),
  );
  expectDefect(root, /networkidle.*audit/i);
});

test('rejects an unknown route that falls back to home independently', (t) => {
  const root = fixture(t);
  write(root, 'web/apps/operation/src/app/operation-router.tsx', "const routes = [{ path: '*', element: <Navigate to=\"/\" replace /> }];\n");
  expectDefect(root, /Operation unknown route.*home/i);
});

test('rejects a missing root completion script independently', (t) => {
  const root = fixture(t);
  write(root, 'package.json', '{"scripts":{}}\n');
  expectDefect(root, /check:mobile-clients-foundation.*script/i);
});

test('rejects a missing E2E completion script independently', (t) => {
  const root = fixture(t);
  write(root, 'web/e2e/package.json', '{"scripts":{}}\n');
  expectDefect(root, /test:mobile-clients-foundation.*script/i);
});
