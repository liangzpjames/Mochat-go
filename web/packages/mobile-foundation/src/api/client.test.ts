import { afterEach, describe, expect, it, vi } from 'vitest';

import { createMobileApiClient, MobileApiError } from './client';

type FetchCall = [RequestInfo | URL, RequestInit | undefined];

function jsonResponse(
  body: unknown,
  init: ResponseInit = {},
): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });
}

function stubFetch(response: Response): ReturnType<typeof vi.fn> {
  const fetchMock = vi.fn<typeof fetch>().mockImplementation(() => Promise.resolve(response.clone()));
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

async function expectMobileError(
  request: Promise<unknown>,
  expected: Partial<MobileApiError>,
): Promise<void> {
  const error = await request.catch((reason: unknown) => reason);
  expect(error).toBeInstanceOf(MobileApiError);
  expect(error).toMatchObject(expected);
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

describe('createMobileApiClient', () => {
  it('adds scoped base path, same-origin credentials and optional bearer token', async () => {
    const fetchMock = stubFetch(jsonResponse({ code: 0, msg: 'ok', data: { id: '7' } }));
    const getToken = vi.fn<() => string | null>(() => 'sidebar-token');
    const client = createMobileApiClient({ basePath: '/sidebar', getToken });

    await client.request('/contacts/current?view=summary');

    expect(getToken).toHaveBeenCalledOnce();
    const [path, init] = fetchMock.mock.calls[0] as FetchCall;
    expect(path).toBe('/sidebar/contacts/current?view=summary');
    expect(init?.credentials).toBe('same-origin');
    expect(new Headers(init?.headers).get('Authorization')).toBe('Bearer sidebar-token');

    getToken.mockReturnValue(null);
    await client.request('/contacts/current');
    const [, anonymousInit] = fetchMock.mock.calls[1] as FetchCall;
    expect(new Headers(anonymousInit?.headers).has('Authorization')).toBe(false);
  });

  it('isolates authorization from caller headers', async () => {
    const fetchMock = stubFetch(jsonResponse({ code: 0, msg: 'ok', data: null }));
    const anonymousClient = createMobileApiClient({ basePath: '/operation' });

    await anonymousClient.request('/session', {
      headers: { Authorization: 'Bearer caller-token' },
    });
    const [, anonymousInit] = fetchMock.mock.calls[0] as FetchCall;
    expect(new Headers(anonymousInit?.headers).has('Authorization')).toBe(false);

    const authenticatedClient = createMobileApiClient({
      basePath: '/sidebar',
      getToken: () => 'scoped-token',
    });
    await authenticatedClient.request('/contacts/current', {
      headers: { Authorization: 'Bearer caller-token' },
    });
    const [, authenticatedInit] = fetchMock.mock.calls[1] as FetchCall;
    expect(new Headers(authenticatedInit?.headers).get('Authorization')).toBe(
      'Bearer scoped-token',
    );
  });

  it('unwraps { code, msg, data, errorCode, requestId }', async () => {
    stubFetch(jsonResponse({
      code: 0,
      msg: 'ok',
      data: { id: 'contact-7' },
      errorCode: '',
      requestId: 'req-envelope-success',
    }));
    const client = createMobileApiClient({ basePath: '/sidebar' });

    await expect(client.request('/contacts/current')).resolves.toEqual({ id: 'contact-7' });
  });

  it('maps 401, 403, validation, server, network and abort separately', async () => {
    const onUnauthorized = vi.fn();
    const client = createMobileApiClient({ basePath: '/operation', onUnauthorized });

    stubFetch(jsonResponse(
      { code: 401, msg: 'login required', data: null, errorCode: 'AUTH_REQUIRED', requestId: 'req-envelope' },
      { status: 401, headers: { 'X-Request-Id': 'req-header' } },
    ));
    await expectMobileError(client.request('/session'), {
      kind: 'unauthorized',
      status: 401,
      code: 'AUTH_REQUIRED',
      requestId: 'req-envelope',
      retryable: false,
    });
    expect(onUnauthorized).toHaveBeenCalledOnce();

    stubFetch(jsonResponse(
      { code: 403, msg: 'forbidden', data: null },
      { status: 403, headers: { 'X-Request-Id': 'req-forbidden' } },
    ));
    await expectMobileError(client.request('/session'), {
      kind: 'forbidden',
      status: 403,
      code: null,
      requestId: 'req-forbidden',
      retryable: false,
    });

    stubFetch(jsonResponse(
      { code: 422, msg: 'invalid activity', data: null, errorCode: 'ACTIVITY_INVALID' },
      { status: 422 },
    ));
    await expectMobileError(client.request('/activity'), {
      kind: 'validation',
      status: 422,
      code: 'ACTIVITY_INVALID',
      retryable: false,
    });

    stubFetch(jsonResponse(
      { code: 503, msg: 'temporarily unavailable', data: null },
      { status: 503 },
    ));
    await expectMobileError(client.request('/activity'), {
      kind: 'server',
      status: 503,
      retryable: true,
    });

    vi.stubGlobal('fetch', vi.fn<typeof fetch>().mockRejectedValue(new TypeError('offline')));
    await expectMobileError(client.request('/activity'), {
      kind: 'network',
      status: null,
      retryable: true,
    });

    const controller = new AbortController();
    vi.stubGlobal('fetch', vi.fn<typeof fetch>().mockImplementation((_path, init) => {
      return new Promise((_resolve, reject) => {
        init?.signal?.addEventListener('abort', () => reject(new Error('aborted')), { once: true });
      });
    }));
    const abortedRequest = client.request('/activity', { signal: controller.signal });
    controller.abort();
    await expectMobileError(abortedRequest, {
      kind: 'aborted',
      status: null,
      retryable: false,
    });
  });

  it('maps 404 and 409 to stable non-retryable errors', async () => {
    const client = createMobileApiClient({ basePath: '/operation' });
    stubFetch(jsonResponse({ code: 404, msg: 'missing', data: null }, { status: 404 }));
    await expectMobileError(client.request('/activity'), {
      kind: 'not-found',
      status: 404,
      retryable: false,
    });

    stubFetch(jsonResponse({ code: 409, msg: 'conflict', data: null }, { status: 409 }));
    await expectMobileError(client.request('/activity'), {
      kind: 'conflict',
      status: 409,
      retryable: false,
    });
  });

  it('uses AbortSignal.any for timeout and caller cancellation', async () => {
    vi.useFakeTimers();
    const anySpy = vi.spyOn(AbortSignal, 'any');
    const controller = new AbortController();
    vi.stubGlobal('fetch', vi.fn<typeof fetch>().mockImplementation((_path, init) => {
      return new Promise((_resolve, reject) => {
        init?.signal?.addEventListener('abort', () => reject(new Error('aborted')), { once: true });
      });
    }));
    const client = createMobileApiClient({ basePath: '/sidebar', timeoutMs: 25 });

    const request = client.request('/slow', { signal: controller.signal });
    const assertion = expectMobileError(request, { kind: 'aborted', retryable: true });
    expect(anySpy).toHaveBeenCalledOnce();
    await vi.advanceTimersByTimeAsync(25);
    await assertion;
  });

  it('rejects absolute request URLs', async () => {
    const fetchMock = vi.fn<typeof fetch>();
    vi.stubGlobal('fetch', fetchMock);
    const client = createMobileApiClient({ basePath: '/sidebar' });

    await expectMobileError(client.request('https://evil.example/steal'), {
      kind: 'validation',
      status: null,
      retryable: false,
    });
    await expectMobileError(client.request('//evil.example/steal'), {
      kind: 'validation',
      status: null,
      retryable: false,
    });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('rejects path traversal outside the scoped base path', async () => {
    const fetchMock = stubFetch(jsonResponse({ code: 0, msg: 'ok', data: null }));
    const client = createMobileApiClient({
      basePath: '/sidebar',
      getToken: () => 'sidebar-token',
    });

    for (const path of ['/../dashboard', '/%2e%2e/dashboard', '/.%2e/dashboard']) {
      await expectMobileError(client.request(path), {
        kind: 'validation',
        status: null,
        retryable: false,
      });
    }
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
