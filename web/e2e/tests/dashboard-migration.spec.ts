import { expect, test } from '@playwright/test';
import { readFileSync } from 'node:fs';
import { mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';

import {
  mockDashboardBackend,
  seedSession,
} from './helpers';
type Route = { path: string };
const routes = (app: string): Route[] => JSON.parse(
  readFileSync(resolve(process.cwd(), `../apps/${app}/src/migration-routes.json`), 'utf8'),
) as Route[];
const dashboardRoutes = routes('dashboard');
const sidebarRoutes = routes('sidebar');
const operationRoutes = routes('operation');

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
      const mountedPath = `/${app}-app${route.path === '/' ? '/' : route.path}`;
      await page.setViewportSize({ width: 1440, height: 900 });
      await page.goto(`${mountedPath}?phase2=e2e#visual`);
      await expect(page.getByTestId('react-migrated-page')).toHaveAttribute('data-route', route.path);
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
