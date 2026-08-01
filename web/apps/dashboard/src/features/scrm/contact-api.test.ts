import { describe, expect, it, vi } from 'vitest';
import { createContactApi } from './contact-api';

describe('contact api', () => {
  it('serializes combined filters and detail scope', async () => {
    const request = vi.fn().mockResolvedValue({ items: [], nextCursor: '' });
    const api = createContactApi({ request });
    await api.listContacts({ corpId: 7, keyword: ' Ada ', ownerIds: [11], tagIds: ['vip'], statuses: ['owned'], cursor: '20', pageSize: 30 });
    expect(request.mock.calls[0]![0]).toBe('/scrm/contacts?corpId=7&pageSize=30&keyword=Ada&ownerId=11&tagId=vip&status=owned&cursor=20');
    await api.getContact({ corpId: 7, contactId: 'c/1' });
    expect(request.mock.calls[1]![0]).toBe('/scrm/contacts/c%2F1?corpId=7');
  });

  it('sends versioned idempotent lifecycle mutations', async () => {
    const request = vi.fn().mockResolvedValue({});
    const api = createContactApi({ request });
    await api.releaseToPublicPool({ corpId: 7, contactId: 'c1', version: 3, idempotencyKey: 'release-c1-3' });
    expect(request).toHaveBeenCalledWith('/scrm/assignments/release', expect.objectContaining({ method: 'POST', headers: expect.objectContaining({ 'Idempotency-Key': 'release-c1-3' }) }));
  });
});
