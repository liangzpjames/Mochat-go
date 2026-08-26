import { createApiClient } from '@mochat/api-client';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { createArchiveMediaClient } from './archive-media-client';

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('createArchiveMediaClient', () => {
  it('uses the session api client to attach Bearer authorization to binary fetches', async () => {
    const fetcher = vi.fn().mockResolvedValue(new Response(new Blob(['image-bytes'], { type: 'image/png' }), {
      status: 200,
      headers: { 'Content-Disposition': 'inline; filename="safe.png"' },
    }));
    vi.stubGlobal('fetch', fetcher);
    const apiClient = createApiClient({
      baseUrl: 'https://dashboard.example/dashboard/',
      getToken: () => 'session-token',
      onUnauthorized: vi.fn(),
    });
    const client = createArchiveMediaClient(apiClient, 'https://dashboard.example');

    const result = await client.download('/dashboard/archive/media/8ff7bf2d-5604-43bc-a600-3ec91d575085/content');

    expect(result.filename).toBe('safe.png');
    expect(fetcher).toHaveBeenCalledOnce();
    const [input, init] = fetcher.mock.calls[0] as [URL, RequestInit];
    expect(input.toString()).toBe('https://dashboard.example/dashboard/archive/media/8ff7bf2d-5604-43bc-a600-3ec91d575085/content');
    expect(new Headers(init.headers).get('Authorization')).toBe('Bearer session-token');
  });

  it('rejects URLs outside the canonical archive-media endpoint before fetch', async () => {
    const apiClient = createApiClient({
      baseUrl: 'https://dashboard.example/dashboard/',
      getToken: () => 'session-token',
      onUnauthorized: vi.fn(),
    });
    const client = createArchiveMediaClient(apiClient, 'https://dashboard.example');
    await expect(client.download('https://attacker.example/media')).rejects.toThrow('invalid archive media URL');
  });
});
