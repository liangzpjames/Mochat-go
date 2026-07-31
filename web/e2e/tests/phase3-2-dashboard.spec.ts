import { expect, test } from '@playwright/test';

import { mockDashboardBackend, seedSession } from './helpers';

test.describe('Phase 3.2 SCRM real pages', () => {
  test.beforeEach(async ({ page }) => {
    await seedSession(page);
    await mockDashboardBackend(page, { menuRoutes: ['/customer/opportunity', '/customer/public-sea', '/customer/tags'] });
    await page.route('**/dashboard/scrm/opportunities*', async (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 200, msg: 'success', data: { items: [{ id: 'o1', contactId: 'c1', stage: 'proposal', amount: 100, startDate: '2026-08-01', endDate: '2026-08-02', ownerId: 1, status: 'open', version: 1 }], nextCursor: '' } }) }));
    await page.route('**/dashboard/scrm/assignments*', async (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 200, msg: 'success', data: { items: [{ id: 'a1', contactId: 'c1', ownerId: null, collaboratorIds: [], status: 'public_pool', version: 1 }], nextCursor: '' } }) }));
    await page.route('**/dashboard/scrm/tags*', async (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 200, msg: 'success', data: { items: [{ id: 't1', name: '重点', version: 1 }], nextCursor: '' } }) }));
    await page.route('**/dashboard/workContact/index*', async (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 200, msg: 'success', data: { list: [{ id: 'c1', name: '张三', phone: '13800000000' }] } }) }));
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

  test('renders contacts through the scoped legacy adapter', async ({ page }) => {
    await page.goto('/customer/contact');
    await expect(page.getByRole('heading', { name: '联系人' })).toBeVisible();
    await expect(page.getByText('张三')).toBeVisible();
  });

  test('captures the eight Phase 3.2 route acceptance evidence at 1440x1000', async ({ page }) => {
    test.setTimeout(60_000);
    const routes = [
      ['/index', 'index'],
      ['/chat/v2-all', 'chat-v2-all'],
      ['/ai-insight/v2/sensitive-word', 'sensitive-word'],
      ['/customer/clue/default', 'customer-clue-default'],
      ['/customer/contact', 'customer-contact'],
      ['/customer/opportunity', 'customer-opportunity'],
      ['/customer/public-sea', 'customer-public-sea'],
      ['/customer/tags', 'customer-tags'],
    ] as const;
    await page.setViewportSize({ width: 1440, height: 1000 });
    for (const [route, name] of routes) {
      await page.goto(route);
      await expect(page).toHaveURL(new RegExp(`${route.replaceAll('/', '\\/')}$`));
      await expect(page.locator('body')).toBeVisible();
      await page.screenshot({ path: `../../docs/phase/phase-3.2-dashboard-completion/evidence/${name}.png`, fullPage: false });
    }
  });
});
