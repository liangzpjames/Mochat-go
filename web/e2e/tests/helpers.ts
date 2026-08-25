import type { Page, Route } from '@playwright/test';

type BackendOptions = {
  menuRoutes?: string[];
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
      children: routes.map((routePath) => ({
        name: routePath,
        linkUrl: routePath,
        linkType: 1,
        children: actions.map((action) => ({
          name: action,
          linkUrl: `${routePath}@${action}`,
          linkType: 1,
          children: [],
        })),
      })),
    }],
  }];
}

function accessProfile(menuRoutes: string[]) {
  const catalog = menuRoutes.map((routePath, index) => ({
    id: index + 1,
    code: `page-${index + 1}`,
    path: routePath,
    name: routePath,
    groupCode: 'dashboard',
    sort: index,
    superadminOnly: false,
    scopeRequired: false,
  }));
  return {
    userId: 1,
    userName: 'e2e',
    tenantId: 1,
    corpId: 7,
    corpName: '测试企业',
    workEmployeeId: 1,
    departmentIds: [],
    departmentEmployeeIds: [],
    isSuperAdmin: false,
    catalog,
    effectivePermissions: catalog.map((item) => ({
      code: item.code,
      path: item.path,
      name: item.name,
      scope: 'tenant',
      sources: [],
    })),
    allowedRoutes: menuRoutes,
  };
}

export async function mockDashboardBackend(
  page: Page,
  options: BackendOptions = {},
) {
  const menuRoutes = options.menuRoutes ?? ['/index', '/workContact/index'];
  await page.route('**/dashboard/**', async (route) => {
    const url = new URL(route.request().url());
    if (!url.pathname.startsWith('/dashboard/')) {
      await route.continue();
      return;
    }
    const routePath = url.pathname.replace(/^\/dashboard/, '');
    if (routePath === '/user/auth') {
      await json(route, envelope({ token: jwt, expire: 3600 }));
      return;
    }
    if (routePath === '/access/profile') {
      await json(route, envelope(accessProfile(menuRoutes)));
      return;
    }
    if (routePath === '/role/permissionByUser') {
      await json(route, envelope(menu(menuRoutes)));
      return;
    }
    if (routePath.startsWith('/contactField/index')) {
      await json(route, envelope({ list: [], page: { page: 1, perPage: 10, total: 0, totalPage: 0 } }));
      return;
    }
    if (routePath.startsWith('/workDepartment/pageIndex')) {
      await json(route, envelope({ list: [], page: { page: 1, perPage: 10, total: 0, totalPage: 0 } }));
      return;
    }
    if (routePath === '/workEmployee/searchCondition') {
      await json(route, envelope({ status: [], contactAuth: [], syncTime: '' }));
      return;
    }
    if (routePath.startsWith('/workEmployee/index')) {
      await json(route, envelope({ list: [], page: { page: 1, perPage: 10, total: 0, totalPage: 0 } }));
      return;
    }
    if (routePath.startsWith('/workContactTag/index')) {
      await json(route, envelope({ list: [], syncTagTime: '', page: { page: 1, perPage: 10, total: 0, totalPage: 0 } }));
      return;
    }
    if (routePath === '/scrm/tags' && route.request().method() === 'GET') {
      await json(route, envelope({ groups: [], tags: [] }));
      return;
    }
    if (routePath.startsWith('/ai-insight/')) {
      await json(route, envelope({
        page: routePath.replace('/ai-insight/', ''),
        title: '',
        capability: 'limited',
        provider: 'unavailable',
        limitations: ['Provider 未接入'],
        data: [],
      }));
      return;
    }
    if (route.request().method() === 'GET' && (
      routePath === '/access/employees'
      || routePath === '/access/users'
      || routePath === '/access/roles'
      || routePath === '/access/audits'
    )) {
      await json(route, envelope({ list: [], page: { page: 1, perPage: 50, total: 0, totalPage: 0 } }));
      return;
    }
    if (routePath === '/access/catalog') {
      await json(route, envelope([]));
      return;
    }
    if (routePath === '/workContactTagGroup/index' || routePath === '/menu/select' || routePath === '/menu/iconIndex') {
      await json(route, envelope([]));
      return;
    }
    if (routePath.startsWith('/menu/index') || routePath.startsWith('/role/index')) {
      await json(route, envelope({ list: [], page: { page: 1, perPage: 10, total: 0, totalPage: 0 } }));
      return;
    }
    await json(route, envelope([]));
  });
}

export async function seedSession(page: Page) {
  await page.addInitScript(({ token }) => {
    localStorage.setItem('mochat_dashboard_token', JSON.stringify(token));
    localStorage.setItem('mochat_dashboard_user_id', JSON.stringify('1'));
    localStorage.setItem('mochat_dashboard_expires_at', JSON.stringify(Date.now() + 3_600_000));
  }, { token: `Bearer ${jwt}` });
}

async function json(route: Route, body: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(body),
  });
}
