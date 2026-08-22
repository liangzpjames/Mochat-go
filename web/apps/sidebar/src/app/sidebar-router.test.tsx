import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { StrictMode } from 'react';
import { renderToString } from 'react-dom/server';
import { RouterProvider } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import manifest from '../migration-routes.json';
import {
  createCookieSidebarSessionAdapter,
  createSessionStorageSidebarSessionAdapter,
  type CookieAdapter,
  type SidebarSessionAdapter,
} from '../auth/sidebar-session';
import { sidebarRouteRegistry } from '../routes/registry';
import type { WeComBridge } from '../wecom/wecom-bridge';
import { createSidebarRouter, type SidebarRequest } from './sidebar-router';

const emptyCookies: CookieAdapter = {
  get: vi.fn(() => null),
  set: vi.fn(),
};

const bridge: WeComBridge = {
  available: () => false,
  sendChatMessage: vi.fn(),
  navigateToAddCustomer: vi.fn(),
};

const businessRequest: SidebarRequest = <T,>(path: string): Promise<T> => {
  if (path === '/workbench/summary') return Promise.resolve({
    employee: { id: 7, name: '测试员工', avatar: null, departmentNames: ['客户成功部'], corpName: '测试企业' },
    customers: { total: 12, addedToday: 2, taggedTotal: 8, ownedRoomTotal: 3 },
    tasks: { contactSopPending: 4, roomSopPending: 5, batchAddPending: 6 },
  } as T);
  if (path === '/mediumGroup/index') return Promise.resolve([] as T);
  if (path.startsWith('/medium/index?')) {
    return Promise.resolve({ page: { perPage: 20, total: 0, totalPage: 0 }, list: [] } as T);
  }
  return Promise.reject(new Error(`Unexpected request: ${path}`));
};

function successfulAuthPath(target = '/login?agentId=7'): string {
  const state = btoa(JSON.stringify({
    code: 200,
    msg: '',
    data: { token: 'sidebar-token', expire: 7200 },
  }));
  return `/auth?${new URLSearchParams({ agentId: '7', state, target }).toString()}`;
}

function statefulCookieAdapter() {
  const values = new Map<string, string>();
  const writes = vi.fn((serialized: string) => {
    const pair = serialized.split(';', 1)[0] ?? '';
    const separator = pair.indexOf('=');
    if (separator <= 0) throw new Error('Invalid serialized cookie fixture');
    values.set(pair.slice(0, separator), decodeURIComponent(pair.slice(separator + 1)));
  });
  return {
    cookies: {
      get: (name: string) => values.get(name) ?? null,
      set: writes,
    },
    writes,
  };
}

function authRuntime(session: SidebarSessionAdapter) {
  return {
    basename: '/',
    session,
    origin: window.location.origin,
    request: businessRequest,
    bridge,
  };
}

function authenticatedRuntime() {
  const cookies: CookieAdapter = {
    get: (name: string) => ({ token: 'sidebar-token', agentId: '7' })[name] ?? null,
    set: vi.fn(),
  };
  return authRuntime(createCookieSidebarSessionAdapter(cookies, false));
}

function renderPath(path: string) {
  window.history.replaceState(null, '', path);
    const router = createSidebarRouter({
      basename: '/',
      session: createCookieSidebarSessionAdapter(emptyCookies, false),
    origin: window.location.origin,
    request: businessRequest,
    bridge,
  });
  render(<RouterProvider router={router} />);
  return router;
}

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('Sidebar route registry', () => {
  it('maps every manifest path once with a unique module key and matching auth', () => {
    expect(sidebarRouteRegistry.map((route) => route.path).sort()).toEqual(
      manifest.map((route) => route.path).sort(),
    );
    expect(new Set(sidebarRouteRegistry.map((route) => route.moduleKey)).size).toBe(12);

    for (const manifestRoute of manifest) {
      expect(sidebarRouteRegistry.find((route) => route.path === manifestRoute.path)?.auth)
        .toBe(manifestRoute.auth);
    }
  });

  it('renders a not-found state for an unknown route', () => {
    const router = renderPath('/not-a-sidebar-page');
    expect(screen.getByText('页面不存在')).not.toBeNull();
    router.dispose();
  });

  it.each([
    ['/login?agentId=7', '侧边栏登录'],
    ['/auth', '登录失败'],
    ['/codeAuth', '兼容授权参数无效'],
    ['/not-a-sidebar-page', '页面不存在'],
  ])('does not render employee navigation on %s', (path, expectedText) => {
    const router = renderPath(path);
    expect(screen.getByText(expectedText)).not.toBeNull();
    expect(screen.queryByRole('navigation', { name: '员工工作台' })).toBeNull();
    router.dispose();
  });

  it('renders the real employee workspace and honestly disables only context-bound details', async () => {
    window.history.replaceState(null, '', '/');
    const router = createSidebarRouter(authenticatedRuntime());
    render(<RouterProvider router={router} />);

    expect(await screen.findByText('客户总数')).not.toBeNull();
    expect(screen.getByText('12')).not.toBeNull();
    expect(screen.getByText('当前客户').closest('[aria-disabled="true"]')).not.toBeNull();
    expect(screen.queryByRole('link', { name: /当前客户/ })).toBeNull();
    expect(screen.getByRole('link', { name: /通讯录/ }).getAttribute('href')).toContain('view=contacts');
    expect(screen.getAllByRole('link', { name: /客户|会话|我的/ }).length).toBeGreaterThanOrEqual(3);
    router.dispose();
  });

  it('keeps the active customer context on workbench business links', async () => {
    window.history.replaceState(null, '', '/?wxExternalUserid=external-1&state=callback-state#profile');
    const router = createSidebarRouter(authenticatedRuntime());
    render(<RouterProvider router={router} />);

    expect((await screen.findByRole('link', { name: /^当前客户/ })).getAttribute('href')).toBe(
      '/contact?wxExternalUserid=external-1',
    );
    router.dispose();
  });

  it('builds real task-list links from a restored profile context', async () => {
    window.history.replaceState(null, '', '/?agentId=7&wxExternalUserid=external-1&contactId=23&contactSopId=4&roomSopId=5&batchId=9');
    const router = createSidebarRouter(authenticatedRuntime());
    render(<RouterProvider router={router} />);

    expect((await screen.findByRole('link', { name: /^当前客户/ })).getAttribute('href')).toContain('wxExternalUserid=external-1');
    expect(screen.getByRole('link', { name: /^个人客户 SOP/ }).getAttribute('href')).toContain('view=contactSop');
    expect(screen.getByRole('link', { name: /^客户群 SOP/ }).getAttribute('href')).toContain('view=roomSop');
    expect(screen.getAllByRole('link', { name: /^批量加好友/ })[0]?.getAttribute('href')).toContain('view=batchAdd');
    router.dispose();
  });

  it.each([
    ['/', '客户'],
    ['/contact', '客户资料'],
    ['/contact/editDetail', '编辑客户资料'],
    ['/contact/remark', '客户备注'],
    ['/contact/settingTag', '设置客户标签'],
    ['/contactBatchAdd', '批量加好友'],
    ['/contactSop', '个人客户 SOP'],
    ['/medium', '素材库'],
    ['/roomSop', '客户群 SOP'],
  ])('renders the real protected route shell for %s', (path, title) => {
    window.history.replaceState(null, '', path);
    const router = createSidebarRouter(authenticatedRuntime());
    render(<RouterProvider router={router} />);
    expect(screen.getByRole('heading', { level: 1, name: title })).not.toBeNull();
    expect(screen.queryByText(/模块待迁移/)).toBeNull();
    router.dispose();
  });

  it('keeps workbench module links inside the prefixed Sidebar mount', async () => {
    window.history.replaceState(null, '', '/sidebar-app/?wxExternalUserid=external-1');
    const router = createSidebarRouter({
      ...authenticatedRuntime(),
      basename: '/sidebar-app',
    });
    render(<RouterProvider router={router} />);

    expect((await screen.findByRole('link', { name: /^当前客户/ })).getAttribute('href')).toBe(
      '/sidebar-app/contact?wxExternalUserid=external-1',
    );
    router.dispose();
  });

  it('redirects a protected route before rendering its feature content', async () => {
    const router = renderPath('/medium?agentId=7');

    await waitFor(() => expect(router.state.location.pathname).toBe('/login'));
    expect(screen.queryByText('素材库')).toBeNull();
    expect(screen.getByRole('link', { name: '继续授权' }).getAttribute('href')).toContain(
      '/sidebar/agent/auth?agentId=7&target=',
    );
    router.dispose();
  });

  it('renders callback loading without writing cookies during render', () => {
    const set = vi.fn();
    const cookies: CookieAdapter = { get: vi.fn(() => null), set };
    window.history.replaceState(null, '', successfulAuthPath());
    const router = createSidebarRouter(authRuntime(createCookieSidebarSessionAdapter(cookies, false)));

    const markup = renderToString(<RouterProvider router={router} />);

    expect(markup).toContain('正在处理授权');
    expect(set).not.toHaveBeenCalled();
    router.dispose();
  });

  it('writes a successful callback session once under repeated StrictMode effects', async () => {
    const set = vi.fn();
    const cookies: CookieAdapter = { get: vi.fn(() => null), set };
    window.history.replaceState(null, '', successfulAuthPath());
    const router = createSidebarRouter(authRuntime(createCookieSidebarSessionAdapter(cookies, false)));

    render(
      <StrictMode>
        <RouterProvider router={router} />
      </StrictMode>,
    );

    await waitFor(() => expect(router.state.location.pathname).toBe('/login'));
    expect(set).toHaveBeenCalledTimes(2);
    router.dispose();
  });

  it('lets a prefixed callback persist path-scoped cookies into a protected page', async () => {
    const { cookies, writes } = statefulCookieAdapter();
    const session = createCookieSidebarSessionAdapter(cookies, false);
    window.history.replaceState(
      null,
      '',
      `/sidebar-app${successfulAuthPath(`${window.location.origin}/sidebar-app/medium`)}`,
    );
    const router = createSidebarRouter({
      ...authRuntime(session),
      basename: '/sidebar-app',
    });

    render(<RouterProvider router={router} />);

    await waitFor(() => expect(router.state.location.pathname).toBe('/sidebar-app/medium'));
    expect(await screen.findByText('暂无可用素材')).not.toBeNull();
    expect(writes).toHaveBeenCalledTimes(2);
    expect(writes.mock.calls.every(([value]) => value.includes('Path=/sidebar-app'))).toBe(true);
    router.dispose();
  });

  it('lets a root-mount callback persist into a protected page in the same tab', async () => {
    const session = createSessionStorageSidebarSessionAdapter(window.sessionStorage);
    window.history.replaceState(null, '', successfulAuthPath('/medium'));
    const router = createSidebarRouter(authRuntime(session));

    render(<RouterProvider router={router} />);

    await waitFor(() => expect(router.state.location.pathname).toBe('/medium'));
    expect(await screen.findByText('暂无可用素材')).not.toBeNull();
    router.dispose();
  });
});
