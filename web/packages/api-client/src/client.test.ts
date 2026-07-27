import { HttpResponse, delay, http, setupServer } from '@mochat/testing';
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from 'vitest';

import { createApiClient } from './client';
import { ApiError } from './errors';

const server = setupServer();

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

async function expectApiError(
  request: Promise<unknown>,
  expected: Partial<ApiError>,
): Promise<void> {
  const error = await request.catch((reason: unknown) => reason);
  expect(error).toBeInstanceOf(ApiError);
  expect(error).toMatchObject(expected);
}

describe('createApiClient', () => {
  it('adds a Bearer token to a relative request', async () => {
    let authorization: string | null = null;
    server.use(
      http.get('https://api.example.test/users/current', ({ request }) => {
        authorization = request.headers.get('authorization');
        return HttpResponse.json({ code: 0, msg: 'ok', data: { id: '7' } });
      }),
    );
    const client = createApiClient({
      baseUrl: 'https://api.example.test/',
      getToken: () => 'secret-token',
      onUnauthorized: vi.fn(),
    });

    await client.request('/users/current');

    expect(authorization).toBe('Bearer secret-token');
  });

  it('unwraps the data from a successful API envelope', async () => {
    server.use(
      http.get('https://api.example.test/users/current', () =>
        HttpResponse.json({ code: 0, msg: 'ok', data: { id: '7' } }),
      ),
    );
    const client = createApiClient({
      baseUrl: 'https://api.example.test/',
      getToken: () => null,
      onUnauthorized: vi.fn(),
    });

    const result = await client.request<{ id: string }>('/users/current');

    expect(result).toEqual({ id: '7' });
  });

  it('maps a 401 response and invokes onUnauthorized exactly once', async () => {
    server.use(
      http.get('https://api.example.test/private', () =>
        HttpResponse.json({ code: 40101, msg: 'login required', data: null }, { status: 401 }),
      ),
    );
    const onUnauthorized = vi.fn();
    const client = createApiClient({
      baseUrl: 'https://api.example.test/',
      getToken: () => null,
      onUnauthorized,
    });

    const request = client.request('/private');

    await expectApiError(request, {
      kind: 'unauthorized',
      status: 401,
      code: 40101,
      message: 'login required',
    });
    expect(onUnauthorized).toHaveBeenCalledTimes(1);
  });

  it('maps a 403 response to a forbidden API error', async () => {
    server.use(
      http.get('https://api.example.test/restricted', () =>
        HttpResponse.json({ code: 40301, msg: 'not allowed', data: null }, { status: 403 }),
      ),
    );
    const client = createApiClient({
      baseUrl: 'https://api.example.test/',
      getToken: () => null,
      onUnauthorized: vi.fn(),
    });

    await expectApiError(client.request('/restricted'), {
      kind: 'forbidden',
      status: 403,
      code: 40301,
      message: 'not allowed',
    });
  });

  it('maps a nonzero business code to a validation API error', async () => {
    server.use(
      http.post('https://api.example.test/users', () =>
        HttpResponse.json({ code: 1007, msg: 'phone is invalid', data: null }),
      ),
    );
    const client = createApiClient({
      baseUrl: 'https://api.example.test/',
      getToken: () => null,
      onUnauthorized: vi.fn(),
    });

    await expectApiError(client.request('/users', { method: 'POST' }), {
      kind: 'validation',
      status: 200,
      code: 1007,
      message: 'phone is invalid',
    });
  });

  it('maps a non-JSON 5xx response to a server API error', async () => {
    server.use(
      http.get(
        'https://api.example.test/unavailable',
        () => new HttpResponse('upstream unavailable', { status: 503, statusText: 'Service Unavailable' }),
      ),
    );
    const client = createApiClient({
      baseUrl: 'https://api.example.test/',
      getToken: () => null,
      onUnauthorized: vi.fn(),
    });

    await expectApiError(client.request('/unavailable'), {
      kind: 'server',
      status: 503,
      message: 'Service Unavailable',
    });
  });

  it('maps a JSON 5xx envelope to a server API error', async () => {
    server.use(
      http.get('https://api.example.test/failed', () =>
        HttpResponse.json({ code: 50001, msg: 'database failed', data: null }, { status: 500 }),
      ),
    );
    const client = createApiClient({
      baseUrl: 'https://api.example.test/',
      getToken: () => null,
      onUnauthorized: vi.fn(),
    });

    await expectApiError(client.request('/failed'), {
      kind: 'server',
      status: 500,
      code: 50001,
      message: 'database failed',
    });
  });

  it('maps a fetch failure to a network API error', async () => {
    server.use(
      http.get('https://api.example.test/offline', () => HttpResponse.error()),
    );
    const client = createApiClient({
      baseUrl: 'https://api.example.test/',
      getToken: () => null,
      onUnauthorized: vi.fn(),
    });

    await expectApiError(client.request('/offline'), {
      kind: 'network',
    });
  });

  it('propagates AbortError unchanged', async () => {
    server.use(
      http.get('https://api.example.test/slow', async () => {
        await delay('infinite');
        return HttpResponse.json({ code: 0, msg: 'ok', data: null });
      }),
    );
    const client = createApiClient({
      baseUrl: 'https://api.example.test/',
      getToken: () => null,
      onUnauthorized: vi.fn(),
    });
    const controller = new AbortController();

    const request = client.request('/slow', { signal: controller.signal });
    controller.abort();

    await expect(request).rejects.toBe(controller.signal.reason);
  });
});
