import { describe, expect, it, vi } from 'vitest';

import { createDashboardUnauthorizedHandler } from './unauthorized-handler';

function deps(path = '/ai-insight/session-analysis?range=7d#records') {
  const calls: string[] = [];
  let sessionKey: string | null = 'token-a';
  return {
    calls,
    clearQueries: vi.fn(() => calls.push('queries')),
    clearSession: vi.fn(() => { calls.push('session'); sessionKey = null; }),
    getCurrentPath: vi.fn(() => path),
    getSessionKey: vi.fn(() => sessionKey),
    navigateToLogin: vi.fn((target: string) => calls.push(`navigate:${target}`)),
    setSessionKey: (value: string | null) => { sessionKey = value; },
  };
}

describe('createDashboardUnauthorizedHandler', () => {
  it('clears session and queries before one encoded returnTo navigation', () => {
    const input = deps();
    const handleUnauthorized = createDashboardUnauthorizedHandler(input);

    handleUnauthorized();

    expect(input.calls).toEqual([
      'session',
      'queries',
      'navigate:/login?returnTo=%2Fai-insight%2Fsession-analysis%3Frange%3D7d%23records',
    ]);
  });

  it('handles concurrent and repeated 401 callbacks only once', async () => {
    const input = deps('/chat/v2-all');
    const handleUnauthorized = createDashboardUnauthorizedHandler(input);

    await Promise.all([
      Promise.resolve().then(handleUnauthorized),
      Promise.resolve().then(handleUnauthorized),
      Promise.resolve().then(handleUnauthorized),
    ]);
    handleUnauthorized();

    expect(input.clearSession).toHaveBeenCalledOnce();
    expect(input.clearQueries).toHaveBeenCalledOnce();
    expect(input.navigateToLogin).toHaveBeenCalledOnce();
  });

  it('allows a new session generation to exit after the prior token was handled', async () => {
    const input = deps('/chat/v2-all');
    const handleSessionInvalid = createDashboardUnauthorizedHandler(input);

    await Promise.all([Promise.resolve().then(handleSessionInvalid), Promise.resolve().then(handleSessionInvalid)]);
    input.setSessionKey('token-b');
    handleSessionInvalid();
    handleSessionInvalid();

    expect(input.clearSession).toHaveBeenCalledTimes(2);
    expect(input.clearQueries).toHaveBeenCalledTimes(2);
    expect(input.navigateToLogin).toHaveBeenCalledTimes(2);
  });

  it('uses the same generation-safe exit for a later tenant denial', () => {
    const input = deps('/company-setting/website');
    const handleTenantAccessDenied = createDashboardUnauthorizedHandler(input);

    handleTenantAccessDenied();
    input.setSessionKey('token-after-login');
    handleTenantAccessDenied();

    expect(input.clearSession).toHaveBeenCalledTimes(2);
    expect(input.navigateToLogin).toHaveBeenNthCalledWith(2, '/login?returnTo=%2Fcompany-setting%2Fwebsite');
  });

  it('does not create a returnTo loop when the current page is login', () => {
    const input = deps('/login?returnTo=%2Findex');

    createDashboardUnauthorizedHandler(input)();

    expect(input.navigateToLogin).toHaveBeenCalledWith('/login');
  });
});
