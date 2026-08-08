import { mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';

import { expect, test } from '@playwright/test';

import manifest from '../../apps/dashboard/src/benchmark/manifest.json' with { type: 'json' };

const base = process.env.MOCHAT_E2E_LIVE_BASE ?? 'http://127.0.0.1:18080';
const phone = process.env.PHASE35_ACCEPT_PHONE ?? '13800000000';
const password = process.env.PHASE35_ACCEPT_PASSWORD ?? 'MochatLocal@123';
const evidenceDir = resolve('artifacts/dashboard-interaction-unification');
const baseOrigin = new URL(base).origin;
const dashboardRoutes = manifest.pages.map(({ path, title }) => ({ path, title }));
const shellRequestPaths = new Set([
  '/dashboard/user/auth',
  '/dashboard/user/logout',
  '/dashboard/corp/select',
  '/dashboard/role/permissionByUser',
]);

const viewports = [
  { name: 'desktop', width: 1440, height: 1000 },
  { name: 'mobile-390', width: 390, height: 844 },
] as const;

const isAcceptedBusinessResponse = (status: number) => (
  (status >= 200 && status < 400) || status === 403 || status === 409 || status === 422
);

test.beforeAll(async () => {
  await mkdir(evidenceDir, { recursive: true });
});

test.beforeEach(async ({ page }) => {
  await page.goto(`${base}/login`, { waitUntil: 'domcontentloaded' });
  await page.locator('#login-phone').fill(phone);
  await page.locator('#login-password').fill(password);
  await page.getByRole('button', { name: '登录' }).click();
  await page.waitForURL(/\/index(?:\?|$)/, { timeout: 20_000 });
  await page.waitForLoadState('networkidle');
});

for (const route of dashboardRoutes) {
  test(`${route.path} desktop and 390px real interaction`, async ({ page }) => {
    const runtimeErrors: string[] = [];
    const businessResponses: Array<{ status: number; url: string }> = [];
    page.on('pageerror', (error) => runtimeErrors.push(error.message));
    page.on('response', (response) => {
      const responseUrl = new URL(response.url());
      if (
        responseUrl.origin === baseOrigin
        && responseUrl.pathname.startsWith('/dashboard/')
        && !shellRequestPaths.has(responseUrl.pathname)
      ) {
        businessResponses.push({ status: response.status(), url: responseUrl.pathname });
      }
    });

    for (const viewport of viewports) {
      const responseOffset = businessResponses.length;
      await page.setViewportSize({ width: viewport.width, height: viewport.height });
      await page.goto(`${base}${route.path}`, { waitUntil: 'domcontentloaded' });
      await expect(page.locator('h1').first()).toContainText(route.title, { timeout: 15_000 });
      await expect(page.locator('.dashboard-content > *').first()).toBeVisible();

      const body = page.locator('body');
      await expect(body).not.toContainText(/Cannot read properties|出错了|数据加载失败/);

      const content = page.locator('.dashboard-content');
      const businessSurface = content.locator('table, form, [role="region"], .dashboard-data-card:not(.dashboard-page-header), .phase35-card, .dashboard-stat-grid, .phase35-kpis, .page-state').filter({ visible: true }).first();
      await expect(businessSurface, `${route.path} must expose a real business or Provider surface`).toBeVisible({ timeout: 15_000 });

      const refreshAction = content.getByRole('button', { name: /^(刷新|重新加载|重试)/ }).filter({ visible: true }).first();
      const queryAction = content.getByRole('button', { name: /^(查询|搜索)/ }).filter({ visible: true }).first();
      const refreshIsUsable = await refreshAction.isVisible().catch(() => false) && await refreshAction.isEnabled();
      const queryIsUsable = await queryAction.isVisible().catch(() => false) && await queryAction.isEnabled();
      const refreshesBusinessData = refreshIsUsable;
      const safeAction = refreshIsUsable ? refreshAction : queryAction;
      if (refreshIsUsable || queryIsUsable) {
        const interactionResponseOffset = businessResponses.length;
        const textFilter = content.locator('input:not([type]):not([readonly]):not([disabled]), input[type="text"]:not([readonly]):not([disabled]), input[type="search"]:not([readonly]):not([disabled]), textarea:not([readonly]):not([disabled])').filter({ visible: true }).first();
        let provesLocalQuery = false;
        if (!refreshesBusinessData) {
          if (await textFilter.isVisible().catch(() => false)) {
            await textFilter.fill('验收');
            await expect(textFilter).toHaveValue('验收');
            provesLocalQuery = true;
          } else {
            const selectFilters = content.locator('select:not([disabled])').filter({ visible: true });
            for (let index = 0; index < await selectFilters.count(); index += 1) {
              const selectFilter = selectFilters.nth(index);
              const currentValue = await selectFilter.inputValue();
              const nextValue = await selectFilter.locator('option:not([disabled])').evaluateAll((options, selected) => (
                options.map((option) => (option as HTMLOptionElement).value).find((value) => Boolean(value) && value !== selected)
              ), currentValue);
              if (nextValue) {
                await selectFilter.selectOption(nextValue);
                await expect(selectFilter).toHaveValue(nextValue);
                provesLocalQuery = true;
                break;
              }
            }
          }
        }
        await safeAction.click();
        if (!provesLocalQuery) {
          await expect.poll(
            () => businessResponses.slice(interactionResponseOffset).some(({ status }) => isAcceptedBusinessResponse(status)),
            { message: `${route.path} safe action must complete a page business request`, timeout: 15_000 },
          ).toBe(true);
        }
      } else {
        const inactiveSegment = content.locator('button[aria-pressed="false"]').filter({ visible: true }).first();
        const inactiveTab = content.locator('[role="tab"][aria-selected="false"]').filter({ visible: true }).first();
        const textFilter = content.locator('input:not([type]):not([readonly]):not([disabled]), input[type="text"]:not([readonly]):not([disabled]), input[type="search"]:not([readonly]):not([disabled]), textarea:not([readonly]):not([disabled])').filter({ visible: true }).first();
        const selectFilter = content.locator('select:not([disabled])').filter({ visible: true }).first();
        const summary = content.locator('summary').filter({ visible: true }).first();
        const safeLink = content.locator('a[href^="/"]:not([download]):not([target="_blank"])').filter({ visible: true }).first();
        const dangerAction = content.getByRole('button', { name: /^(停用|启用|删除|移除|发布|授权)/ }).filter({ visible: true }).first();

        if (await inactiveSegment.isVisible().catch(() => false)) {
          await inactiveSegment.click();
          await expect(inactiveSegment).toHaveAttribute('aria-pressed', 'true');
        } else if (await inactiveTab.isVisible().catch(() => false)) {
          await inactiveTab.click();
          await expect(inactiveTab).toHaveAttribute('aria-selected', 'true');
        } else if (await textFilter.isVisible().catch(() => false)) {
          await textFilter.fill('验收');
          await expect(textFilter).toHaveValue('验收');
        } else if (await selectFilter.isVisible().catch(() => false)) {
          const currentValue = await selectFilter.inputValue();
          const optionValues = await selectFilter.locator('option:not([disabled])').evaluateAll((options) => (
            options.map((option) => (option as HTMLOptionElement).value).filter(Boolean)
          ));
          const nextValue = optionValues.find((value) => value !== currentValue);
          if (!nextValue) {
            throw new Error(`${route.path} select must offer a safe alternate filter`);
          }
          await selectFilter.selectOption(nextValue);
          await expect(selectFilter).toHaveValue(nextValue);
        } else if (await summary.isVisible().catch(() => false)) {
          const details = summary.locator('xpath=..');
          const wasOpen = await details.evaluate((element) => element.hasAttribute('open'));
          await summary.click();
          if (wasOpen) {
            await expect(details).not.toHaveAttribute('open', '');
          } else {
            await expect(details).toHaveAttribute('open', '');
          }
        } else if (await safeLink.isVisible().catch(() => false)) {
          const urlBefore = page.url();
          await safeLink.click();
          await expect.poll(() => page.url(), { message: `${route.path} safe link must navigate` }).not.toBe(urlBefore);
          await page.goBack({ waitUntil: 'domcontentloaded' });
          await expect(page.locator('h1').first()).toContainText(route.title);
        } else if (await dangerAction.isVisible().catch(() => false)) {
          const dangerItem = dangerAction.locator('xpath=..');
          const stateBefore = await dangerItem.innerText();
          let nativeConfirmation = '';
          page.once('dialog', async (dialog) => {
            nativeConfirmation = dialog.message();
            await dialog.dismiss();
          });
          await dangerAction.click();
          const confirmation = page.locator('[role="dialog"], [role="tooltip"]').filter({ visible: true }).first();
          await expect.poll(async () => (
            Boolean(nativeConfirmation) || await confirmation.isVisible().catch(() => false)
          ), { message: `${route.path} dangerous action must open confirmation before mutation` }).toBe(true);
          if (await confirmation.isVisible().catch(() => false)) {
            const cancel = confirmation.getByRole('button', { name: /^(取消|关闭)$/ }).filter({ visible: true }).first();
            await expect(cancel, `${route.path} confirmation must offer a non-mutating exit`).toBeVisible();
            await cancel.click();
            await expect(confirmation).toBeHidden();
          }
          await expect(dangerItem).toHaveText(stateBefore);
        } else {
          const openAction = content.getByRole('button', { name: /^(新增|添加|新建|创建|查看|详情|编辑|配置|管理)/ }).filter({ visible: true }).first();
          await expect(openAction, `${route.path} must expose an observable safe business interaction`).toBeVisible();
          const urlBefore = page.url();
          await openAction.click();
          const dialog = page.getByRole('dialog').filter({ visible: true }).first();
          await expect.poll(async () => (
            await dialog.isVisible().catch(() => false)
            || page.url() !== urlBefore
            || await openAction.getAttribute('aria-expanded') === 'true'
            || await openAction.getAttribute('aria-pressed') === 'true'
          ), { message: `${route.path} safe action must expose a dialog, route, or expanded state` }).toBe(true);
          if (await dialog.isVisible().catch(() => false)) {
            const cancel = dialog.getByRole('button', { name: /^(取消|关闭)$/ }).filter({ visible: true }).first();
            await expect(cancel, `${route.path} dialog must offer a non-mutating exit`).toBeVisible();
            await cancel.click();
            await expect(dialog).toBeHidden();
          }
        }
      }

      await expect.poll(
        () => businessResponses.slice(responseOffset).some(({ status }) => isAcceptedBusinessResponse(status)),
        { message: `${route.path} must complete a real dashboard API request at ${viewport.width}px`, timeout: 15_000 },
      ).toBe(true);

      const overflow = await page.evaluate(() => ({
        clientWidth: document.documentElement.clientWidth,
        scrollWidth: document.documentElement.scrollWidth,
      }));
      expect(overflow.scrollWidth, `${route.path} overflow at ${viewport.width}px`).toBeLessThanOrEqual(overflow.clientWidth);

      const slug = route.path === '/index' ? 'index' : route.path.slice(1).replaceAll('/', '__');
      await page.screenshot({ fullPage: true, path: resolve(evidenceDir, `${slug}-${viewport.name}.png`) });
    }

    expect(runtimeErrors, `${route.path} runtime errors`).toEqual([]);
  });
}
