import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { RouterProvider } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import manifest from '../migration-routes.json';
import type { CookieAdapter } from '../auth/sidebar-session';
import { sidebarRouteRegistry } from '../routes/registry';
import { createSidebarRouter } from './sidebar-router';

const emptyCookies: CookieAdapter = {
  get: vi.fn(() => null),
  set: vi.fn(),
};

function renderPath(path: string) {
  window.history.replaceState(null, '', path);
  const router = createSidebarRouter({
    basename: '/',
    cookies: emptyCookies,
    origin: window.location.origin,
    request: vi.fn(),
    secure: false,
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

  it('redirects a protected route before rendering its feature content', async () => {
    const router = renderPath('/medium?agentId=7');

    await waitFor(() => expect(router.state.location.pathname).toBe('/login'));
    expect(screen.queryByText('素材库模块待迁移')).toBeNull();
    expect(screen.getByRole('link', { name: '继续授权' }).getAttribute('href')).toContain(
      '/sidebar/agent/auth?agentId=7&target=',
    );
    router.dispose();
  });
});
