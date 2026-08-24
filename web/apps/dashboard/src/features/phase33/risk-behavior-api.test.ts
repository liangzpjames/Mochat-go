import { describe, expect, it, vi } from 'vitest';

import { createRiskBehaviorApi } from './risk-behavior-api';

describe('RiskBehaviorApi', () => {
  it('serializes explicit record filters and preserves real summary', async () => {
    const request = vi.fn().mockResolvedValue({
      items: [{ id: 9, behavior: 'private_transaction', riskLevel: 'high', auditStatus: 'pending' }],
      total: 1,
      page: 2,
      perPage: 20,
      summary: { total: 1, pending: 1, highRisk: 1, processed: 0 },
    });
    const api = createRiskBehaviorApi({ request });

    await expect(api.records({
      riskLevel: 'high', behavior: 'private_transaction', auditStatus: 'pending', conversationType: 'customer',
      employeeIds: [3, 5], occurredFrom: '2026-08-01 00:00:00', occurredTo: '2026-08-21 23:59:59', page: 2,
    })).resolves.toEqual(expect.objectContaining({ summary: { total: 1, pending: 1, highRisk: 1, processed: 0 }, perPage: 20 }));
    expect(request).toHaveBeenCalledWith('/risk/records?riskLevel=high&behavior=private_transaction&auditStatus=pending&conversationType=customer&employeeIds=3&employeeIds=5&occurredFrom=2026-08-01+00%3A00%3A00&occurredTo=2026-08-21+23%3A59%3A59&page=2&perPage=20');
  });

  it('does not invent a zero summary when the backend omits it', async () => {
    const api = createRiskBehaviorApi({ request: vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, perPage: 20 }) });
    await expect(api.records({ page: 1 })).resolves.toEqual(expect.objectContaining({ summary: null, perPage: 20 }));
  });

  it('parses a detail response with conversation availability', async () => {
    const request = vi.fn().mockResolvedValue({ record: { id: 9 }, audits: [], conversationAvailable: true });
    const api = createRiskBehaviorApi({ request });
    const detail = await api.recordDetail(9);
    expect(detail.record.id).toBe(9);
    expect(detail.audits).toEqual([]);
    expect(detail.conversationAvailable).toBe(true);
    expect(request).toHaveBeenCalledWith('/risk/records/detail?id=9');
  });

  it('normalizes the real related-user array into displayable names', async () => {
    const api = createRiskBehaviorApi({ request: vi.fn().mockResolvedValue({
      items: [{ id: 1, relatedUser: [{ userId: 1006, userName: '赵磊', role: 'employee' }, { userId: 2010, userName: '罗敏', role: 'customer' }] }], total: 1, page: 1,
    }) });
    const result = await api.records({ page: 1 });
    expect(result.items[0]?.relatedUser).toEqual({ employeeId: 1006, employeeName: '赵磊', customerId: 2010, customerName: '罗敏' });
  });
});
