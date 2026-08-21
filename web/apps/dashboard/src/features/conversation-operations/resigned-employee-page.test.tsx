import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import { ResignedEmployeePage } from './resigned-employee-page';
import type { ConversationGlobalApi } from '../conversation-global/conversation-global-api';

afterEach(() => cleanup());

const employee = { id: 9, name: '张三', avatar: '', status: 5 as const, departmentIds: [], archived: true, conversationCount: 1, focusedConversationCount: 0, lastConversationAt: '' };
const conversation = { id: 'row-1', employeeId: 9, employeeName: '张三', employeeAvatar: '', targetType: 'customer' as const, targetId: 31, targetName: '星河科技', targetAvatar: '', lastMessage: '你好', sentAt: '2026-08-20 10:00:00', conversationId: '9:1:31', messageTotal: 1 };
const detail = { conversationId: '9:1:31', employeeId: 9, employeeName: '张三', targetType: 'customer' as const, targetId: 31, targetName: '星河科技', focused: false, stats: { communicationDays: 1, messageTotal: 1, inboundTotal: 1, outboundTotal: 0 }, messages: [], nextBefore: '', hasMore: false, capabilities: [] };

function renderPage(api: ConversationGlobalApi) {
  const access = { corp: { id: '9' }, session: {}, menu: [], allowedRoutes: new Set(), allowedActions: new Set() } as never;
  return render(<MemoryRouter initialEntries={['/chat/resign-staff']}><DashboardAccessProvider value={access}><QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><ResignedEmployeePage api={api} /></QueryClientProvider></DashboardAccessProvider></MemoryRouter>);
}

describe('ResignedEmployeePage', () => {
  it('requests departed employees, opens their conversation and hides deferred types', async () => {
    const api: ConversationGlobalApi = {
      search: vi.fn().mockResolvedValue({ list: [conversation], total: 1, page: 1, pageSize: 20 }),
      detail: vi.fn().mockResolvedValue({ id: 'unused', employeeId: 9, employeeName: '张三', targetType: 'customer', targetId: 31, targetName: '星河科技', messageTotal: 1, truncated: false, window: 'latest', messages: [] }),
      staffDirectory: vi.fn().mockResolvedValue({ departments: [], employees: [employee], counts: { all: 1, focused: 0, archived: 1, departed: 1 }, page: 1, pageSize: 50, total: 1, limitations: [], capabilities: [] }),
      staffDetail: vi.fn().mockResolvedValue(detail),
    };
    renderPage(api);
    await waitFor(() => expect(api.staffDirectory).toHaveBeenCalledWith(expect.objectContaining({ mode: 'departed', pageSize: 50 })));
    fireEvent.click(await screen.findByRole('button', { name: /张三.*已离职/ }));
    await waitFor(() => expect(api.search).toHaveBeenCalledWith(expect.objectContaining({ employeeIds: ['9'], pageSize: 20 })));
    expect(screen.queryByRole('button', { name: '内部群' })).toBeNull();
    fireEvent.click(await screen.findByRole('button', { name: /星河科技/ }));
    await waitFor(() => expect(api.staffDetail).toHaveBeenCalledWith(expect.objectContaining({ conversationId: '9:1:31', pageSize: 50 })));
  });

  it('filters departed employees through the shared compact department picker', async () => {
    const staffDirectory = vi.fn().mockResolvedValue({
      departments: [{ id: 10, parentId: 0, name: '销售部', employeeCount: 1, children: [] }],
      employees: [employee], counts: { all: 1, focused: 0, archived: 1, departed: 1 }, page: 1, pageSize: 50, total: 1, limitations: [], capabilities: [],
    });
    const api: ConversationGlobalApi = {
      search: vi.fn().mockResolvedValue({ list: [], total: 0, page: 1, pageSize: 20 }),
      detail: vi.fn(),
      staffDirectory,
    };
    renderPage(api);
    fireEvent.click(await screen.findByRole('button', { name: '部门：全部部门' }));
    fireEvent.click(screen.getByRole('option', { name: '销售部' }));
    await waitFor(() => expect(staffDirectory).toHaveBeenCalledWith(expect.objectContaining({ mode: 'departed', departmentId: 10 })));
    expect(screen.queryByRole('button', { name: /销售部 1/ })).toBeNull();
  });
});
