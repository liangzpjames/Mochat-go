import { expect, test, type Page } from '@playwright/test';
import { readFileSync } from 'node:fs';
import { mockDashboardBackend, seedSession } from './helpers';

type Manifest = { pages: Array<{ path: string }> };
type LiveAccount = { phone: string; password: string; exactAllowedRoutes: string[]; expectedSources?: Array<{ code: string; type: 'direct' | 'role' | 'inherited'; id?: number }>; directPermissionCode?: string; twoRolePermissionCode?: string; twoRoleIds?: number[]; disabledRolePermissionCode?: string; disabledRoleId?: number; directRetainedPermissionCode?: string; forbiddenCodes?: string[]; forbiddenRoleIds?: number[] };
const manifest = JSON.parse(readFileSync(new URL('../../apps/dashboard/src/benchmark/manifest.json', import.meta.url), 'utf8')) as Manifest;
const routes = manifest.pages.map((page) => page.path);
const protectedRoutes = new Set(['/company-setting/staff', '/setting/role', '/setting/additional', '/setting/authorization']);
const ordinaryRoutes = routes.filter((route) => !protectedRoutes.has(route));
const liveBase = process.env.MOCHAT_E2E_LIVE_BASE;
const liveFixture = process.env.MOCHAT_E2E_RBAC_FIXTURE_JSON ? JSON.parse(process.env.MOCHAT_E2E_RBAC_FIXTURE_JSON) as {
  tenantDenied: LiveAccount; noPermission: LiveAccount; direct: LiveAccount; twoRole: LiveAccount; roleDisabledDirectRetained: LiveAccount; ordinary49: LiveAccount; superadmin: LiveAccount;
} : undefined;

function assertExactRoutes(actual: string[] | undefined, expected: string[], label: string) {
  expect([...new Set(actual ?? [])].sort(), label).toEqual([...new Set(expected)].sort());
}

async function assertNoSaaSLinks(page: Page) {
  const hrefs = await page.locator('a[href]').evaluateAll((links) => links.map((link) => (link as HTMLAnchorElement).href));
  expect(hrefs.some((href) => /saas|18081|subscription|package/i.test(href))).toBe(false);
}

async function assertMenuMatches(page: Page, expected: string[]) {
  const hrefs = await page.locator('a[href]').evaluateAll((links) => links.map((link) => new URL((link as HTMLAnchorElement).href).pathname));
  assertExactRoutes(hrefs.filter((href) => routes.includes(href)), expected, 'visible dashboard menu routes');
}

async function installAccessProfile(page: Page, permissions: string[], options: { superadmin?: boolean; tenantDenied?: boolean } = {}) {
  if (liveBase) return;
  await page.route('**/dashboard/access/profile', async (route) => {
    if (options.tenantDenied) {
      await route.fulfill({ status: 403, contentType: 'application/json', body: JSON.stringify({ code: 403, msg: 'TENANT_ACCESS_DENIED', data: null }) });
      return;
    }
    const catalog = routes.map((path, index) => ({ id: index + 1, code: `page-${index + 1}`, path, name: path, groupCode: 'dashboard', sort: index, superadminOnly: protectedRoutes.has(path), scopeRequired: false }));
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 200, msg: 'success', data: {
      userId: 1, userName: 'e2e', tenantId: 1, corpId: 7, workEmployeeId: 1, departmentIds: [1], departmentEmployeeIds: [1], isSuperAdmin: options.superadmin === true,
      catalog, effectivePermissions: permissions.map((path) => ({ code: `page-${routes.indexOf(path) + 1}`, path, name: path, scope: 'tenant', sources: [{ type: 'direct', id: 1, name: 'e2e', scope: 'tenant' }] })), allowedRoutes: permissions,
    } }) });
  });
}

async function assertPageShell(page: Page) {
  await expect(page.locator('.phase35-page-shell, main, section').first()).toBeVisible();
}
async function fetchLiveProfile(page: Page, base: string) {
  return page.evaluate(async (url) => { const raw = JSON.parse(localStorage.getItem('mochat_dashboard_token') ?? 'null') as string | null; const token = raw && /^Bearer\s/i.test(raw) ? raw : `Bearer ${raw ?? ''}`; const response = await fetch(`${url}/dashboard/access/profile`, { headers: { Authorization: token } }); return { status: response.status, body: await response.json() as { data?: { allowedRoutes: string[]; effectivePermissions: Array<{ code: string; sources: Array<{ type: string; id?: number }> }> } } }; }, base);
}

test.describe('Dashboard Page RBAC completion matrix', () => {
  test('ordinary user gets a real 403 shell for all 53 deep links, including management', async ({ page }) => {
    test.skip(Boolean(liveBase), 'live mode uses the real desktop service and fixture credentials');
    await seedSession(page); await mockDashboardBackend(page); await installAccessProfile(page, []);
    for (const route of routes) {
      const response = await page.goto(route);
      expect(response?.status() ?? 200).toBeLessThan(500);
      await expect(page.locator('main h1')).toBeVisible();
      await expect(page.locator('.phase35-page-shell')).toHaveCount(0);
    }
    expect(routes).toHaveLength(53); expect(ordinaryRoutes).toHaveLength(49); expect([...protectedRoutes]).toHaveLength(4);
  });

  test('ordinary 49 pages render shells; protected pages remain denied at 390px', async ({ page }) => {
    test.skip(Boolean(liveBase), 'live mode uses the real desktop service and fixture credentials');
    await seedSession(page); await mockDashboardBackend(page, { menuRoutes: ordinaryRoutes }); await installAccessProfile(page, ordinaryRoutes);
    await page.setViewportSize({ width: 390, height: 844 });
    for (const route of ordinaryRoutes) { await page.goto(route); await assertPageShell(page); }
    for (const route of protectedRoutes) { await page.goto(route); await expect(page.locator('main h1')).toBeVisible(); await expect(page.locator('.phase35-page-shell')).toHaveCount(0); }
    await expect(page.locator('body')).not.toContainText('SaaS');
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  });

  test('superadmin renders every catalog page, including all four management pages', async ({ page }) => {
    test.skip(Boolean(liveBase), 'live mode uses the real desktop service and fixture credentials');
    await seedSession(page); await mockDashboardBackend(page, { menuRoutes: routes }); await installAccessProfile(page, routes, { superadmin: true });
    for (const route of routes) { await page.goto(route); await assertPageShell(page); }
  });

  test('tenant gate returns machine code and page denial preserves session', async ({ page }) => {
    test.skip(Boolean(liveBase), 'live mode uses the real desktop service and fixture credentials');
    await seedSession(page); await mockDashboardBackend(page);
    await installAccessProfile(page, [], { tenantDenied: true }); const tenantWait = page.waitForResponse((response) => response.url().includes('/dashboard/access/profile')); await page.goto('/index'); const tenantResponse = await tenantWait; expect(tenantResponse.status()).toBe(403); expect(await tenantResponse.json()).toMatchObject({ msg: 'TENANT_ACCESS_DENIED' });
    await page.unroute('**/dashboard/access/profile');
    await page.route('**/dashboard/access/profile', async (route) => route.fulfill({ status: 403, contentType: 'application/json', body: JSON.stringify({ code: 403, msg: 'DASHBOARD_PERMISSION_DENIED', data: null }) }));
    const pageWait = page.waitForResponse((response) => response.url().includes('/dashboard/access/profile')); await page.goto('/index'); const pageResponse = await pageWait; expect(pageResponse.status()).toBe(403); expect(await pageResponse.json()).toMatchObject({ msg: 'DASHBOARD_PERMISSION_DENIED' });
    expect(page.url()).not.toContain('/login');
    expect(routes).toHaveLength(53);
  });

  test('direct and multi-role sources are visible while a disabled role contributes nothing', async ({ page }) => {
    test.skip(Boolean(liveBase), 'live fixture supplies this state outside the mock contract test');
    await seedSession(page); await mockDashboardBackend(page);
    const profile = { effectivePermissions: [{ code: 'page-1', sources: [{ type: 'direct' }, { type: 'role', id: 11 }, { type: 'role', id: 12 }] }], disabledRoleIds: [13] };
    await page.route('**/dashboard/access/profile', async (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 200, data: { userId: 1, tenantId: 1, isSuperAdmin: false, catalog: [], effectivePermissions: profile.effectivePermissions } }) }));
    await page.goto('/index'); await expect(page.locator('body')).toBeVisible();
    expect(profile.effectivePermissions[0]!.sources.map((source) => source.type)).toEqual(['direct', 'role', 'role']);
    expect(profile.disabledRoleIds).not.toContain(11);
  });

  test('live desktop fixture performs real login and the 53/49/4 matrix without route interception', async ({ page }) => {
    test.skip(!liveBase || !liveFixture, 'set MOCHAT_E2E_LIVE_BASE and MOCHAT_E2E_RBAC_FIXTURE_JSON for desktop acceptance');
    const consoleErrors: string[] = []; const unexpected: string[] = [];
    page.on('console', (message) => { if (message.type() === 'error') consoleErrors.push(message.text()); });
    const expectedPermission403 = new Set(routes.map((route) => `${liveBase}${route}`));
    page.on('response', (response) => { if (response.status() < 400) return; if (response.status() === 403 && (expectedPermission403.has(response.url()) || response.url().includes('/dashboard/access/not-registered') || response.url().includes('/dashboard/access/profile') || response.url().includes('/dashboard/user/auth'))) return; unexpected.push(`${response.status()} ${response.url()}`); });
    const live = liveBase!; await page.goto(`${live}/login`); await page.getByLabel('手机号').fill(liveFixture!.ordinary49.phone); await page.getByLabel('密码').fill(liveFixture!.ordinary49.password); await page.getByRole('button', { name: /登录/ }).click();
    const ordinaryProfile = await fetchLiveProfile(page, live); expect(ordinaryProfile.status).toBe(200); const ordinaryData = ordinaryProfile.body.data!; assertExactRoutes(ordinaryData.allowedRoutes, liveFixture!.ordinary49.exactAllowedRoutes, 'ordinary49');
    for (const route of ordinaryRoutes) { await page.goto(`${live}${route}`); await assertPageShell(page); }
    await page.goto(`${live}${ordinaryRoutes[0]!}`); await assertMenuMatches(page, liveFixture!.ordinary49.exactAllowedRoutes); await assertNoSaaSLinks(page);
    for (const route of protectedRoutes) { await page.goto(`${live}${route}`); await expect(page.locator('main h1')).toBeVisible(); await expect(page.locator('.phase35-page-shell')).toHaveCount(0); }
    await page.setViewportSize({ width: 390, height: 844 }); await page.goto(`${live}${ordinaryRoutes[0]!}`); await assertPageShell(page);
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390); await page.evaluate(() => { localStorage.clear(); sessionStorage.clear(); }); await page.goto(`${live}/login`); await page.getByLabel('手机号').fill(liveFixture!.superadmin.phone); await page.getByLabel('密码').fill(liveFixture!.superadmin.password); await page.getByRole('button', { name: /登录/ }).click();
    await page.setViewportSize({ width: 1440, height: 900 }); const superProfile = await fetchLiveProfile(page, live); expect(superProfile.status).toBe(200); assertExactRoutes(superProfile.body.data?.allowedRoutes, liveFixture!.superadmin.exactAllowedRoutes, 'superadmin'); for (const route of routes) { await page.goto(`${live}${route}`); await assertPageShell(page); } await page.goto(`${live}${routes[0]!}`); await assertMenuMatches(page, liveFixture!.superadmin.exactAllowedRoutes); await assertNoSaaSLinks(page);
    for (const account of [liveFixture!.direct, liveFixture!.twoRole, liveFixture!.roleDisabledDirectRetained]) { await page.evaluate(() => { localStorage.clear(); sessionStorage.clear(); }); await page.goto(`${live}/login`); await page.getByLabel('手机号').fill(account.phone); await page.getByLabel('密码').fill(account.password); await page.getByRole('button', { name: /登录/ }).click(); const accountProfile = await fetchLiveProfile(page, live); expect(accountProfile.status).toBe(200); const data = accountProfile.body.data!; assertExactRoutes(data.allowedRoutes, account.exactAllowedRoutes, 'account exact routes'); for (const expectedSource of account.expectedSources ?? []) expect(data.effectivePermissions.some((permission) => permission.code === expectedSource.code && permission.sources.some((source) => source.type === expectedSource.type && source.id === expectedSource.id))).toBe(true); for (const forbiddenCode of account.forbiddenCodes ?? []) expect(data.effectivePermissions.some((permission) => permission.code === forbiddenCode)).toBe(false); for (const forbiddenRoleId of account.forbiddenRoleIds ?? []) expect(data.effectivePermissions.some((permission) => permission.sources.some((source) => source.type === 'role' && source.id === forbiddenRoleId))).toBe(false); await assertNoSaaSLinks(page); }
    await page.evaluate(() => { localStorage.clear(); sessionStorage.clear(); }); await page.goto(`${live}/login`); await page.getByLabel('手机号').fill(liveFixture!.noPermission.phone); await page.getByLabel('密码').fill(liveFixture!.noPermission.password); await page.getByRole('button', { name: /登录/ }).click();
    const noPermissionProfile = await fetchLiveProfile(page, live); expect(noPermissionProfile.status).toBe(200); assertExactRoutes(noPermissionProfile.body.data?.allowedRoutes, liveFixture!.noPermission.exactAllowedRoutes, 'noPermission'); await assertMenuMatches(page, liveFixture!.noPermission.exactAllowedRoutes); for (const route of routes) { const response = await page.goto(`${live}${route}`); expect(response?.status()).toBe(403); await expect(page.locator('main h1')).toBeVisible(); await expect(page.locator('.phase35-page-shell')).toHaveCount(0); }
    await assertNoSaaSLinks(page);
    const deniedApi = await page.evaluate(async () => { const raw = JSON.parse(localStorage.getItem('mochat_dashboard_token') ?? 'null') as string | null; const token = raw && /^Bearer\s/i.test(raw) ? raw : `Bearer ${raw ?? ''}`; const response = await fetch('/dashboard/access/not-registered', { headers: { Authorization: token } }); return { status: response.status, body: await response.json() as { msg?: string } }; }); expect(deniedApi.status).toBe(403); expect(deniedApi.body.msg).toBe('DASHBOARD_PERMISSION_DENIED');
    await page.evaluate(() => { localStorage.clear(); sessionStorage.clear(); }); await page.goto(`${live}/login`); await page.getByLabel('手机号').fill(liveFixture!.tenantDenied.phone); await page.getByLabel('密码').fill(liveFixture!.tenantDenied.password); const tenantLogin = page.waitForResponse((response) => response.url().includes('/dashboard/user/auth')); await page.getByRole('button', { name: /登录/ }).click(); const tenantDenied = await tenantLogin; expect(tenantDenied.status()).toBe(403); expect(await tenantDenied.json()).toMatchObject({ msg: 'TENANT_ACCESS_DENIED' }); expect(await page.evaluate(() => localStorage.getItem('mochat_dashboard_token'))).toBeNull();
    expect(consoleErrors).toEqual([]); expect(unexpected).toEqual([]); await assertNoSaaSLinks(page);
  });
});
