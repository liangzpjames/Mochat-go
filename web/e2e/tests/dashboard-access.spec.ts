import { expect, test } from '@playwright/test';

import { mockDashboardBackend, seedSession } from './helpers';

test.beforeEach(async ({ page }) => {
  await seedSession(page);
  await mockDashboardBackend(page);
});

test('React route renders the migrated page and authorized actions', async ({ page }) => {
  await page.goto('/corp/index');

  await expect(page.getByText('企业微信授权', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: '修改' })).toBeVisible();
});

test('unknown route renders React 404 without legacy fallback', async ({ page }) => {
  await page.goto('/not-in-manifest');

  await expect(page).toHaveURL(/\/not-in-manifest$/);
  await expect(page.getByRole('heading', { name: '页面不存在' })).toBeVisible();
});
