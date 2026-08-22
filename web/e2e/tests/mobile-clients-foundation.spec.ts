import { expect, test, type Page } from '@playwright/test';
import { mkdirSync } from 'node:fs';
import { resolve } from 'node:path';

const sidebarCases = [
  { path: '/', title: '客户侧边栏', moduleLabel: '从当前客户会话开始工作', needsSession: true, expectsAction: false, activeNavigation: '我的' },
  { path: '/auth', title: '企业微信授权回调', moduleLabel: '登录失败', needsSession: false, expectsAction: false },
  { path: '/codeAuth', title: '企业微信扫码授权', moduleLabel: '兼容授权参数无效', needsSession: false, expectsAction: false },
  { path: '/contact', title: '客户资料', moduleLabel: '浏览器验收客户', needsSession: true, expectsAction: false, activeNavigation: '客户', query: '?wxExternalUserid=external-user-1&agentId=7' },
  { path: '/contact/editDetail', title: '编辑客户资料', moduleLabel: '画像备注', needsSession: true, expectsAction: false, activeNavigation: '客户', query: '?wxExternalUserid=external-user-1&agentId=7' },
  { path: '/contact/remark', title: '客户备注', moduleLabel: '保存备注', needsSession: true, expectsAction: false, activeNavigation: '客户', query: '?wxExternalUserid=external-user-1&agentId=7' },
  { path: '/contact/settingTag', title: '设置客户标签', moduleLabel: '保存标签', needsSession: true, expectsAction: false, activeNavigation: '客户', query: '?wxExternalUserid=external-user-1&agentId=7' },
  { path: '/contactBatchAdd', title: '批量加好友', moduleLabel: '13800000000', needsSession: true, expectsAction: false, activeNavigation: '客户', query: '?batchId=9&agentId=7' },
  { path: '/contactSop', title: '个人客户 SOP', moduleLabel: '浏览器验收客户', needsSession: true, expectsAction: false, activeNavigation: '会话', query: '?id=4&agentId=7' },
  { path: '/login', title: '侧边栏登录', moduleLabel: '继续授权', needsSession: false, expectsAction: true, query: '?agentId=7&target=%2Fcontact' },
  { path: '/medium', title: '素材库', moduleLabel: '浏览器素材', needsSession: true, expectsAction: false, activeNavigation: '客户', query: '?agentId=7' },
  { path: '/roomSop', title: '客户群 SOP', moduleLabel: '浏览器客户群', needsSession: true, expectsAction: false, activeNavigation: '会话', query: '?id=5&agentId=7' },
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
  { path: '/workFission', title: '任务宝活动', moduleLabel: '任务 1：邀请 3 位好友', expectsAction: false, query: '?id=17' },
  { path: '/speed', title: '任务宝进度', moduleLabel: '任务宝进度模块待迁移', expectsAction: false, query: '?union_id=union-browser-1&fission_id=17' },
] as const;

const viewports = [
  { name: 'mobile-390', width: 390, height: 844 },
  { name: 'desktop-1280', width: 1280, height: 900 },
] as const;

const browserContract = {
  fixtures: {
    contact: '**/sidebar/workContact/detail?*',
    openUserInfo: '**/operation/openUserInfo/workFission?*',
    taskData: '**/operation/workFission/taskData?*',
  },
  minimumActionHeight: 44,
  employeeNavigationLabel: '员工工作台',
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
  end_time: 4_102_444_800,
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

const rawWorkFissionParticipant = {
  openid: 'openid-browser-1',
  unionid: 'union-browser-session-1',
  nickname: '浏览器会话参与者',
  headimgurl: 'https://avatar.example/browser-session-1.png',
};

type BrowserAudit = {
  consoleErrors: string[];
  pageErrors: string[];
  requestFailures: string[];
  unexpectedResponses: string[];
  unexpectedRequests: string[];
  contactRequests: string[];
  workFissionParticipantRequests: string[];
  workFissionRequests: string[];
  workFissionEvidenceChecks: number;
  participantData: unknown;
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
    workFissionParticipantRequests: [],
    workFissionRequests: [],
    workFissionEvidenceChecks: 0,
    participantData: rawWorkFissionParticipant,
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
  for (const script of ['**/jweixin-1.2.0.js', '**/jwxwork-1.0.0.js']) {
    await page.route(script, async (route) => route.fulfill({ status: 200, contentType: 'text/javascript', body: '' }));
  }
  const sidebarFixture = async (pattern: string, data: unknown, requestId: string) => {
    await page.route(pattern, async (route) => {
      expect(route.request().headers().authorization).toBe('Bearer sidebar-browser-token');
      await route.fulfill({ status: 200, contentType: 'application/json', body: rawGoEnvelope(data, requestId) });
    });
  };
  await sidebarFixture('**/sidebar/workContact/show?*', {
    name: '浏览器验收客户', avatar: null, gender: 2, genderText: '女', businessNo: 'C-23',
    remark: '重点客户', description: '偏好下午沟通', tag: [{ tagId: 7, tagName: '高意向' }],
    roomName: ['浏览器客户群'], employeeName: ['员工甲'],
  }, 'sidebar-show-e2e');
  await sidebarFixture('**/sidebar/workContact/track?*', [{ id: 1, content: '已完成首次沟通', createdAt: '2026-08-22 09:00' }], 'sidebar-track-e2e');
  await sidebarFixture('**/sidebar/contactFieldPivot/index?*', [{ contactFieldId: 31, contactFieldPivotId: 901, name: '画像备注', type: 0, typeText: '文本', options: [], value: '已核验' }], 'sidebar-portrait-e2e');
  await sidebarFixture('**/sidebar/workContactTagGroup/index', [{ groupId: 3, groupName: '阶段' }], 'sidebar-tag-groups-e2e');
  await sidebarFixture('**/sidebar/workContactTag/allTag*', [{ id: 7, name: '高意向' }, { id: 9, name: '待回访' }], 'sidebar-tags-e2e');
  await sidebarFixture('**/sidebar/contactBatchAdd/detail?*', { employeeName: '员工甲', list: [{ id: 1, phone: '13800000000', status: '待添加' }] }, 'sidebar-batch-e2e');
  await sidebarFixture('**/sidebar/contactSop/getSopInfo?*', { id: 4, contactSopId: 12, creator: '员工甲', time: '09:00', tipTime: '2026-08-22 09:00', task: { content: [{ type: 0, value: '请今日回访' }] }, contact: { id: 23, name: '浏览器验收客户', avatar: null } }, 'sidebar-contact-sop-e2e');
  await sidebarFixture('**/sidebar/contactSop/getSopTipInfo?*', [{ id: 4, time: '2026-08-22 09:00' }], 'sidebar-contact-sop-tip-e2e');
  await sidebarFixture('**/sidebar/roomSop/getSopInfo?*', { id: 5, roomSopId: 13, creator: '员工甲', time: '10:00', state: 0, task: { content: [{ type: 0, value: '群内发送活动提醒' }] }, room: { id: 21, name: '浏览器客户群' } }, 'sidebar-room-sop-e2e');
  await sidebarFixture('**/sidebar/mediumGroup/index', [{ id: 0, name: '未分组' }], 'sidebar-medium-group-e2e');
  await sidebarFixture('**/sidebar/medium/index?*', { page: { perPage: 20, total: 1, totalPage: 1 }, list: [{ id: 8, type: '文本', mediaId: '', content: { content: '浏览器素材' } }] }, 'sidebar-medium-e2e');
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
    expect(route.request().headers().cookie ?? '').not.toMatch(/(?:^|;\s*)(?:token|agentId)=/);
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: rawGoEnvelope(rawTaskData, 'operation-e2e-request-1'),
    });
  });
  await page.route(browserContract.fixtures.openUserInfo, async (route) => {
    audit.workFissionParticipantRequests.push(route.request().url());
    expect(route.request().headers().authorization).toBeUndefined();
    expect(route.request().headers().cookie ?? '').not.toMatch(/(?:^|;\s*)(?:token|agentId)=/);
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: rawGoEnvelope(audit.participantData, 'operation-participant-e2e-request-1'),
    });
  });
  return audit;
}

async function injectSidebarSession(page: Page): Promise<void> {
  await page.context().addCookies([
    { name: 'token', value: 'sidebar-browser-token', domain: '127.0.0.1', path: '/sidebar-app' },
    { name: 'agentId', value: '7', domain: '127.0.0.1', path: '/sidebar-app' },
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
    'button:visible, a.sidebar-primary-link:visible, a.operation-primary-link:visible',
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

async function assertContentAboveBottomNavigation(page: Page): Promise<void> {
  const navigation = page.getByRole('navigation', {
    name: browserContract.employeeNavigationLabel,
  });
  const lastContent = page.locator('main > :last-child');
  await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
  await expect(lastContent).toBeVisible();
  const navigationBox = await navigation.boundingBox();
  const contentBox = await lastContent.boundingBox();
  expect(navigationBox).not.toBeNull();
  expect(contentBox).not.toBeNull();
  expect((contentBox?.y ?? 0) + (contentBox?.height ?? 0)).toBeLessThanOrEqual(
    navigationBox?.y ?? 0,
  );
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
        const navigation = page.getByRole('navigation', {
          name: browserContract.employeeNavigationLabel,
        });
        if (routeCase.needsSession) {
          await expect(navigation).toBeVisible();
          await expect(
            navigation.getByRole('link', { name: routeCase.activeNavigation }),
          ).toHaveAttribute('aria-current', 'page');
          await assertContentAboveBottomNavigation(page);
          const navigationHrefs = await navigation.locator('a').evaluateAll((links) => (
            links.map((link) => link.getAttribute('href'))
          ));
          expect(navigationHrefs.every((href) => href?.startsWith('/sidebar-app'))).toBe(true);
          if (routeCase.path === '/') {
            await navigation.getByRole('link', { name: '会话' }).click();
            expect(new URL(page.url()).pathname).toBe('/sidebar-app/contactSop');
          }
        } else {
          await expect(navigation).toHaveCount(0);
        }
        expect(audit.contactRequests).toHaveLength(['/contact', '/contact/editDetail', '/contact/remark', '/contact/settingTag'].includes(routeCase.path) ? 1 : 0);
        await assertStableCleanAudit(page, audit, `${viewport.name} Sidebar ${routeCase.path}`);
      });
    }

    for (const routeCase of operationCases) {
      test(`Operation ${routeCase.path} renders its route title and module`, async ({ page }) => {
        const audit = await installRawGoFixtures(page);
        await injectSidebarSession(page);
        await page.goto(mountedURL('/operation-app', routeCase.path, 'query' in routeCase ? routeCase.query : ''));
        const operationCookies = await page.evaluate(() => document.cookie);
        expect(operationCookies).not.toContain('sidebar-browser-token');
        expect(operationCookies).not.toMatch(/(?:^|;\s*)agentId=/);
        await assertVisiblePage(page, routeCase.title, routeCase.moduleLabel, routeCase.expectsAction);
        await expect(page.getByRole('navigation', {
          name: browserContract.employeeNavigationLabel,
        })).toHaveCount(0);
        expect(audit.workFissionRequests).toHaveLength(routeCase.path === '/workFission' ? 1 : 0);
        expect(audit.workFissionParticipantRequests).toHaveLength(
          routeCase.path === '/workFission' ? 1 : 0,
        );
        if (routeCase.path === '/workFission') {
          audit.workFissionEvidenceChecks += 1;
          await expect(page.getByText('已邀请 2 位好友', { exact: true })).toBeVisible();
          await expect(page.getByText('距下一任务还差 1 位', { exact: true })).toBeVisible();
          expect(new URL(audit.workFissionParticipantRequests[0] ?? '').searchParams.get('id')).toBe('17');
          expect(new URL(audit.workFissionRequests[0] ?? '').searchParams.get('union_id')).toBe(
            'union-browser-session-1',
          );

          audit.participantData = [];
          await page.goto('/operation-app/workFission?id=17&union_id=attacker-controlled');
          const authLink = page.getByRole('link', { name: '重新授权' });
          await expect(authLink).toBeVisible();
          await expect(authLink).toHaveAttribute(
            'href',
            '/auth/workFission?id=17&target=%2FworkFission%3Fid%3D17%26union_id%3Dattacker-controlled',
          );
          expect(audit.workFissionParticipantRequests).toHaveLength(2);
          expect(audit.workFissionRequests).toHaveLength(1);
          expect(audit.workFissionEvidenceChecks).toBe(1);
        }
        await assertStableCleanAudit(page, audit, `${viewport.name} Operation ${routeCase.path}`);
      });
    }

    test('Sidebar unknown path shows 404 and does not render home content', async ({ page }) => {
      const audit = await installRawGoFixtures(page);
      await page.goto('/sidebar-app/not-a-sidebar-page');
      await expect(page.getByRole('heading', { name: '页面不存在' })).toBeVisible();
      await expect(page.getByRole('heading', { level: 1, name: '客户侧边栏' })).toHaveCount(0);
      await expect(page.getByRole('navigation', {
        name: browserContract.employeeNavigationLabel,
      })).toHaveCount(0);
      await assertVisiblePage(page, '未找到页面', '页面不存在', false);
      await assertStableCleanAudit(page, audit, `${viewport.name} Sidebar unknown`);
    });

    test('Operation unknown path shows 404 and does not render home content', async ({ page }) => {
      const audit = await installRawGoFixtures(page);
      await page.goto('/operation-app/not-an-operation-page');
      await expect(page.getByRole('heading', { name: '页面不存在' })).toBeVisible();
      await expect(page.getByRole('heading', { level: 1, name: '营销活动中心' })).toHaveCount(0);
      await expect(page.getByRole('navigation', {
        name: browserContract.employeeNavigationLabel,
      })).toHaveCount(0);
      await assertVisiblePage(page, '未找到活动页面', '页面不存在', false);
      await assertStableCleanAudit(page, audit, `${viewport.name} Operation unknown`);
    });
  });
}

const employeeSidebarEvidenceDir = resolve(
  process.cwd(),
  '../../docs/reviews/evidence/employee-sidebar-mobile',
);

test.describe('employee Sidebar visual acceptance', () => {
  test.beforeAll(() => mkdirSync(employeeSidebarEvidenceDir, { recursive: true }));

  const visualCases = [
    { name: 'reference-2e667-home-360x800.png', width: 360, height: 800, path: '/', label: '从当前客户会话开始工作' },
    { name: 'reference-b8e1-contact-390x844.png', width: 390, height: 844, path: '/contact?wxExternalUserid=external-user-1&agentId=7', label: '客户概览' },
    { name: 'reference-7837-contact-sop-430x932.png', width: 430, height: 932, path: '/contactSop?id=4&agentId=7', label: '请今日回访' },
    { name: 'reference-cb8f-batch-add-360x800.png', width: 360, height: 800, path: '/contactBatchAdd?batchId=9&agentId=7', label: '13800000000' },
  ] as const;

  for (const visual of visualCases) {
    test(`${visual.width}x${visual.height} captures ${visual.path}`, async ({ page }) => {
      await page.setViewportSize({ width: visual.width, height: visual.height });
      const audit = await installRawGoFixtures(page);
      await injectSidebarSession(page);
      await page.goto(`/sidebar-app${visual.path}`);
      await expect(page.getByText(visual.label, { exact: true })).toBeVisible();
      const dimensions = await page.evaluate(() => ({
        clientWidth: document.documentElement.clientWidth,
        scrollWidth: document.documentElement.scrollWidth,
      }));
      expect(dimensions.scrollWidth).toBeLessThanOrEqual(dimensions.clientWidth);
      await page.screenshot({ path: resolve(employeeSidebarEvidenceDir, visual.name) });
      await assertStableCleanAudit(page, audit, `visual ${visual.name}`);
    });
  }

  test('320x568 narrow material view remains usable', async ({ page }) => {
    await page.setViewportSize({ width: 320, height: 568 });
    const audit = await installRawGoFixtures(page);
    await injectSidebarSession(page);
    await page.goto('/sidebar-app/medium?agentId=7');
    await expect(page.getByText('浏览器素材', { exact: true })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(320);
    const checkbox = page.getByRole('checkbox', { name: '浏览器素材' });
    const box = await checkbox.boundingBox();
    expect(box?.width ?? 0).toBeGreaterThanOrEqual(22);
    await page.screenshot({ path: resolve(employeeSidebarEvidenceDir, 'narrow-medium-320x568.png') });
    await assertStableCleanAudit(page, audit, 'visual narrow medium');
  });

  test('844x390 focused form is not horizontally clipped', async ({ page }) => {
    await page.setViewportSize({ width: 844, height: 390 });
    const audit = await installRawGoFixtures(page);
    await injectSidebarSession(page);
    await page.goto('/sidebar-app/contact/remark?wxExternalUserid=external-user-1&agentId=7');
    const input = page.getByLabel('备注名');
    await input.focus();
    await expect(input).toBeFocused();
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(844);
    await page.getByRole('button', { name: '保存备注' }).scrollIntoViewIfNeeded();
    await expect(page.getByRole('button', { name: '保存备注' })).toBeVisible();
    await page.screenshot({ path: resolve(employeeSidebarEvidenceDir, 'landscape-focused-remark-844x390.png') });
    await assertStableCleanAudit(page, audit, 'visual landscape focused form');
  });

  test('remark validation, save and cancel use one persisted write', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    const audit = await installRawGoFixtures(page);
    await injectSidebarSession(page);
    let updates = 0;
    await page.route('**/sidebar/workContact/update', async (route) => {
      updates += 1;
      await route.fulfill({ status: 200, contentType: 'application/json', body: rawGoEnvelope([], 'sidebar-remark-write-e2e') });
    });
    await page.goto('/sidebar-app/contact/remark?wxExternalUserid=external-user-1&agentId=7');
    await page.getByLabel('备注名').fill('');
    await page.getByRole('button', { name: '保存备注' }).click();
    await expect(page.getByRole('alert')).toContainText('请输入 1 至 10 个字符');
    expect(updates).toBe(0);
    await page.getByLabel('备注名').fill('新备注');
    await page.getByRole('button', { name: '保存备注' }).click();
    await expect(page.getByRole('heading', { name: '浏览器验收客户' })).toBeVisible();
    expect(updates).toBe(1);
    await page.goto('/sidebar-app/contact/remark?wxExternalUserid=external-user-1&agentId=7');
    await page.getByRole('button', { name: '取消' }).click();
    await expect(page.getByRole('heading', { name: '浏览器验收客户' })).toBeVisible();
    expect(updates).toBe(1);
    await assertStableCleanAudit(page, audit, 'remark persisted flow');
  });

  test('batch add accepts the real string-timestamp JSSDK contract and copies the chosen number', async ({ page }) => {
    await page.setViewportSize({ width: 430, height: 932 });
    await page.addInitScript(() => {
      const state = window as typeof window & { copiedPhone?: string; invokedAction?: string; ww?: unknown };
      Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: (value: string) => { state.copiedPhone = value; return Promise.resolve(); } } });
      state.ww = {
        agentConfig: (input: { success: () => void }) => input.success(),
        invoke: (name: string, _payload: unknown, callback: (result: Record<string, unknown>) => void) => { state.invokedAction = name; callback({ err_msg: `${name}:ok` }); },
      };
    });
    const audit = await installRawGoFixtures(page);
    await injectSidebarSession(page);
    await page.route('**/sidebar/agent/jssdkConfig?*', async (route) => route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: rawGoEnvelope({ corpid: 'wx-corp', agentid: '100001', timestamp: '1787360000', nonceStr: 'nonce', signature: 'sig' }, 'sidebar-jssdk-e2e'),
    }));
    await page.goto('/sidebar-app/contactBatchAdd?batchId=9&agentId=7');
    await page.getByRole('button', { name: '复制并添加' }).click();
    await expect(page.getByText(/已复制 13800000000/)).toBeVisible();
    expect(await page.evaluate(() => (window as typeof window & { copiedPhone?: string }).copiedPhone)).toBe('13800000000');
    expect(await page.evaluate(() => (window as typeof window & { invokedAction?: string }).invokedAction)).toBe('navigateToAddCustomer');
    await assertStableCleanAudit(page, audit, 'batch add real JSSDK contract');
  });
});
