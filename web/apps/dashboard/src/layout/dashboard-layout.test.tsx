import { QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { RouterProvider } from 'react-router';

import { createDashboardQueryClient } from '../app/providers';
import { createDashboardRouter } from '../app/router';

afterEach(cleanup);

function renderDashboard(options: {
  session: boolean;
  initialPath?: string;
  loadInitialData?: () => Promise<void>;
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
    expect(consoleError).not.toHaveBeenCalled();
    expect(consoleWarn).not.toHaveBeenCalled();
  });

  it('renders the header, sidebar, and content for a valid session', async () => {
    renderDashboard({ session: true });

    expect(await screen.findByRole('banner')).toBeTruthy();
    expect(screen.getByRole('navigation', { name: '主菜单' })).toBeTruthy();
    expect(screen.getByRole('main')).toBeTruthy();
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

  it('uses the approved query retry and focus defaults', () => {
    const queryClient = createDashboardQueryClient();
    const defaults = queryClient.getDefaultOptions();

    expect(defaults.queries).toMatchObject({ retry: 1, refetchOnWindowFocus: false });
    expect(defaults.mutations).toMatchObject({ retry: 0 });
  });
});
