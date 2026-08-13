import { expect, test, type Page } from '@playwright/test';

const APP_ORIGIN = 'http://127.0.0.1:4174';

const sidebarCases = [
  { path: '/', title: '客户侧边栏', moduleLabel: '客户侧边栏模块待迁移', needsSession: true, expectsAction: false },
  { path: '/auth', title: '企业微信授权回调', moduleLabel: '登录失败', needsSession: false, expectsAction: false },
  { path: '/codeAuth', title: '企业微信扫码授权', moduleLabel: '企业微信扫码授权模块待迁移', needsSession: false, expectsAction: false },
  { path: '/contact', title: '客户资料', moduleLabel: '浏览器验收客户', needsSession: true, expectsAction: false, query: '?wxExternalUserid=external-user-1&agentId=7' },
  { path: '/contact/editDetail', title: '编辑客户资料', moduleLabel: '编辑客户资料模块待迁移', needsSession: true, expectsAction: false },
  { path: '/contact/remark', title: '客户备注', moduleLabel: '客户备注模块待迁移', needsSession: true, expectsAction: false },
  { path: '/contact/settingTag', title: '设置客户标签', moduleLabel: '设置客户标签模块待迁移', needsSession: true, expectsAction: false },
  { path: '/contactBatchAdd', title: '批量加好友', moduleLabel: '批量加好友模块待迁移', needsSession: true, expectsAction: false },
  { path: '/contactSop', title: '个人客户 SOP', moduleLabel: '个人客户 SOP模块待迁移', needsSession: true, expectsAction: false },
  { path: '/login', title: '侧边栏登录', moduleLabel: '继续授权', needsSession: false, expectsAction: true, query: '?agentId=7&target=%2Fcontact' },
  { path: '/medium', title: '素材库', moduleLabel: '素材库模块待迁移', needsSession: true, expectsAction: false },
  { path: '/roomSop', title: '客户群 SOP', moduleLabel: '客户群 SOP模块待迁移', needsSession: true, expectsAction: false },
] as const;

const operationCases = [
  { path: '/', title: '营销活动中心', moduleLabel: '营销活动中心模块待迁移', expectsAction: false },
  { path: '/explain', title: '活动说明', moduleLabel: '活动说明模块待迁移', expectsAction: false },
  { path: '/lottery', title: '抽奖活动', moduleLabel: '抽奖活动模块待迁移', expectsAction: false },
  { path: '/roomClockIn', title: '群打卡', moduleLabel: '群打卡模块待迁移', expectsAction: false },
  { path: '/roomFission', title: '群裂变活动', moduleLabel: '群裂变活动模块待迁移', expectsAction: false },
  { path: '/fissionSpeed', title: '群裂变进度', moduleLabel: '群裂变进度模块待迁移', expectsAction: false },
  { path: '/roomInfinitePull', title: '无限拉群', moduleLabel: '无限拉群模块待迁移', expectsAction: false },
  { path: '/shopCode', title: '门店活码', moduleLabel: '门店活码模块待迁移', expectsAction: false },
  { path: '/workFission', title: '任务宝活动', moduleLabel: '任务 1：邀请 3 位好友', expectsAction: false, query: '?union_id=union-browser-1&fission_id=17' },
  { path: '/speed', title: '任务宝进度', moduleLabel: '任务宝进度模块待迁移', expectsAction: false, query: '?union_id=union-browser-1&fission_id=17' },
] as const;

const viewports = [
  { name: 'mobile-390', width: 390, height: 844 },
  { name: 'desktop-1280', width: 1280, height: 900 },
] as const;

const browserContract = {
  fixtures: {
    contact: '**/sidebar/workContact/detail?*',
    taskData: '**/operation/workFission/taskData?*',
  },
  minimumActionHeight: 44,
} as const;

const rawContactData = {
  id: 23,
  name: '浏览器验收客户',
  avatar: null,
  corpId: 9,
};

const rawTaskData = {
  invite_count: 2,
  differ_count: 1,
  end_time: 0,
  task: [
    {
      count: 3,
      status: 0,
      receive_status: 0,
      gift_type: 1,
      gift_url: '/operation/reward/17',
    },
  ],
};

type BrowserAudit = {
  consoleErrors: string[];
  pageErrors: string[];
  requestFailures: string[];
  unexpectedResponses: string[];
  unexpectedRequests: string[];
  contactRequests: string[];
  workFissionRequests: string[];
};

function mountedURL(mount: '/sidebar-app' | '/operation-app', path: string, query = ''): string {
  return `${mount}${path === '/' ? '/' : path}${query}`;
}

function rawGoEnvelope(data: unknown, requestId: string): string {
  return JSON.stringify({ code: 200, msg: 'ok', data, requestId });
}

async function installRawGoFixtures(page: Page): Promise<BrowserAudit> {
  const audit: BrowserAudit = {
    consoleErrors: [],
    pageErrors: [],
    requestFailures: [],
    unexpectedResponses: [],
    unexpectedRequests: [],
    contactRequests: [],
    workFissionRequests: [],
  };

  page.on('console', (message) => {
    if (message.type() === 'error') audit.consoleErrors.push(message.text());
  });
  page.on('pageerror', (error) => audit.pageErrors.push(error.message));
  page.on('requestfailed', (request) => {
    audit.requestFailures.push(`${request.method()} ${request.url()} ${request.failure()?.errorText ?? ''}`);
  });
  page.on('response', (response) => {
    if (response.status() >= 400) {
      audit.unexpectedResponses.push(`${response.status()} ${response.request().method()} ${response.url()}`);
    }
  });

  await page.route('**/sidebar/**', async (route) => {
    audit.unexpectedRequests.push(`${route.request().method()} ${route.request().url()}`);
    await route.fulfill({
      status: 500,
      contentType: 'application/json',
      body: JSON.stringify({ code: 500, msg: 'unexpected Sidebar request', data: null }),
    });
  });
  await page.route('**/operation/**', async (route) => {
    audit.unexpectedRequests.push(`${route.request().method()} ${route.request().url()}`);
    await route.fulfill({
      status: 500,
      contentType: 'application/json',
      body: JSON.stringify({ code: 500, msg: 'unexpected Operation request', data: null }),
    });
  });
  await page.route(browserContract.fixtures.contact, async (route) => {
    audit.contactRequests.push(route.request().url());
    expect(route.request().headers().authorization).toBe('Bearer sidebar-browser-token');
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: rawGoEnvelope(rawContactData, 'sidebar-e2e-request-1'),
    });
  });
  await page.route(browserContract.fixtures.taskData, async (route) => {
    audit.workFissionRequests.push(route.request().url());
    expect(route.request().headers().authorization).toBeUndefined();
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: rawGoEnvelope(rawTaskData, 'operation-e2e-request-1'),
    });
  });
  return audit;
}

async function injectSidebarSession(page: Page): Promise<void> {
  await page.context().addCookies([
    { name: 'token', value: 'sidebar-browser-token', url: APP_ORIGIN },
    { name: 'agentId', value: '7', url: APP_ORIGIN },
  ]);
}

async function assertVisiblePage(
  page: Page,
  title: string,
  moduleLabel: string,
  expectsAction: boolean,
): Promise<void> {
  await expect(page.locator('main')).toBeVisible();
  await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();
  await expect(page.getByText(moduleLabel, { exact: true })).toBeVisible();
  const pageShape = await page.evaluate(() => ({
    clientWidth: document.documentElement.clientWidth,
    scrollWidth: document.documentElement.scrollWidth,
    text: document.querySelector('main')?.textContent?.trim() ?? '',
  }));
  expect(pageShape.text, `${title} rendered a blank main region`).not.toBe('');
  expect(pageShape.scrollWidth, `${title} overflowed horizontally`).toBeLessThanOrEqual(pageShape.clientWidth);

  const controls = page.locator(
    'a.sidebar-primary-link:visible, a.operation-primary-link:visible, button.mobile-state__action:visible',
  );
  if (expectsAction) {
    expect(await controls.count()).toBeGreaterThanOrEqual(1);
  }
  for (let index = 0; index < await controls.count(); index += 1) {
    const box = await controls.nth(index).boundingBox();
    expect(box, `${title} primary or retry control is not visible`).not.toBeNull();
    expect(box?.height ?? 0, `${title} primary or retry control is shorter than 44px`).toBeGreaterThanOrEqual(browserContract.minimumActionHeight);
  }
}

function assertCleanAudit(audit: BrowserAudit, label: string): void {
  expect(audit.consoleErrors, `${label} console errors`).toEqual([]);
  expect(audit.pageErrors, `${label} uncaught page errors`).toEqual([]);
  expect(audit.requestFailures, `${label} request failures`).toEqual([]);
  expect(audit.unexpectedResponses, `${label} unexpected 4xx/5xx responses`).toEqual([]);
  expect(audit.unexpectedRequests, `${label} unexpected API requests`).toEqual([]);
}

async function assertStableCleanAudit(page: Page, audit: BrowserAudit, label: string): Promise<void> {
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(0);
  assertCleanAudit(audit, label);
}

for (const viewport of viewports) {
  test.describe(viewport.name, () => {
    test.use({ viewport: { width: viewport.width, height: viewport.height } });

    for (const routeCase of sidebarCases) {
      test(`Sidebar ${routeCase.path} renders its route title and module`, async ({ page }) => {
        const audit = await installRawGoFixtures(page);
        if (routeCase.needsSession) await injectSidebarSession(page);
        await page.goto(mountedURL('/sidebar-app', routeCase.path, 'query' in routeCase ? routeCase.query : ''));
        await assertVisiblePage(page, routeCase.title, routeCase.moduleLabel, routeCase.expectsAction);
        expect(audit.contactRequests).toHaveLength(routeCase.path === '/contact' ? 1 : 0);
        await assertStableCleanAudit(page, audit, `${viewport.name} Sidebar ${routeCase.path}`);
      });
    }

    for (const routeCase of operationCases) {
      test(`Operation ${routeCase.path} renders its route title and module`, async ({ page }) => {
        const audit = await installRawGoFixtures(page);
        await page.goto(mountedURL('/operation-app', routeCase.path, 'query' in routeCase ? routeCase.query : ''));
        await assertVisiblePage(page, routeCase.title, routeCase.moduleLabel, routeCase.expectsAction);
        expect(audit.workFissionRequests).toHaveLength(routeCase.path === '/workFission' ? 1 : 0);
        if (routeCase.path === '/workFission') {
          await expect(page.getByText('已邀请 2 位好友', { exact: true })).toBeVisible();
          await expect(page.getByText('距下一任务还差 1 位', { exact: true })).toBeVisible();
        }
        await assertStableCleanAudit(page, audit, `${viewport.name} Operation ${routeCase.path}`);
      });
    }

    test('Sidebar unknown path shows 404 and does not render home content', async ({ page }) => {
      const audit = await installRawGoFixtures(page);
      await page.goto('/sidebar-app/not-a-sidebar-page');
      await expect(page.getByRole('heading', { name: '页面不存在' })).toBeVisible();
      await expect(page.getByRole('heading', { level: 1, name: '客户侧边栏' })).toHaveCount(0);
      await assertVisiblePage(page, '未找到页面', '页面不存在', false);
      await assertStableCleanAudit(page, audit, `${viewport.name} Sidebar unknown`);
    });

    test('Operation unknown path shows 404 and does not render home content', async ({ page }) => {
      const audit = await installRawGoFixtures(page);
      await page.goto('/operation-app/not-an-operation-page');
      await expect(page.getByRole('heading', { name: '页面不存在' })).toBeVisible();
      await expect(page.getByRole('heading', { level: 1, name: '营销活动中心' })).toHaveCount(0);
      await assertVisiblePage(page, '未找到活动页面', '页面不存在', false);
      await assertStableCleanAudit(page, audit, `${viewport.name} Operation unknown`);
    });
  });
}
