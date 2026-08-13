import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  clearSidebarSession,
  completeSidebarAuthCallback,
  documentCookieAdapter,
  readSidebarSession,
  sidebarLoginHref,
  writeSidebarSession,
  type CookieAdapter,
} from './sidebar-session';

afterEach(() => {
  vi.restoreAllMocks();
});

function cookieAdapter(values: Record<string, string> = {}): {
  adapter: CookieAdapter;
  get: ReturnType<typeof vi.fn>;
  set: ReturnType<typeof vi.fn>;
} {
  const get = vi.fn((name: string) => values[name] ?? null);
  const set = vi.fn();
  return { adapter: { get, set }, get, set };
}

function authState(data: unknown, code = 200, msg = ''): string {
  return btoa(JSON.stringify({ code, msg, data }));
}

describe('Sidebar session', () => {
  it('returns null instead of throwing for a malformed percent-encoded cookie', () => {
    vi.spyOn(document, 'cookie', 'get').mockReturnValue('token=%E0%A4%A');

    expect(documentCookieAdapter.get('token')).toBeNull();
  });

  it('reads only the Sidebar token and agentId cookies', () => {
    const { adapter, get } = cookieAdapter({ token: 'sidebar-token', agentId: '7' });

    expect(readSidebarSession(adapter)).toEqual({ token: 'sidebar-token', agentId: '7' });
    expect(get.mock.calls).toEqual([['token'], ['agentId']]);
  });

  it('writes and clears only Sidebar cookies with constrained attributes', () => {
    const { adapter, set } = cookieAdapter();

    writeSidebarSession(adapter, {
      token: 'sidebar-token',
      agentId: '7',
      expiresInSeconds: 3600,
    }, true);
    clearSidebarSession(adapter, true);

    expect(set).toHaveBeenNthCalledWith(
      1,
      'token=sidebar-token; Max-Age=3600; Path=/; SameSite=Lax; Secure',
    );
    expect(set).toHaveBeenNthCalledWith(
      2,
      'agentId=7; Max-Age=3600; Path=/; SameSite=Lax; Secure',
    );
    expect(set).toHaveBeenNthCalledWith(
      3,
      'token=; Max-Age=0; Path=/; SameSite=Lax; Secure',
    );
    expect(set).toHaveBeenNthCalledWith(
      4,
      'agentId=; Max-Age=0; Path=/; SameSite=Lax; Secure',
    );
  });

  it('builds the real OAuth URL with an encoded safe internal target', () => {
    expect(sidebarLoginHref('7', '/contact?wxExternalUserid=external-user-1#profile')).toBe(
      '/sidebar/agent/auth?agentId=7&target=%2Fcontact%3FwxExternalUserid%3Dexternal-user-1%23profile',
    );
    expect(sidebarLoginHref('7', 'https://evil.example/steal')).toBe(
      '/sidebar/agent/auth?agentId=7&target=%2F',
    );
  });

  it('stores a successful callback session and returns to its safe target', () => {
    const { adapter, set } = cookieAdapter();
    const params = new URLSearchParams({
      agentId: '7',
      state: authState({ token: 'sidebar-token', expire: 7200 }),
      target: '/contact?wxExternalUserid=external-user-1',
    });

    expect(completeSidebarAuthCallback(params, adapter, false)).toEqual({
      ok: true,
      target: '/contact?wxExternalUserid=external-user-1',
    });
    expect(set).toHaveBeenNthCalledWith(
      1,
      'token=sidebar-token; Max-Age=7200; Path=/; SameSite=Lax',
    );
    expect(set).toHaveBeenNthCalledWith(
      2,
      'agentId=7; Max-Age=7200; Path=/; SameSite=Lax',
    );
  });

  it('falls back to the root when a callback target is external', () => {
    const { adapter } = cookieAdapter();
    const params = new URLSearchParams({
      agentId: '7',
      state: authState({ token: 'sidebar-token', expire: 7200 }),
      target: '//evil.example/steal',
    });

    expect(completeSidebarAuthCallback(params, adapter, false, 'https://sidebar.example.com')).toEqual({
      ok: true,
      target: '/',
    });
  });

  it('preserves the internal path when the backend returns the Sidebar target as an absolute URL', () => {
    const { adapter } = cookieAdapter();
    const params = new URLSearchParams({
      agentId: '7',
      state: authState({ token: 'sidebar-token', expire: 7200 }),
      target: 'https://sidebar.example.com/contact?wxExternalUserid=external-user-1#profile',
    });

    expect(completeSidebarAuthCallback(params, adapter, false, 'https://sidebar.example.com')).toEqual({
      ok: true,
      target: '/contact?wxExternalUserid=external-user-1#profile',
    });
  });
});
