import { describe, expect, it, vi } from 'vitest';
import { createScrmApi } from './scrm-api';

describe('ScrmApi', () => {
  it('uses public pool and atomic assignment endpoints', async () => {
    const request = vi.fn().mockResolvedValue({ data: { items: [], nextCursor: '' } });
    const api = createScrmApi({ request });
    await api.listPublicPool({ corpId: 7 });
    await api.claimFromPublicPool({ corpId: 7, contactId: 'c1', version: 2, idempotencyKey: 'claim-1' });
    expect(request.mock.calls[0]?.[0]).toBe('/dashboard/scrm/assignments?corpId=7&pageSize=20');
    expect(request.mock.calls[1]?.[0]).toBe('/dashboard/scrm/assignments/claim');
    expect(request.mock.calls[1]?.[1]).toMatchObject({ method: 'POST', headers: expect.objectContaining({ 'Idempotency-Key': 'claim-1' }) });
  });
});
