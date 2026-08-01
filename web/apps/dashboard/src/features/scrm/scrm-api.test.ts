import { describe, expect, it, vi } from 'vitest';
import { createScrmApi } from './scrm-api';

describe('ScrmApi', () => {
  it('serializes public pool filters and sends complete single and batch claim commands', async () => {
    const request = vi.fn().mockResolvedValue({ data: { items: [], nextCursor: '' } });
    const api = createScrmApi({ request });
    await api.listPublicPool({ corpId: 7, keyword: 'Ada', sources: ['wecom'], businessTypes: ['retail'], tagIds: ['tag-1'], regions: ['Shanghai'], reasons: ['expired'], previousOwnerIds: [18], cursor: '20', pageSize: 25 });
    await api.claimFromPublicPool({ corpId: 7, contactId: 'c1', userId: 42, version: 2, idempotencyKey: 'claim-1' });
    await api.batchClaimFromPublicPool({ corpId: 7, userId: 42, targets: [{ contactId: 'c1', version: 2, idempotencyKey: 'batch-1' }] });
    expect(request.mock.calls[0]?.[0]).toBe('/scrm/assignments?corpId=7&pageSize=25&keyword=Ada&source=wecom&businessType=retail&tagId=tag-1&region=Shanghai&reason=expired&previousOwnerId=18&cursor=20');
    expect(request.mock.calls[1]?.[0]).toBe('/scrm/assignments/claim');
    expect(request.mock.calls[1]?.[1]).toMatchObject({ method: 'POST', headers: expect.objectContaining({ 'Idempotency-Key': 'claim-1' }), body: JSON.stringify({ corpId: 7, contactId: 'c1', userId: 42, version: 2 }) });
    expect(request.mock.calls[2]?.[0]).toBe('/scrm/assignments/claim/batch');
    expect(request.mock.calls[2]?.[1]).toMatchObject({ method: 'POST', body: JSON.stringify({ corpId: 7, userId: 42, targets: [{ contactId: 'c1', version: 2, idempotencyKey: 'batch-1' }] }) });
  });

  it('persists the explicit public-pool action and reason', async () => {
    const request = vi.fn().mockResolvedValue({});
    const api = createScrmApi({ request });
    await api.releaseToPublicPool({ corpId: 7, contactId: 'c1', version: 4, action: 'reclaim', reason: '超时未跟进', idempotencyKey: 'reclaim-1' });
    expect(request).toHaveBeenCalledWith('/scrm/assignments/release', expect.objectContaining({
      method: 'POST', headers: expect.objectContaining({ 'Idempotency-Key': 'reclaim-1' }),
      body: JSON.stringify({ corpId: 7, contactId: 'c1', version: 4, action: 'reclaim', reason: '超时未跟进' }),
    }));
  });

  it('serializes opportunity filters and complete stage commands', async () => {
    const request = vi.fn().mockResolvedValue({ data: { items: [], nextCursor: '' } });
    const api = createScrmApi({ request });
    await api.listOpportunities({ corpId: 7, stage: 'proposal', status: 'open', ownerId: 9, cursor: 'o9', pageSize: 25 });
    await api.changeOpportunityStage({ corpId: 7, opportunityId: 'o1', stageId: 'lost', lostReason: '预算取消', version: 2, idempotencyKey: 'stage-1' });
    expect(request.mock.calls[0]?.[0]).toBe('/scrm/opportunities?corpId=7&pageSize=25&stage=proposal&status=open&ownerId=9&cursor=o9');
    expect(request.mock.calls[1]?.[0]).toBe('/scrm/opportunities/o1/stage');
    expect(request.mock.calls[1]?.[1]).toMatchObject({ method: 'POST', headers: expect.objectContaining({ 'Idempotency-Key': 'stage-1' }), body: JSON.stringify({ corpId: 7, stageId: 'lost', lostReason: '预算取消', version: 2 }) });
  });

  it('sends the minimal follow-up body and keeps contact identity in the path', async () => {
    const request = vi.fn().mockResolvedValue({});
    const api = createScrmApi({ request });
    await api.appendFollowUp({ corpId: 7, contactId: 'c/1', content: 'sent proposal', idempotencyKey: 'follow-1' });
    expect(request).toHaveBeenCalledWith('/scrm/contacts/c%2F1/follow-ups', expect.objectContaining({
      method: 'POST',
      headers: expect.objectContaining({ 'Idempotency-Key': 'follow-1' }),
      body: JSON.stringify({ corpId: 7, content: 'sent proposal' }),
    }));
  });
});
