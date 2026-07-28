import type { Session } from '@mochat/auth';

type ApiClient = {
  request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown>;
};

export type LoginInput = {
  phone: string;
  password: string;
};

type AuthResponse = {
  token: string;
  expire: number;
};

function tokenUserId(token: string): string {
  const compactToken = token.replace(/^Bearer\s+/i, '');
  const payload = compactToken.split('.')[1];
  if (payload === undefined) {
    throw new Error('登录响应包含无效令牌');
  }
  try {
    const normalized = payload.replaceAll('-', '+').replaceAll('_', '/');
    const padding = '='.repeat((4 - normalized.length % 4) % 4);
    const decoded = JSON.parse(atob(normalized + padding)) as { uid?: unknown };
    if (
      (typeof decoded.uid !== 'number' && typeof decoded.uid !== 'string')
      || String(decoded.uid) === ''
    ) {
      throw new Error('missing uid');
    }
    return String(decoded.uid);
  } catch {
    throw new Error('登录响应包含无效令牌');
  }
}

export async function authenticate(
  client: ApiClient,
  input: LoginInput,
  now = Date.now(),
): Promise<Session> {
  const result = await client.request('/user/auth', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  }) as AuthResponse;
  return {
    token: result.token,
    userId: tokenUserId(result.token),
    corpId: null,
    expiresAt: now + result.expire * 1_000,
  };
}
