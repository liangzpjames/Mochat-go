import { ApiError } from '@mochat/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type { ConversationGlobalApi, CustomerConversationDetail, CustomerConversationPage, CustomerDirectoryPage } from './conversation-global-api';
import { CustomerConversationPage as CustomerConversationPageView } from './customer-conversation-page';

const access: AccessContext = {
  session: { token: 'Bearer test', userId: '1', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true }, menu: [],
  allowedRoutes: new Set(['/chat/v2-customer']), allowedActions: new Set(),
};

const directory: CustomerDirectoryPage = {
  customers: [{ id: 31, name: '星河科技', avatar: '', profileStatus: 'available', activeRelationCount: 1, lostRelationCount: 0, directConversationCount: 1, groupConversationCount: 1, focusedConversationCount: 1, lastConversationAt: '' }],
  counts: { all: 1, focused: 1, active: 1, lost: 0 }, page: 2, pageSize: 50, total: 51, limitations: [], capabilities: [],
};
const conversations: CustomerConversationPage = {
  customer: { id: 31, name: '星河科技', avatar: '', profileStatus: 'available' }, mode: 'group', list: [{
    id: 'row-1', conversationId: '9:2:44', employeeId: 9, employeeName: '张伟', employeeAvatar: '', targetType: 'room', targetId: 44, targetName: '产品群', targetAvatar: '', lastMessage: '报价', sentAt: '',
  }], total: 1, page: 3, pageSize: 20, capabilities: [],
};
const detail: CustomerConversationDetail = {
  conversationId: '9:2:44', customerId: 31, customerName: '星河科技', profile: { id: 31, name: '星河科技', avatar: '', profileStatus: 'available' }, employeeId: 9, employeeName: '张伟', targetType: 'room', targetId: 44, targetName: '产品群', focused: false,
  stats: { communicationDays: 1, messageTotal: 2, inboundTotal: 1, outboundTotal: 1 }, messages: [{ id: 'm1', senderName: '张伟', senderAvatar: '', direction: 'outbound', type: 1, content: { text: '你好' }, sentAt: '' }], nextBefore: 'cursor-1', hasMore: true, capabilities: [],
};

function LocationProbe() { const location = useLocation(); return <output data-testid="location">{location.search}</output>; }
function createApi(overrides: Partial<ConversationGlobalApi> = {}): ConversationGlobalApi {
  return { search: vi.fn(), detail: vi.fn(), customerDirectory: vi.fn(() => Promise.resolve(directory)), customerConversations: vi.fn(() => Promise.resolve(conversations)), customerDetail: vi.fn(() => Promise.resolve(detail)), setFocus: vi.fn(() => Promise.resolve()), removeFocus: vi.fn(() => Promise.resolve()), ...overrides };
}
function renderPage(api: ConversationGlobalApi, entry = '/chat/v2-customer') {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return render(<MemoryRouter initialEntries={[entry]}><QueryClientProvider client={queryClient}><DashboardAccessProvider value={access}><CustomerConversationPageView api={api} /></DashboardAccessProvider><LocationProbe /></QueryClientProvider></MemoryRouter>);
}

afterEach(cleanup);

describe('CustomerConversationPage', () => {
  it('normalizes URL values and queries all three layers with fixed page sizes', async () => {
    const api = createApi();
    renderPage(api, '/chat/v2-customer?customerMode=focused&customerPage=2&customerId=31&conversationMode=group&page=3&pageSize=99&conversationId=9:2:44&messageTypes=image');
    await waitFor(() => expect(api.customerDirectory).toHaveBeenCalledWith({ mode: 'focused', keyword: '', page: 2, pageSize: 50 }));
    await waitFor(() => expect(api.customerConversations).toHaveBeenCalledWith({ customerId: 31, mode: 'group', page: 3, pageSize: 20 }));
    await waitFor(() => expect(api.customerDetail).toHaveBeenCalledWith(expect.objectContaining({ customerId: 31, conversationId: '9:2:44', messageTypes: ['image'], pageSize: 50 })));
    expect(screen.getByTestId('location').textContent).toContain('pageSize=20');
  });

  it('resets customer and conversation selection when scopes change', async () => {
    const api = createApi();
    renderPage(api, '/chat/v2-customer?customerId=31&conversationMode=group&page=3&conversationId=9%3A2%3A44');
    await screen.findByRole('button', { name: /星河科技/ });
    fireEvent.click(screen.getByRole('tab', { name: /客户列表/ }));
    await waitFor(() => expect(screen.getByTestId('location').textContent).not.toContain('customerId=31'));
    expect(screen.getByTestId('location').textContent).not.toContain('conversationId=');
  });

  it('merges older cursor pages in chronological order without duplicates', async () => {
    const older = { ...detail, messages: [{ ...detail.messages[0]!, id: 'm0', sentAt: '2026-08-18' }], hasMore: false, nextBefore: '' };
    const customerDetail = vi.fn((input: { before?: string }) => Promise.resolve(input.before ? older : detail));
    const api = createApi({ customerDetail });
    renderPage(api, '/chat/v2-customer?customerId=31&conversationId=9%3A2%3A44');
    await screen.findByText('你好');
    fireEvent.click(screen.getByRole('button', { name: '加载更早消息' }));
    await waitFor(() => expect(customerDetail).toHaveBeenCalledWith(expect.objectContaining({ before: 'cursor-1' })));
    const messages = screen.getByLabelText('客户消息详情').querySelectorAll('.customer-conversation-message');
    expect(Array.from(messages).map((node) => node.textContent)).toEqual(['张伟2026-08-18你好', '张伟你好']);
  });

  it('invalidates customer query layers after focus succeeds', async () => {
    const api = createApi();
    renderPage(api, '/chat/v2-customer?customerId=31&conversationId=9%3A2%3A44');
    await screen.findByText('你好');
    fireEvent.click(screen.getByRole('button', { name: '重点关注' }));
    await waitFor(() => expect(api.setFocus).toHaveBeenCalledWith('9:2:44'));
    await waitFor(() => expect(api.customerDirectory).toHaveBeenCalledTimes(2));
  });

  it('blocks the entire page when archive authorization returns 40301', async () => {
    renderPage(createApi({ customerDirectory: vi.fn(() => Promise.reject(new ApiError('forbidden', 'archive', { status: 403, code: 40301 }))) }));
    expect(await screen.findByText('会话归档未开通')).toBeTruthy();
  });

  it('keeps the workspace usable when customer profile is missing', async () => {
    const missing = { ...directory, customers: [{ ...directory.customers[0]!, profileStatus: 'missing' as const }] };
    const api = createApi({ customerDirectory: vi.fn(() => Promise.resolve(missing)) });
    renderPage(api);
    expect(await screen.findByText('客户资料未同步')).toBeTruthy();
    expect(screen.getByRole('heading', { name: '请选择会话' })).toBeTruthy();
  });
});
