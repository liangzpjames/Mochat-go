import type { Session } from '@mochat/auth';
import { ApiError } from '@mochat/api-client';
import { describe, expect, it, vi } from 'vitest';

import { createAccessLoader } from './access-loader';
import type { CorpOption } from '../features/corp/corp-api';
import type { MenuNode } from '../features/navigation/menu-tree';

const session: Session = {
  token: 'token',
  userId: '7',
  corpId: '3',
  expiresAt: Date.now() + 60_000,
};
const corps: CorpOption[] = [
  { id: '3', name: '迁移企业', authorized: true },
];
const menus: MenuNode[] = [{
  name: 'top',
  icon: null,
  linkUrl: null,
  linkType: 1,
  children: [{
    name: 'section',
    icon: null,
    linkUrl: null,
    linkType: 1,
    children: [{
      name: 'contacts',
      icon: null,
      linkUrl: '/workContact/index',
      linkType: 1,
      children: [],
    }],
  }],
}];

function deps(overrides: Partial<Parameters<typeof createAccessLoader>[0]> = {}) {
  return {
    clearSession: vi.fn(),
    getSession: () => session,
    loadCorps: vi.fn(() => Promise.resolve(corps)),
    loadMenu: vi.fn(() => Promise.resolve(menus)),
    knownRoutes: new Set(['/workContact/index', '/known-but-forbidden']),
    now: () => Date.now(),
    ...overrides,
  };
}

async function expectRedirect(result: Promise<unknown>, location: string) {
  const response = await result.catch((error: unknown) => error);
  expect(response).toBeInstanceOf(Response);
  expect(response).toMatchObject({ status: 302 });
  expect((response as Response).headers.get('Location')).toBe(location);
}

describe('createAccessLoader', () => {
  it('redirects a missing session to login with an encoded local return path', async () => {
    const loader = createAccessLoader(deps({ getSession: () => null }));

    await expectRedirect(
      loader({ request: new Request('https://app.test/workContact/index?q=1#tab') }),
      '/login?returnTo=%2FworkContact%2Findex%3Fq%3D1%23tab',
    );
  });

  it('clears an expired session and redirects to login', async () => {
    const clearSession = vi.fn();
    const loader = createAccessLoader(deps({
      clearSession,
      getSession: () => ({ ...session, expiresAt: Date.now() - 1 }),
    }));

    await expectRedirect(
      loader({ request: new Request('https://app.test/workContact/index') }),
      '/login',
    );
    expect(clearSession).toHaveBeenCalledOnce();
  });

  it('maps an API 401 to the same cleared login state', async () => {
    const clearSession = vi.fn();
    const loader = createAccessLoader(deps({
      clearSession,
      loadCorps: vi.fn(() => Promise.reject(
        new ApiError('unauthorized', 'expired', { status: 401 }),
      )),
    }));

    await expectRedirect(
      loader({ request: new Request('https://app.test/workContact/index') }),
      '/login',
    );
    expect(clearSession).toHaveBeenCalledOnce();
  });

  it('maps an API 403 to a forbidden route response', async () => {
    const loader = createAccessLoader(deps({
      loadCorps: vi.fn(() => Promise.reject(
        new ApiError('forbidden', 'denied', { status: 403 }),
      )),
    }));

    const error = await loader({
      request: new Request('https://app.test/workContact/index'),
    }).catch((reason: unknown) => reason);
    expect(error).toBeInstanceOf(Response);
    expect(error).toMatchObject({ status: 403 });
  });

  it('returns enterprise selection state when no enterprise is active', async () => {
    const loader = createAccessLoader(deps({
      getSession: () => ({ ...session, corpId: null }),
    }));

    await expect(loader({
      request: new Request('https://app.test/workContact/index'),
    })).resolves.toEqual({ state: 'select-corp', corps });
  });

  it('returns 404 before enterprise selection for an unknown route', async () => {
    const loader = createAccessLoader(deps({
      getSession: () => ({ ...session, corpId: null }),
    }));

    await expect(loader({
      request: new Request('https://app.test/not-registered'),
    })).rejects.toMatchObject({ status: 404 });
  });

  it('returns access context for an allowed route', async () => {
    const loader = createAccessLoader(deps());

    const result = await loader({
      request: new Request('https://app.test/workContact/index'),
    });

    expect(result).toMatchObject({ session, corp: corps[0] });
    expect(result).toMatchObject({ menu: menus });
    expect(result).not.toHaveProperty('state');
  });

  it('allows documented benchmark routes even when legacy menu permissions use different paths', async () => {
    const loader = createAccessLoader(deps({
      loadMenu: vi.fn(() => Promise.resolve([])),
      knownRoutes: new Set(['/benchmark/demo']),
      benchmarkRoutes: new Set(['/benchmark/demo']),
    }));

    await expect(loader({
      request: new Request('https://app.test/benchmark/demo'),
    })).resolves.toMatchObject({
      allowedRoutes: new Set(['/benchmark/demo']),
    });
  });

  it('returns 403 for a known route without permission and 404 for an unknown route', async () => {
    const loader = createAccessLoader(deps());

    await expect(loader({
      request: new Request('https://app.test/known-but-forbidden'),
    })).rejects.toMatchObject({ status: 403 });
    await expect(loader({
      request: new Request('https://app.test/not-registered'),
    })).rejects.toMatchObject({ status: 404 });
  });

  it('surfaces server and network failures without automatic retry', async () => {
    const loadCorps = vi.fn(() => Promise.reject(
      new ApiError('network', 'offline'),
    ));
    const loader = createAccessLoader(deps({ loadCorps }));

    await expect(loader({
      request: new Request('https://app.test/workContact/index'),
    })).rejects.toMatchObject({ kind: 'network' });
    expect(loadCorps).toHaveBeenCalledOnce();
  });
});
