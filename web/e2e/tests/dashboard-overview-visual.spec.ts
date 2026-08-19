import { mkdirSync } from 'node:fs';
import { resolve } from 'node:path';

import { expect, test } from '@playwright/test';

import { mockDashboardBackend, seedSession } from './helpers';

const evidenceRoot = resolve(
  process.env.MOCHAT_EVIDENCE_ROOT ?? 'D:/workspace/mochat-go/output',
  'overview-ui-refresh-20260818',
);
mkdirSync(evidenceRoot, { recursive: true });

const overviewPayload = {
  summary: { customer: 137, lead: 58, contact: 46, opportunity: 30, won: 15, order: 12, behavior: 41, employee: 0 },
  series: [
    { at: '2026-08-12T00:00:00+08:00', value: 4 },
    { at: '2026-08-13T00:00:00+08:00', value: 7 },
    { at: '2026-08-14T00:00:00+08:00', value: 5 },
    { at: '2026-08-15T00:00:00+08:00', value: 9 },
    { at: '2026-08-16T00:00:00+08:00', value: 12 },
  ],
  aiInsight: {
    capability: 'ready', provider: 'dashscope', generatedAt: '2026-08-16T00:00:00+08:00',
    summary: '✅ **核心客户意图**\n1. 商务洽谈与价格协商\n- 今日内发送报价单',
  },
  conversation: {
    customer: { sessions: 11, employeeMessages: 22, customerMessages: 16 },
    room: { sessions: 4, employeeMessages: 6, customerMessages: 6 },
    trend: [{ date: '2026-08-16', customerSessions: 7, customerEmployeeMessages: 9, customerCustomerMessages: 5, roomSessions: 2, roomEmployeeMessages: 3, roomCustomerMessages: 2 }],
  },
  limitations: [],
  freshness: { provider: 'scrm', status: 'available', dataThrough: '2026-08-16T09:30:00+08:00' },
  pagination: { page: 1, pageSize: 20, total: 5 },
};

test.beforeEach(async ({ page }) => {
  await seedSession(page);
  await mockDashboardBackend(page, { menuRoutes: ['/index', '/ai-insight/smart-analysis', '/ai-setting/ai-knowledge-base'] });
  await page.route('**/dashboard/reports/overview**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ code: 200, msg: 'success', data: overviewPayload }),
    });
  });
});

test('renders the desktop operating cockpit', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto('/index');
  await expect(page.getByRole('region', { name: '经营快照' })).toBeVisible();
  await expect(page.getByRole('article', { name: '分析状态' })).toBeVisible();
  await expect(page.getByRole('article', { name: '重点洞察' })).toBeVisible();
  await expect(page.getByRole('region', { name: '会话工作台' })).toBeVisible();
  await expect(page.getByRole('region', { name: '质检数据' })).toBeVisible();
  await expect(page.getByRole('region', { name: '经营趋势明细' })).toHaveCount(0);
  await page.screenshot({ fullPage: true, path: resolve(evidenceRoot, 'overview-desktop.png') });
});

test('keeps the mobile cockpit within the 390px viewport', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/index');
  await expect(page.getByRole('region', { name: '经营快照' })).toBeVisible();
  const size = await page.evaluate(() => ({ client: document.documentElement.clientWidth, scroll: document.documentElement.scrollWidth }));
  expect(size.scroll).toBeLessThanOrEqual(size.client);
  await page.screenshot({ fullPage: true, path: resolve(evidenceRoot, 'overview-mobile-390.png') });
});
