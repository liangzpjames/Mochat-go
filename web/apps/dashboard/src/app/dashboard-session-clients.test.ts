import { afterEach, describe, expect, it, vi } from 'vitest';

import { ApiError } from '@mochat/api-client';

import { createDashboardSessionClients } from './dashboard-session-clients';
import { createAccessLoader } from './access-loader';

afterEach(() => vi.unstubAllGlobals());

describe('dashboard session client entry wiring', () => {
  it('lets the access loader own tenant cleanup and redirect without global double navigation', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(new Response(JSON.stringify({
      code: 403,
      errorCode: 'TENANT_ACCESS_DENIED',
      msg: 'tenant access denied',
      data: null,
    }), { status: 403, headers: { 'Content-Type': 'application/json' } }))));
    const onSessionInvalid = vi.fn();
    const clearSession = vi.fn();
    const clearQueries = vi.fn();
    const { accessProfileClient } = createDashboardSessionClients({
      baseUrl: 'https://app.test/dashboard/',
      getToken: () => 'token-a',
      onSessionInvalid,
    });
    const loadAccess = createAccessLoader({
      clearQueries,
      clearSession,
      getSession: () => ({ token: 'token-a', userId: '7', expiresAt: null }),
      knownRoutes: new Set(['/chat/v2-all']),
      manifestRoutes: new Set(['/chat/v2-all']),
      loadProfile: () => accessProfileClient.request('/access/profile'),
    });

    const result = await loadAccess({ request: new Request('https://app.test/chat/v2-all?q=1') }).catch((error: unknown) => error);

    expect(result).toBeInstanceOf(Response);
    expect((result as Response).headers.get('Location')).toBe('/login?returnTo=%2Fchat%2Fv2-all%3Fq%3D1');
    expect(clearSession).toHaveBeenCalledOnce();
    expect(clearQueries).toHaveBeenCalledOnce();
    expect(onSessionInvalid).not.toHaveBeenCalled();
  });

  it('keeps business tenant denial global and page permission denial local', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ code: 403, errorCode: 'TENANT_ACCESS_DENIED', msg: 'tenant denied', data: null }), { status: 403, headers: { 'Content-Type': 'application/json' } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ code: 403, errorCode: 'DASHBOARD_PERMISSION_DENIED', msg: 'page denied', data: null }), { status: 403, headers: { 'Content-Type': 'application/json' } }));
    vi.stubGlobal('fetch', fetch);
    const onSessionInvalid = vi.fn();
    const { apiClient } = createDashboardSessionClients({ baseUrl: 'https://app.test/dashboard/', getToken: () => 'token-a', onSessionInvalid });

    await expect(apiClient.request('/tenant')).rejects.toBeInstanceOf(ApiError);
    await expect(apiClient.request('/page')).rejects.toBeInstanceOf(ApiError);

    expect(onSessionInvalid).toHaveBeenCalledOnce();
  });
});
