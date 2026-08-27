import { describe, expect, it, vi } from 'vitest';

import { createConversationGlobalApi } from './conversation-global-api';

describe('createConversationGlobalApi', () => {
  it('使用当前鉴权企业的会话同步合同', async () => {
    const response = { status: 'queued', available: true, fetched: 0, processed: 0, skipped: 0, failed: 0 };
    const request = vi.fn<() => Promise<unknown>>().mockResolvedValue(response);
    const api = createConversationGlobalApi({ request });

    await expect(api.getArchiveSyncStatus!()).resolves.toEqual(response);
    await expect(api.startArchiveSync!({ requestId: 'archive-sync-1' })).resolves.toEqual(response);
    expect(request).toHaveBeenNthCalledWith(1, '/company/archive-sync-status');
    expect(request).toHaveBeenNthCalledWith(2, '/company/archive-sync', expect.objectContaining({ method: 'POST', body: '{"requestId":"archive-sync-1"}' }));
  });
  it('loads the real group conversation workspace contracts with fixed page sizes', async () => {
    const room = {
      id: 71, externalId: 'wr_71', name: '星河客户群', avatar: '', ownerId: 9, ownerName: '张三',
      memberCount: 6, employeeCount: 2, customerCount: 4, messageCount: 18, lastMessage: '请确认排期',
      lastMessageAt: '2026-08-19 11:00:00', focused: false, riskCount: 1, timeoutCount: 0, dissolved: false,
    };
    const profile = {
      id: 71, externalId: 'wr_71', name: '星河客户群', avatar: '', ownerId: 9, ownerName: '张三',
      memberCount: 6, employeeCount: 2, customerCount: 4, createdAt: '2026-08-01 10:00:00', status: 'active',
      dissolved: false, focused: false, riskCount: 1, timeoutCount: 0, capabilities: [], limitations: [],
    };
    const messages = {
      roomId: 71,
      stats: { messageTotal: 18, employeeTotal: 8, customerTotal: 10, riskTotal: 1, timeoutTotal: 0 },
      messages: [{ id: 'msg:room-1', senderId: 9, senderName: '张三', senderAvatar: '', senderKind: 'employee', direction: 'outbound', sentAt: '2026-08-19 11:00:00', archiveSource: 'external', archiveSourceId: 'wecom', type: 1, content: { text: '请确认排期' } }],
      nextBefore: 'cursor-1', hasMore: true, capabilities: [],
    };
    const members = { items: [{ id: 9, externalId: 'zhangsan', kind: 'employee', name: '张三', avatar: '', joinedAt: '2026-08-01', leftAt: '', status: 'active', employeeId: 9, customerId: 0 }], total: 1, page: 2, pageSize: 50, capabilities: [] };
    const options = { employees: [{ value: '9', label: '张三', count: 4 }], customers: [], groups: [], capabilities: [] };
    const request = vi.fn<() => Promise<unknown>>()
      .mockResolvedValueOnce({ items: [room], total: 1, page: 1, pageSize: 50, capabilities: [], limitations: [] })
      .mockResolvedValueOnce(profile)
      .mockResolvedValueOnce(messages)
      .mockResolvedValueOnce(members)
      .mockResolvedValueOnce(options);
    const api = createConversationGlobalApi({ request });

    await expect(api.groupRoomDirectory!({ mode: 'active', keyword: '星河', page: 1, pageSize: 50 })).resolves.toMatchObject({ items: [room], pageSize: 50 });
    await expect(api.groupRoomProfile!(71)).resolves.toEqual(profile);
    await expect(api.groupRoomMessages!({ roomId: 71, keyword: '排期', date: '2026-08-19', messageTypes: ['1'], before: 'cursor-1', pageSize: 50 })).resolves.toEqual(messages);
    await expect(api.groupRoomMembers!({ roomId: 71, mode: 'employee', keyword: '张', page: 2, pageSize: 50 })).resolves.toEqual(members);
    await expect(api.groupRoomFilterOptions!('employee')).resolves.toEqual(options);

    expect(request).toHaveBeenNthCalledWith(1, '/workMessage/roomDirectory?roomMode=active&keyword=%E6%98%9F%E6%B2%B3&page=1&pageSize=50');
    expect(request).toHaveBeenNthCalledWith(2, '/workMessage/roomProfile?roomId=71');
    expect(request).toHaveBeenNthCalledWith(3, '/workMessage/roomMessages?roomId=71&keyword=%E6%8E%92%E6%9C%9F&date=2026-08-19&messageTypes=1&before=cursor-1&pageSize=50');
    expect(request).toHaveBeenNthCalledWith(4, '/workMessage/roomMembers?roomId=71&mode=employee&keyword=%E5%BC%A0&page=2&pageSize=50');
    expect(request).toHaveBeenNthCalledWith(5, '/workMessage/roomFilterOptions?kind=employee');
  });

  it('rejects group room payloads that hide missing business data behind partial values', async () => {
    const request = vi.fn<() => Promise<unknown>>().mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 20, capabilities: [], limitations: [] });
    const api = createConversationGlobalApi({ request });
    await expect(api.groupRoomDirectory!({ mode: 'active', keyword: '', page: 1, pageSize: 50 })).rejects.toThrow('群聊目录接口返回了无效数据');
  });
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

  it('loads employees for the staff conversation sidebar', async () => {
    const request = vi.fn<() => Promise<unknown>>(() => Promise.resolve([
      { id: 9, name: '张三', avatar: '' },
    ]));
    const api = createConversationGlobalApi({ request });

    await expect(api.employees!({ keyword: '张' })).resolves.toEqual([
      { id: 9, name: '张三', avatar: '' },
    ]);
    expect(request).toHaveBeenCalledWith('/workMessage/fromUsers?page=1&perPage=100&name=%E5%BC%A0');
  });

  it('loads overview metrics and serializes status filters', async () => {
    const request = vi.fn<() => Promise<unknown>>(() => Promise.resolve({
      metrics: {
        customerConversations: { value: 12, changeRate: null, status: 'available' },
        timeoutConversations: { value: null, changeRate: null, status: 'unavailable', reason: '未接入' },
      },
      capabilities: [{ key: 'aiSummary', available: false, reason: '未接入' }],
    }));
    const api = createConversationGlobalApi({ request });

    await expect(api.overview!({
      keyword: '', conversationType: '', employeeIds: ['9'], startAt: '2026-08-01', endAt: '2026-08-19',
      bucket: 'risk', messageTypes: ['text', 'image'],
    })).resolves.toMatchObject({ metrics: { customerConversations: { value: 12 } } });
    expect(request).toHaveBeenCalledWith('/workMessage/globalOverview?view=global&employeeIds=9&startAt=2026-08-01&endAt=2026-08-19&bucket=risk&messageTypes=text&messageTypes=image');
  });

  it('writes personal focus changes without putting scope in the client body', async () => {
    const request = vi.fn<() => Promise<unknown>>(() => Promise.resolve({}));
    const api = createConversationGlobalApi({ request });

    await api.setFocus!('9:1:31');
    await api.removeFocus!('9:1:31');

    expect(request).toHaveBeenNthCalledWith(1, '/workMessage/focus', expect.objectContaining({ method: 'PUT', body: '{"conversationId":"9:1:31"}' }));
    expect(request).toHaveBeenNthCalledWith(2, '/workMessage/focus', expect.objectContaining({ method: 'DELETE', body: '{"conversationId":"9:1:31"}' }));
  });

  it('loads the scoped employee directory with fixed pagination', async () => {
    const response = {
      departments: [{ id: 10, parentId: 0, name: '销售部', employeeCount: 1, children: [] }],
      employees: [{
        id: 9, name: '张三', avatar: '', status: 1, departmentIds: [10], archived: true,
        conversationCount: 3, focusedConversationCount: 1, lastConversationAt: '2026-08-16 10:00:00',
      }],
      counts: { all: 1, focused: 1, archived: 1, departed: 0 },
      page: 2, pageSize: 50, total: 51, limitations: [],
      capabilities: [{ key: 'internalGroup', available: false, reason: '当前归档数据未提供内部群聊能力' }],
    };
    const request = vi.fn<() => Promise<unknown>>(() => Promise.resolve(response));
    const api = createConversationGlobalApi({ request });

    await expect(api.staffDirectory!({ mode: 'focused', keyword: '张', departmentId: 10, page: 2, pageSize: 50 })).resolves.toEqual(response);
    expect(request).toHaveBeenCalledWith('/workMessage/staffDirectory?mode=focused&keyword=%E5%BC%A0&departmentId=10&page=2&pageSize=50');

    request.mockResolvedValueOnce({ ...response, counts: undefined });
    await expect(api.staffDirectory!({ mode: 'all', keyword: '', departmentId: null, page: 1, pageSize: 50 })).rejects.toThrow('员工目录接口返回了无效数据');
  });

  it('loads a strict Shanghai day trajectory contract', async () => {
    const response = {
      employee: { id: 9, name: '张三', avatar: '' }, date: '2026-08-19', timezone: 'Asia/Shanghai',
      metrics: { internalSingle: { status: 'available', subjectTotal: 1, messageTotal: 2 }, internalGroup: { status: 'unavailable', subjectTotal: null, messageTotal: null, reason: '未接入' } },
      events: [{ id: '9:2:3001@2026-08-19T23', conversationId: '9:2:3001', hour: '23', targetType: 'room', targetId: 3001, targetName: '客户群', targetAvatar: '', targetStatus: 'available', messageTotal: 1, firstMessageAt: '2026-08-19 23:08:00', lastMessageAt: '2026-08-19 23:08:00' }],
      unmatchedTargetMessages: 0, limitations: [], capabilities: [],
    };
    const request = vi.fn<() => Promise<unknown>>(() => Promise.resolve(response));
    const api = createConversationGlobalApi({ request });
    await expect(api.trajectoryDay!({ employeeId: 9, date: '2026-08-19', conversationType: 'room' })).resolves.toEqual(response);
    expect(request).toHaveBeenCalledWith('/workMessage/trajectoryDay?employeeId=9&date=2026-08-19&conversationType=room');
    request.mockResolvedValueOnce({ ...response, timezone: 'UTC' });
    await expect(api.trajectoryDay!({ employeeId: 9, date: '2026-08-19', conversationType: 'all' })).rejects.toThrow('会话轨迹接口返回了无效数据');
  });

  it('loads filtered staff detail by stable conversation id', async () => {
    const response = {
      conversationId: '9:1:31', employeeId: 9, employeeName: '张三', targetType: 'customer', targetId: 31, targetName: '星河科技', focused: true,
      stats: { communicationDays: 2, messageTotal: 15, inboundTotal: 5, outboundTotal: 10 },
      messages: [{ id: 'table:1:17', senderName: '张三', senderAvatar: '', direction: 'outbound', type: 1, content: { text: '你好' }, sentAt: '2026-08-16 10:00:00', archiveSource: 'external', archiveSourceId: 'wecom' }],
      nextBefore: 'opaque-cursor', hasMore: true,
      capabilities: [{ key: 'internalGroup', available: false, reason: '当前归档数据未提供内部群聊能力' }],
    };
    const request = vi.fn<() => Promise<unknown>>(() => Promise.resolve(response));
    const api = createConversationGlobalApi({ request });

    await expect(api.staffDetail!({ conversationId: '9:1:31', keyword: '报价', messageTypes: ['image', 'file'], date: '2026-08-16', pageSize: 50 })).resolves.toEqual(response);
    expect(request).toHaveBeenCalledWith('/workMessage/staffDetail?conversationId=9%3A1%3A31&keyword=%E6%8A%A5%E4%BB%B7&messageTypes=image&messageTypes=file&date=2026-08-16&pageSize=50');

    request.mockResolvedValueOnce({ ...response, targetType: 'internalGroup' });
    await expect(api.staffDetail!({ conversationId: '9:1:31', keyword: '', messageTypes: [], date: '', pageSize: 50 })).rejects.toThrow('员工会话详情接口返回了无效数据');
  });

  it('requests canonical customer workspace endpoints', async () => {
    const client = { request: vi.fn<() => Promise<unknown>>() };
    client.request
      .mockResolvedValueOnce({ customers: [], counts: { all: 0, focused: 0, active: 0, lost: 0 }, page: 1, pageSize: 50, total: 0, limitations: [], capabilities: [] })
      .mockResolvedValueOnce({ customer: { id: 31, name: '陈晓明', avatar: '', profileStatus: 'available' }, mode: 'direct', list: [], total: 0, page: 1, pageSize: 20, capabilities: [] })
      .mockResolvedValueOnce({ conversationId: '9:1:31', customerId: 31, customerName: '陈晓明', profile: { id: 31, name: '陈晓明', avatar: '', profileStatus: 'available' }, employeeId: 9, employeeName: '张伟', targetType: 'customer', targetId: 31, targetName: '陈晓明', focused: false, stats: { communicationDays: 1, messageTotal: 1, inboundTotal: 1, outboundTotal: 0 }, messages: [], nextBefore: '', hasMore: false, capabilities: [] });
    const api = createConversationGlobalApi(client);
    await api.customerDirectory?.({ mode: 'focused', keyword: '陈', page: 2, pageSize: 50 });
    await api.customerConversations?.({ customerId: 31, mode: 'group', page: 3, pageSize: 20 });
    await api.customerDetail?.({ customerId: 31, conversationId: '9:1:31', keyword: '报价', messageTypes: ['image', 'file'], date: '2026-08-16', pageSize: 50 });
    expect(client.request).toHaveBeenNthCalledWith(1, '/workMessage/customerDirectory?mode=focused&keyword=%E9%99%88&page=2&pageSize=50');
    expect(client.request).toHaveBeenNthCalledWith(2, '/workMessage/customerConversations?customerId=31&mode=group&page=3&pageSize=20');
    expect(client.request).toHaveBeenNthCalledWith(3, '/workMessage/customerDetail?customerId=31&conversationId=9%3A1%3A31&keyword=%E6%8A%A5%E4%BB%B7&messageTypes=image&messageTypes=file&date=2026-08-16&pageSize=50');
  });

  it('rejects partial directory data', async () => {
    const client = { request: vi.fn<() => Promise<unknown>>() };
    client.request.mockResolvedValue({ customers: [], counts: { all: 0 }, page: 1, pageSize: 50, total: 0, limitations: [], capabilities: [] });
    const api = createConversationGlobalApi(client);
    await expect(api.customerDirectory?.({ mode: 'all', keyword: '', page: 1, pageSize: 50 })).rejects.toThrow('客户目录接口返回了无效数据');
  });

  it('rejects customer conversation rows without a non-blank stable conversation id', async () => {
    const client = { request: vi.fn<() => Promise<unknown>>() };
    client.request.mockResolvedValue({
      customer: { id: 31, name: '陈晓明', avatar: '', profileStatus: 'available' }, mode: 'direct',
      list: [{ id: 'row-1', employeeId: 9, employeeName: '张伟', employeeAvatar: '', targetType: 'customer', targetId: 31, targetName: '陈晓明', targetAvatar: '', lastMessage: '你好', sentAt: '2026-08-16', conversationId: '   ' }],
      total: 1, page: 1, pageSize: 20, capabilities: [],
    });
    const api = createConversationGlobalApi(client);
    await expect(api.customerConversations?.({ customerId: 31, mode: 'direct', page: 1, pageSize: 20 })).rejects.toThrow('客户会话列表接口返回了无效数据');
  });

  it('rejects customer detail outside the customer detail contract', async () => {
    const client = { request: vi.fn<() => Promise<unknown>>() };
    const valid = {
      conversationId: '9:1:31', customerId: 31, customerName: '陈晓明',
      profile: { id: 31, name: '陈晓明', avatar: '', profileStatus: 'available' },
      employeeId: 9, employeeName: '张伟', targetType: 'customer', targetId: 31, targetName: '陈晓明', focused: false,
      stats: { communicationDays: 1, messageTotal: 1, inboundTotal: 1, outboundTotal: 0 }, messages: [], nextBefore: '', hasMore: false, capabilities: [],
    };
    const api = createConversationGlobalApi(client);
    for (const response of [
      { ...valid, profile: undefined },
      { ...valid, targetType: 'employee' },
      { ...valid, conversationId: '9:1:32' },
      { ...valid, stats: { ...valid.stats, messageTotal: '1' } },
      { ...valid, messages: [{ id: 'bad' }] },
      { ...valid, capabilities: [{ key: 'x', available: 'yes' }] },
    ]) {
      client.request.mockResolvedValueOnce(response);
      await expect(api.customerDetail?.({ customerId: 31, conversationId: '9:1:31', keyword: '', messageTypes: [], date: '', pageSize: 50 })).rejects.toThrow('客户会话详情接口返回了无效数据');
    }
  });

  it('rejects customer pages with non-canonical page sizes or enum values', async () => {
    const client = { request: vi.fn<() => Promise<unknown>>() };
    const api = createConversationGlobalApi(client);
    client.request.mockResolvedValueOnce({ customers: [], counts: { all: 0, focused: 0, active: 0, lost: 0 }, page: 1, pageSize: 20, total: 0, limitations: [], capabilities: [] });
    await expect(api.customerDirectory?.({ mode: 'all', keyword: '', page: 1, pageSize: 50 })).rejects.toThrow('客户目录接口返回了无效数据');
    client.request.mockResolvedValueOnce({ customer: { id: 31, name: '陈晓明', avatar: '', profileStatus: 'available' }, mode: 'invalid', list: [], total: 0, page: 1, pageSize: 20, capabilities: [] });
    await expect(api.customerConversations?.({ customerId: 31, mode: 'direct', page: 1, pageSize: 20 })).rejects.toThrow('客户会话列表接口返回了无效数据');
  });

  it('uses the fixed 20-row export endpoints and authenticated binary download', async () => {
    const task = { id: 7, exportType: 'customer', objectCount: 1, startAt: '2026-08-01T00:00:00+08:00', endAt: '2026-08-20T23:59:59+08:00', fileMode: 'split', format: 'zip', status: 'completed', estimatedMessageCount: 2, messageCount: 2, fileCount: 1, artifactName: 'conversation-export-7.zip', artifactSize: 128, expiresAt: '2026-08-27T00:00:00+08:00', createdAt: '2026-08-20T00:00:00+08:00' };
    const request = vi.fn<() => Promise<unknown>>()
      .mockResolvedValueOnce({ items: [{ id: 31, name: '星河科技', avatar: '', subtitle: 'customer', conversationCount: 1, messageCount: 2, selectable: true }], total: 1, page: 1, pageSize: 20, limitations: [], capabilities: [] })
      .mockResolvedValueOnce({ items: [task], total: 1, page: 1, pageSize: 20 })
      .mockResolvedValueOnce({ task, reused: false, limitations: [] });
    const download = vi.fn().mockResolvedValue({ blob: new Blob(['zip']), filename: task.artifactName });
    const api = createConversationGlobalApi({ request, download });

    await expect(api.exportCandidates?.({ type: 'customer', keyword: '星河', departmentId: null, page: 1, pageSize: 20 })).resolves.toMatchObject({ pageSize: 20 });
    await expect(api.exportTasks?.(1)).resolves.toMatchObject({ pageSize: 20 });
    await expect(api.createExportTask?.({ exportType: 'customer', objectIds: [31], conversationScopes: ['customer_direct'], employeeIds: [], startAt: task.startAt, endAt: task.endAt, fileMode: 'split', format: 'zip' })).resolves.toMatchObject({ task });
    await expect(api.downloadExport?.(7)).resolves.toMatchObject({ filename: task.artifactName });
    expect(request).toHaveBeenNthCalledWith(1, '/workMessage/exportCandidates?type=customer&page=1&pageSize=20&keyword=%E6%98%9F%E6%B2%B3');
    expect(request).toHaveBeenNthCalledWith(2, '/workMessage/exportTasks?page=1&pageSize=20');
    expect(request).toHaveBeenNthCalledWith(3, '/workMessage/exportTasks', expect.objectContaining({ method: 'POST' }));
    expect(download).toHaveBeenCalledWith('/workMessage/exportDownload?taskId=7');
  });
});
