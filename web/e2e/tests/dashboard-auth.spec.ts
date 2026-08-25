import { expect, test } from '@playwright/test';

import { mockDashboardBackend, seedSession } from './helpers';

test('protected URL redirects to Dashboard login and returns to the requested page', async ({ page }) => {
  await mockDashboardBackend(page);
  await page.goto('/index?from=e2e');
  await expect(page).toHaveURL(/\/login\?returnTo=%2Findex%3Ffrom%3De2e$/);

  await page.locator('input[autocomplete="username"]').fill('13800000000');
  await page.locator('input[autocomplete="current-password"]').fill('secret');
  await page.locator('button[type="submit"]').click();

  await expect(page).toHaveURL(/\/index\?from=e2e$/);
  await expect(page.locator('.dashboard-content > *').first()).toBeVisible();
});

test('401 clears the Dashboard session and returns to login', async ({ page }) => {
  await seedSession(page);
  await mockDashboardBackend(page);
  await page.unroute('**/dashboard/access/profile');
  await page.route('**/dashboard/access/profile', async (route) => {
    await route.fulfill({
      status: 401,
      contentType: 'application/json',
      body: JSON.stringify({ code: 401, msg: 'unauthorized', data: null }),
    });
  });
  await page.goto('/index');

  await expect(page).toHaveURL(/\/login/);
  await expect.poll(() => page.evaluate(() => localStorage.getItem('mochat_dashboard_token'))).toBeNull();
  await expect.poll(() => page.evaluate(() => localStorage.getItem('mochat_dashboard_corp_id'))).toBeNull();
});

test('403 remains inside the React error boundary without an enterprise selector', async ({ page }) => {
  await seedSession(page);
  await mockDashboardBackend(page, { menuRoutes: ['/workContact/index'] });
  await page.goto('/index');

  await expect(page).toHaveURL(/\/index$/);
  await expect(page.getByRole('heading', { name: '无权访问' })).toBeVisible();
  await expect(page.locator('.dashboard-corp-switcher')).toHaveCount(0);
});
