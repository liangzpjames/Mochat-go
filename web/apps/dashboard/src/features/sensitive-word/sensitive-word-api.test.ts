import { describe, expect, it, vi } from 'vitest';

import { createSensitiveWordApi } from './sensitive-word-api';

describe('SensitiveWordApi', () => {
  it('serializes list filters and normalizes the page response', async () => {
    const request = vi.fn().mockResolvedValue({
      page: { perPage: 10, total: 1, totalPage: 1 },
      list: [{ sensitiveWordId: 11, groupId: 2, groupName: '默认', name: '报价', status: 1 }],
    });
    const api = createSensitiveWordApi({ request });

    await expect(api.list({ groupId: 2, keywords: '报价', page: 2, perPage: 10 })).resolves.toEqual({
      items: [{ id: 11, groupId: 2, groupName: '默认', name: '报价', status: 1 }],
      total: 1,
      page: 2,
      perPage: 10,
    });
    expect(request).toHaveBeenCalledWith('/sensitiveWord/index?groupId=2&keyWords=%E6%8A%A5%E4%BB%B7&page=2&perPage=10');
  });

  it('sends stable mutation payloads', async () => {
    const request = vi.fn().mockResolvedValue(undefined);
    const api = createSensitiveWordApi({ request });

    await api.create({ groupId: 2, names: ['报价', '合同'], idempotencyKey: 'sensitive-1' });

    expect(request).toHaveBeenCalledWith('/sensitiveWord/store', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ groupId: 2, name: '报价,合同', idempotencyKey: 'sensitive-1' }),
    }));
  });
});
