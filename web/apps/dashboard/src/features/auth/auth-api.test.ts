import { describe, expect, it, vi } from 'vitest';

import { activate, authenticate, completeMFA, logout } from './auth-api';

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

    expect('token' in result && result.userName).toBe('超级管理员');
  });

  it('maps the enrollment response without creating a session', async () => {
    const request = vi.fn(() => Promise.resolve({
      enrollmentToken: 'enrollment-token',
      enrollmentSecret: 'one-time-secret',
      otpAuthURL: 'otpauth://dashboard/test',
      expiresAt: 101,
    }));

    await expect(authenticate({ request }, { phone: '13800138000', password: 'secret' }, 1_000))
      .resolves.toEqual({
        kind: 'mfa-enrollment',
        enrollmentToken: 'enrollment-token',
        enrollmentSecret: 'one-time-secret',
        otpAuthURL: 'otpauth://dashboard/test',
        expiresAt: 101_000,
      });
  });

  it('posts MFA completion and maps a password-change challenge', async () => {
    const request = vi.fn(() => Promise.resolve({
      passwordChangeToken: 'password-change-token',
      expiresAt: 102,
    }));

    await expect(completeMFA(
      { request },
      { challengeToken: 'enrollment-token', code: '123456' },
      1_000,
    )).resolves.toEqual({
      kind: 'password-change',
      passwordChangeToken: 'password-change-token',
      expiresAt: 102_000,
    });
    expect(request).toHaveBeenCalledWith('/user/authMFA', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ challengeToken: 'enrollment-token', code: '123456' }),
    });
  });

  it('posts activation credentials and does not echo the one-time token into another request', async () => {
    const request = vi.fn(() => Promise.resolve(undefined));

    await activate({ request }, {
      activationToken: 'one-time-activation-token',
      password: 'new-dashboard-password',
    });

    expect(request).toHaveBeenCalledWith('/auth/activate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        activationToken: 'one-time-activation-token',
        password: 'new-dashboard-password',
      }),
    });
    expect(request).toHaveBeenCalledTimes(1);
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
