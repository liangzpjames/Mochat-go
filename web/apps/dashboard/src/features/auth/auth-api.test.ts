import { describe, expect, it, vi } from 'vitest';

import { authenticate } from './auth-api';

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
      corpId: null,
      expiresAt: 3_601_000,
    });
  });
});
