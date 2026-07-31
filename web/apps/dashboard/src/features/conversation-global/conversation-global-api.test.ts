import { describe, expect, it, vi } from 'vitest';

import { createConversationGlobalApi } from './conversation-global-api';

describe('createConversationGlobalApi', () => {
  it('serializes current-corp search filters and parses the explicit page contract', async () => {
    const response = {
      list: [{
        id: '9:1:31',
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
    const api = createConversationGlobalApi({ request }, () => '7');

    await expect(api.search({
      corpId: '7',
      keyword: '报价',
      employeeId: '9',
      customerId: '31',
      roomId: '',
      from: '2026-07-01',
      to: '2026-07-31',
      page: 2,
      pageSize: 20,
    })).resolves.toEqual(response);

    expect(request).toHaveBeenCalledWith(
      '/workMessage/toUsers?view=global&corpId=7&keyword=%E6%8A%A5%E4%BB%B7&employeeId=9&customerId=31&from=2026-07-01&to=2026-07-31&page=2&pageSize=20',
    );
  });

  it('loads detail with the active corp and rejects incompatible responses', async () => {
    const response = {
      id: '9:1:31',
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
    const api = createConversationGlobalApi({ request }, () => '7');

    await expect(api.detail('9:1:31')).resolves.toEqual(response);
    expect(request).toHaveBeenCalledWith(
      '/workMessage/detail?corpId=7&id=9%3A1%3A31',
    );

    request.mockResolvedValueOnce({ id: '9:1:31', messages: 'invalid' });
    await expect(api.detail('9:1:31')).rejects.toThrow('会话详情接口返回了无效数据');
  });

  it('requires current enterprise context for detail', async () => {
    const request = vi.fn();
    const api = createConversationGlobalApi({ request }, () => null);

    await expect(api.detail('9:1:31')).rejects.toThrow('请先选择企业');
    expect(request).not.toHaveBeenCalled();
  });

  it('uses the current enterprise after a corp switch instead of the previous search corp', async () => {
    let corpId = '7';
    const request = vi.fn<() => Promise<unknown>>()
      .mockResolvedValueOnce({ list: [], total: 0, page: 1, pageSize: 20 })
      .mockResolvedValueOnce({
        id: '9:1:31',
        employeeId: 9,
        employeeName: '张三',
        targetType: 'customer',
        targetId: 31,
        targetName: '星河科技',
        messageTotal: 0,
        truncated: false,
        window: 'latest',
        messages: [],
      });
    const api = createConversationGlobalApi({ request }, () => corpId);
    await api.search({
      corpId: '7',
      keyword: '',
      employeeId: '',
      customerId: '',
      roomId: '',
      from: '',
      to: '',
      page: 1,
      pageSize: 20,
    });

    corpId = '8';
    await api.detail('9:1:31');

    expect(request).toHaveBeenLastCalledWith('/workMessage/detail?corpId=8&id=9%3A1%3A31');
  });
});
