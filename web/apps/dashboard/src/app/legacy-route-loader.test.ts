import type { MigrationRoute } from '@mochat/routing';
import { describe, expect, it, vi } from 'vitest';

import { createLegacyRouteLoader } from './legacy-route-loader';

const routes = [
  {
    path: '/workContact/index',
    target: 'legacy',
    auth: true,
    corpContext: true,
    permission: null,
  },
  {
    path: '/corp/index',
    target: 'react',
    auth: true,
    corpContext: true,
    permission: null,
  },
] as const satisfies readonly MigrationRoute[];

async function caught(promise: Promise<unknown>) {
  return promise.catch((error: unknown) => error);
}

describe('createLegacyRouteLoader', () => {
  it('replaces the page with the mounted legacy URL including query and hash', async () => {
    const replace = vi.fn();
    const loader = createLegacyRouteLoader({
      allowedRoutes: new Set(['/workContact/index']),
      manifest: routes,
      replace,
    });

    await loader({
      request: new Request('https://app.test/workContact/index?tab=1#top'),
    });

    expect(replace).toHaveBeenCalledWith(
      '/_legacy/dashboard/workContact/index?tab=1#top',
    );
  });

  it('rejects React and unknown routes without navigating', async () => {
    const replace = vi.fn();
    const loader = createLegacyRouteLoader({
      allowedRoutes: new Set(['/corp/index', '/unknown']),
      manifest: routes,
      replace,
    });

    expect(await caught(loader({
      request: new Request('https://app.test/corp/index'),
    }))).toMatchObject({ status: 404 });
    expect(await caught(loader({
      request: new Request('https://app.test/unknown'),
    }))).toMatchObject({ status: 404 });
    expect(replace).not.toHaveBeenCalled();
  });

  it('rejects a legacy route without permission', async () => {
    const replace = vi.fn();
    const loader = createLegacyRouteLoader({
      allowedRoutes: new Set(),
      manifest: routes,
      replace,
    });

    expect(await caught(loader({
      request: new Request('https://app.test/workContact/index'),
    }))).toMatchObject({ status: 403 });
    expect(replace).not.toHaveBeenCalled();
  });
});
