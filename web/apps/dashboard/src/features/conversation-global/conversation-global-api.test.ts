import { describe, expect, it, vi } from 'vitest';

import { createConversationGlobalApi } from './conversation-global-api';

describe('createConversationGlobalApi', () => {
  it('serializes current-corp search filters and parses the explicit page contract', async () => {
    const response = {
      list: [{
        id: 'msg:archive-31',
        employeeId: 9,
        employeeName: '张三',
        employeeAvatar: '',
        targetType: 'customer',
        targetId: 31,
        targetName: '星河科技',
        targetAvatar: '',
        lastMessage: '请确认报价',
        sentAt: '2026-07-05 11:00:00',
      }],
      total: 1,
      page: 2,
      pageSize: 20,
    };
    const request = vi.fn<() => Promise<unknown>>(() => Promise.resolve(response));
    const api = createConversationGlobalApi({ request });

    await expect(api.search({
      keyword: '报价',
      conversationType: 'customer',
      employeeIds: ['9', '12'],
      startAt: '2026-07-01',
      endAt: '2026-07-31',
      page: 2,
      pageSize: 20,
    })).resolves.toEqual(response);

    expect(request).toHaveBeenCalledWith(
      '/workMessage/toUsers?view=global&keyword=%E6%8A%A5%E4%BB%B7&conversationType=customer&employeeIds=9&employeeIds=12&startAt=2026-07-01&endAt=2026-07-31&page=2&pageSize=20',
    );
  });

  it('loads detail by a real archive message id without client-owned scope', async () => {
    const response = {
      id: 'msg:archive-31',
      employeeId: 9,
      employeeName: '张三',
      targetType: 'customer',
      targetId: 31,
      targetName: '星河科技',
      messageTotal: 1,
      truncated: false,
      window: 'latest',
      messages: [{
        id: 'table:1:17',
        senderName: '张三',
        senderAvatar: '',
        direction: 'outbound',
        type: 1,
        content: { content: '你好' },
        sentAt: '2026-07-05 11:00:00',
      }],
    };
    const request = vi.fn<() => Promise<unknown>>(() => Promise.resolve(response));
    const api = createConversationGlobalApi({ request });

    await expect(api.detail('msg:archive-31')).resolves.toEqual(response);
    expect(request).toHaveBeenCalledWith(
      '/workMessage/detail?id=msg%3Aarchive-31',
    );

    request.mockResolvedValueOnce({ id: 'msg:archive-31', messages: 'invalid' });
    await expect(api.detail('msg:archive-31')).rejects.toThrow('会话详情接口返回了无效数据');
  });

  it('omits blank optional filters while preserving explicit pagination', async () => {
    const request = vi.fn<() => Promise<unknown>>(() => Promise.resolve({
      list: [], total: 0, page: 1, pageSize: 100,
    }));
    const api = createConversationGlobalApi({ request });

    await api.search({
      keyword: '',
      conversationType: '',
      employeeIds: [],
      startAt: '',
      endAt: '',
      page: 1,
      pageSize: 100,
    });

    expect(request).toHaveBeenCalledWith('/workMessage/toUsers?view=global&page=1&pageSize=100');
  });
});
