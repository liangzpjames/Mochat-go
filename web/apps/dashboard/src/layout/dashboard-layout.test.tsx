import { QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { RouterProvider } from 'react-router';

import { createDashboardQueryClient } from '../app/providers';
import { createDashboardRouter } from '../app/router';
import type {
  AccessContext,
  CorpSelection,
} from '../app/access-loader';

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
  accessLoader?: (args: { request: Request }) => Promise<
    AccessContext | CorpSelection
  >;
}) {
  const queryClient = createDashboardQueryClient();
  const router = createDashboardRouter({
    getSession: () => options.session ? {
      token: 'Bearer test',
      userId: '7',
      corpId: '12',
      expiresAt: null,
    } : null,
    initialEntries: [options.initialPath ?? '/'],
    loadInitialData: options.loadInitialData ?? (() => Promise.resolve()),
    ...(options.accessLoader === undefined
      ? {}
      : { accessLoader: options.accessLoader }),
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

describe('Dashboard shell', () => {
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
    renderDashboard({ session: true });

    expect(await screen.findByRole('banner')).toBeTruthy();
    expect(screen.getByRole('navigation', { name: '主菜单' })).toBeTruthy();
    expect(screen.getByRole('main')).toBeTruthy();
  });

  it('renders authorized menu routes and the SaaS Admin entry', async () => {
    renderDashboard({
      session: true,
      accessLoader: () => Promise.resolve({
        session: {
          token: 'Bearer test',
          userId: '7',
          corpId: '12',
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

    expect(
      (await screen.findByRole('link', { name: '客户列表' })).getAttribute('href'),
    ).toBe('/workContact/index');
    expect(
      screen.getByRole('link', { name: 'SaaS 管理后台' }).getAttribute('href'),
    ).toBe('/saas-admin/');
    expect(screen.queryByText('菜单将在权限加载后显示')).toBeNull();
  });

  it('renders the React 404 page for an unknown route', async () => {
    renderDashboard({ session: true, initialPath: '/not-registered' });

    expect(await screen.findByRole('heading', { name: '页面不存在' })).toBeTruthy();
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
