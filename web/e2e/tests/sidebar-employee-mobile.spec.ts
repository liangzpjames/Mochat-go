import { expect, test, type Page } from '@playwright/test';
import { mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import reviewFixture from '../fixtures/sidebar-employee-review.json' with { type: 'json' };

const referenceEvidenceDir = resolve(
  import.meta.dirname,
  '../../../docs/reviews/evidence/2026-08-23-sidebar-reference-replica',
);
mkdirSync(referenceEvidenceDir, { recursive: true });

const routes = [
  { path: '/', title: '客户', label: '客户经营工作台', protected: true },
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
    expect(route.request().headers().authorization).toBe(`Bearer ${reviewFixture.token}`);
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
  await fixture(page, '**/sidebar/workbench/summary', reviewFixture.workbenchSummary);
  await page.route('**/sidebar/workContact/index?*', async (route) => {
    expect(route.request().headers().authorization).toBe(`Bearer ${reviewFixture.token}`);
    const keyword = new URL(route.request().url()).searchParams.get('keyword')?.trim() ?? '';
    const items = reviewFixture.workContacts.items.filter((item) => keyword === ''
      || item.name.includes(keyword) || item.remark.includes(keyword));
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: envelope({ ...reviewFixture.workContacts, total: items.length, totalPage: items.length === 0 ? 0 : 1, items }),
    });
  });
  for (const [kind, taskPage] of Object.entries(reviewFixture.workbenchTasks)) {
    await fixture(page, `**/sidebar/workbench/tasks?kind=${kind}&*`, taskPage);
  }
  await fixture(page, '**/sidebar/workContact/detail?*', reviewFixture.contactDetail);
  await fixture(page, '**/sidebar/workContact/show?*', reviewFixture.contactSummary);
  await fixture(page, '**/sidebar/workContact/track?*', reviewFixture.tracks);
  await fixture(page, '**/sidebar/contactFieldPivot/index?*', reviewFixture.portraitFields);
  await fixture(page, '**/sidebar/workContactTagGroup/index', reviewFixture.tagGroups);
  await fixture(page, '**/sidebar/workContactTag/allTag*', reviewFixture.tags);
  await fixture(page, '**/sidebar/contactBatchAdd/detail?*', reviewFixture.batch);
  await fixture(page, '**/sidebar/contactSop/getSopInfo?*', reviewFixture.contactSop);
  await fixture(page, '**/sidebar/contactSop/getSopTipInfo?*', reviewFixture.contactSopTips);
  await fixture(page, '**/sidebar/roomSop/getSopInfo?*', reviewFixture.roomSop);
  await fixture(page, '**/sidebar/mediumGroup/index', reviewFixture.mediumGroups);
  await fixture(page, '**/sidebar/medium/index?*', reviewFixture.mediumPage);
  return audit;
}

async function injectSession(page: Page): Promise<void> {
  await page.context().addCookies([
    { name: 'token', value: reviewFixture.token, domain: '127.0.0.1', path: '/sidebar-app' },
    { name: 'agentId', value: reviewFixture.agentId, domain: '127.0.0.1', path: '/sidebar-app' },
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

test.describe('employee Sidebar compact density', () => {
  test.use({ viewport: { width: 360, height: 800 } });

  test('keeps the workbench compact without shrinking touch targets', async ({ page }) => {
    await installFixtures(page);
    await injectSession(page);
    await page.goto('/sidebar-app/');
    const features = page.locator('.workbench-features--five .workbench-feature');
    await expect(features).toHaveCount(5);
    const featureHeights: number[] = [];
    for (let index = 0; index < await features.count(); index += 1) {
      const box = await features.nth(index).boundingBox();
      expect(box).not.toBeNull();
      expect(box?.width ?? 0).toBeGreaterThan(0);
      expect(box?.height ?? 0).toBeGreaterThan(0);
      featureHeights.push(box?.height ?? 0);
    }
    const density = await page.evaluate(() => {
      const hero = document.querySelector('.workbench-hero')?.getBoundingClientRect();
      const metrics = document.querySelector('.workbench-metrics')?.getBoundingClientRect();
      return {
        heroHeight: hero?.height ?? 999,
        metricsHeight: metrics?.height ?? 999,
      };
    });
    expect(density.heroHeight).toBeLessThanOrEqual(170);
    expect(density.metricsHeight).toBeLessThanOrEqual(74);
    expect(Math.max(...featureHeights)).toBeLessThanOrEqual(82);
    expect(Math.min(...featureHeights)).toBeGreaterThanOrEqual(44);
  });
});

test.describe('employee Sidebar business states at 390x844', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('renders all four reference-mapped workspaces from real review APIs', async ({ page }) => {
    const audit = await installFixtures(page);
    await injectSession(page);
    await page.goto('/sidebar-app/?agentId=7');
    await expect(page.getByText('126', { exact: true })).toBeVisible();
    await page.screenshot({ path: resolve(referenceEvidenceDir, 'implemented-customer-390x844.png') });

    await page.getByTestId('mobile-shell').getByRole('link', { name: '会话', exact: true }).click();
    await expect(page.getByText('数据范围说明', { exact: true })).toBeVisible();
    await page.screenshot({ path: resolve(referenceEvidenceDir, 'implemented-conversation-390x844.png') });
    await page.getByRole('link', { name: /^个人客户 SOP/ }).click();
    await expect(page.getByText('新客首日回访', { exact: true })).toBeVisible();

    await page.getByTestId('mobile-shell').getByRole('link', { name: '我的', exact: true }).click();
    await expect(page.getByText('员工会话已建立', { exact: true })).toBeVisible();
    await page.screenshot({ path: resolve(referenceEvidenceDir, 'implemented-profile-390x844.png') });

    await page.getByTestId('mobile-shell').getByRole('link', { name: '客户', exact: true }).click();
    await page.getByRole('link', { name: /^通讯录/ }).click();
    await expect(page.getByText('星河科技有限公司采购负责人', { exact: true })).toBeVisible();
    await page.screenshot({ path: resolve(referenceEvidenceDir, 'implemented-contacts-390x844.png') });
    await page.getByRole('searchbox', { name: '搜索客户' }).fill('陈晨');
    await page.getByRole('searchbox', { name: '搜索客户' }).press('Enter');
    await expect(page.getByText('陈晨', { exact: true })).toBeVisible();
    await expect(page.getByText('专项验收客户', { exact: true })).toHaveCount(0);
    const geometry = await page.evaluate(() => ({ client: document.documentElement.clientWidth, scroll: document.documentElement.scrollWidth }));
    expect(geometry.scroll).toBeLessThanOrEqual(geometry.client);
    expect(audit).toEqual({ console: [], page: [], failed: [], badResponses: [], unexpected: [] });
  });

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

  test('round-trips customer context through profile and opens real SOP reminders', async ({ page }) => {
    const audit = await installFixtures(page);
    await injectSession(page);
    await page.goto('/sidebar-app/contact?wxExternalUserid=external-user-1&agentId=7');
    await expect(page.getByRole('heading', { name: '专项验收客户' })).toBeVisible();

    await page.getByRole('link', { name: '我的' }).click();
    await expect(page).toHaveURL(/\/sidebar-app\/\?agentId=7&wxExternalUserid=external-user-1&tab=profile$/);
    await expect(page.getByText('员工甲', { exact: true })).toBeVisible();
    await page.getByTestId('mobile-shell').getByRole('link', { name: '客户', exact: true }).click();
    await page.getByRole('link', { name: /^当前客户/ }).click();
    await expect(page.getByRole('heading', { name: '专项验收客户' })).toBeVisible();

    await page.getByRole('link', { name: '查看 SOP 提醒' }).click();
    await expect(page).toHaveURL(/\/sidebar-app\/contactSop\?.*contactId=23/);
    await expect(page.getByRole('heading', { name: '个人客户 SOP' })).toBeVisible();
    await expect(page.getByText('专项验收客户', { exact: true })).toBeVisible();
    await page.waitForLoadState('networkidle');
    expect(audit).toEqual({ console: [], page: [], failed: [], badResponses: [], unexpected: [] });
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
      body: envelope(state === reviewFixture.roomSop.state ? reviewFixture.roomSop : { ...reviewFixture.roomSop, state }),
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
      await route.fulfill({ status: 200, contentType: 'application/json', body: envelope(statuses.at(-1) === '1' ? { ...reviewFixture.batch, list: [] } : reviewFixture.batch) });
    });
    await page.goto('/sidebar-app/contactBatchAdd?batchId=9&agentId=7');
    await expect(page.getByText('13800000000')).toBeVisible();
    await page.getByLabel('添加状态').selectOption('1');
    await expect(page.getByText('当前筛选下暂无客户')).toBeVisible();
    expect(statuses).toEqual(['4', '1']);
  });
});
