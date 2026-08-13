import { ApiError } from './errors';
import { parseApiEnvelope } from './schema';

export type ApiClientOptions = {
  baseUrl: string;
  getToken: () => string | null;
  onUnauthorized: () => void;
  onTenantAccessDenied?: () => void;
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
      if (response.status === 204) {
        return undefined as T;
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
      const errorDetails = {
        status: response.status,
        code: envelope.code,
        ...(envelope.errorCode === undefined ? {} : { machineCode: envelope.errorCode }),
      };
      if (response.status === 401) {
        options.onUnauthorized();
        throw new ApiError('unauthorized', envelope.msg, {
          ...errorDetails,
        });
      }
      if (response.status === 403) {
        if (envelope.errorCode === 'TENANT_ACCESS_DENIED') {
          options.onTenantAccessDenied?.();
        }
        throw new ApiError('forbidden', envelope.msg, {
          ...errorDetails,
        });
      }
      if (response.status >= 500) {
        throw new ApiError('server', envelope.msg, {
          ...errorDetails,
        });
      }
      if (!response.ok) {
        throw new ApiError('validation', envelope.msg, {
          ...errorDetails,
        });
      }
      if (
        typeof envelope.code !== 'number'
        || (envelope.code !== 0 && (envelope.code < 200 || envelope.code >= 300))
      ) {
        throw new ApiError('validation', envelope.msg, {
          ...errorDetails,
        });
      }
      return envelope.data;
    },
  };
}
