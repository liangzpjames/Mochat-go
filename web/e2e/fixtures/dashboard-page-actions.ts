import { expect, type Page } from '@playwright/test';

export type DashboardPageAction = {
  route: string;
  action: string;
  run: (page: Page) => Promise<void>;
};

type ActionSpec =
  | { kind: 'button'; name: string }
  | { kind: 'link'; href: string };

const specs: Readonly<Record<string, ActionSpec>> = {
  '/index': { kind: 'button', name: '刷新' },
  '/chat/v2-all': { kind: 'button', name: '刷新消息' },
  '/chat/v2-staff': { kind: 'button', name: '搜索员工' },
  '/chat/v2-customer': { kind: 'button', name: '刷新消息' },
  '/chat/v2-group': { kind: 'button', name: '刷新消息' },
  '/chat/trajectory': { kind: 'button', name: '刷新' },
  '/chat/export': { kind: 'button', name: '刷新' },
  '/chat/file-audio': { kind: 'button', name: '查询' },
  '/chat/resign-staff': { kind: 'button', name: '查询' },
  '/chat/refuse-archive': { kind: 'button', name: '查询' },
  '/customer/inheritance': { kind: 'button', name: '查询' },
  '/ai-insight/v2/risk': { kind: 'button', name: '刷新' },
  '/ai-insight/v2/sensitive-word': { kind: 'button', name: '刷新记录' },
  '/ai-insight/v2/timeout': { kind: 'button', name: '刷新' },
  '/ai-insight/v2/customer-loss': { kind: 'button', name: '刷新' },
  '/ai-insight/v2/message-intercept': { kind: 'button', name: '刷新' },
  '/ai-insight/v2/keyword-library': { kind: 'button', name: '刷新' },
  '/ai-insight/v2/silent-customer': { kind: 'button', name: '刷新' },
  '/ai-insight/session-analysis': { kind: 'link', href: '/index' },
  '/ai-insight/smart-analysis': { kind: 'link', href: '/index' },
  '/ai-insight/emotion': { kind: 'link', href: '/index' },
  '/ai-insight/employee-score': { kind: 'link', href: '/index' },
  '/ai-insight/communication-keyword': { kind: 'link', href: '/index' },
  '/acquisition/v2-channel-code': { kind: 'button', name: '刷新' },
  '/acquisition/group-code': { kind: 'button', name: '刷新' },
  '/acquisition/redirect-link': { kind: 'button', name: '刷新' },
  '/acquisition/wechat-customer-service': { kind: 'button', name: '刷新' },
  '/acquisition/live-code-short-chain': { kind: 'button', name: '刷新' },
  '/acquisition/group-template': { kind: 'button', name: '刷新' },
  '/acquisition/precise-group-send': { kind: 'button', name: '刷新' },
  '/acquisition/friends-circle': { kind: 'button', name: '刷新' },
  '/acquisition/material-management': { kind: 'button', name: '刷新' },
  '/customer/clue/default': { kind: 'button', name: '刷新线索' },
  '/customer/contact': { kind: 'button', name: '刷新联系人' },
  '/customer/friends': { kind: 'button', name: '查询' },
  '/customer/opportunity': { kind: 'button', name: '刷新商机' },
  '/customer/public-sea': { kind: 'button', name: '刷新公海' },
  '/customer/group': { kind: 'button', name: '查询' },
  '/customer/tags': { kind: 'button', name: '查询' },
  '/customer/order': { kind: 'button', name: '快速创建联系人' },
  '/customer/settings': { kind: 'link', href: '/index' },
  '/data/customer': { kind: 'button', name: '查询' },
  '/data/employee': { kind: 'button', name: '查询' },
  '/data/conversion': { kind: 'button', name: '查询' },
  '/data/behavior': { kind: 'button', name: '查询' },
  '/data/report': { kind: 'button', name: '查询' },
  '/ai-setting/ai-knowledge-base': { kind: 'button', name: '新建知识库' },
  '/ai-setting/agent': { kind: 'button', name: '新建智能体' },
  '/company-setting/website': { kind: 'button', name: '刷新同步状态' },
  '/company-setting/staff': { kind: 'button', name: '查询' },
  '/setting/role': { kind: 'button', name: '查询' },
  '/setting/additional': { kind: 'button', name: '查询' },
  '/setting/authorization': { kind: 'link', href: '/index' },
};

async function runSpec(page: Page, route: string, spec: ActionSpec): Promise<void> {
  if (spec.kind === 'button') {
    const button = page.getByRole('button', { name: spec.name, exact: true }).first();
    await expect(button).toBeVisible();
    await expect(button).toBeEnabled();
    await button.click();
    return;
  }
  const link = page.locator(`a[href="${spec.href}"]`).first();
  await expect(link).toBeVisible();
  await link.click();
  await expect(page).toHaveURL(new RegExp(`${spec.href.replaceAll('/', '\\/')}(?:[?#]|$)`));
  await page.goBack({ waitUntil: 'domcontentloaded' });
  await expect(page).toHaveURL(new RegExp(`${route.replaceAll('/', '\\/')}(?:[?#]|$)`));
}

export const dashboardPageActions: readonly DashboardPageAction[] = Object.entries(specs).map(([route, spec]) => ({
  route,
  action: spec.kind === 'button' ? `button:${spec.name}` : `link:${spec.href}`,
  run: (page) => runSpec(page, route, spec),
}));
