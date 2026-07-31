import { expect, test } from '@playwright/test';

import { mockDashboardBackend, seedSession } from './helpers';

test.describe('Phase 3.2 SCRM real pages', () => {
  test.beforeEach(async ({ page }) => {
    await seedSession(page);
    await mockDashboardBackend(page, { menuRoutes: ['/customer/opportunity', '/customer/public-sea', '/customer/tags'] });
    await page.route('**/dashboard/scrm/opportunities*', async (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 200, msg: 'success', data: { items: [{ id: 'o1', contactId: 'c1', stage: 'proposal', amount: 100, startDate: '2026-08-01', endDate: '2026-08-02', ownerId: 1, status: 'open', version: 1 }], nextCursor: '' } }) }));
    await page.route('**/dashboard/scrm/assignments*', async (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 200, msg: 'success', data: { items: [{ id: 'a1', contactId: 'c1', ownerId: null, collaboratorIds: [], status: 'public_pool', version: 1 }], nextCursor: '' } }) }));
    await page.route('**/dashboard/scrm/tags*', async (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 200, msg: 'success', data: { items: [{ id: 't1', name: '重点', version: 1 }], nextCursor: '' } }) }));
  });

  test('renders the opportunity page from the registered route', async ({ page }) => {
    await page.goto('/customer/opportunity');
    await expect(page.getByRole('heading', { name: '机会' })).toBeVisible();
    await expect(page.getByText('c1')).toBeVisible();
  });

  test('renders public pool and tags from the registered routes', async ({ page }) => {
    await page.goto('/customer/public-sea');
    await expect(page.getByText('c1')).toBeVisible();
    await page.goto('/customer/tags');
    await expect(page.getByRole('heading', { name: '客户标签' })).toBeVisible();
    await expect(page.getByText('重点')).toBeVisible();
  });
});
