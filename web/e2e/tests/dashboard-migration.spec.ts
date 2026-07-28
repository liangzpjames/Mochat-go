import { expect, test } from '@playwright/test';

import {
  mockDashboardBackend,
  seedSession,
} from './helpers';

test.beforeEach(async ({ page }) => {
  await seedSession(page);
  await mockDashboardBackend(page);
});

test('allowlisted legacy route preserves query and hash', async ({ page }) => {
  await page.goto('/workContact/index?source=e2e#details');

  await expect(page).toHaveURL(
    /\/_legacy\/dashboard\/workContact\/index\?source=e2e#details$/,
  );
  await expect(page.locator('#app')).toBeAttached();
});

test('unknown legacy URL returns 404', async ({ page }) => {
  const response = await page.goto('/_legacy/dashboard/not-in-manifest');

  expect(response?.status()).toBe(404);
});

test('API requests never return the SPA document', async ({ page }) => {
  await page.goto('/corp/index');
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
