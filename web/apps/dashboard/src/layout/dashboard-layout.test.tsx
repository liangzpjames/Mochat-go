import { QueryClientProvider } from '@tanstack/react-query';
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { RouterProvider } from 'react-router';

import { createDashboardQueryClient } from '../app/providers';
import { createDashboardRouter } from '../app/router';
import type { AccessContext } from '../app/access-loader';
import { benchmarkManifest } from '../benchmark/benchmark-manifest';
import { DashboardSessionActionsProvider } from '../features/auth/session-actions';

afterEach(cleanup);

function rejectRouteResponse(status: number): Promise<never> {
  // React Router represents loader HTTP failures with Response objects.
  // eslint-disable-next-line @typescript-eslint/prefer-promise-reject-errors
  return Promise.reject(new Response(null, { status }));
}

function renderDashboard(options: {
  session: boolean;
  initialPath?: string;
  loadInitialData?: () => Promise<void>;
  accessLoader?: (args: { request: Request }) => Promise<AccessContext>;
  onLogout?: () => Promise<void>;
  reactPages?: Readonly<Record<string, ReactNode>>;
}) {
  const queryClient = createDashboardQueryClient();
  const router = createDashboardRouter({
    getSession: () => options.session ? {
      token: 'Bearer test',
      userId: '7',
      expiresAt: null,
    } : null,
    initialEntries: [options.initialPath ?? '/'],
    loadInitialData: options.loadInitialData ?? (() => Promise.resolve()),
    ...(options.accessLoader === undefined
      ? {}
      : { accessLoader: options.accessLoader }),
    ...(options.reactPages === undefined ? {} : { reactPages: options.reactPages }),
  });

  return render(
    <DashboardSessionActionsProvider
      onLogout={options.onLogout ?? (() => Promise.resolve())}
      userId={options.session ? '7' : null}
    >
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </DashboardSessionActionsProvider>,
  );
}

describe('Dashboard shell', () => {
  it('marks the sidebar and page content as independent scroll regions', async () => {
    renderDashboard({ session: true, initialPath: '/index' });

    expect((await screen.findByRole('navigation', { name: '主菜单' })).getAttribute('data-scroll-region')).toBe('navigation');
    expect(screen.getByRole('main').getAttribute('data-scroll-region')).toBe('content');
  });

  it('redirects a visitor without a session to the login route', async () => {
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    const consoleWarn = vi.spyOn(console, 'warn').mockImplementation(() => undefined);
    renderDashboard({ session: false });

    expect(await screen.findByRole('heading', { name: '登录' })).toBeTruthy();
    expect(screen.getByLabelText('手机号')).toBeTruthy();
    expect(consoleError).not.toHaveBeenCalled();
    expect(consoleWarn).not.toHaveBeenCalled();
  });

  it('renders the header, sidebar, and content for a valid session', async () => {
    const onLogout = vi.fn(() => Promise.resolve());
    renderDashboard({
      session: true,
      onLogout,
      accessLoader: () => Promise.resolve({
        session: { token: 'Bearer test', userId: '7', expiresAt: null },
        corp: { id: '12', name: '极义科技', authorized: true },
        profile: {
          userId: 7,
          userName: '王管理员',
          tenantId: 1,
          corpId: 12,
          corpName: '极义科技',
          workEmployeeId: 0,
          departmentIds: [],
          departmentEmployeeIds: [],
          isSuperAdmin: true,
          corpBindingStatus: 'verified',
          catalog: [],
          effectivePermissions: [],
          allowedRoutes: ['/index'],
        },
        allowedRoutes: new Set(['/index']),
        allowedActions: new Set(),
      }),
    });

    expect(await screen.findByRole('banner')).toBeTruthy();
    expect(screen.getByRole('navigation', { name: '主菜单' })).toBeTruthy();
    expect(screen.getByRole('main')).toBeTruthy();
    expect(screen.getByText('极义科技')).toBeTruthy();
    expect(screen.getByText('王管理员')).toBeTruthy();
    expect(screen.queryByText('账号 7')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '退出登录' }));
    await waitFor(() => expect(onLogout).toHaveBeenCalledOnce());
  });

  it('opens data overview as the default route and exposes it as a top-level menu item', async () => {
    renderDashboard({
      session: true,
      accessLoader: () => Promise.resolve({
        session: {
          token: 'Bearer test',
          userId: '7',
          expiresAt: null,
        },
        corp: { id: '12', name: '测试企业', authorized: true },
        menu: [],
        allowedRoutes: new Set(['/index']),
        allowedActions: new Set(),
      }),
      reactPages: { '/index': <h1>鏁版嵁姒傝</h1> },
    });

    expect(await screen.findByRole('heading', { name: '鏁版嵁姒傝' })).toBeTruthy();
    expect(screen.getByRole('link', { name: '数据概览' }).getAttribute('href')).toBe('/index');
  });

  it('redirects the legacy corpData browser route to data overview', async () => {
    renderDashboard({
      session: true,
      initialPath: '/corpData/index',
      reactPages: { '/index': <h1>数据概览内容</h1> },
    });

    expect(await screen.findByRole('heading', { name: '数据概览内容' })).not.toBeNull();
  });

  it('does not render an SaaS entry inside Dashboard', async () => {
    renderDashboard({
      session: true,
      accessLoader: () => Promise.resolve({
        session: {
          token: 'Bearer test',
          userId: '7',
          expiresAt: null,
        },
        corp: { id: '12', name: '测试企业', authorized: true },
        menu: [{
          name: '客户管理',
          icon: null,
          linkUrl: null,
          linkType: 1,
          children: [{
            name: '客户列表',
            icon: null,
            linkUrl: '/workContact/index',
            linkType: 1,
            children: [],
          }],
        }],
        allowedRoutes: new Set(['/workContact/index']),
        allowedActions: new Set(),
      }),
    });

    await screen.findByRole('banner');
    /* legacy navigation assertion superseded by the manifest-driven shell
    expect(
      (await screen.findByRole('link', { name: '客户列表' })).getAttribute('href'),
    ).toBe('/workContact/index'); */
    expect(
      screen.queryByRole('link', { name: 'SaaS 管理后台' }),
    ).toBeNull();
    expect(screen.queryByText('菜单将在权限加载后显示')).toBeNull();
  });

  it('renders the manifest-driven groups, current route, search, and logout controls', async () => {
    renderDashboard({
      session: true,
      initialPath: '/chat/v2-all',
      accessLoader: () => Promise.resolve({
        session: { token: 'Bearer test', userId: '7', expiresAt: null },
        corp: { id: '12', name: '测试企业', authorized: true },
        menu: [],
        allowedRoutes: new Set(benchmarkManifest.pages.map(({ path }) => path)),
        allowedActions: new Set(),
      }),
    });

    expect(await screen.findByRole('navigation', { name: '主菜单' })).toBeTruthy();
    expect(screen.getByRole('link', { name: 'MoChat AI' })).toBeTruthy();
    expect(screen.getAllByRole('button', { name: /会话|风险预警|AI 洞察|营销工具|SCRM|数据报表|AI 设置|企业设置/ })).toHaveLength(8);
    expect(screen.getByRole('button', { name: /会话/ }).getAttribute('aria-expanded')).toBe('true');
    expect(screen.getByRole('link', { name: '全局消息' }).getAttribute('href')).toBe('/chat/v2-all');
    fireEvent.click(screen.getByRole('button', { name: /会话/ }));
    expect(screen.queryByRole('link', { name: '全局消息' })).toBeNull();
    expect(screen.getByRole('searchbox', { name: '搜索功能' })).toBeTruthy();
    expect(screen.queryByRole('link', { name: '任务中心' })).toBeNull();
    expect(screen.getByRole('button', { name: '退出登录' })).toBeTruthy();
  });

  it('exposes an accessible mobile navigation drawer trigger', async () => {
    renderDashboard({ session: true, initialPath: '/index' });

    const trigger = await screen.findByRole('button', { name: '打开主菜单' });
    expect(trigger.getAttribute('aria-controls')).toBe('dashboard-sidebar');
    expect(trigger.getAttribute('aria-expanded')).toBe('false');

    fireEvent.click(trigger);
    expect(trigger.getAttribute('aria-expanded')).toBe('true');
    expect(screen.getByRole('navigation', { name: '主菜单' }).className).toContain('dashboard-sidebar-open');
  });

  it('filters authorized navigation and supports keyboard navigation from search', async () => {
    renderDashboard({
      session: true,
      initialPath: '/index',
      accessLoader: () => Promise.resolve({
        session: { token: 'Bearer test', userId: '7', expiresAt: null },
        corp: { id: '12', name: '测试企业', authorized: true },
        menu: [],
        allowedRoutes: new Set(['/index', '/customer/tags']),
        allowedActions: new Set(),
      }),
      reactPages: {
        '/index': <h1>数据概览内容</h1>,
        '/customer/tags': <h1>客户标签内容</h1>,
      },
    });

    const search = await screen.findByRole('searchbox', { name: '搜索功能' });
    fireEvent.change(search, { target: { value: '标签' } });
    expect(screen.getByRole('link', { name: '标签' }).getAttribute('href')).toBe('/customer/tags');

    fireEvent.keyDown(search, { key: 'Enter' });
    expect(await screen.findByRole('heading', { name: '客户标签内容' })).toBeTruthy();
  });

  it('renders one empty state when no manifest page is authorized', async () => {
    renderDashboard({
      session: true,
      accessLoader: () => Promise.resolve({
        session: { token: 'Bearer test', userId: '7', expiresAt: null },
        corp: { id: '12', name: '测试企业', authorized: true },
        menu: [],
        allowedRoutes: new Set(),
        allowedActions: new Set(),
      }),
    });

    expect(await screen.findByText('暂无可访问功能')).toBeTruthy();
  });

  it('renders the React 404 page for an unknown route', async () => {
    renderDashboard({ session: true, initialPath: '/not-registered' });

    expect(await screen.findByRole('heading', { name: '页面不存在' })).toBeTruthy();
  });

  it('renders the migrated password page at its existing route', async () => {
    renderDashboard({
      session: true,
      initialPath: '/passwordUpdate/index',
      reactPages: {
        '/passwordUpdate/index': <h1>修改密码</h1>,
      },
    });

    expect(await screen.findByRole('heading', { name: '修改密码' })).toBeTruthy();
  });

  it('renders the migrated employee page at its existing route', async () => {
    renderDashboard({
      session: true,
      initialPath: '/workEmployee/index',
      reactPages: {
        '/workEmployee/index': <h1>企业成员</h1>,
      },
    });

    expect(await screen.findByRole('heading', { name: '企业成员' })).toBeTruthy();
  });

  it('renders the route error boundary when initial data loading fails', async () => {
    renderDashboard({
      session: true,
      loadInitialData: () => Promise.reject(new Error('bootstrap failed')),
    });

    expect(await screen.findByRole('heading', { name: '加载失败' })).toBeTruthy();
    await waitFor(() => expect(screen.getByText('bootstrap failed')).toBeTruthy());
  });

  it('renders forbidden and not-found responses from the access loader', async () => {
    const forbidden = renderDashboard({
      session: true,
      initialPath: '/restricted',
      accessLoader: () => rejectRouteResponse(403),
    });
    expect(await screen.findByRole('heading', { name: '无权访问' })).toBeTruthy();
    forbidden.unmount();

    renderDashboard({
      session: true,
      initialPath: '/unknown',
      accessLoader: () => rejectRouteResponse(404),
    });
    expect(await screen.findByRole('heading', { name: '页面不存在' })).toBeTruthy();
  });

  it('offers a manual retry for network failures from the access loader', async () => {
    const accessLoader = vi.fn(() => Promise.reject(new Error('offline')));
    renderDashboard({
      session: true,
      initialPath: '/',
      accessLoader,
    });

    expect(await screen.findByText('offline')).toBeTruthy();
    expect(accessLoader).toHaveBeenCalledOnce();
    screen.getByRole('button', { name: '重试' }).click();
    await waitFor(() => expect(accessLoader).toHaveBeenCalledTimes(2));
  });

  it('uses the approved query retry and focus defaults', () => {
    const queryClient = createDashboardQueryClient();
    const defaults = queryClient.getDefaultOptions();

    expect(defaults.queries).toMatchObject({ retry: 1, refetchOnWindowFocus: false });
    expect(defaults.mutations).toMatchObject({ retry: 0 });
  });
});
