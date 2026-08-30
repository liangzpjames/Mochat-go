import { expect, type Page } from '@playwright/test';

import reviewFixture from './sidebar-employee-review.json' with { type: 'json' };

type Phase2MobileRouteContract = {
  heading: string;
  moduleLabel: string;
  needsSession?: boolean;
  query?: string;
};

export type Phase2MobileAudit = {
  consoleErrors: string[];
  pageErrors: string[];
  requestFailures: string[];
  unexpectedResponses: string[];
  unexpectedRequests: string[];
};

export const phase2MobileRouteContracts = {
  sidebar: {
    '/': { heading: '客户', moduleLabel: '客户经营工作台', needsSession: true },
    '/auth': { heading: '企业微信授权回调', moduleLabel: '登录失败' },
    '/codeAuth': { heading: '企业微信扫码授权', moduleLabel: '兼容授权参数无效' },
    '/contact': { heading: '客户资料', moduleLabel: '专项验收客户', needsSession: true, query: 'wxExternalUserid=external-user-1&agentId=7' },
    '/contact/editDetail': { heading: '编辑客户资料', moduleLabel: '画像备注', needsSession: true, query: 'wxExternalUserid=external-user-1&agentId=7' },
    '/contact/remark': { heading: '客户备注', moduleLabel: '保存备注', needsSession: true, query: 'wxExternalUserid=external-user-1&agentId=7' },
    '/contact/settingTag': { heading: '设置客户标签', moduleLabel: '保存标签', needsSession: true, query: 'wxExternalUserid=external-user-1&agentId=7' },
    '/contactBatchAdd': { heading: '批量加好友', moduleLabel: '13800000000', needsSession: true, query: 'batchId=9&agentId=7' },
    '/contactSop': { heading: '个人客户 SOP', moduleLabel: '专项验收客户', needsSession: true, query: 'id=4&agentId=7' },
    '/login': { heading: '侧边栏登录', moduleLabel: '继续授权', query: 'agentId=7&target=%2Fcontact' },
    '/medium': { heading: '素材库', moduleLabel: '专项验收素材', needsSession: true, query: 'agentId=7' },
    '/roomSop': { heading: '客户群 SOP', moduleLabel: '专项验收客户群', needsSession: true, query: 'id=5&agentId=7' },
  },
  operation: {
    '/': { heading: '营销活动中心', moduleLabel: '营销活动中心模块待迁移' },
    '/explain': { heading: '活动说明', moduleLabel: '活动说明模块待迁移' },
    '/lottery': { heading: '抽奖活动', moduleLabel: '抽奖活动模块待迁移' },
    '/roomClockIn': { heading: '群打卡', moduleLabel: '群打卡模块待迁移' },
    '/roomFission': { heading: '群裂变活动', moduleLabel: '群裂变活动模块待迁移' },
    '/fissionSpeed': { heading: '群裂变进度', moduleLabel: '群裂变进度模块待迁移' },
    '/roomInfinitePull': { heading: '无限拉群', moduleLabel: '无限拉群模块待迁移' },
    '/shopCode': { heading: '门店活码', moduleLabel: '门店活码模块待迁移' },
    '/workFission': { heading: '任务宝活动', moduleLabel: '任务 1：邀请 3 位好友', query: 'id=17' },
    '/speed': { heading: '任务宝进度', moduleLabel: '任务宝进度模块待迁移', query: 'union_id=union-browser-1&fission_id=17' },
  },
} satisfies Record<'sidebar' | 'operation', Record<string, Phase2MobileRouteContract>>;

export function getPhase2MobileRouteContract(
  app: 'sidebar' | 'operation',
  path: string,
): Phase2MobileRouteContract | undefined {
  return (phase2MobileRouteContracts[app] as Record<string, Phase2MobileRouteContract>)[path];
}

function rawGoEnvelope(data: unknown, requestId: string): string {
  return JSON.stringify({ code: 200, msg: 'ok', data, requestId });
}

export async function installPhase2MobileFixtures(page: Page): Promise<Phase2MobileAudit> {
  const audit: Phase2MobileAudit = {
    consoleErrors: [],
    pageErrors: [],
    requestFailures: [],
    unexpectedResponses: [],
    unexpectedRequests: [],
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
      expect(route.request().headers().authorization).toBe(`Bearer ${reviewFixture.token}`);
      await route.fulfill({ status: 200, contentType: 'application/json', body: rawGoEnvelope(data, requestId) });
    });
  };
  await sidebarFixture('**/sidebar/workbench/summary', reviewFixture.workbenchSummary, 'phase2-workbench');
  await sidebarFixture('**/sidebar/workContact/index?*', reviewFixture.workContacts, 'phase2-work-contacts');
  await sidebarFixture('**/sidebar/workContact/detail?*', reviewFixture.contactDetail, 'phase2-contact-detail');
  await sidebarFixture('**/sidebar/workContact/show?*', reviewFixture.contactSummary, 'phase2-contact-summary');
  await sidebarFixture('**/sidebar/workContact/track?*', reviewFixture.tracks, 'phase2-contact-tracks');
  await sidebarFixture('**/sidebar/contactFieldPivot/index?*', reviewFixture.portraitFields, 'phase2-portrait-fields');
  await sidebarFixture('**/sidebar/workContactTagGroup/index', reviewFixture.tagGroups, 'phase2-tag-groups');
  await sidebarFixture('**/sidebar/workContactTag/allTag*', reviewFixture.tags, 'phase2-tags');
  await sidebarFixture('**/sidebar/contactBatchAdd/detail?*', reviewFixture.batch, 'phase2-batch-add');
  await sidebarFixture('**/sidebar/contactSop/getSopInfo?*', reviewFixture.contactSop, 'phase2-contact-sop');
  await sidebarFixture('**/sidebar/contactSop/getSopTipInfo?*', reviewFixture.contactSopTips, 'phase2-contact-sop-tips');
  await sidebarFixture('**/sidebar/roomSop/getSopInfo?*', reviewFixture.roomSop, 'phase2-room-sop');
  await sidebarFixture('**/sidebar/mediumGroup/index', reviewFixture.mediumGroups, 'phase2-medium-groups');
  await sidebarFixture('**/sidebar/medium/index?*', reviewFixture.mediumPage, 'phase2-medium');

  await page.route('**/operation/openUserInfo/workFission?*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: rawGoEnvelope({
        openid: 'openid-browser-1',
        unionid: 'union-browser-session-1',
        nickname: '浏览器会话参与者',
        headimgurl: '',
      }, 'phase2-operation-participant'),
    });
  });
  await page.route('**/operation/workFission/taskData?*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: rawGoEnvelope({
        invite_count: 2,
        differ_count: 1,
        end_time: 4_102_444_800,
        task: [{ count: 3, status: 0, receive_status: 0, gift_type: 1, gift_url: '/operation/reward/17' }],
      }, 'phase2-operation-task'),
    });
  });

  return audit;
}

export async function injectPhase2SidebarSession(page: Page): Promise<void> {
  await page.context().addCookies([
    { name: 'token', value: reviewFixture.token, domain: '127.0.0.1', path: '/sidebar-app' },
    { name: 'agentId', value: reviewFixture.agentId, domain: '127.0.0.1', path: '/sidebar-app' },
  ]);
}

export async function assertPhase2MobileAuditClean(
  page: Page,
  audit: Phase2MobileAudit,
  label: string,
): Promise<void> {
  await page.waitForLoadState('networkidle');
  expect(audit.consoleErrors, `${label} console errors`).toEqual([]);
  expect(audit.pageErrors, `${label} uncaught page errors`).toEqual([]);
  expect(audit.requestFailures, `${label} request failures`).toEqual([]);
  expect(audit.unexpectedResponses, `${label} unexpected 4xx/5xx responses`).toEqual([]);
  expect(audit.unexpectedRequests, `${label} unexpected API requests`).toEqual([]);
}
