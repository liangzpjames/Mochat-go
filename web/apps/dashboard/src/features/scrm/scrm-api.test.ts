/* eslint-disable @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-return -- request mocks intentionally expose raw protocol payloads */
import { describe, expect, it, vi } from 'vitest';
import { createScrmApi } from './scrm-api';

describe('ScrmApi', () => {
  it('loads business selectors from contacts, employees and enabled funnel settings', async () => {
    const request = vi.fn()
      .mockResolvedValueOnce({ items: [{ id: 'c1', name: '客户甲', version: 2, assignmentVersion: 7 }] })
      .mockResolvedValueOnce({ list: [{ id: 12, name: '销售小王' }] })
      .mockResolvedValueOnce([{ type: 'funnel_stage', key: 'proposal', value: '方案确认', enabled: true }, { type: 'funnel_stage', key: 'disabled', value: '停用', enabled: false }]);
    const api = createScrmApi({ request });
    await expect(api.listContactOptions!({ corpId: 7 })).resolves.toEqual([{ id: 'c1', name: '客户甲', version: 7 }]);
    await expect(api.listEmployeeOptions!()).resolves.toEqual([{ id: '12', name: '销售小王' }]);
    await expect(api.listStageOptions!({ corpId: 7 })).resolves.toEqual([{ id: 'proposal', name: '方案确认' }]);
    expect(request.mock.calls.map(([path]) => path)).toEqual(['/scrm/contacts?corpId=7&pageSize=100', '/workEmployee/index?page=1&perPage=200', '/scrm/settings?corpId=7']);
  });

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

  it('serializes the customer-tag catalog and all versioned mutations', async () => {
    const request = vi.fn().mockResolvedValue({});
    const api = createScrmApi({ request });
    await api.listTagCatalog!({ corpId: 7, groupId: 'g/1', keyword: ' VIP ' });
    await api.createTagGroup!({ corpId: 7, name: '客户等级', idempotencyKey: 'group-create' });
    await api.renameTagGroup!({ corpId: 7, groupId: 'g/1', name: '客户分层', version: 1, idempotencyKey: 'group-rename' });
    await api.createTag({ corpId: 7, groupId: 'g/1', name: 'VIP', idempotencyKey: 'tag-create' });
    await api.moveTag!({ corpId: 7, tagId: 't/1', groupId: 'g2', version: 2, idempotencyKey: 'tag-move' });
    await api.maintainTagContacts!({ corpId: 7, tagId: 't/1', addContactIds: ['c1'], removeContactIds: ['c2'], version: 3, idempotencyKey: 'tag-contacts' });
    await api.previewDeleteTag!({ corpId: 7, tagId: 't/1' });
    await api.deleteTag!({ corpId: 7, tagId: 't/1', version: 4, idempotencyKey: 'tag-delete' });
    expect(request.mock.calls[0]?.[0]).toBe('/scrm/tags?corpId=7&groupId=g%2F1&keyword=VIP');
    expect(request.mock.calls[1]).toEqual(['/scrm/tag-groups', expect.objectContaining({ headers: expect.objectContaining({ 'Idempotency-Key': 'group-create' }) })]);
    expect(request.mock.calls[2]?.[0]).toBe('/scrm/tag-groups/g%2F1');
    expect(request.mock.calls[3]?.[1]).toMatchObject({ body: JSON.stringify({ corpId: 7, groupId: 'g/1', name: 'VIP' }) });
    expect(request.mock.calls[4]?.[0]).toBe('/scrm/tags/t%2F1/move');
    expect(request.mock.calls[5]?.[1]).toMatchObject({ method: 'PUT', body: JSON.stringify({ corpId: 7, addContactIds: ['c1'], removeContactIds: ['c2'], version: 3 }) });
    expect(request.mock.calls[6]?.[0]).toBe('/scrm/tags/t%2F1/delete-preview?corpId=7');
    expect(request.mock.calls[7]?.[1]).toMatchObject({ method: 'DELETE', body: JSON.stringify({ corpId: 7, version: 4 }) });
  });
});
