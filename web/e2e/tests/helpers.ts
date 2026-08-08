import type { Page, Route } from '@playwright/test';

type BackendOptions = {
  corps?: Array<{ corpId: number; corpName: string }>;
  menuRoutes?: string[];
  unauthorizedCorpSelect?: boolean;
};

const jwt = 'header.eyJ1aWQiOjF9.signature';

function envelope(data: unknown, code = 200, msg = 'success') {
  return { code, msg, data };
}

function menu(routes: string[]) {
  const actions = ['search', 'addwx', 'check', 'edit', 'advanced', 'all', 'add', 'batch', 'close', 'sync'];
  return [{
    name: 'root',
    linkUrl: null,
    linkType: 1,
    children: [{
      name: 'section',
      linkUrl: null,
      linkType: 1,
      children: routes.map((path) => ({
        name: path,
        linkUrl: path,
        linkType: 1,
        children: actions.map((action) => ({
              name: action,
              linkUrl: `${path}@${action}`,
              linkType: 1,
              children: [],
            })),
      })),
    }],
  }];
}

export async function mockDashboardBackend(
  page: Page,
  options: BackendOptions = {},
) {
  const corps = options.corps ?? [{ corpId: 7, corpName: '测试企业' }];
  const menuRoutes = options.menuRoutes ?? ['/corp/index', '/workContact/index'];
  await page.route('**/dashboard/**', async (route) => {
    const url = new URL(route.request().url());
    if (!url.pathname.startsWith('/dashboard/')) {
      await route.continue();
      return;
    }
    const path = url.pathname.replace(/^\/dashboard/, '');
    if (path === '/user/auth') {
      await json(route, envelope({ token: jwt, expire: 3600 }));
      return;
    }
    if (path === '/corp/select' && options.unauthorizedCorpSelect) {
      await json(route, envelope(null, 401, 'unauthorized'), 401);
      return;
    }
    if (path === '/corp/select') {
      await json(route, envelope(corps));
      return;
    }
    if (path === '/corp/bind') {
      await json(route, envelope([]));
      return;
    }
    if (path === '/role/permissionByUser') {
      await json(route, envelope(menu(menuRoutes)));
      return;
    }
    if (path === '/corp/index') {
      await json(route, envelope({
        list: [{ corpId: 7, corpName: '测试企业', wxCorpId: 'wx-7', createdAt: '2026-07-28' }],
        page: { perPage: 10, total: 1, totalPage: 1 },
      }));
      return;
    }
    if (path.startsWith('/contactField/index')) {
      await json(route, envelope({
        list: [],
        page: { page: 1, perPage: 10, total: 0, totalPage: 0 },
      }));
      return;
    }
    if (path.startsWith('/workDepartment/pageIndex')) {
      await json(route, envelope({
        list: [],
        page: { page: 1, perPage: 10, total: 0, totalPage: 0 },
      }));
      return;
    }
    if (path === '/workEmployee/searchCondition') {
      await json(route, envelope({ status: [], contactAuth: [], syncTime: '' }));
      return;
    }
    if (path.startsWith('/workEmployee/index')) {
      await json(route, envelope({
        list: [],
        page: { page: 1, perPage: 10, total: 0, totalPage: 0 },
      }));
      return;
    }
    if (path.startsWith('/workContactTag/index')) {
      await json(route, envelope({
        list: [],
        syncTagTime: '',
        page: { page: 1, perPage: 10, total: 0, totalPage: 0 },
      }));
      return;
    }
    if (path === '/scrm/tags' && route.request().method() === 'GET') {
      await json(route, envelope({ groups: [], tags: [] }));
      return;
    }
    if (path.startsWith('/ai-insight/')) {
      await json(route, envelope({
        page: path.replace('/ai-insight/', ''),
        title: '',
        capability: 'limited',
        provider: 'unavailable',
        limitations: ['Provider 未接入'],
        data: [],
      }));
      return;
    }
    if (path === '/workContactTagGroup/index') {
      await json(route, envelope([]));
      return;
    }
    if (path.startsWith('/menu/index')) {
      await json(route, envelope({
        list: [],
        page: { page: 1, perPage: 10, total: 0, totalPage: 0 },
      }));
      return;
    }
    if (path === '/menu/select' || path === '/menu/iconIndex') {
      await json(route, envelope([]));
      return;
    }
    if (path.startsWith('/role/index')) {
      await json(route, envelope({
        list: [],
        page: { page: 1, perPage: 10, total: 0, totalPage: 0 },
      }));
      return;
    }
    await json(route, envelope([]));
  });
}

export async function seedSession(page: Page, corpId: string | null = '7') {
  await page.addInitScript(({ token, selectedCorp }) => {
    localStorage.setItem('mochat_dashboard_token', JSON.stringify(token));
    localStorage.setItem('mochat_dashboard_user_id', JSON.stringify('1'));
    localStorage.setItem('mochat_dashboard_corp_id', JSON.stringify(selectedCorp));
    localStorage.setItem('mochat_dashboard_expires_at', JSON.stringify(Date.now() + 3_600_000));
  }, { token: `Bearer ${jwt}`, selectedCorp: corpId });
}

async function json(route: Route, body: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(body),
  });
}
