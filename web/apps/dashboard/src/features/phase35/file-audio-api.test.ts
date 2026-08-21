import { describe, expect, it, vi } from 'vitest';

import { createFileAudioApi } from './file-audio-api';

describe('createFileAudioApi', () => {
  it('sends sender, receiver and synced date filters without a client corp override', async () => {
    const request = vi.fn().mockResolvedValue({ list: [], total: 0, page: 1, perPage: 20 });
    const api = createFileAudioApi({ request });

    await api.list(1, 20, {
      sender: '张伟',
      receiver: '陈经理',
      from: '2026-08-01',
      to: '2026-08-20',
    });

    expect(request).toHaveBeenCalledWith('/chat/media?page=1&perPage=20&sender=%E5%BC%A0%E4%BC%9F&receiver=%E9%99%88%E7%BB%8F%E7%90%86&from=2026-08-01&to=2026-08-20');
  });

  it('exposes a read-only list API without upload or delete methods', () => {
    const api = createFileAudioApi({ request: vi.fn() });

    expect(Object.keys(api)).toEqual(['list', 'content']);
  });

  it('downloads recording content through the authenticated client', async () => {
    const request = vi.fn();
    const blob = new Blob(['wav']);
    const download = vi.fn().mockResolvedValue({ blob, filename: 'recording.wav' });
    const api = createFileAudioApi({ request, download });

    if (api.content === undefined) throw new Error('content API is missing');
    await expect(api.content(7)).resolves.toBe(blob);
    expect(download).toHaveBeenCalledWith('/chat/media/7/content');
  });
});
