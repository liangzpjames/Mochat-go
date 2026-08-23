import { expect, test } from '@playwright/test';

test.use({ viewport: { width: 390, height: 844 } });

test('continue authorization completes the local review callback and opens the workbench', async ({ page }) => {
  const consoleErrors: string[] = [];
  const pageErrors: string[] = [];
  page.on('console', (message) => {
    if (message.type() === 'error') consoleErrors.push(message.text());
  });
  page.on('pageerror', (error) => pageErrors.push(error.message));

  await page.goto('/sidebar-app/login?agentId=7&target=%2F');
  await page.getByRole('link', { name: '继续授权' }).click();

  await expect(page).toHaveURL(/\/sidebar-app\/?(?:\?.*)?$/);
  await expect(page.getByRole('heading', { name: '客户' })).toBeVisible();
  await expect(page.getByText('客户经营工作台', { exact: true })).toBeVisible();
  const cookies = await page.context().cookies();
  expect(cookies.some((cookie) => cookie.name === 'token' && cookie.path === '/sidebar-app')).toBe(true);
  expect(cookies.some((cookie) => cookie.name === 'agentId' && cookie.value === '7' && cookie.path === '/sidebar-app')).toBe(true);
  expect(consoleErrors).toEqual([]);
  expect(pageErrors).toEqual([]);
});
