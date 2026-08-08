import { describe, expect, it, vi } from 'vitest';

import { createLeadApi } from './lead-api';

describe('LeadApi', () => {
  it('loads owner options from the real employee list', async () => {
    const request = vi.fn().mockResolvedValue({ list: [{ id: 12, name: '销售小王' }] });
    await expect(createLeadApi({ request }).listOwnerOptions()).resolves.toEqual([{ id: 12, name: '销售小王' }]);
    expect(request).toHaveBeenCalledWith('/workEmployee/index?page=1&perPage=200');
  });

  it('uses the formal SCRM leads endpoint', async () => {
    const request = vi.fn().mockResolvedValue({ items: [{ id: 'lead-1', name: '客户甲', status: 'new' }], nextCursor: '' });
    const api = createLeadApi({ request });

    await expect(api.list({ corpId: 7, pageSize: 20 })).resolves.toEqual({ items: [{ id: 'lead-1', name: '客户甲', status: 'new' }], nextCursor: '' });
    expect(request).toHaveBeenCalledWith('/scrm/leads?corpId=7&pageSize=20');
  });

  it('creates a lead with an explicit business key and source', async () => {
    const request = vi.fn().mockResolvedValue({ id: 'lead-1', name: '客户甲', status: 'new' });
    const api = createLeadApi({ request });

    await api.create({ corpId: 7, businessKey: 'wx:customer-1', name: '客户甲', phone: '13800000000', source: 'wecom' });

    expect(request).toHaveBeenCalledWith('/scrm/leads', expect.objectContaining({ method: 'POST' }));
  });

  it('serializes all combined filters without dropping repeated values', async () => {
    const request = vi.fn().mockResolvedValue({ items: [], nextCursor: '' });
    const api = createLeadApi({ request });
    await api.list({ corpId: 7, keyword: '客户 甲', statuses: ['new', 'qualified'], sources: ['manual'], ownerIds: [12, 13], createdFrom: '2026-08-01T00:00:00Z', createdTo: '2026-08-02T00:00:00Z', cursor: 'next', pageSize: 30 });
    expect(request).toHaveBeenCalledWith('/scrm/leads?corpId=7&keyword=%E5%AE%A2%E6%88%B7+%E7%94%B2&status=new&status=qualified&source=manual&ownerId=12&ownerId=13&createdFrom=2026-08-01T00%3A00%3A00Z&createdTo=2026-08-02T00%3A00%3A00Z&cursor=next&pageSize=30');
  });

  it('supports duplicate detection, partial batch assignment, and transitions', async () => {
    const request = vi.fn().mockResolvedValue({ items: [] }); const api = createLeadApi({ request });
    await api.findDuplicates({ corpId: 7, phone: '13800000000', businessKey: 'wx:1' });
    await api.assign({ corpId: 7, ownerId: 12, targets: [{ id: 'a', version: 1 }, { id: 'b', version: 2 }] });
    await api.transition({ corpId: 7, id: 'a', toStatus: 'qualified', version: 1, discardReason: '' });
    expect(request.mock.calls[0]?.[0]).toContain('/scrm/leads/duplicates?corpId=7');
    expect(request.mock.calls[1]?.[0]).toBe('/scrm/leads/assignments');
    expect(request.mock.calls[2]?.[0]).toBe('/scrm/leads/transition');
  });
});
