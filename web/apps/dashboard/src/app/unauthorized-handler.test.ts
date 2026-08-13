import { describe, expect, it, vi } from 'vitest';

import { createDashboardUnauthorizedHandler } from './unauthorized-handler';

function deps(path = '/ai-insight/session-analysis?range=7d#records') {
  const calls: string[] = [];
  return {
    calls,
    clearQueries: vi.fn(() => calls.push('queries')),
    clearSession: vi.fn(() => calls.push('session')),
    getCurrentPath: vi.fn(() => path),
    navigateToLogin: vi.fn((target: string) => calls.push(`navigate:${target}`)),
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

  it('does not create a returnTo loop when the current page is login', () => {
    const input = deps('/login?returnTo=%2Findex');

    createDashboardUnauthorizedHandler(input)();

    expect(input.navigateToLogin).toHaveBeenCalledWith('/login');
  });
});
