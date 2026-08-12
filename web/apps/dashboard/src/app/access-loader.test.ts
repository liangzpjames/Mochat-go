import type { Session } from '@mochat/auth';
import { ApiError } from '@mochat/api-client';
import { describe, expect, it, vi } from 'vitest';

import { createAccessLoader } from './access-loader';
import type { AccessProfile } from '../features/access/access-api';

const session: Session = { token: 'token', userId: '7', expiresAt: Date.now() + 60_000 };
const profile = {
  userId: 7, userName: '王管理员', tenantId: 1, corpId: 3, corpName: '极义科技', workEmployeeId: 9,
  departmentIds: [], departmentEmployeeIds: [], isSuperAdmin: false,
  corpBindingStatus: 'verified',
  catalog: [{ id: 1, code: 'contacts', path: '/chat/v2-all', name: '会话', groupCode: 'conversation', sort: 1, superadminOnly: false, scopeRequired: false }],
  effectivePermissions: [{ code: 'contacts', path: '/chat/v2-all', name: '会话', scope: 'self', sources: [] }],
  allowedRoutes: ['/chat/v2-all'],
} as AccessProfile;

function deps(overrides: Partial<Parameters<typeof createAccessLoader>[0]> = {}) {
  return {
    clearSession: vi.fn(),
    getSession: () => session,
    loadProfile: vi.fn(() => Promise.resolve(profile)),
    knownRoutes: new Set(['/chat/v2-all', '/known-but-forbidden', '/contactField/index']),
    manifestRoutes: new Set(['/chat/v2-all']),
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
    await expectRedirect(createAccessLoader(deps({ getSession: () => null }))({ request: new Request('https://app.test/chat/v2-all?q=1#tab') }), '/login?returnTo=%2Fchat%2Fv2-all%3Fq%3D1%23tab');
  });

  it('clears an expired session and redirects to login', async () => {
    const clearSession = vi.fn();
    await expectRedirect(createAccessLoader(deps({ clearSession, getSession: () => ({ ...session, expiresAt: Date.now() - 1 }) }))({ request: new Request('https://app.test/chat/v2-all') }), '/login');
    expect(clearSession).toHaveBeenCalledOnce();
  });

  it('clears session for TENANT_ACCESS_DENIED but preserves it for page denial', async () => {
    const clearSession = vi.fn();
    await expectRedirect(createAccessLoader(deps({ clearSession, loadProfile: vi.fn(() => Promise.reject(new ApiError('forbidden', 'denied', { status: 403, code: 403, machineCode: 'TENANT_ACCESS_DENIED' }))) }))({ request: new Request('https://app.test/chat/v2-all') }), '/login');
    expect(clearSession).toHaveBeenCalledOnce();
    const pageError = await createAccessLoader(deps({ loadProfile: vi.fn(() => Promise.reject(new ApiError('forbidden', 'denied', { status: 403, code: 403, machineCode: 'DASHBOARD_PERMISSION_DENIED' }))) }))({ request: new Request('https://app.test/chat/v2-all') }).catch((reason: unknown) => reason);
    expect(pageError).toMatchObject({ status: 403 });
  });

  it('keeps the session and redirects CORP_CONFIGURATION_REQUIRED to the company settings page', async () => {
    const clearSession = vi.fn();
    await expectRedirect(createAccessLoader(deps({
      clearSession,
      loadProfile: vi.fn(() => Promise.reject(new ApiError('forbidden', 'configuration required', { status: 403, code: 403, machineCode: 'CORP_CONFIGURATION_REQUIRED' }))),
    }))({ request: new Request('https://app.test/chat/v2-all') }), '/company-setting/website');
    expect(clearSession).not.toHaveBeenCalled();
  });

  it('uses the server profile binding without returning an enterprise selection state', async () => {
    await expect(createAccessLoader(deps())({ request: new Request('https://app.test/chat/v2-all') })).resolves.toMatchObject({
      session,
      corp: { id: '3', name: '极义科技', authorized: true },
      profile,
    });
  });

  it('returns 404 before loading a server binding for an unknown route', async () => {
    const loadProfile = vi.fn(() => Promise.resolve(profile));
    await expect(createAccessLoader(deps({ loadProfile }))({ request: new Request('https://app.test/not-registered') })).rejects.toMatchObject({ status: 404 });
    expect(loadProfile).not.toHaveBeenCalled();
  });

  it('returns the complete AccessProfile and grouped-route set for an allowed route', async () => {
    const result = await createAccessLoader(deps())({ request: new Request('https://app.test/chat/v2-all') });
    expect(result).toMatchObject({ session, corp: { id: '3', name: '极义科技', authorized: true }, profile });
    expect((result as { allowedRoutes: ReadonlySet<string> }).allowedRoutes).toEqual(new Set(['/chat/v2-all']));
  });

  it('redirects a pending binding to settings and keeps the session', async () => {
    const pendingProfile = { ...profile, corpBindingStatus: 'pending' as const };
    const loadProfile = vi.fn(() => Promise.resolve(pendingProfile));
    const pendingDeps = deps({
      loadProfile,
      knownRoutes: new Set(['/index', '/company-setting/website', '/chat/v2-all']),
      manifestRoutes: new Set(['/index', '/company-setting/website', '/chat/v2-all']),
    });
    await expectRedirect(
      createAccessLoader(pendingDeps)({ request: new Request('https://app.test/index') }),
      '/company-setting/website',
    );
    expect(pendingDeps.clearSession).not.toHaveBeenCalled();
    await expect(createAccessLoader(pendingDeps)({
      request: new Request('https://app.test/company-setting/website'),
    })).resolves.toMatchObject({
      profile: pendingProfile,
      allowedRoutes: new Set(['/company-setting/website']),
    });
  });

  it('does not let a flat benchmark route set grant access', async () => {
    await expect(createAccessLoader(deps({ knownRoutes: new Set(['/benchmark/demo']), manifestRoutes: new Set(['/benchmark/demo']), loadProfile: vi.fn(() => Promise.resolve({ ...profile, effectivePermissions: [] })) }))({ request: new Request('https://app.test/benchmark/demo') })).rejects.toMatchObject({ status: 403 });
  });

  it('returns 403 for a known route without permission and for non-manifest legacy deep links', async () => {
    await expect(createAccessLoader(deps())({ request: new Request('https://app.test/known-but-forbidden') })).rejects.toMatchObject({ status: 403 });
    await expect(createAccessLoader(deps())({ request: new Request('https://app.test/contactField/index') })).rejects.toMatchObject({ status: 403 });
  });
});
