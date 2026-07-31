import { expect, test } from '@playwright/test';

import { mockDashboardBackend, seedSession } from './helpers';

test('MoChat benchmark shell shows collapsed navigation and branded header', async ({ page }) => {
  await seedSession(page);
  await mockDashboardBackend(page);
  const consoleErrors: string[] = [];
  page.on('console', (message) => {
    if (message.type() === 'error') consoleErrors.push(message.text());
  });

  await page.goto('/');
  await expect(page.getByRole('link', { name: 'MoChat AI' })).toBeVisible();
  await expect(page).toHaveURL(/\/index$/);
  await expect(page.getByRole('link', { name: '数据概览' })).toBeVisible();
  await expect(page.getByRole('button', { name: /会话/ })).toBeVisible();
  await expect(page.getByRole('link', { name: '全局消息' })).toHaveCount(0);
  await page.getByRole('button', { name: /会话/ }).click();
  await expect(page.getByRole('link', { name: '全局消息' })).toBeVisible();
  await page.reload();
  await expect(page.getByRole('link', { name: 'MoChat AI' })).toBeVisible();
  expect(consoleErrors).toEqual([]);
});
