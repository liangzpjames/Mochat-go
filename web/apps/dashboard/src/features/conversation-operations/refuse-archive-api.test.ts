import { describe, expect, it, vi } from 'vitest';

import { createRefuseArchiveApi } from './refuse-archive-api';

describe('createRefuseArchiveApi', () => {
  it('serializes subject, employee, date and status filters', async () => {
    const request = vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, perPage: 20 });
    const api = createRefuseArchiveApi({ request });
    await api.list({ subjectType: 'room', subject: '群', employeeId: 9, refusedFrom: '2026-08-01', refusedTo: '2026-08-20', authorizationStatus: 'refused', followUpStatus: 'unfollowed', page: 2, perPage: 20 });
    expect(request).toHaveBeenCalledWith('/refuse-archive/records?subjectType=room&subject=%E7%BE%A4&employeeId=9&refusedFrom=2026-08-01&refusedTo=2026-08-20&authorizationStatus=refused&followUpStatus=unfollowed&page=2&perPage=20');
  });

  it('posts follow-up updates as JSON', async () => {
    const request = vi.fn().mockResolvedValue({ updated: true });
    const api = createRefuseArchiveApi({ request });
    await api.followUp({ id: 3, status: 'contacted', note: '已电话沟通' });
    expect(request).toHaveBeenCalledWith('/refuse-archive/follow-up', expect.objectContaining({ method: 'POST', body: JSON.stringify({ id: 3, status: 'contacted', note: '已电话沟通' }) }));
  });
});
