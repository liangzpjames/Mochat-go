import assert from 'node:assert/strict';
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
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
  return `const ${name}Cases = [\n${paths.map((path) => `  { path: ${JSON.stringify(path)}, title: ${JSON.stringify(`${name}-${path}`)} },`).join('\n')}\n] as const;`;
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
for (const viewport of viewports) {
  for (const routeCase of [...sidebarCases, ...operationCases]) {
    test(
      \`${'${viewport.name}'} ${'${routeCase.path}'} renders its route title\`,
      async ({ page }) => {
        await page.setViewportSize(viewport);
        await page.goto(routeCase.path);
        await expect(page.getByRole('heading', { name: routeCase.title })).toBeVisible();
      },
    );
  }
}
test('unknown paths show 404 and do not render home content', async ({ page }) => {
  await page.goto('/sidebar-app/not-a-sidebar-page');
  await expect(page.getByRole('heading', { name: '页面不存在' })).toBeVisible();
  await expect(page.getByRole('heading', { name: '客户侧边栏' })).toHaveCount(0);
  await page.goto('/operation-app/not-an-operation-page');
  await expect(page.getByRole('heading', { name: '页面不存在' })).toBeVisible();
  await expect(page.getByRole('heading', { name: '营销活动中心' })).toHaveCount(0);
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
