import { expect, test } from '@playwright/test';

import { mockDashboardBackend, seedSession } from './helpers';

const overflowRegressionRoutes = [
  '/index',
  '/chat/trajectory',
  '/chat/export',
  '/ai-insight/session-analysis',
  '/ai-insight/smart-analysis',
  '/ai-insight/emotion',
  '/ai-insight/communication-keyword',
  '/acquisition/v2-channel-code',
  '/acquisition/group-code',
  '/acquisition/redirect-link',
  '/acquisition/wechat-customer-service',
  '/acquisition/group-template',
  '/acquisition/precise-group-send',
  '/acquisition/material-management',
  '/customer/friends',
  '/customer/group',
  '/customer/tags',
  '/customer/order',
  '/data/customer',
  '/data/employee',
  '/data/conversion',
  '/data/behavior',
  '/data/report',
  '/ai-setting/ai-knowledge-base',
  '/ai-setting/agent',
  '/company-setting/website',
  '/company-setting/staff',
  '/setting/role',
  '/setting/additional',
] as const;

test.beforeEach(async ({ page }) => {
  await seedSession(page);
  await mockDashboardBackend(page, { menuRoutes: [...overflowRegressionRoutes] });
  await page.setViewportSize({ width: 390, height: 844 });
});

for (const route of overflowRegressionRoutes) {
  test(`${route} has no page-level horizontal overflow at 390px`, async ({ page }) => {
    await page.goto(route);
    await page.waitForLoadState('networkidle');
    await expect(page.locator('.dashboard-content > *').first()).toBeVisible();

    const metrics = await page.evaluate(() => ({
      clientWidth: document.documentElement.clientWidth,
      scrollWidth: document.documentElement.scrollWidth,
      offenders: [...document.querySelectorAll<HTMLElement>('body *')]
        .map((element) => {
          const bounds = element.getBoundingClientRect();
          return {
            className: element.className.toString().slice(0, 120),
            right: Math.round(bounds.right),
            tag: element.tagName,
            width: Math.round(bounds.width),
          };
        })
        .filter((item) => item.right > document.documentElement.clientWidth + 1)
        .slice(0, 8),
    }));

    expect(metrics.scrollWidth, JSON.stringify(metrics.offenders)).toBeLessThanOrEqual(metrics.clientWidth);
  });
}
