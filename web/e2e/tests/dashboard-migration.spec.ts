import { expect, test } from '@playwright/test';
import { mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';

import { currentPhase2Routes } from '../../../scripts/phase2_current_routes.mjs';
import {
  assertPhase2MobileAuditClean,
  getPhase2MobileRouteContract,
  injectPhase2SidebarSession,
  installPhase2MobileFixtures,
} from '../fixtures/phase2-mobile';
import {
  mockDashboardBackend,
  seedSession,
} from './helpers';
const currentRoutes = currentPhase2Routes(resolve(process.cwd(), '../..'));
const dashboardRoutes = currentRoutes.dashboard;
const sidebarRoutes = currentRoutes.sidebar;
const operationRoutes = currentRoutes.operation;

const evidenceRoot = resolve(
  process.cwd(),
  '../../docs/phases/phase-2-frontend-migration/evidence/screenshots',
);
const slug = (value: string) => value === '/' ? 'root' : value.slice(1).replaceAll('/', '__');

test.beforeEach(async ({ page }) => {
  await seedSession(page);
  await mockDashboardBackend(page, {
    menuRoutes: dashboardRoutes.map((route) => route.path),
  });
});

test('unknown legacy URL returns 404', async ({ page }) => {
  const response = await page.goto('/_legacy/dashboard/not-in-manifest');

  expect(response?.status()).toBe(404);
});

for (const route of dashboardRoutes) {
  test(`dashboard ${route.path} renders React and records visual evidence`, async ({ page }) => {
    await mkdir(evidenceRoot, { recursive: true });
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`${route.path}?phase2=e2e#visual`);
    await expect(page).toHaveURL(new RegExp(`${route.path.replaceAll('/', '\\/')}\\?phase2=e2e#visual$`));
    await expect(page.locator('.dashboard-content > *').first()).toBeVisible();
    await page.screenshot({
      fullPage: true,
      path: resolve(evidenceRoot, `dashboard--${slug(route.path)}--desktop.png`),
    });
    await page.setViewportSize({ width: 390, height: 844 });
    await page.screenshot({
      fullPage: true,
      path: resolve(evidenceRoot, `dashboard--${slug(route.path)}--mobile.png`),
    });
  });
}

for (const [app, routes] of [
  ['sidebar', sidebarRoutes],
  ['operation', operationRoutes],
] as const) {
  for (const route of routes) {
    test(`${app} ${route.path} renders React and records visual evidence`, async ({ page }) => {
      await mkdir(evidenceRoot, { recursive: true });
      const audit = await installPhase2MobileFixtures(page);
      const contract = getPhase2MobileRouteContract(app, route.path);
      if (!contract) throw new Error(`${app} ${route.path} must have a current route contract`);
      if ('needsSession' in contract && contract.needsSession) {
        await injectPhase2SidebarSession(page);
      }
      const mountedPath = `/${app}-app${route.path === '/' ? '/' : route.path}`;
      const search = new URLSearchParams(contract.query ?? '');
      search.set('phase2', 'e2e');
      await page.setViewportSize({ width: 1440, height: 900 });
      await page.goto(`${mountedPath}?${search.toString()}#visual`);
      await expect(page.locator('main')).toBeVisible();
      await expect(page.getByRole('heading', { level: 1, name: contract.heading })).toBeVisible();
      await expect(page.getByText(contract.moduleLabel, { exact: true })).toBeVisible();
      await expect(page.getByRole('heading', { name: '页面不存在' })).toHaveCount(0);
      const currentURL = new URL(page.url());
      expect(currentURL.pathname).toBe(mountedPath);
      expect([...currentURL.searchParams.entries()]).toEqual([...search.entries()]);
      expect(currentURL.hash).toBe('#visual');
      await assertPhase2MobileAuditClean(page, audit, `${app} ${route.path}`);
      await page.screenshot({
        fullPage: true,
        path: resolve(evidenceRoot, `${app}--${slug(route.path)}--desktop.png`),
      });
      await page.setViewportSize({ width: 390, height: 844 });
      await page.screenshot({
        fullPage: true,
        path: resolve(evidenceRoot, `${app}--${slug(route.path)}--mobile.png`),
      });
    });
  }
}

test('API requests never return the SPA document', async ({ page }) => {
  await page.goto('/index');
  const response = await page.evaluate(async () => {
    const result = await fetch('/api/not-found');
    return {
      status: result.status,
      contentType: result.headers.get('content-type'),
    };
  });

  expect(response.status).toBe(404);
  expect(response.contentType ?? '').not.toContain('text/html');
});
