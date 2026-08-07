import { describe, expect, it, vi } from 'vitest';

import { authenticate, logout } from './auth-api';

describe('authenticate', () => {
  it('posts only the audited login fields and maps the token lifetime', async () => {
    const payload = btoa(JSON.stringify({ uid: 7 }))
      .replaceAll('+', '-')
      .replaceAll('/', '_')
      .replaceAll('=', '');
    const token = `header.${payload}.signature`;
    const request = vi.fn(() => Promise.resolve({ token, expire: 3600 }));

    const result = await authenticate(
      { request },
      { phone: '13800138000', password: 'secret' },
      1_000,
    );

    expect(request).toHaveBeenCalledWith('/user/auth', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ phone: '13800138000', password: 'secret' }),
    });
    expect(result).toEqual({
      token: `Bearer ${token}`,
      userId: '7',
      userName: null,
      corpId: null,
      expiresAt: 3_601_000,
    });
  });

  it('maps the backend user name into the session', async () => {
    const payload = btoa(JSON.stringify({ uid: 7 }))
      .replaceAll('+', '-')
      .replaceAll('/', '_')
      .replaceAll('=', '');
    const request = vi.fn(() => Promise.resolve({
      token: `header.${payload}.signature`,
      expire: 3600,
      session: { userName: '超级管理员' },
    }));

    const result = await authenticate(
      { request },
      { phone: '13800138000', password: 'secret' },
      1_000,
    );

    expect(result.userName).toBe('超级管理员');
  });
});

describe('logout', () => {
  it('invalidates the authenticated server session', async () => {
    const request = vi.fn(() => Promise.resolve({}));

    await logout({ request });

    expect(request).toHaveBeenCalledWith('/user/logout', {
      method: 'PUT',
    });
  });
});
