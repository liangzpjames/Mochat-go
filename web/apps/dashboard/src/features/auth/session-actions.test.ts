import { describe, expect, it, vi } from 'vitest';

import { createLogoutAction } from './session-actions';

function dependencies(
  invalidateServerSession: () => Promise<void>,
) {
  return {
    clearQueries: vi.fn(),
    clearSession: vi.fn(),
    invalidateServerSession,
    navigateToLogin: vi.fn(),
  };
}

describe('createLogoutAction', () => {
  it('clears local state and returns to login after server logout', async () => {
    const deps = dependencies(vi.fn(() => Promise.resolve()));

    await createLogoutAction(deps)();

    expect(deps.invalidateServerSession).toHaveBeenCalledOnce();
    expect(deps.clearSession).toHaveBeenCalledOnce();
    expect(deps.clearQueries).toHaveBeenCalledOnce();
    expect(deps.navigateToLogin).toHaveBeenCalledOnce();
  });

  it('still clears local state when server logout fails', async () => {
    const deps = dependencies(
      vi.fn(() => Promise.reject(new Error('offline'))),
    );

    await expect(createLogoutAction(deps)()).resolves.toBeUndefined();
    expect(deps.clearSession).toHaveBeenCalledOnce();
    expect(deps.clearQueries).toHaveBeenCalledOnce();
    expect(deps.navigateToLogin).toHaveBeenCalledOnce();
  });
});
