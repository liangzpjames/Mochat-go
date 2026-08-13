import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  clearSidebarSession,
  completeSidebarAuthCallback,
  createCookieSidebarSessionAdapter,
  createSessionStorageSidebarSessionAdapter,
  documentCookieAdapter,
  readSidebarSession,
  sidebarLoginHref,
  writeSidebarSession,
  type CookieAdapter,
} from './sidebar-session';

afterEach(() => {
  vi.restoreAllMocks();
  window.sessionStorage.clear();
});

function cookieAdapter(values: Record<string, string> = {}) {
  const get = vi.fn((name: string) => values[name] ?? null);
  const set = vi.fn((_serializedCookie: string) => undefined);
  return { adapter: { get, set } satisfies CookieAdapter, get, set };
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
    const { adapter: cookies, get } = cookieAdapter({ token: 'sidebar-token', agentId: '7' });
    const adapter = createCookieSidebarSessionAdapter(cookies, false);

    expect(readSidebarSession(adapter)).toEqual({ token: 'sidebar-token', agentId: '7' });
    expect(get.mock.calls).toEqual([['token'], ['agentId']]);
  });

  it('writes and clears only Sidebar cookies with constrained attributes', () => {
    const { adapter: cookies, set } = cookieAdapter();
    const adapter = createCookieSidebarSessionAdapter(cookies, true);

    writeSidebarSession(adapter, {
      token: 'sidebar-token',
      agentId: '7',
      expiresInSeconds: 3600,
    });
    clearSidebarSession(adapter);

    expect(set).toHaveBeenNthCalledWith(
      1,
      'token=sidebar-token; Max-Age=3600; Path=/sidebar-app; SameSite=Lax; Secure',
    );
    expect(set).toHaveBeenNthCalledWith(
      2,
      'agentId=7; Max-Age=3600; Path=/sidebar-app; SameSite=Lax; Secure',
    );
    expect(set).toHaveBeenNthCalledWith(
      3,
      'token=; Max-Age=0; Path=/sidebar-app; SameSite=Lax; Secure',
    );
    expect(set).toHaveBeenNthCalledWith(
      4,
      'agentId=; Max-Age=0; Path=/sidebar-app; SameSite=Lax; Secure',
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
    const { adapter: cookies, set } = cookieAdapter();
    const adapter = createCookieSidebarSessionAdapter(cookies, false);
    const params = new URLSearchParams({
      agentId: '7',
      state: authState({ token: 'sidebar-token', expire: 7200 }),
      target: '/contact?wxExternalUserid=external-user-1',
    });

    expect(completeSidebarAuthCallback(params, adapter)).toEqual({
      ok: true,
      target: '/contact?wxExternalUserid=external-user-1',
    });
    expect(set).toHaveBeenNthCalledWith(
      1,
      'token=sidebar-token; Max-Age=7200; Path=/sidebar-app; SameSite=Lax',
    );
    expect(set).toHaveBeenNthCalledWith(
      2,
      'agentId=7; Max-Age=7200; Path=/sidebar-app; SameSite=Lax',
    );
  });

  it('stores and reads a root-mount callback session only through Sidebar sessionStorage', () => {
    vi.spyOn(Date, 'now').mockReturnValue(2_000_000_000_000);
    const cookieSet = vi.spyOn(document, 'cookie', 'set');
    const adapter = createSessionStorageSidebarSessionAdapter(window.sessionStorage);
    const params = new URLSearchParams({
      agentId: '7',
      state: authState({ token: 'root-sidebar-token', expire: 7200 }),
      target: '/contact?wxExternalUserid=external-user-1',
    });

    expect(completeSidebarAuthCallback(params, adapter)).toEqual({
      ok: true,
      target: '/contact?wxExternalUserid=external-user-1',
    });
    expect(readSidebarSession(adapter)).toEqual({
      token: 'root-sidebar-token',
      agentId: '7',
    });
    expect(window.sessionStorage.length).toBe(1);
    expect(window.sessionStorage.key(0)).toBe('mochat_sidebar_session_v1');
    expect(cookieSet).not.toHaveBeenCalled();
  });

  it.each([
    '{bad json',
    JSON.stringify({ token: 'root-token', agentId: '7' }),
    JSON.stringify({ token: '', agentId: '7', expiresAt: 2_000_000_001_000 }),
    JSON.stringify({ token: 'root-token', agentId: '0', expiresAt: 2_000_000_001_000 }),
    JSON.stringify({ token: 'root-token', agentId: '7', expiresAt: 2_000_000_001_000.5 }),
    JSON.stringify({ token: 'root-token', agentId: '7', expiresAt: 1_999_999_999_000 }),
  ])('fails a malformed or expired root session closed: %s', (stored) => {
    vi.spyOn(Date, 'now').mockReturnValue(2_000_000_000_000);
    window.sessionStorage.setItem('mochat_sidebar_session_v1', stored);
    const adapter = createSessionStorageSidebarSessionAdapter(window.sessionStorage);

    expect(readSidebarSession(adapter)).toEqual({ token: null, agentId: null });
    expect(window.sessionStorage.getItem('mochat_sidebar_session_v1')).toBeNull();
  });

  it('keeps the prefixed adapter on path-scoped cookies', () => {
    const { adapter: cookies, set } = cookieAdapter();
    const adapter = createCookieSidebarSessionAdapter(cookies, false);

    writeSidebarSession(adapter, {
      token: 'prefixed-sidebar-token',
      agentId: '7',
      expiresInSeconds: 7200,
    });

    expect(set).toHaveBeenNthCalledWith(
      1,
      'token=prefixed-sidebar-token; Max-Age=7200; Path=/sidebar-app; SameSite=Lax',
    );
    expect(set).toHaveBeenNthCalledWith(
      2,
      'agentId=7; Max-Age=7200; Path=/sidebar-app; SameSite=Lax',
    );
    expect(set.mock.calls.some(([value]) => /Path=\/(?:;|$)/.test(value))).toBe(false);
  });

  it('falls back to the root when a callback target is external', () => {
    const { adapter: cookies } = cookieAdapter();
    const adapter = createCookieSidebarSessionAdapter(cookies, false);
    const params = new URLSearchParams({
      agentId: '7',
      state: authState({ token: 'sidebar-token', expire: 7200 }),
      target: '//evil.example/steal',
    });

    expect(completeSidebarAuthCallback(params, adapter, 'https://sidebar.example.com')).toEqual({
      ok: true,
      target: '/',
    });
  });

  it('preserves the internal path when the backend returns the Sidebar target as an absolute URL', () => {
    const { adapter: cookies } = cookieAdapter();
    const adapter = createCookieSidebarSessionAdapter(cookies, false);
    const params = new URLSearchParams({
      agentId: '7',
      state: authState({ token: 'sidebar-token', expire: 7200 }),
      target: 'https://sidebar.example.com/contact?wxExternalUserid=external-user-1#profile',
    });

    expect(completeSidebarAuthCallback(params, adapter, 'https://sidebar.example.com')).toEqual({
      ok: true,
      target: '/contact?wxExternalUserid=external-user-1#profile',
    });
  });
});
