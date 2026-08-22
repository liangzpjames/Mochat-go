import { expect, test, type Page } from '@playwright/test';

const routes = [
  { path: '/', title: '客户侧边栏', label: '从当前客户会话开始工作', protected: true },
  { path: '/auth', title: '企业微信授权回调', label: '登录失败', protected: false },
  { path: '/codeAuth', title: '企业微信扫码授权', label: '兼容授权参数无效', protected: false },
  { path: '/contact', title: '客户资料', label: '专项验收客户', protected: true, query: '?wxExternalUserid=external-user-1&agentId=7' },
  { path: '/contact/editDetail', title: '编辑客户资料', label: '画像备注', protected: true, query: '?wxExternalUserid=external-user-1&agentId=7' },
  { path: '/contact/remark', title: '客户备注', label: '保存备注', protected: true, query: '?wxExternalUserid=external-user-1&agentId=7' },
  { path: '/contact/settingTag', title: '设置客户标签', label: '保存标签', protected: true, query: '?wxExternalUserid=external-user-1&agentId=7' },
  { path: '/contactBatchAdd', title: '批量加好友', label: '13800000000', protected: true, query: '?batchId=9&agentId=7' },
  { path: '/contactSop', title: '个人客户 SOP', label: '专项验收客户', protected: true, query: '?id=4&agentId=7' },
  { path: '/login', title: '侧边栏登录', label: '继续授权', protected: false, query: '?agentId=7&target=%2Fcontact' },
  { path: '/medium', title: '素材库', label: '专项验收素材', protected: true, query: '?agentId=7' },
  { path: '/roomSop', title: '客户群 SOP', label: '专项验收客户群', protected: true, query: '?id=5&agentId=7' },
] as const;

const viewports = [
  { name: '360x800', width: 360, height: 800 },
  { name: '390x844', width: 390, height: 844 },
  { name: '430x932', width: 430, height: 932 },
] as const;

type Audit = { console: string[]; page: string[]; failed: string[]; badResponses: string[]; unexpected: string[] };

function envelope(data: unknown): string {
  return JSON.stringify({ code: 200, msg: 'ok', data, requestId: 'sidebar-employee-mobile-e2e' });
}

async function fixture(page: Page, pattern: string, data: unknown): Promise<void> {
  await page.route(pattern, async (route) => {
    expect(route.request().headers().authorization).toBe('Bearer sidebar-browser-token');
    await route.fulfill({ status: 200, contentType: 'application/json', body: envelope(data) });
  });
}

async function installFixtures(page: Page): Promise<Audit> {
  const audit: Audit = { console: [], page: [], failed: [], badResponses: [], unexpected: [] };
  page.on('console', (message) => { if (message.type() === 'error') audit.console.push(message.text()); });
  page.on('pageerror', (error) => audit.page.push(error.message));
  page.on('requestfailed', (request) => audit.failed.push(`${request.method()} ${request.url()}`));
  page.on('response', (response) => { if (response.status() >= 400) audit.badResponses.push(`${response.status()} ${response.url()}`); });
  await page.route('**/sidebar/**', async (route) => {
    audit.unexpected.push(`${route.request().method()} ${route.request().url()}`);
    await route.fulfill({ status: 500, contentType: 'application/json', body: JSON.stringify({ code: 500, msg: 'unexpected request', data: null }) });
  });
  for (const script of ['**/jweixin-1.2.0.js', '**/jwxwork-1.0.0.js']) {
    await page.route(script, async (route) => route.fulfill({ status: 200, contentType: 'text/javascript', body: '' }));
  }
  await fixture(page, '**/sidebar/workContact/detail?*', { id: 23, name: '专项验收客户', avatar: null, corpId: 9 });
  await fixture(page, '**/sidebar/workContact/show?*', { name: '专项验收客户', avatar: null, gender: 2, genderText: '女', businessNo: 'C-23', remark: '重点客户', description: '偏好下午沟通', tag: [{ tagId: 7, tagName: '高意向' }], roomName: ['专项验收客户群'], employeeName: ['员工甲'] });
  await fixture(page, '**/sidebar/workContact/track?*', [{ id: 1, content: '已完成首次沟通', createdAt: '2026-08-22 09:00' }]);
  await fixture(page, '**/sidebar/contactFieldPivot/index?*', [{ contactFieldId: 31, contactFieldPivotId: 901, name: '画像备注', type: 0, typeText: '文本', options: [], value: '已核验' }]);
  await fixture(page, '**/sidebar/workContactTagGroup/index', [{ groupId: 3, groupName: '阶段' }]);
  await fixture(page, '**/sidebar/workContactTag/allTag*', [{ id: 7, name: '高意向' }, { id: 9, name: '待回访' }]);
  await fixture(page, '**/sidebar/contactBatchAdd/detail?*', { employeeName: '员工甲', list: [{ id: 1, phone: '13800000000', status: '待添加' }] });
  await fixture(page, '**/sidebar/contactSop/getSopInfo?*', { id: 4, contactSopId: 12, creator: '员工甲', time: '09:00', tipTime: '2026-08-22 09:00', task: { content: [{ type: 0, value: '请今日回访' }] }, contact: { id: 23, name: '专项验收客户', avatar: null } });
  await fixture(page, '**/sidebar/contactSop/getSopTipInfo?*', [{ id: 4, time: '2026-08-22 09:00' }]);
  await fixture(page, '**/sidebar/roomSop/getSopInfo?*', { id: 5, roomSopId: 13, creator: '员工甲', time: '10:00', state: 0, task: { content: [{ type: 0, value: '群内发送活动提醒' }] }, room: { id: 21, name: '专项验收客户群' } });
  await fixture(page, '**/sidebar/mediumGroup/index', [{ id: 0, name: '未分组' }]);
  await fixture(page, '**/sidebar/medium/index?*', { page: { perPage: 20, total: 1, totalPage: 1 }, list: [{ id: 8, type: '文本', mediaId: '', content: { content: '专项验收素材' } }] });
  return audit;
}

async function injectSession(page: Page): Promise<void> {
  await page.context().addCookies([
    { name: 'token', value: 'sidebar-browser-token', domain: '127.0.0.1', path: '/sidebar-app' },
    { name: 'agentId', value: '7', domain: '127.0.0.1', path: '/sidebar-app' },
  ]);
}

for (const viewport of viewports) {
  test.describe(`employee Sidebar ${viewport.name}`, () => {
    test.use({ viewport: { width: viewport.width, height: viewport.height } });

    for (const route of routes) {
      test(`${route.path} is reachable without overflow or unsafe failures`, async ({ page }) => {
        const audit = await installFixtures(page);
        if (route.protected) await injectSession(page);
        await page.goto(`/sidebar-app${route.path === '/' ? '/' : route.path}${'query' in route ? route.query : ''}`);
        await expect(page.getByRole('heading', { level: 1, name: route.title })).toBeVisible();
        await expect(page.getByText(route.label, { exact: true })).toBeVisible();
        const geometry = await page.evaluate(() => ({ client: document.documentElement.clientWidth, scroll: document.documentElement.scrollWidth }));
        expect(geometry.scroll).toBeLessThanOrEqual(geometry.client);
        const actions = page.locator('button:visible, a.sidebar-primary-link:visible');
        for (let index = 0; index < await actions.count(); index += 1) {
          const box = await actions.nth(index).boundingBox();
          expect(box).not.toBeNull();
          expect(box?.height ?? 0).toBeGreaterThanOrEqual(44);
        }
        await page.waitForLoadState('networkidle');
        expect(audit).toEqual({ console: [], page: [], failed: [], badResponses: [], unexpected: [] });
      });
    }
  });
}

test.describe('employee Sidebar business states at 390x844', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('shows delayed loading, retryable server error and persisted empty state', async ({ page }) => {
    await installFixtures(page);
    await injectSession(page);
    let attempts = 0;
    await page.route('**/sidebar/medium/index?*', async (route) => {
      attempts += 1;
      if (attempts === 1) {
        await new Promise((resolve) => setTimeout(resolve, 150));
        await route.fulfill({ status: 500, contentType: 'application/json', body: JSON.stringify({ code: 500, msg: '专项服务器错误', data: null }) });
        return;
      }
      await route.fulfill({ status: 200, contentType: 'application/json', body: envelope({ page: { perPage: 20, total: 0, totalPage: 0 }, list: [] }) });
    });
    await page.goto('/sidebar-app/medium?agentId=7');
    await expect(page.getByText('正在读取持久化数据。')).toBeVisible();
    await expect(page.getByRole('heading', { name: '加载失败' })).toBeVisible();
    await page.getByRole('button', { name: '重试' }).click();
    await expect(page.getByText('暂无可用素材')).toBeVisible();
    expect(attempts).toBe(2);
  });

  test('validates, saves and cancels a persisted remark without duplicate writes', async ({ page }) => {
    await installFixtures(page);
    await injectSession(page);
    const writes: Array<Record<string, unknown>> = [];
    await page.route('**/sidebar/workContact/update', async (route) => {
      writes.push(route.request().postDataJSON() as Record<string, unknown>);
      await route.fulfill({ status: 200, contentType: 'application/json', body: envelope([]) });
    });
    await page.goto('/sidebar-app/contact/remark?wxExternalUserid=external-user-1&agentId=7');
    await page.getByLabel('备注名').fill('');
    await page.getByRole('button', { name: '保存备注' }).click();
    await expect(page.getByRole('alert')).toContainText('请输入 1 至 10 个字符');
    expect(writes).toHaveLength(0);
    await page.getByLabel('备注名').fill('新备注');
    await page.getByRole('button', { name: '保存备注' }).click();
    await expect(page.getByRole('heading', { name: '专项验收客户' })).toBeVisible();
    expect(writes).toEqual([{ contactId: 23, remark: '新备注' }]);
    await page.goto('/sidebar-app/contact/remark?wxExternalUserid=external-user-1&agentId=7');
    await page.getByRole('button', { name: '取消' }).click();
    expect(writes).toHaveLength(1);
  });

  test('appends only a new tag and saves an edited portrait field', async ({ page }) => {
    await installFixtures(page);
    await injectSession(page);
    const contactWrites: Array<Record<string, unknown>> = [];
    const portraitWrites: Array<Record<string, unknown>> = [];
    await page.route('**/sidebar/workContact/update', async (route) => {
      contactWrites.push(route.request().postDataJSON() as Record<string, unknown>);
      await route.fulfill({ status: 200, contentType: 'application/json', body: envelope([]) });
    });
    await page.route('**/sidebar/contactFieldPivot/update', async (route) => {
      portraitWrites.push(route.request().postDataJSON() as Record<string, unknown>);
      await route.fulfill({ status: 200, contentType: 'application/json', body: envelope([]) });
    });
    await page.goto('/sidebar-app/contact/settingTag?wxExternalUserid=external-user-1&agentId=7');
    await page.getByLabel('待回访').check();
    await page.getByRole('button', { name: '保存标签' }).click();
    await expect(page.getByRole('heading', { name: '专项验收客户' })).toBeVisible();
    expect(contactWrites).toEqual([{ contactId: 23, tag: [9] }]);
    await page.goto('/sidebar-app/contact/editDetail?wxExternalUserid=external-user-1&agentId=7');
    await page.getByLabel('画像备注').fill('浏览器新画像');
    await page.getByRole('button', { name: '保存画像' }).click();
    await expect(page.getByRole('heading', { name: '专项验收客户' })).toBeVisible();
    expect(portraitWrites).toHaveLength(1);
    expect(portraitWrites[0]?.contactId).toBe(23);
    expect(portraitWrites[0]?.userPortrait).toEqual([expect.objectContaining({ contactFieldId: 31, value: '浏览器新画像' })]);
  });

  test('keeps material browsing honest when the WeCom SDK is unavailable', async ({ page }) => {
    await installFixtures(page);
    await injectSession(page);
    await page.goto('/sidebar-app/medium?agentId=7');
    await expect(page.getByText('专项验收素材')).toBeVisible();
    await expect(page.getByText(/需在企业微信客户端中打开才能发送/)).toBeVisible();
    await page.getByRole('checkbox', { name: '专项验收素材' }).check();
    await page.getByRole('button', { name: '发送已选素材' }).click();
    await expect(page.getByText('需在企业微信客户端中打开才能发送素材。')).toBeVisible();
  });

  test('persists room SOP completion and confirms the non-zero terminal state', async ({ page }) => {
    await installFixtures(page);
    await injectSession(page);
    let state = 0;
    let updates = 0;
    await page.route('**/sidebar/roomSop/getSopInfo?*', async (route) => route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: envelope({ id: 5, roomSopId: 13, creator: '员工甲', time: '10:00', state, task: { content: [] }, room: { id: 21, name: '专项验收客户群' } }),
    }));
    await page.route('**/sidebar/roomSop/logState', async (route) => {
      updates += 1;
      state = 2;
      await route.fulfill({ status: 200, contentType: 'application/json', body: envelope([]) });
    });
    await page.goto('/sidebar-app/roomSop?id=5&agentId=7');
    await page.getByRole('button', { name: '标记为已完成' }).click();
    const completed = page.getByRole('button', { name: '已完成' });
    await expect(completed).toBeDisabled();
    expect(updates).toBe(1);
  });

  test('reloads the persisted batch for the selected status filter', async ({ page }) => {
    await installFixtures(page);
    await injectSession(page);
    const statuses: string[] = [];
    await page.route('**/sidebar/contactBatchAdd/detail?*', async (route) => {
      statuses.push(new URL(route.request().url()).searchParams.get('status') ?? '');
      await route.fulfill({ status: 200, contentType: 'application/json', body: envelope(statuses.at(-1) === '1' ? { employeeName: '员工甲', list: [] } : { employeeName: '员工甲', list: [{ id: 1, phone: '13800000000', status: '待添加' }] }) });
    });
    await page.goto('/sidebar-app/contactBatchAdd?batchId=9&agentId=7');
    await expect(page.getByText('13800000000')).toBeVisible();
    await page.getByLabel('添加状态').selectOption('1');
    await expect(page.getByText('当前筛选下暂无客户')).toBeVisible();
    expect(statuses).toEqual(['4', '1']);
  });
});
