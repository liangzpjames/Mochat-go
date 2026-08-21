import { describe, expect, it, vi } from 'vitest';

import { createMessageInterceptApi } from './message-intercept-api';

describe('message intercept api', () => {
  it('sends explicit filters and normalizes paginated records', async () => {
    const read = vi.fn().mockResolvedValue({
      data: {
        items: [{ id: 9, ruleName: '报价拦截', decision: 'blocked', auditStatus: 'pending', occurredAt: '2026-08-21T10:00:00+08:00' }],
        total: 41,
        page: 2,
        perPage: 20,
      },
    });
    const api = createMessageInterceptApi({ read, write: vi.fn() });

    const result = await api.records({ keyword: '转账', page: 2, perPage: 20 });

    expect(read).toHaveBeenCalledWith('/message-intercept/records', { keyword: '转账', page: 2, perPage: 20 });
    expect(result).toEqual({
      items: [expect.objectContaining({ id: 9, ruleName: '报价拦截', decision: 'blocked', auditStatus: 'pending' })],
      total: 41,
      page: 2,
      perPage: 20,
    });
  });

  it('normalizes legacy reviewed audit status without exposing raw fields', async () => {
    const read = vi.fn().mockResolvedValue({ items: [{ id: 4, auditStatus: 'reviewed', senderName: '罗敏' }] });
    const api = createMessageInterceptApi({ read, write: vi.fn() });

    const result = await api.records({ keyword: '', page: 1, perPage: 20 });

    expect(result.items[0]).toMatchObject({ auditStatus: 'reviewed', senderName: '罗敏' });
    expect(result.items[0]).not.toHaveProperty('raw');
  });
});
