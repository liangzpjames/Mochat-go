import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { MemoryRouter, RouterProvider } from 'react-router';

import { benchmarkManifest } from './benchmark-manifest';
import { createPageRegistry } from './page-registry';
import { createDashboardRouter } from '../app/router';
import { DashboardSessionActionsProvider } from '../features/auth/session-actions';

const manifest = {
  groups: [{ id: 'conversation', title: '会话' }],
  pages: [
    { path: '/index', title: '数据概览', groupId: null, level: 'P0' },
    { path: '/chat/staff', title: '员工会话', groupId: 'conversation', level: 'P1' },
    { path: '/chat/export', title: '会话导出', groupId: 'conversation', level: 'P2' },
  ],
} as const;

afterEach(cleanup);

function renderDashboardRoute(path: string) {
  const router = createDashboardRouter({
    getSession: () => ({
      token: 'Bearer test',
      userId: '7',
      corpId: '12',
      expiresAt: null,
    }),
    initialEntries: [path],
    loadInitialData: () => Promise.resolve(),
    reactPages: createPageRegistry({
      manifest: benchmarkManifest,
      p0Pages: {},
      p1Pages: {},
    }),
  });

  return render(
    <DashboardSessionActionsProvider onLogout={() => Promise.resolve()} userId="7">
      <RouterProvider router={router} />
    </DashboardSessionActionsProvider>,
  );
}

describe('createPageRegistry', () => {
  it('prefers P0 implementations over P1 implementations at the same path', () => {
    const pages = createPageRegistry({
      manifest,
      p0Pages: { '/index': <p>P0 overview</p> },
      p1Pages: { '/index': <p>P1 overview</p>, '/chat/staff': <p>Staff chat</p> },
    });

    render(<>{pages['/index']}</>);
    expect(screen.getByText('P0 overview')).toBeTruthy();
    expect(screen.queryByText('P1 overview')).toBeNull();
  });

  it('uses a construction placeholder for an unimplemented P2 page', () => {
    const pages = createPageRegistry({ manifest, p0Pages: {}, p1Pages: {} });

    render(<MemoryRouter>{pages['/chat/export']}</MemoryRouter>);
    expect(screen.getByRole('heading', { name: '会话导出' })).toBeTruthy();
    expect(screen.getByText('功能建设中')).toBeTruthy();
  });

  it('returns an element for every manifest path', () => {
    const pages = createPageRegistry({ manifest, p0Pages: {}, p1Pages: {} });

    expect(Object.keys(pages).sort()).toEqual(
      manifest.pages.map((page) => page.path).sort(),
    );
  });

  it('renders a manifest P2 route instead of the 404 page', async () => {
    renderDashboardRoute('/chat/trajectory');

    expect(await screen.findByRole('heading', { name: '会话轨迹' })).toBeTruthy();
    expect(screen.queryByRole('heading', { name: '页面不存在' })).toBeNull();
  });

  it('keeps unknown routes on the NotFound page', async () => {
    renderDashboardRoute('/not-registered');

    expect(await screen.findByRole('heading', { name: '页面不存在' })).toBeTruthy();
  });
});
