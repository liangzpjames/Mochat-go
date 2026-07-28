import { expect, test } from '@playwright/test';

import { mockDashboardBackend, seedSession } from './helpers';

test('protected URL redirects to login and login returns safely', async ({ page }) => {
  await mockDashboardBackend(page);
  await page.goto('/corp/index?from=e2e');
  await expect(page).toHaveURL(/\/login\?returnTo=%2Fcorp%2Findex%3Ffrom%3De2e$/);

  await page.locator('input[autocomplete="username"]').fill('13800000000');
  await page.locator('input[autocomplete="current-password"]').fill('secret');
  await page.locator('button[type="submit"]').click();

  await expect(page).toHaveURL(/\/corp\/index$/);
  await expect(page.getByText('企业微信授权', { exact: true })).toBeVisible();
});

test('multiple enterprises require selection and switching binds the choice', async ({ page }) => {
  await seedSession(page, null);
  await mockDashboardBackend(page, {
    corps: [
      { corpId: 7, corpName: '企业甲' },
      { corpId: 8, corpName: '企业乙' },
    ],
  });
  await page.goto('/');

  await expect(page.getByRole('button', { name: '企业甲' })).toBeVisible();
  await page.getByRole('button', { name: '企业乙' }).click();
  await expect(page).toHaveURL(/\/corp\/index$/);
  await expect.poll(() => page.evaluate(() => {
    const value = localStorage.getItem('corpId');
    if (value === null) return null;
    const parsed: unknown = JSON.parse(value);
    return typeof parsed === 'string' ? parsed : null;
  })).toBe('8');
});

test('401 clears the session and returns to login', async ({ page }) => {
  await seedSession(page);
  await mockDashboardBackend(page, { unauthorizedCorpSelect: true });
  await page.goto('/corp/index');

  await expect(page).toHaveURL(/\/login/);
  await expect.poll(() => page.evaluate(() => localStorage.getItem('ACCESS_TOKEN'))).toBeNull();
});

test('403 remains inside the React error boundary', async ({ page }) => {
  await seedSession(page);
  await mockDashboardBackend(page, { menuRoutes: ['/workContact/index'] });
  await page.goto('/corp/index');

  await expect(page).toHaveURL(/\/corp\/index$/);
  await expect(page.getByRole('heading', { name: '无权访问' })).toBeVisible();
});
