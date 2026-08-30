import { expect, test, type Page, type Route } from '@playwright/test';

import { mockDashboardBackend, seedSession } from './helpers';

const phase32PageContracts = [
  { route: '/index', closure: 'query-export-refresh' },
  { route: '/chat/v2-all', closure: 'filter-detail-refresh' },
  { route: '/ai-insight/v2/sensitive-word', closure: 'create-word-refresh' },
  { route: '/customer/clue/default', closure: 'create-lead-refresh' },
  { route: '/customer/contact', closure: 'append-follow-up-refresh' },
  { route: '/customer/opportunity', closure: 'advance-stage-refresh' },
  { route: '/customer/public-sea', closure: 'claim-refresh' },
  { route: '/customer/tags', closure: 'create-tag-refresh' },
] as const;

type Phase32Route = typeof phase32PageContracts[number]['route'];

async function json(route: Route, data: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify({ code: status === 200 ? 200 : status, msg: status === 200 ? 'success' : 'error', data }),
  });
}

function requestBody<T>(route: Route): T {
  return JSON.parse(route.request().postData() ?? '{}') as T;
}

async function installPhase32ContractBackend(page: Page) {
  const sensitiveWords = [{ sensitiveWordId: 1, groupId: 1, groupName: '重点词组', name: '泄密', status: 1, version: 'v1' }];
  const leads = [{ id: 'lead-1', businessKey: 'wx:ada', name: 'Ada', phone: '13800000000', source: 'wecom', status: 'new', ownerId: null, convertedContactId: '', discardReason: '', version: 1 }];
  const followUps = [{ id: 'follow-1', contactId: 'c1', content: '首次沟通', createdAt: '2026-08-01T08:00:00Z', createdBy: 1 }];
  const opportunity = { id: 'o1', contactId: 'c1', stage: 'proposal', amount: 100, startDate: '2026-08-01', endDate: '2026-08-15', ownerId: 1, status: 'open', lostReason: '', version: 1 };
  const publicPool = [{ id: 'a1', contactId: 'c-public', contactName: 'Grace', ownerId: null, collaboratorIds: [], status: 'public_pool', version: 1, source: 'wecom', businessType: '零售', tagNames: ['重点'], region: '上海', recycleCount: 1, poolAction: 'return', poolReason: '长期未联系', previousOwnerId: 18, lastFollowUpAt: '2026-07-31T08:00:00Z' }];
  const tagGroups = [{ id: 'g1', name: '核心组', version: 1, tagCount: 1 }];
  const tags = [{ id: 't1', groupId: 'g1', name: '重点', version: 1, usageCount: 1 }];
  const contact = { id: 'c1', name: '张三', phone: '13900000000', ownerId: 1, assignmentStatus: 'assigned', tagNames: ['重点'], version: 4, assignmentVersion: 4, updatedAt: '2026-08-01T08:00:00Z' };

  await page.route('**/dashboard/reports/overview*', async (route) => json(route, {
    summary: { customer: 12, lead: 4, contact: 8, opportunity: 2, won: 1, order: 1, behavior: 6, employee: 1, contactRate: 0.67, opportunityRate: 0.25, wonRate: 0.5, orderRate: 1 },
    series: [{ at: '2026-08-01T00:00:00Z', value: 12 }],
    items: [{ id: 'c1', day: '2026-08-01', ownerId: 12, ownerName: '销售一号' }],
    pagination: { page: 1, pageSize: 20, total: 1 },
    freshness: { provider: 'scrm', status: 'available', dataThrough: '2026-08-01T08:00:00Z' },
    limitations: [],
  }));
  await page.route('**/dashboard/workMessage/toUsers*', async (route) => json(route, {
    list: [{ id: 'msg-1', employeeId: 1, employeeName: '销售一号', employeeAvatar: '', targetType: 'customer', targetId: 9, targetName: '张三', targetAvatar: '', lastMessage: '合同已发送', sentAt: '2026-08-01T08:00:00Z' }],
    total: 1, page: 1, pageSize: 20,
  }));
  await page.route('**/dashboard/workMessage/detail*', async (route) => json(route, {
    id: 'msg-1', employeeId: 1, employeeName: '销售一号', targetType: 'customer', targetId: 9, targetName: '张三', messageTotal: 1, truncated: false, window: 'latest',
    messages: [{ id: 'message-1', senderName: '销售一号', senderAvatar: '', direction: 'outbound', type: 0, content: { text: '合同已发送' }, sentAt: '2026-08-01T08:00:00Z' }],
  }));
  await page.route('**/dashboard/sensitiveWordGroup/select*', async (route) => json(route, [{ groupId: 1, name: '重点词组', version: 'g1' }]));
  await page.route('**/dashboard/sensitiveWord/index*', async (route) => json(route, { list: sensitiveWords, page: { total: sensitiveWords.length, perPage: 10, totalPage: 1 } }));
  await page.route('**/dashboard/sensitiveWord/store*', async (route) => {
    const body = requestBody<{ groupId: number; name: string }>(route);
    sensitiveWords.push({ sensitiveWordId: sensitiveWords.length + 1, groupId: body.groupId, groupName: '重点词组', name: body.name, status: 1, version: `v${sensitiveWords.length + 1}` });
    await json(route, { version: sensitiveWords.at(-1)?.version, idempotent: false });
  });
  await page.route('**/dashboard/sensitiveWordsMonitor/index*', async (route) => json(route, { list: [], page: { total: 0, perPage: 10 } }));
  await page.route('**/dashboard/workEmployee/index*', async (route) => json(route, {
    list: [{ id: 12, name: '销售一号' }],
    page: { page: 1, perPage: 200, total: 1, totalPage: 1 },
  }));

  await page.route('**/dashboard/scrm/**', async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname.replace(/^\/dashboard/, '');
    const method = route.request().method();

    if (path === '/scrm/leads/duplicates' && method === 'GET') return json(route, { items: [] });
    if (path === '/scrm/settings' && method === 'GET') return json(route, [
      { type: 'funnel_stage', key: 'proposal', value: '方案', enabled: true },
      { type: 'funnel_stage', key: 'negotiation', value: '谈判', enabled: true },
    ]);
    if (path === '/scrm/leads' && method === 'GET') return json(route, { items: leads, nextCursor: '' });
    if (path === '/scrm/leads' && method === 'POST') {
      const body = requestBody<{ businessKey: string; name: string; phone: string; source: 'manual' | 'import' | 'wecom' }>(route);
      const lead = { id: `lead-${leads.length + 1}`, ...body, status: 'new', ownerId: null, convertedContactId: '', discardReason: '', version: 1 };
      leads.push(lead);
      return json(route, lead);
    }
    if (path === '/scrm/contacts' && method === 'GET') return json(route, { items: [contact], nextCursor: '' });
    if (path === '/scrm/contacts/c1' && method === 'GET') return json(route, {
      ...contact,
      assignment: { id: 'assignment-1', contactId: 'c1', ownerId: 1, collaboratorIds: [], status: 'assigned', version: 4 },
      tags, wecomFriends: [], wecomFriendsAvailable: false, opportunities: [opportunity], followUps,
    });
    if (path === '/scrm/contacts/c1/follow-ups' && method === 'GET') return json(route, { items: followUps, nextCursor: '' });
    if (path === '/scrm/contacts/c1/follow-ups' && method === 'POST') {
      const body = requestBody<{ content: string }>(route);
      const item = { id: `follow-${followUps.length + 1}`, contactId: 'c1', content: body.content, createdAt: '2026-08-01T09:00:00Z', createdBy: 1 };
      followUps.push(item);
      return json(route, item);
    }
    if (path === '/scrm/opportunities' && method === 'GET') return json(route, { items: [opportunity], nextCursor: '' });
    if (path === '/scrm/opportunities/o1/stage' && method === 'POST') {
      const body = requestBody<{ stageId: string; lostReason: string; version: number }>(route);
      opportunity.stage = body.stageId;
      opportunity.status = body.stageId === 'won' || body.stageId === 'lost' ? body.stageId : 'open';
      opportunity.lostReason = body.lostReason;
      opportunity.version = body.version + 1;
      return json(route, opportunity);
    }
    if (path === '/scrm/assignments' && method === 'GET') return json(route, { items: publicPool, nextCursor: '' });
    if (path === '/scrm/assignments/claim' && method === 'POST') {
      const body = requestBody<{ contactId: string }>(route);
      const index = publicPool.findIndex((item) => item.contactId === body.contactId);
      const [item] = index >= 0 ? publicPool.splice(index, 1) : [];
      return json(route, { ...item, ownerId: 1, status: 'assigned', version: (item?.version ?? 0) + 1 });
    }
    if (path === '/scrm/tags' && method === 'GET') {
      const groupId = url.searchParams.get('groupId');
      const keyword = url.searchParams.get('keyword')?.trim();
      return json(route, { groups: tagGroups, tags: tags.filter((tag) => (!groupId || tag.groupId === groupId) && (!keyword || tag.name.includes(keyword))) });
    }
    if (path === '/scrm/tags' && method === 'POST') {
      const body = requestBody<{ groupId: string; name: string }>(route);
      const tag = { id: `t${tags.length + 1}`, groupId: body.groupId, name: body.name, version: 1, usageCount: 0 };
      tags.push(tag);
      const group = tagGroups.find((item) => item.id === body.groupId);
      if (group) group.tagCount = tags.length;
      return json(route, tag);
    }
    return json(route, []);
  });
}

async function runClosure(page: Page, route: Phase32Route) {
  await page.goto(route);
  await expect(page).toHaveURL(new RegExp(`${route.replaceAll('/', '\\/')}(?:\\?.*)?$`));

  switch (route) {
    case '/index': {
      await expect(page.getByRole('heading', { name: '数据概览', exact: true })).toBeVisible();
      await page.getByLabel('开始日期').fill('2026-07-01');
      await page.getByLabel('结束日期').fill('2026-08-01');
      await page.getByRole('button', { name: '查询' }).click();
      await expect(page).toHaveURL((url) => url.searchParams.get('startDate') === '2026-07-01'
        && url.searchParams.get('endDate') === '2026-08-01');
      const download = page.waitForEvent('download');
      await page.getByRole('button', { name: '导出 CSV' }).click();
      expect((await download).suggestedFilename()).toBe('dashboard-overview.csv');
      await page.reload();
      await expect(page.getByLabel('开始日期')).toHaveValue('2026-07-01');
      await expect(page.getByLabel('结束日期')).toHaveValue('2026-08-01');
      break;
    }
    case '/chat/v2-all': {
      await expect(page.getByRole('region', { name: '查询概览' })).toBeVisible();
      await page.getByLabel('关键词').fill('合同');
      await page.getByRole('button', { name: '查询' }).click();
      await expect(page).toHaveURL(/keyword=%E5%90%88%E5%90%8C/);
      await page.getByRole('button', { name: '查看会话' }).click();
      await expect(page.getByRole('dialog', { name: '会话详情' })).toContainText('合同已发送');
      await page.reload();
      await expect(page.getByLabel('关键词')).toHaveValue('合同');
      break;
    }
    case '/ai-insight/v2/sensitive-word': {
      await expect(page.getByRole('heading', { name: '敏感词', exact: true })).toBeVisible();
      await page.getByRole('button', { name: '敏感词配置' }).click();
      await page.getByRole('button', { name: '新增敏感词' }).click();
      await page.getByLabel('新敏感词').fill('账号密码');
      await page.getByLabel('新词所属词组').selectOption('1');
      await page.getByRole('button', { name: '保存敏感词' }).click();
      await expect(page.getByText('账号密码')).toBeVisible();
      await page.reload();
      await page.getByRole('button', { name: '敏感词配置' }).click();
      await expect(page.getByText('账号密码')).toBeVisible();
      break;
    }
    case '/customer/clue/default': {
      await expect(page.getByRole('heading', { name: '线索池' })).toBeVisible();
      await page.getByLabel('客户名称').fill('新线索');
      await page.getByLabel('联系电话').fill('13700000000');
      await page.getByLabel('业务标识').fill('manual:new-lead');
      await page.getByRole('button', { name: '新增线索' }).click();
      await expect(page.getByText('新线索')).toBeVisible();
      await page.reload();
      await expect(page.getByText('新线索')).toBeVisible();
      break;
    }
    case '/customer/contact': {
      await expect(page.getByRole('heading', { name: '联系人', exact: true })).toBeVisible();
      await page.getByRole('button', { name: '查看张三' }).click();
      await page.getByLabel('跟进内容').fill('二次跟进');
      await page.getByRole('button', { name: '添加跟进' }).click();
      await expect(page.getByText(/二次跟进/)).toBeVisible();
      await page.reload();
      await expect(page.getByText(/二次跟进/)).toBeVisible();
      break;
    }
    case '/customer/opportunity': {
      await expect(page.getByRole('heading', { name: '商机管理' })).toBeVisible();
      await page.getByLabel('目标阶段 o1').selectOption('negotiation');
      await page.getByRole('button', { name: '推进阶段 o1' }).click();
      await expect(page.getByRole('cell', { name: '谈判', exact: true })).toBeVisible();
      await page.reload();
      await expect(page.getByRole('cell', { name: '谈判', exact: true })).toBeVisible();
      break;
    }
    case '/customer/public-sea': {
      await expect(page.getByRole('heading', { name: '客户公海' })).toBeVisible();
      await page.getByRole('button', { name: '领取 Grace' }).click();
      await expect(page.getByText('Grace')).toHaveCount(0);
      await page.reload();
      await expect(page.getByText('Grace')).toHaveCount(0);
      break;
    }
    case '/customer/tags': {
      await expect(page.getByRole('heading', { name: '客户标签' })).toBeVisible();
      await page.getByLabel('标签组筛选').selectOption('g1');
      await page.getByLabel('新标签', { exact: true }).fill('高意向');
      await page.getByRole('button', { name: '新增标签', exact: true }).click();
      await expect(page.getByText('高意向')).toBeVisible();
      await page.reload();
      await expect(page.getByText('高意向')).toBeVisible();
      break;
    }
  }
}

test.describe('Phase 3.2 eight-page contract regression', () => {
  test.beforeEach(async ({ page }) => {
    await seedSession(page);
    await mockDashboardBackend(page, { menuRoutes: phase32PageContracts.map((item) => item.route) });
    await installPhase32ContractBackend(page);
    await page.setViewportSize({ width: 1440, height: 1000 });
  });

  for (const contract of phase32PageContracts) {
    test(`${contract.route} closes ${contract.closure}`, async ({ page }) => {
      await runClosure(page, contract.route);
    });
  }
});
