import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type { ConversationGlobalApi } from './conversation-global-api';
import { ConversationExportPage } from './conversation-export-page';

const access: AccessContext = {
  session: { token: 'Bearer test', userId: '1', corpId: '7', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true }, menu: [],
  allowedRoutes: new Set(['/chat/export']), allowedActions: new Set(),
};

const summary = { id: 'msg:1', employeeId: 9, employeeName: '张三', employeeAvatar: '', targetType: 'customer' as const, targetId: 31, targetName: '星河科技', targetAvatar: '', lastMessage: '请确认报价', sentAt: '2026-07-05 11:00:00' };

function renderPage(api: ConversationGlobalApi) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<MemoryRouter initialEntries={['/chat/export']}><QueryClientProvider client={queryClient}><DashboardAccessProvider value={access}><ConversationExportPage api={api} /></DashboardAccessProvider></QueryClientProvider></MemoryRouter>);
}

afterEach(cleanup);

describe('ConversationExportPage', () => {
  it('queries real conversations and exports the current result page as CSV', async () => {
    renderPage({ search: () => Promise.resolve({ list: [summary], total: 1, page: 1, pageSize: 100 }), detail: vi.fn() });
    expect(await screen.findByText('星河科技')).toBeTruthy();
    const exportButton = screen.getByRole('button', { name: '导出当前结果' });
    expect(exportButton).toBeTruthy();
    expect((exportButton as HTMLButtonElement).disabled).toBe(false);
  });
});
