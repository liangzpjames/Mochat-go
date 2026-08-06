import { expect, test } from '@playwright/test';

/**
 * Phase 3.5 真实部署验收（live acceptance）。
 *
 * 默认跳过；设置 MOCHAT_E2E_LIVE_BASE 指向真实部署后运行，例如：
 *   $env:MOCHAT_E2E_LIVE_BASE='http://127.0.0.1:18080'
 *   pnpm --filter @mochat/e2e exec playwright test tests/phase35-live.spec.ts --workers=1
 *
 * 账号默认使用部署管理员（13800000000 / MochatLocal@123），
 * 可通过 PHASE35_ACCEPT_PHONE / PHASE35_ACCEPT_PASSWORD 覆盖。
 */

const base = process.env.MOCHAT_E2E_LIVE_BASE ?? '';
const phone = process.env.PHASE35_ACCEPT_PHONE ?? '13800000000';
const password = process.env.PHASE35_ACCEPT_PASSWORD ?? 'MochatLocal@123';

const phase35Routes = [
  { route: '/customer/friends', h1: '好友' },
  { route: '/customer/group', h1: '客户群' },
  { route: '/customer/order', h1: '订单' },
  { route: '/customer/settings', h1: '客户设置' },
  { route: '/data/customer', h1: '客户分析' },
  { route: '/data/employee', h1: '会话分析' },
  { route: '/data/conversion', h1: '转化分析' },
  { route: '/data/behavior', h1: '行为分析' },
  { route: '/data/report', h1: '综合报表' },
] as const;

test.describe('Phase 3.5 真实部署验收', () => {
  test.skip(!base, '设置 MOCHAT_E2E_LIVE_BASE 指向真实部署后运行');

  test.beforeEach(async ({ page }) => {
    await page.goto(`${base}/login`, { waitUntil: 'domcontentloaded' });
    await page.locator('#login-phone').fill(phone);
    await page.locator('#login-password').fill(password);
    await page.getByRole('button', { name: '登录' }).click();
    await page.waitForURL(/\/index/, { timeout: 20000 });
    await page.waitForTimeout(800);
  });

  test('九页可访问、标题正确且无运行时错误', async ({ page }) => {
    for (const item of phase35Routes) {
      await page.goto(`${base}${item.route}`, { waitUntil: 'domcontentloaded' });
      await page.waitForSelector('h1', { timeout: 15000 });
      await expect(page.locator('h1').first()).toHaveText(item.h1);
      const body = await page.locator('body').innerText();
      expect(/加载失败|出错了|Cannot read properties of null/.test(body)).toBe(false);
    }
  });

  test('订单跨页工作流：复用/创建订单 → 已支付 → 转化报表订单阶段可见', async ({ page }) => {
    await page.goto(`${base}/customer/order`, { waitUntil: 'domcontentloaded' });
    await page.waitForSelector('h1', { timeout: 15000 });
    await page.waitForTimeout(1200);

    const existing = page.locator('tr', { hasText: 'P35-ACCEPT-验收订单' }).first();
    if ((await existing.count()) > 0) {
      const status = await existing.locator('td').nth(4).innerText().catch(() => '');
      await existing.getByRole('button', { name: '查看详情' }).click();
      await page.waitForTimeout(1000);
      if (status.includes('待支付')) {
        const paid = page.getByRole('button', { name: '变更为已支付' }).first();
        if (await paid.isVisible().catch(() => false)) {
          await paid.click();
          await page.waitForTimeout(1200);
        }
      }
    } else {
      const select = page.getByRole('combobox', { name: '联系人' });
      if ((await select.locator('option').count()) > 1) {
        await select.selectOption({ index: 1 });
      }
      await page.getByRole('textbox', { name: '订单标题' }).fill(`P35-ACCEPT-验收订单-${Date.now()}`);
      await page.getByRole('textbox', { name: '金额（元）' }).fill('128');
      await page.getByRole('button', { name: '创建订单' }).click();
      await page.waitForTimeout(1500);
    }

    await page.goto(`${base}/data/conversion`, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(1500);
    const body = await page.locator('body').innerText();
    const orderStage = body.match(/订单\s*(\d+)/);
    expect(orderStage && Number(orderStage[1]) > 0).toBe(true);
  });
});
