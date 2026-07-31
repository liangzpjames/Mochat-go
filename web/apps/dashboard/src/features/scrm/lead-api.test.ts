import { describe, expect, it, vi } from 'vitest';

import { createLeadApi } from './lead-api';

describe('LeadApi', () => {
  it('uses the formal SCRM leads endpoint', async () => {
    const request = vi.fn().mockResolvedValue({ items: [{ id: 'lead-1', name: '客户甲', status: 'new' }], nextCursor: '' });
    const api = createLeadApi({ request });

    await expect(api.list({ pageSize: 20 })).resolves.toEqual({ items: [{ id: 'lead-1', name: '客户甲', status: 'new' }], nextCursor: '' });
    expect(request).toHaveBeenCalledWith('/dashboard/scrm/leads?pageSize=20');
  });

  it('creates a lead with an explicit business key and source', async () => {
    const request = vi.fn().mockResolvedValue({ id: 'lead-1', name: '客户甲', status: 'new' });
    const api = createLeadApi({ request });

    await api.create({ businessKey: 'wx:customer-1', name: '客户甲', source: 'wecom' });

    expect(request).toHaveBeenCalledWith('/dashboard/scrm/leads', expect.objectContaining({ method: 'POST' }));
  });
});
