import { safeInternalTarget } from '@mochat/mobile-foundation';

const TOKEN_COOKIE = 'token';
const AGENT_ID_COOKIE = 'agentId';
const SIDEBAR_SESSION_STORAGE_KEY = 'mochat_sidebar_session_v1';

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

export type SidebarSessionAdapter = {
  read(): SidebarSession;
  write(session: WritableSidebarSession): boolean;
  clear(): void;
};

type SessionStorageAdapter = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>;

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

export function readSidebarSession(session: SidebarSessionAdapter): SidebarSession {
  return session.read();
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
    'Path=/sidebar-app',
    'SameSite=Lax',
    ...(secure ? ['Secure'] : []),
  ].join('; ');
}

export function createCookieSidebarSessionAdapter(
  cookies: CookieAdapter,
  secure: boolean,
): SidebarSessionAdapter {
  return {
    read() {
      return {
        token: cookies.get(TOKEN_COOKIE),
        agentId: cookies.get(AGENT_ID_COOKIE),
      };
    },
    write(session) {
      cookies.set(serializeCookie(TOKEN_COOKIE, session.token, session.expiresInSeconds, secure));
      cookies.set(serializeCookie(AGENT_ID_COOKIE, session.agentId, session.expiresInSeconds, secure));
      return true;
    },
    clear() {
      cookies.set(serializeCookie(TOKEN_COOKIE, '', 0, secure));
      cookies.set(serializeCookie(AGENT_ID_COOKIE, '', 0, secure));
    },
  };
}

function emptySidebarSession(): SidebarSession {
  return { token: null, agentId: null };
}

function isPositiveAgentId(value: unknown): value is string {
  return typeof value === 'string' && /^\d+$/.test(value) && value !== '0';
}

export function createSessionStorageSidebarSessionAdapter(
  storage: SessionStorageAdapter,
): SidebarSessionAdapter {
  const clear = () => {
    try {
      storage.removeItem(SIDEBAR_SESSION_STORAGE_KEY);
    } catch {
      // Storage access can be denied; the caller still fails closed.
    }
  };

  return {
    read() {
      try {
        const raw = storage.getItem(SIDEBAR_SESSION_STORAGE_KEY);
        if (raw === null) return emptySidebarSession();
        const value = JSON.parse(raw) as unknown;
        if (typeof value !== 'object' || value === null) {
          clear();
          return emptySidebarSession();
        }
        const fields = value as Record<string, unknown>;
        if (
          typeof fields.token !== 'string'
          || fields.token.length === 0
          || !isPositiveAgentId(fields.agentId)
          || typeof fields.expiresAt !== 'number'
          || !Number.isSafeInteger(fields.expiresAt)
          || fields.expiresAt <= Date.now()
        ) {
          clear();
          return emptySidebarSession();
        }
        return { token: fields.token, agentId: fields.agentId };
      } catch {
        clear();
        return emptySidebarSession();
      }
    },
    write(session) {
      const expiresInSeconds = Math.floor(session.expiresInSeconds);
      const expiresAt = Date.now() + expiresInSeconds * 1000;
      if (
        session.token.length === 0
        || !isPositiveAgentId(session.agentId)
        || !Number.isSafeInteger(expiresAt)
        || expiresInSeconds <= 0
      ) {
        clear();
        return false;
      }
      try {
        storage.setItem(SIDEBAR_SESSION_STORAGE_KEY, JSON.stringify({
          token: session.token,
          agentId: session.agentId,
          expiresAt,
        }));
        return true;
      } catch {
        clear();
        return false;
      }
    },
    clear,
  };
}

export function writeSidebarSession(
  adapter: SidebarSessionAdapter,
  session: WritableSidebarSession,
): boolean {
  return adapter.write(session);
}

export function clearSidebarSession(adapter: SidebarSessionAdapter): void {
  adapter.clear();
}

export function sidebarLoginHref(agentId: string, rawTarget: string): string {
  const query = new URLSearchParams({
    agentId,
    target: safeInternalTarget(rawTarget, '/'),
  });
  return `/sidebar/agent/auth?${query.toString()}`;
}

function targetWithinBasename(target: string, basename: string): string {
  if (basename === '/') return target;
  if (target === basename) return '/';
  return target.startsWith(`${basename}/`) ? target.slice(basename.length) : target;
}

function callbackTarget(
  rawTarget: string | null,
  currentOrigin?: string,
  basename = '/',
): string {
  if (rawTarget && currentOrigin && /^[a-z][a-z\d+.-]*:/i.test(rawTarget)) {
    try {
      const parsed = new URL(rawTarget);
      if (parsed.origin !== currentOrigin) return '/';
      return targetWithinBasename(
        safeInternalTarget(`${parsed.pathname}${parsed.search}${parsed.hash}`, '/'),
        basename,
      );
    } catch {
      return '/';
    }
  }
  return targetWithinBasename(safeInternalTarget(rawTarget, '/'), basename);
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
  session: SidebarSessionAdapter,
  currentOrigin?: string,
  basename = '/',
): SidebarAuthCallbackResult {
  const target = callbackTarget(params.get('target'), currentOrigin, basename);
  const agentId = params.get('agentId')?.trim() ?? '';
  const state = callbackData(decodeCallbackState(params.get('state')));
  if (state === null || state.code !== 200 || !/^\d+$/.test(agentId) || agentId === '0') {
    return {
      ok: false,
      message: state?.msg || '登录状态无效，请重新授权。',
      target,
    };
  }

  const stored = writeSidebarSession(session, {
    token: state.token,
    agentId,
    expiresInSeconds: state.expire,
  });
  if (!stored) {
    return {
      ok: false,
      message: '登录状态无法保存，请重新授权。',
      target,
    };
  }
  return { ok: true, target };
}
