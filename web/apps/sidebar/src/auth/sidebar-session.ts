import { safeInternalTarget } from '@mochat/mobile-foundation';

const TOKEN_COOKIE = 'token';
const AGENT_ID_COOKIE = 'agentId';

export type CookieAdapter = {
  get(name: string): string | null;
  set(serializedCookie: string): void;
};

export type SidebarSession = {
  token: string | null;
  agentId: string | null;
};

export type WritableSidebarSession = {
  token: string;
  agentId: string;
  expiresInSeconds: number;
};

export type SidebarAuthCallbackResult =
  | { ok: true; target: string }
  | { ok: false; message: string; target: string };

export const documentCookieAdapter: CookieAdapter = {
  get(name) {
    const prefix = `${encodeURIComponent(name)}=`;
    const cookie = document.cookie
      .split(';')
      .map((part) => part.trim())
      .find((part) => part.startsWith(prefix));
    if (cookie === undefined) return null;
    try {
      return decodeURIComponent(cookie.slice(prefix.length));
    } catch {
      return null;
    }
  },
  set(serializedCookie) {
    document.cookie = serializedCookie;
  },
};

export function readSidebarSession(cookies: CookieAdapter): SidebarSession {
  return {
    token: cookies.get(TOKEN_COOKIE),
    agentId: cookies.get(AGENT_ID_COOKIE),
  };
}

function serializeCookie(
  name: typeof TOKEN_COOKIE | typeof AGENT_ID_COOKIE,
  value: string,
  maxAge: number,
  secure: boolean,
): string {
  return [
    `${name}=${encodeURIComponent(value)}`,
    `Max-Age=${Math.max(0, Math.floor(maxAge))}`,
    'Path=/',
    'SameSite=Lax',
    ...(secure ? ['Secure'] : []),
  ].join('; ');
}

export function writeSidebarSession(
  cookies: CookieAdapter,
  session: WritableSidebarSession,
  secure: boolean,
): void {
  cookies.set(serializeCookie(TOKEN_COOKIE, session.token, session.expiresInSeconds, secure));
  cookies.set(serializeCookie(AGENT_ID_COOKIE, session.agentId, session.expiresInSeconds, secure));
}

export function clearSidebarSession(cookies: CookieAdapter, secure: boolean): void {
  cookies.set(serializeCookie(TOKEN_COOKIE, '', 0, secure));
  cookies.set(serializeCookie(AGENT_ID_COOKIE, '', 0, secure));
}

export function sidebarLoginHref(agentId: string, rawTarget: string): string {
  const query = new URLSearchParams({
    agentId,
    target: safeInternalTarget(rawTarget, '/'),
  });
  return `/sidebar/agent/auth?${query.toString()}`;
}

function callbackTarget(rawTarget: string | null, currentOrigin?: string): string {
  if (rawTarget && currentOrigin && /^[a-z][a-z\d+.-]*:/i.test(rawTarget)) {
    try {
      const parsed = new URL(rawTarget);
      if (parsed.origin !== currentOrigin) return '/';
      return safeInternalTarget(`${parsed.pathname}${parsed.search}${parsed.hash}`, '/');
    } catch {
      return '/';
    }
  }
  return safeInternalTarget(rawTarget, '/');
}

function decodeCallbackState(rawState: string | null): unknown {
  if (!rawState) return null;
  try {
    const bytes = Uint8Array.from(atob(rawState), (character) => character.charCodeAt(0));
    return JSON.parse(new TextDecoder().decode(bytes)) as unknown;
  } catch {
    return null;
  }
}

function callbackData(value: unknown): {
  code: number;
  msg: string;
  token: string;
  expire: number;
} | null {
  if (typeof value !== 'object' || value === null) return null;
  const envelope = value as Record<string, unknown>;
  const data = envelope.data;
  if (typeof data !== 'object' || data === null) return null;
  const fields = data as Record<string, unknown>;
  if (
    typeof envelope.code !== 'number'
    || typeof envelope.msg !== 'string'
    || typeof fields.token !== 'string'
    || fields.token.length === 0
    || typeof fields.expire !== 'number'
    || !Number.isFinite(fields.expire)
    || fields.expire <= 0
  ) {
    return null;
  }
  return {
    code: envelope.code,
    msg: envelope.msg,
    token: fields.token,
    expire: fields.expire,
  };
}

export function completeSidebarAuthCallback(
  params: URLSearchParams,
  cookies: CookieAdapter,
  secure: boolean,
  currentOrigin?: string,
): SidebarAuthCallbackResult {
  const target = callbackTarget(params.get('target'), currentOrigin);
  const agentId = params.get('agentId')?.trim() ?? '';
  const state = callbackData(decodeCallbackState(params.get('state')));
  if (state === null || state.code !== 200 || !/^\d+$/.test(agentId) || agentId === '0') {
    return {
      ok: false,
      message: state?.msg || '登录状态无效，请重新授权。',
      target,
    };
  }

  writeSidebarSession(cookies, {
    token: state.token,
    agentId,
    expiresInSeconds: state.expire,
  }, secure);
  return { ok: true, target };
}
