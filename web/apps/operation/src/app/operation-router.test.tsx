import { cleanup, render, screen } from '@testing-library/react';
import { RouterProvider } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import manifest from '../migration-routes.json';
import { operationRouteRegistry } from '../routes/registry';
import { createOperationRouter } from './operation-router';

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

function renderPath(path: string) {
  window.history.replaceState(null, '', path);
  const router = createOperationRouter({
    basename: '/',
    request: vi.fn(),
  });
  render(<RouterProvider router={router} />);
  return router;
}

describe('Operation exact route registry', () => {
  it('maps each manifest URL exactly once to a unique named module', () => {
    expect(operationRouteRegistry.map((route) => route.path).sort()).toEqual(
      manifest.map((route) => route.path).sort(),
    );
    expect(operationRouteRegistry).toHaveLength(10);
    expect(new Set(operationRouteRegistry.map((route) => route.path)).size).toBe(10);
    expect(new Set(operationRouteRegistry.map((route) => route.moduleKey)).size).toBe(10);

    for (const manifestRoute of manifest) {
      const registered = operationRouteRegistry.find((route) => route.path === manifestRoute.path);
      expect(registered?.auth).toBe(manifestRoute.auth);
      expect(registered?.title).toMatch(/[\u4e00-\u9fff]/);
    }

    expect(operationRouteRegistry.find((route) => route.path === '/workFission')).toMatchObject({
      activityKind: 'workFission',
      requiredParams: ['id'],
      requiresActivitySession: true,
    });
    expect(operationRouteRegistry.find((route) => route.path === '/speed')).toMatchObject({
      activityKind: null,
      requiredParams: [],
      requiresActivitySession: false,
    });
  });

  it('consumes activity-session route metadata before rendering work-fission', async () => {
    const request = vi.fn().mockResolvedValue([]);
    window.history.replaceState(null, '', '/workFission?id=17');
    const router = createOperationRouter({ basename: '/', request });

    render(<RouterProvider router={router} />);

    expect(await screen.findByRole('link', { name: '重新授权' })).not.toBeNull();
    expect(request).toHaveBeenCalledWith(
      '/openUserInfo/workFission?id=17',
      { method: 'GET' },
    );
    router.dispose();
  });

  it('renders the named module boundary instead of a generic executable activity page', () => {
    const router = renderPath('/lottery?activityId=7#draw');

    expect(screen.getByRole('heading', { name: '抽奖活动' })).not.toBeNull();
    expect(screen.getByText('抽奖活动模块待迁移')).not.toBeNull();
    expect(screen.queryByRole('button', { name: '立即抽奖' })).toBeNull();
    router.dispose();
  });

  it('renders a 404 state for an unknown path without falling back to home', () => {
    const router = renderPath('/not-an-operation-page');

    expect(screen.getByRole('heading', { name: '页面不存在' })).not.toBeNull();
    expect(screen.queryByRole('heading', { name: '营销活动中心' })).toBeNull();
    router.dispose();
  });
});
