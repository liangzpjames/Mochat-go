import { describe, expect, it, vi } from 'vitest';
import { createContactTagApi } from './contact-tag-api';
describe('contact tag api', () => {
  it('uses all audited contracts', async () => {
    const request = vi.fn(() => Promise.resolve([])); const api = createContactTagApi({ request });
    await api.list({ groupId: 2, page: 3, perPage: 20 });
    expect(request).toHaveBeenLastCalledWith('/workContactTag/index?groupId=2&page=3&perPage=20');
    await api.groups(); expect(request).toHaveBeenLastCalledWith('/workContactTagGroup/index');
    await api.detail(8); expect(request).toHaveBeenLastCalledWith('/workContactTag/detail?tagId=8');
    await api.createTags(2, ['意向']); expect(request).toHaveBeenLastCalledWith('/workContactTag/store', expect.objectContaining({ method: 'POST', body: JSON.stringify({ groupId: 2, tagName: ['意向'] }) }));
    await api.updateTag({ tagId: 8, groupId: 2, tagName: '高意向', isUpdate: 1 });
    expect(request).toHaveBeenLastCalledWith('/workContactTag/update', expect.objectContaining({ method: 'PUT' }));
    await api.removeTags([8, 9]); expect(request).toHaveBeenLastCalledWith('/workContactTag/destroy', expect.objectContaining({ method: 'DELETE', body: JSON.stringify({ tagId: '8,9' }) }));
    await api.moveTags([8], 3); expect(request).toHaveBeenLastCalledWith('/workContactTag/move', expect.objectContaining({ method: 'PUT', body: JSON.stringify({ tagId: '8', groupId: 3 }) }));
    await api.sync(); expect(request).toHaveBeenLastCalledWith('/workContactTag/synContactTag', expect.objectContaining({ method: 'PUT' }));
    await api.createGroup('阶段'); expect(request).toHaveBeenLastCalledWith('/workContactTagGroup/store', expect.objectContaining({ method: 'POST' }));
    await api.updateGroup(2, '阶段二'); expect(request).toHaveBeenLastCalledWith('/workContactTagGroup/update', expect.objectContaining({ method: 'PUT' }));
    await api.removeGroup(2); expect(request).toHaveBeenLastCalledWith('/workContactTagGroup/destroy', expect.objectContaining({ method: 'DELETE' }));
  });
});
