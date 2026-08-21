import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type { ConversationGlobalApi } from './conversation-global-api';
import { GroupConversationPage } from './group-conversation-page';

const access: AccessContext = { session: { token: 'Bearer test', userId: '1', expiresAt: null }, corp: { id: '7', name: '测试企业', authorized: true }, menu: [], allowedRoutes: new Set(['/chat/v2-group']), allowedActions: new Set() };
const room = { id: 71, externalId: 'wr_71', name: '星河客户群', avatar: '', ownerId: 9, ownerName: '张三', memberCount: 2, employeeCount: 1, customerCount: 1, messageCount: 1, lastMessage: '你好', lastMessageAt: '2026-08-19 11:00:00', focused: false, riskCount: 0, timeoutCount: 0, dissolved: false };
const profile = { ...room, createdAt: '2026-08-01 10:00:00', status: 'active', dissolved: false, capabilities: [], limitations: [] };
const messages = { roomId: 71, stats: { messageTotal: 1, employeeTotal: 1, customerTotal: 0, riskTotal: 0, timeoutTotal: 0 }, messages: [{ id: 'm1', senderId: 9, senderName: '张三', senderAvatar: '', senderKind: 'employee', direction: 'outbound' as const, sentAt: '2026-08-19 11:00:00', archiveSource: 'external', archiveSourceId: 'wecom', type: 1, content: { text: '你好' } }], nextBefore: '', hasMore: false, capabilities: [] };
const members = { items: [], total: 0, page: 1, pageSize: 50 as const, capabilities: [] };

function createApi(): ConversationGlobalApi {
  return { search: vi.fn(), detail: vi.fn(), groupRoomDirectory: vi.fn(() => Promise.resolve({ items: [room], total: 1, page: 1, pageSize: 50 as const, capabilities: [], limitations: [] })), groupRoomProfile: vi.fn(() => Promise.resolve(profile)), groupRoomMessages: vi.fn(() => Promise.resolve(messages)), groupRoomMembers: vi.fn(() => Promise.resolve(members)), groupRoomFilterOptions: vi.fn(() => Promise.resolve({ employees: [], customers: [], groups: [], capabilities: [] })) };
}

function renderPage(api: ConversationGlobalApi, entry = '/chat/v2-group') { const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); return render(<MemoryRouter initialEntries={[entry]}><QueryClientProvider client={client}><DashboardAccessProvider value={access}><GroupConversationPage api={api} /></DashboardAccessProvider></QueryClientProvider></MemoryRouter>); }

describe('GroupConversationPage', () => {
  afterEach(cleanup);
  it('loads the directory first, then loads the three selected-room data streams', async () => {
    const api = createApi();
    renderPage(api);
    expect(await screen.findByRole('button', { name: /星河客户群/ })).toBeTruthy();
    expect(api.groupRoomProfile).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: /星河客户群/ }));
    await waitFor(() => expect(api.groupRoomProfile).toHaveBeenCalledWith(71));
    expect(api.groupRoomMessages).toHaveBeenCalledWith(expect.objectContaining({ roomId: 71, pageSize: 50 }));
    expect((api.groupRoomMessages as ReturnType<typeof vi.fn>).mock.calls[0]?.[0]).not.toHaveProperty('before');
    expect(api.groupRoomMembers).toHaveBeenCalledWith(expect.objectContaining({ roomId: 71, pageSize: 50 }));
    expect(await screen.findByText('你好')).toBeTruthy();
  });

  it('keeps message keyword local until the message query is submitted', async () => {
    const api = createApi();
    renderPage(api, '/chat/v2-group?roomId=71');
    await screen.findByRole('button', { name: /星河客户群/ });
    const input = await screen.findByLabelText('搜索群消息');
    const calls = (api.groupRoomMessages as ReturnType<typeof vi.fn>).mock.calls.length;
    fireEvent.change(input, { target: { value: '报价' } });
    expect((api.groupRoomMessages as ReturnType<typeof vi.fn>).mock.calls.length).toBe(calls);
    fireEvent.click(screen.getByRole('button', { name: '查询消息' }));
    await waitFor(() => expect(api.groupRoomMessages).toHaveBeenLastCalledWith(expect.objectContaining({ keyword: '报价' })));
  });

  it('collapses the wide-screen profile column from the right-side close button and reopens it from the message header', async () => {
    const api = createApi();
    renderPage(api, '/chat/v2-group?roomId=71');
    await screen.findByText('wr_71');

    const workspace = document.querySelector('.group-conversation-workspace');
    expect(workspace?.className).not.toContain('is-profile-closed');
    fireEvent.click(screen.getByRole('button', { name: '关闭群资料' }));
    expect(workspace?.className).toContain('is-profile-closed');

    fireEvent.click(screen.getByRole('button', { name: '查看群资料' }));
    expect(workspace?.className).not.toContain('is-profile-closed');
  });
});
