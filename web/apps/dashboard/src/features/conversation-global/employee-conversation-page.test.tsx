import { ApiError } from '@mochat/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type { ConversationGlobalApi } from './conversation-global-api';
import { EmployeeConversationPage } from './employee-conversation-page';

const access: AccessContext = {
  session: { token: 'Bearer test', userId: '1', corpId: '7', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set(['/chat/v2-staff']),
  allowedActions: new Set(),
};

const employee = { id: 9, name: '张三', avatar: '' };
const summary = {
  id: 'msg:archive-31', employeeId: 9, employeeName: '张三', employeeAvatar: '',
  targetType: 'customer' as const, targetId: 31, targetName: '星河科技', targetAvatar: '',
  lastMessage: '请确认报价', sentAt: '2026-07-05 11:00:00',
};

function renderPage(api: ConversationGlobalApi) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <MemoryRouter initialEntries={['/chat/v2-staff']}>
      <QueryClientProvider client={queryClient}>
        <DashboardAccessProvider value={access}>
          <EmployeeConversationPage api={api} />
        </DashboardAccessProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

afterEach(cleanup);

describe('EmployeeConversationPage', () => {
  it('selects an employee and queries conversations scoped to that employee', async () => {
    const employees = vi.fn(() => Promise.resolve([employee]));
    const search = vi.fn(() => Promise.resolve({ list: [summary], total: 1, page: 1, pageSize: 20 }));
    renderPage({ employees, search, detail: vi.fn() });

    fireEvent.change(await screen.findByLabelText('员工名称'), { target: { value: '张' } });
    fireEvent.click(screen.getByRole('button', { name: '搜索员工' }));
    fireEvent.click(await screen.findByRole('button', { name: /张三/ }));

    await waitFor(() => expect(search).toHaveBeenCalledWith(expect.objectContaining({
      employeeIds: ['9'], conversationType: '', page: 1,
    })));
    expect(await screen.findByText('星河科技')).toBeTruthy();
  });

  it('loads the selected conversation detail into the right pane', async () => {
    const detail = vi.fn(() => Promise.resolve({
      id: summary.id, employeeId: 9, employeeName: '张三', targetType: 'customer' as const,
      targetId: 31, targetName: '星河科技', messageTotal: 1, truncated: false, window: 'latest' as const,
      messages: [{ id: 'm1', senderName: '张三', senderAvatar: '', direction: 'outbound' as const,
        type: 1, content: { content: '你好' }, sentAt: '2026-07-05 11:00:00' }],
    }));
    renderPage({
      employees: () => Promise.resolve([employee]),
      search: () => Promise.resolve({ list: [summary], total: 1, page: 1, pageSize: 20 }),
      detail,
    });

    fireEvent.click(await screen.findByRole('button', { name: /张三/ }));
    fireEvent.click(await screen.findByRole('button', { name: /星河科技/ }));

    expect(await screen.findByText('你好')).toBeTruthy();
    expect(detail).toHaveBeenCalledWith(summary.id);
  });

  it('maps the archive Provider error to the shared configuration state', async () => {
    renderPage({
      employees: () => Promise.resolve([employee]),
      search: () => Promise.reject(new ApiError('forbidden', 'archive not authorized', { status: 403, code: 40301 })),
      detail: vi.fn(),
    });

    fireEvent.click(await screen.findByRole('button', { name: /张三/ }));

    const state = (await screen.findByText('会话归档未开通')).closest('[role="status"]');
    expect(state?.textContent).toContain('会话归档未开通');
    expect(screen.getByRole('link', { name: '去配置会话归档' }).getAttribute('href')).toBe('/company-setting/website');
    expect(screen.queryByText('加载失败')).toBeNull();
  });
});
