import { expect, test } from '@playwright/test';

import { mockDashboardBackend, seedSession } from './helpers';

test.beforeEach(async ({ page }) => {
  await seedSession(page);
  await mockDashboardBackend(page);
});

test('React route renders the Dashboard overview without a company selector', async ({ page }) => {
  await page.goto('/index');
  await expect(page.locator('.dashboard-content > *').first()).toBeVisible();
  await expect(page.locator('.dashboard-corp-switcher')).toHaveCount(0);
});

test('unknown route renders React 404 without legacy fallback', async ({ page }) => {
  await page.goto('/not-in-manifest');

  await expect(page).toHaveURL(/\/not-in-manifest$/);
  await expect(page.getByRole('heading', { name: '椤甸潰涓嶅瓨鍦?' })).toBeVisible();
});
