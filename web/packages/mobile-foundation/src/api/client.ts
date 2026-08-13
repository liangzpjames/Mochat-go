export type MobileApiErrorKind =
  | 'unauthorized' | 'forbidden' | 'not-found' | 'conflict'
  | 'validation' | 'server' | 'network' | 'aborted';

export type MobileApiClientOptions = {
  basePath: '/sidebar' | '/operation';
  getToken?: () => string | null;
  onUnauthorized?: () => void;
  timeoutMs?: number;
};

type MobileApiErrorDetails = {
  status?: number | null;
  code?: string | null;
  requestId?: string | null;
  retryable?: boolean;
  cause?: unknown;
};

type MobileApiEnvelope<T> = {
  code: number;
  msg: string;
  data: T;
  errorCode?: string;
  requestId?: string;
};

const REQUEST_ORIGIN = 'https://mobile.internal';

export class MobileApiError extends Error {
  readonly kind: MobileApiErrorKind;
  readonly status: number | null;
  readonly code: string | null;
  readonly requestId: string | null;
  readonly retryable: boolean;

  constructor(kind: MobileApiErrorKind, message: string, details: MobileApiErrorDetails = {}) {
    super(message, details.cause === undefined ? undefined : { cause: details.cause });
    this.name = 'MobileApiError';
    this.kind = kind;
    this.status = details.status ?? null;
    this.code = details.code ?? null;
    this.requestId = details.requestId ?? null;
    this.retryable = details.retryable ?? false;
  }
}

function scopedRequestPath(basePath: MobileApiClientOptions['basePath'], path: string): string {
  if (
    path.length === 0
    || !path.startsWith('/')
    || path.startsWith('//')
    || path.includes('\\')
    || /^[a-z][a-z\d+.-]*:/i.test(path)
  ) {
    throw new MobileApiError('validation', 'Request path must be a relative same-origin path.');
  }

  try {
    const rawPathname = path.split(/[?#]/, 1)[0] ?? '';
    const decodedSegments = decodeURIComponent(rawPathname).split('/');
    const resolved = new URL(`${basePath}${path}`, REQUEST_ORIGIN);
    if (
      decodedSegments.some((segment) => segment === '.' || segment === '..')
      || !resolved.pathname.startsWith(`${basePath}/`)
    ) {
      throw new MobileApiError(
        'validation',
        'Request path must stay within its scoped base path.',
      );
    }
  } catch (error) {
    if (error instanceof MobileApiError) {
      throw error;
    }
    throw new MobileApiError('validation', 'Request path is malformed.', { cause: error });
  }

  return `${basePath}${path}`;
}

function isEnvelope<T>(value: unknown): value is MobileApiEnvelope<T> {
  if (typeof value !== 'object' || value === null) {
    return false;
  }
  const envelope = value as Record<string, unknown>;
  return (
    typeof envelope.code === 'number'
    && typeof envelope.msg === 'string'
    && 'data' in envelope
    && (envelope.errorCode === undefined || typeof envelope.errorCode === 'string')
    && (envelope.requestId === undefined || typeof envelope.requestId === 'string')
  );
}

function errorKind(status: number): MobileApiErrorKind {
  if (status === 401) return 'unauthorized';
  if (status === 403) return 'forbidden';
  if (status === 404) return 'not-found';
  if (status === 409) return 'conflict';
  if (status >= 500) return 'server';
  return 'validation';
}

function isSuccessfulCode(code: number): boolean {
  return code === 0 || (code >= 200 && code < 300);
}

function bearerToken(token: string): string {
  return /^Bearer\s/i.test(token) ? token : `Bearer ${token}`;
}

export function createMobileApiClient(options: MobileApiClientOptions): {
  request<T>(path: string, init?: RequestInit): Promise<T>;
} {
  return {
    async request<T>(path: string, init?: RequestInit): Promise<T> {
      const requestPath = scopedRequestPath(options.basePath, path);
      const headers = new Headers(init?.headers);
      const token = options.getToken?.();
      if (token && !headers.has('Authorization')) {
        headers.set('Authorization', bearerToken(token));
      }

      const timeoutSignal = options.timeoutMs === undefined
        ? undefined
        : AbortSignal.timeout(options.timeoutMs);
      const signal = timeoutSignal && init?.signal
        ? AbortSignal.any([timeoutSignal, init.signal])
        : (timeoutSignal ?? init?.signal);

      let response: Response;
      try {
        response = await fetch(requestPath, {
          ...init,
          credentials: 'same-origin',
          headers,
          ...(signal === undefined ? {} : { signal }),
        });
      } catch (cause) {
        if (signal?.aborted) {
          const timedOut = timeoutSignal?.aborted === true && init?.signal?.aborted !== true;
          throw new MobileApiError(
            'aborted',
            timedOut ? 'Request timed out.' : 'Request was cancelled.',
            { retryable: timedOut, cause },
          );
        }
        throw new MobileApiError('network', 'Network request failed.', {
          retryable: true,
          cause,
        });
      }

      if (response.status === 204) {
        return undefined as T;
      }

      let payload: unknown;
      try {
        payload = await response.json();
      } catch (cause) {
        const kind = errorKind(response.status);
        if (kind === 'unauthorized') {
          options.onUnauthorized?.();
        }
        throw new MobileApiError(kind, response.statusText || 'Invalid API response.', {
          status: response.status,
          requestId: response.headers.get('X-Request-Id'),
          retryable: kind === 'server',
          cause,
        });
      }

      if (!isEnvelope<T>(payload)) {
        throw new MobileApiError('validation', 'Invalid API response.', {
          status: response.status,
          requestId: response.headers.get('X-Request-Id'),
        });
      }

      const requestId = payload.requestId ?? response.headers.get('X-Request-Id');
      if (!response.ok || !isSuccessfulCode(payload.code)) {
        const kind = response.ok ? 'validation' : errorKind(response.status);
        if (kind === 'unauthorized') {
          options.onUnauthorized?.();
        }
        throw new MobileApiError(kind, payload.msg, {
          status: response.status,
          code: payload.errorCode ?? null,
          requestId,
          retryable: kind === 'server',
        });
      }

      return payload.data;
    },
  };
}
