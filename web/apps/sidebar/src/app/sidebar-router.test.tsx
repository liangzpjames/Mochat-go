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
import { createSidebarRouter } from './sidebar-router';

const emptyCookies: CookieAdapter = {
  get: vi.fn(() => null),
  set: vi.fn(),
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
    request: vi.fn(),
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
    request: vi.fn(),
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
    ['/codeAuth', '企业微信扫码授权模块待迁移'],
    ['/not-a-sidebar-page', '页面不存在'],
  ])('does not render employee navigation on %s', (path, expectedText) => {
    const router = renderPath(path);
    expect(screen.getByText(expectedText)).not.toBeNull();
    expect(screen.queryByRole('navigation', { name: '员工工作台' })).toBeNull();
    router.dispose();
  });

  it('renders the workbench with only registered authenticated business links', () => {
    window.history.replaceState(null, '', '/');
    const router = createSidebarRouter(authenticatedRuntime());
    render(<RouterProvider router={router} />);

    const links = screen.getAllByRole('link').filter((link) => link.classList.contains('mobile-icon-tile'));
    const authenticatedPaths = sidebarRouteRegistry
      .filter((route) => route.auth && route.path !== '/')
      .map((route) => route.path)
      .sort();

    expect(links.map((link) => link.getAttribute('href')).sort()).toEqual(authenticatedPaths);
    expect(screen.queryByText(/成功|客户总数|今日/)).toBeNull();
    router.dispose();
  });

  it('keeps the active customer context on workbench business links', () => {
    window.history.replaceState(null, '', '/?wxExternalUserid=external-1&state=callback-state#profile');
    const router = createSidebarRouter(authenticatedRuntime());
    render(<RouterProvider router={router} />);

    expect(screen.getByRole('link', { name: /^客户资料/ }).getAttribute('href')).toBe(
      '/contact?wxExternalUserid=external-1#profile',
    );
    router.dispose();
  });

  it('keeps workbench module links inside the prefixed Sidebar mount', () => {
    window.history.replaceState(null, '', '/sidebar-app/');
    const router = createSidebarRouter({
      ...authenticatedRuntime(),
      basename: '/sidebar-app',
    });
    render(<RouterProvider router={router} />);

    expect(screen.getByRole('link', { name: /^客户资料/ }).getAttribute('href')).toBe(
      '/sidebar-app/contact',
    );
    router.dispose();
  });

  it('redirects a protected route before rendering its feature content', async () => {
    const router = renderPath('/medium?agentId=7');

    await waitFor(() => expect(router.state.location.pathname).toBe('/login'));
    expect(screen.queryByText('素材库模块待迁移')).toBeNull();
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
    expect(screen.getByText('素材库模块待迁移')).not.toBeNull();
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
    expect(screen.getByText('素材库模块待迁移')).not.toBeNull();
    router.dispose();
  });
});
