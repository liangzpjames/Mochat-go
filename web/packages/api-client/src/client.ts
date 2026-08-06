import { ApiError } from './errors';
import { parseApiEnvelope } from './schema';

export type ApiClientOptions = {
  baseUrl: string;
  getToken: () => string | null;
  onUnauthorized: () => void;
};

function bearerToken(token: string): string {
  return /^Bearer\s/i.test(token) ? token : `Bearer ${token}`;
}

function resolveInput(input: RequestInfo | URL, baseUrl: string): RequestInfo | URL {
  if (typeof input === 'string') {
    const normalizedBase = baseUrl.endsWith('/') ? baseUrl : `${baseUrl}/`;
    return new URL(input.replace(/^\/+/, ''), normalizedBase);
  }
  return input;
}

export function createApiClient(options: ApiClientOptions): {
  request<T>(input: RequestInfo | URL, init?: RequestInit): Promise<T>;
} {
  return {
    async request<T>(input: RequestInfo | URL, init?: RequestInit): Promise<T> {
      const headers = new Headers(init?.headers);
      const token = options.getToken();
      if (token && !headers.has('Authorization')) {
        headers.set('Authorization', bearerToken(token));
      }
      let response: Response;
      try {
        response = await fetch(resolveInput(input, options.baseUrl), { ...init, headers });
      } catch (error) {
        if (error instanceof Error && error.name === 'AbortError') {
          throw error;
        }
        const message = error instanceof Error ? error.message : 'Network request failed';
        throw new ApiError('network', message, { cause: error });
      }
      let payload: unknown;
      try {
        payload = await response.json();
      } catch (error) {
        if (response.status === 401) {
          options.onUnauthorized();
          throw new ApiError('unauthorized', response.statusText || 'Unauthorized', {
            status: response.status,
            cause: error,
          });
        }
        if (response.status === 403) {
          throw new ApiError('forbidden', response.statusText || 'Forbidden', {
            status: response.status,
            cause: error,
          });
        }
        if (response.status >= 500) {
          throw new ApiError('server', response.statusText || 'Server error', {
            status: response.status,
            cause: error,
          });
        }
        throw error;
      }
      const envelope = parseApiEnvelope<T>(payload);
      if (response.status === 401) {
        options.onUnauthorized();
        throw new ApiError('unauthorized', envelope.msg, {
          status: response.status,
          code: envelope.code,
        });
      }
      if (response.status === 403) {
        throw new ApiError('forbidden', envelope.msg, {
          status: response.status,
          code: envelope.code,
        });
      }
      if (response.status >= 500) {
        throw new ApiError('server', envelope.msg, {
          status: response.status,
          code: envelope.code,
        });
      }
      if (!response.ok) {
        throw new ApiError('validation', envelope.msg, {
          status: response.status,
          code: envelope.code,
        });
      }
      if (envelope.code !== 0 && (envelope.code < 200 || envelope.code >= 300)) {
        throw new ApiError('validation', envelope.msg, {
          status: response.status,
          code: envelope.code,
        });
      }
      return envelope.data;
    },
  };
}
