import { ApiError } from '@mochat/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type { ConversationGlobalApi, StaffConversationDetail, StaffDirectoryPage } from './conversation-global-api';
import { EmployeeConversationPage } from './employee-conversation-page';

const access: AccessContext = {
  session: { token: 'Bearer test', userId: '1', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [], allowedRoutes: new Set(['/chat/v2-staff']), allowedActions: new Set(),
};

const directory: StaffDirectoryPage = {
  departments: [{ id: 10, parentId: 0, name: '销售部', employeeCount: 1, children: [] }],
  employees: [{ id: 9, name: '张三', avatar: '', status: 1, departmentIds: [10], archived: true, conversationCount: 21, focusedConversationCount: 1, lastConversationAt: '2026-08-16 10:00:00' }],
  counts: { all: 1, focused: 1, archived: 1, departed: 0 }, page: 1, pageSize: 50, total: 1,
  limitations: [], capabilities: [{ key: 'internalGroup', available: false, reason: '当前归档数据未提供内部群聊能力' }],
};

const summary = {
  id: 'msg:archive-31', conversationId: '9:1:31', employeeId: 9, employeeName: '张三', employeeAvatar: '',
  targetType: 'customer' as const, targetId: 31, targetName: '星河科技', targetAvatar: '',
  lastMessage: '请确认报价', sentAt: '2026-08-16 11:00:00', messageTotal: 15, focused: true,
};

const detail: StaffConversationDetail = {
  conversationId: '9:1:31', employeeId: 9, employeeName: '张三', targetType: 'customer', targetId: 31, targetName: '星河科技', focused: true,
  stats: { communicationDays: 2, messageTotal: 15, inboundTotal: 5, outboundTotal: 10 },
  messages: [{ id: 'm1', senderName: '张三', senderAvatar: '', direction: 'outbound', type: 1, content: { text: '你好' }, sentAt: '2026-08-16 11:00:00' }],
  nextBefore: 'cursor-1', hasMore: true, capabilities: [],
};

function createApi(overrides: Partial<ConversationGlobalApi> = {}): ConversationGlobalApi {
  return {
    staffDirectory: vi.fn(() => Promise.resolve(directory)),
    search: vi.fn(() => Promise.resolve({ list: [summary], total: 21, page: 1, pageSize: 20 })),
    staffDetail: vi.fn(() => Promise.resolve(detail)),
    detail: vi.fn(),
    ...overrides,
  };
}

function renderPage(api: ConversationGlobalApi, entry = '/chat/v2-staff') {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return render(<MemoryRouter initialEntries={[entry]}><QueryClientProvider client={queryClient}><DashboardAccessProvider value={access}><EmployeeConversationPage api={api} /></DashboardAccessProvider></QueryClientProvider></MemoryRouter>);
}

afterEach(cleanup);

describe('EmployeeConversationPage', () => {
  it('shows the conversation empty state before a conversation is selected', async () => {
    renderPage(createApi());
    await screen.findByRole('tab', { name: '企业架构' });
    expect(screen.getByText('请选择会话')).toBeTruthy();
    expect(screen.queryByText('正在读取会话')).toBeNull();
  });

  it('drops a legacy message id instead of requesting it as a stable conversation id', async () => {
    const staffDetail = vi.fn(() => Promise.resolve(detail));
    renderPage(createApi({ staffDetail }), '/chat/v2-staff?employeeId=9&conversationId=msg%3ASIM-MSG-0001');
    await screen.findByRole('tab', { name: '企业架构' });
    expect(await screen.findByText('请选择会话')).toBeTruthy();
    expect(staffDetail).not.toHaveBeenCalled();
  });

  it('shows the shared archive configuration state when archive access is unavailable', async () => {
    renderPage(createApi({
      staffDirectory: vi.fn(() => Promise.reject(new ApiError('forbidden', 'archive not authorized', { status: 403, code: 40301 }))),
    }));
    expect(await screen.findByText('会话归档未开通')).toBeTruthy();
    expect(screen.getByRole('link', { name: '去配置会话归档' }).getAttribute('href')).toBe('/company-setting/website');
  });

  it('renders the real directory controls and unavailable internal group state', async () => {
    renderPage(createApi());
    expect((await screen.findByRole('tab', { name: '企业架构' })).getAttribute('aria-selected')).toBe('true');
    expect(screen.getByRole('tab', { name: '重点关注' })).toBeTruthy();
    expect(await screen.findByRole('button', { name: /存档成员 1/ })).toBeTruthy();
    expect(screen.getByRole('button', { name: /离职成员 0/ })).toBeTruthy();
    const internalGroup = screen.getByRole('button', { name: /内部群/ });
    expect(internalGroup.hasAttribute('disabled')).toBe(true);
    expect(internalGroup.getAttribute('title')).toContain('当前归档数据未提供内部群聊能力');
  });

  it('selects employee, queries fixed 20 conversations and opens by stable conversationId', async () => {
    const search = vi.fn(() => Promise.resolve({ list: [summary], total: 21, page: 1, pageSize: 20 }));
    const staffDetail = vi.fn(() => Promise.resolve(detail));
    renderPage(createApi({ search, staffDetail }));
    fireEvent.click(await screen.findByRole('button', { name: /张三/ }));
    await waitFor(() => expect(search).toHaveBeenCalledWith(expect.objectContaining({ employeeIds: ['9'], page: 1, pageSize: 20 })));
    expect((await screen.findByRole('navigation', { name: '会话分页' })).textContent).toContain('共 21 条');
    fireEvent.click(screen.getByRole('button', { name: /星河科技/ }));
    await waitFor(() => expect(staffDetail).toHaveBeenCalledWith(expect.objectContaining({ conversationId: '9:1:31', pageSize: 50 })));
    expect(await screen.findByText('沟通天数')).toBeTruthy();
    expect(screen.getByText('15')).toBeTruthy();
    expect(screen.getByRole('button', { name: '返回会话列表' })).toBeTruthy();
  });

  it('selects only the clicked conversation row when duplicate summaries share a conversation id', async () => {
    const duplicate = { ...summary, id: 'msg:archive-32', sentAt: '2026-08-15 09:00:00', lastMessage: '更早一条消息' };
    const search = vi.fn(() => Promise.resolve({ list: [summary, duplicate], total: 2, page: 1, pageSize: 20 }));
    renderPage(createApi({ search }));
    fireEvent.click(await screen.findByRole('button', { name: /张三/ }));
    const rows = await screen.findAllByRole('button', { name: /星河科技/ });
    fireEvent.click(rows[0]!);
    await waitFor(() => expect(rows[0]!.classList.contains('is-selected')).toBe(true));
    expect(rows.filter((row) => row.classList.contains('is-selected'))).toHaveLength(1);
  });

  it('renders an unmapped customer group as a group and opens its real detail', async () => {
    const roomSummary = { ...summary, id: 'msg:room-0', conversationId: '9:2:0', targetType: 'room' as const, targetId: 0, targetName: '', lastMessage: '群消息' };
    const roomDetail: StaffConversationDetail = { ...detail, conversationId: '9:2:0', targetType: 'room', targetId: 0, targetName: '', messages: [{ ...detail.messages[0]!, content: { text: '群内消息' } }] };
    const search = vi.fn(() => Promise.resolve({ list: [roomSummary], total: 1, page: 1, pageSize: 20 }));
    const staffDetail = vi.fn(() => Promise.resolve(roomDetail));
    renderPage(createApi({ search, staffDetail }));
    fireEvent.click(await screen.findByRole('button', { name: /张三/ }));
    const roomItems = screen.getByRole('region', { name: '会话轨迹' }).querySelector('.employee-conversation-items') as HTMLElement;
    const roomRow = await within(roomItems).findByRole('button', { name: /客户群/ });
    expect(roomRow.textContent).toContain('客户群');
    expect(roomRow.textContent).not.toContain('客户群 0');
    fireEvent.click(roomRow);
    await waitFor(() => expect(staffDetail).toHaveBeenCalledWith(expect.objectContaining({ conversationId: '9:2:0' })));
    expect(await screen.findByText('客户群会话')).toBeTruthy();
    expect(screen.queryByText('会话 0')).toBeNull();
    expect(await screen.findByText('群内消息')).toBeTruthy();
  });

  it('searches directory and refreshes without duplicating the action', async () => {
    const staffDirectory = vi.fn(() => Promise.resolve(directory));
    renderPage(createApi({ staffDirectory }));
    fireEvent.change(await screen.findByLabelText('搜索员工'), { target: { value: '张' } });
    fireEvent.submit(screen.getByLabelText('搜索员工').closest('form')!);
    await waitFor(() => expect(staffDirectory).toHaveBeenCalledWith(expect.objectContaining({ keyword: '张', pageSize: 50 })));
    const refresh = screen.getByRole('button', { name: '刷新员工' });
    expect(refresh.closest('form')).toBe(screen.getByLabelText('搜索员工').closest('form'));
    expect(screen.getAllByRole('button', { name: '刷新员工' })).toHaveLength(1);
  });

  it('keeps department controls mounted while a new department directory is loading', async () => {
    let resolveNextDirectory: ((value: StaffDirectoryPage) => void) | undefined;
    const nextDirectory = new Promise<StaffDirectoryPage>((resolve) => { resolveNextDirectory = resolve; });
    let directoryCalls = 0;
    const staffDirectory = vi.fn((input: { departmentId: number | null }) => {
      directoryCalls += 1;
      return directoryCalls === 1 ? Promise.resolve(directory) : nextDirectory;
    });
    renderPage(createApi({ staffDirectory }));
    fireEvent.click(await screen.findByRole('button', { name: '部门：全部部门' }));
    fireEvent.click(screen.getByRole('option', { name: '销售部' }));
    await waitFor(() => expect(staffDirectory).toHaveBeenCalledWith(expect.objectContaining({ departmentId: 10 })));
    expect(screen.getByRole('button', { name: '部门：销售部' })).toBeTruthy();
    expect(screen.getByRole('button', { name: '全部成员 1' })).toBeTruthy();
    resolveNextDirectory?.(directory);
  });

  it('passes detail filters and loads older messages with the opaque cursor', async () => {
    const older: StaffConversationDetail = { ...detail, messages: [{ ...detail.messages[0]!, id: 'm0', content: { text: '更早消息' } }], hasMore: false, nextBefore: '' };
    const staffDetail = vi.fn((input: { before?: string }) => Promise.resolve(input.before ? older : detail));
    renderPage(createApi({ staffDetail }));
    fireEvent.click(await screen.findByRole('button', { name: /张三/ }));
    fireEvent.click(await screen.findByRole('button', { name: /星河科技/ }));
    const keywordInput = await screen.findByLabelText('搜索会话内容');
    const callsBeforeKeyword = staffDetail.mock.calls.length;
    fireEvent.change(keywordInput, { target: { value: '报价' } });
    expect(staffDetail).toHaveBeenCalledTimes(callsBeforeKeyword);
    fireEvent.click(screen.getByRole('button', { name: '查询会话内容' }));
    fireEvent.change(await screen.findByLabelText('检索日期'), { target: { value: '2026-08-16' } });
    fireEvent.click(await screen.findByLabelText('图片'));
    await waitFor(() => expect(staffDetail).toHaveBeenCalledWith(expect.objectContaining({ keyword: '报价', date: '2026-08-16', messageTypes: ['image'] })));
    fireEvent.click(await screen.findByRole('button', { name: '加载更早消息' }));
    await waitFor(() => expect(staffDetail).toHaveBeenCalledWith(expect.objectContaining({ before: 'cursor-1' })));
    expect(await screen.findByText('更早消息')).toBeTruthy();
  });

  it('only queries detail content after the keyword search is submitted', async () => {
    const staffDetail = vi.fn(() => Promise.resolve(detail));
    renderPage(createApi({ staffDetail }), '/chat/v2-staff?employeeId=9&conversationId=9%3A1%3A31');
    await screen.findByText('沟通天数');
    const searchInput = screen.getByLabelText('搜索会话内容');
    const callsBeforeTyping = staffDetail.mock.calls.length;
    fireEvent.change(searchInput, { target: { value: '报价' } });
    expect(staffDetail).toHaveBeenCalledTimes(callsBeforeTyping);
    fireEvent.click(screen.getByRole('button', { name: '查询会话内容' }));
    await waitFor(() => expect(staffDetail).toHaveBeenCalledWith(expect.objectContaining({ keyword: '报价' })));
  });
});
