import { describe, expect, it, vi } from 'vitest';

import {
  loadBatchAdd,
  loadContactSop,
  loadMediumGroups,
  loadMediums,
  loadRoomSop,
  refreshMediumMediaId,
  updateRoomSopState,
} from './business-api';

describe('Sidebar persisted business APIs', () => {
  it('validates and normalizes medium groups and paged media', async () => {
    const request = vi.fn()
      .mockResolvedValueOnce([{ id: 0, name: '未分组' }, { id: 3, name: '活动素材' }])
      .mockResolvedValueOnce({
        page: { perPage: 20, total: 1, totalPage: 1 },
        list: [{ id: 8, type: '图片', mediaId: 'media-8', content: { imageFullPath: '/static/a.png' } }],
      });

    await expect(loadMediumGroups(request)).resolves.toHaveLength(2);
    await expect(loadMediums(request, { groupId: 3, keyword: '海报', page: 1 })).resolves.toMatchObject({
      total: 1,
      items: [{ id: 8, kind: 'image', mediaId: 'media-8' }],
    });
    expect(request).toHaveBeenNthCalledWith(2, '/medium/index?mediumGroupId=3&searchStr=%E6%B5%B7%E6%8A%A5&page=1&perPage=20', { method: 'GET' });
  });

  it('refreshes a media id through the persisted endpoint', async () => {
    const request = vi.fn().mockResolvedValue({ mediaId: 'fresh-id' });
    await expect(refreshMediumMediaId(request, 8)).resolves.toBe('fresh-id');
    expect(request).toHaveBeenCalledWith('/medium/mediaIdUpdate?mediumId=8', { method: 'GET' });
  });

  it('loads customer and room SOP payloads without inventing missing tasks', async () => {
    const request = vi.fn()
      .mockResolvedValueOnce({
        id: 4, contactSopId: 12, creator: '员工甲', time: '09:00', tipTime: '2026-08-22 09:00',
        task: { content: [{ type: 0, value: '今日回访' }] },
        contact: { id: 11, name: '林晓', avatar: null, wxExternalUserid: 'ext-1', updatedAt: '2026-08-22' },
      })
      .mockResolvedValueOnce({
        id: 5, roomSopId: 13, creator: '员工乙', time: '10:00', state: 0,
        task: { content: [] }, room: { id: 21, name: '客户群', wxChatId: 'chat-1', createTime: '2026-08-20' },
      });

    await expect(loadContactSop(request, 4)).resolves.toMatchObject({ taskId: 4, customerName: '林晓' });
    await expect(loadRoomSop(request, 5)).resolves.toMatchObject({ taskId: 5, state: 0, roomName: '客户群' });
  });

  it('persists room task completion and validates batch-add status filtering', async () => {
    const request = vi.fn()
      .mockResolvedValueOnce({})
      .mockResolvedValueOnce({ employeeName: '员工甲', list: [{ id: 1, phone: '13800000000', status: '已添加' }] });

    await updateRoomSopState(request, 5);
    await expect(loadBatchAdd(request, 9, 3)).resolves.toEqual({
      employeeName: '员工甲', contacts: [{ id: 1, phone: '13800000000', status: '已添加' }],
    });
    expect(request).toHaveBeenNthCalledWith(1, '/roomSop/logState', expect.objectContaining({
      method: 'PUT', body: JSON.stringify({ id: 5 }),
    }));
    expect(request).toHaveBeenNthCalledWith(2, '/contactBatchAdd/detail?batchId=9&status=3', { method: 'GET' });
  });

  it('rejects malformed payloads and invalid identifiers honestly', async () => {
    const request = vi.fn().mockResolvedValue({ list: 'fake' });
    await expect(loadMediums(request, { groupId: null, keyword: '', page: 1 })).rejects.toMatchObject({ kind: 'validation' });
    await expect(loadBatchAdd(request, 0, 4)).rejects.toMatchObject({ kind: 'validation' });
  });
});
