import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type { ConversationGlobalApi } from './conversation-global-api';
import { ConversationTrajectoryPage } from './conversation-trajectory-page';

const access: AccessContext = {
  session: { token: 'Bearer test', userId: '1', corpId: '7', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set(['/chat/trajectory']),
  allowedActions: new Set(),
};

const conversation = {
  id: 'msg:archive-31', employeeId: 9, employeeName: '张三', employeeAvatar: '',
  targetType: 'customer' as const, targetId: 31, targetName: '星河科技', targetAvatar: '',
  lastMessage: '请确认报价', sentAt: '2026-07-05 11:00:00',
};

function renderPage(api: ConversationGlobalApi) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <MemoryRouter initialEntries={['/chat/trajectory']}>
      <QueryClientProvider client={queryClient}>
        <DashboardAccessProvider value={access}>
          <ConversationTrajectoryPage api={api} />
        </DashboardAccessProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

afterEach(cleanup);

describe('ConversationTrajectoryPage', () => {
  it('loads a real conversation and renders its chronological message timeline', async () => {
    const search = vi.fn(() => Promise.resolve({ list: [conversation], total: 1, page: 1, pageSize: 20 }));
    const detail = vi.fn(() => Promise.resolve({
      id: conversation.id, employeeId: 9, employeeName: '张三', targetType: 'customer' as const,
      targetId: 31, targetName: '星河科技', messageTotal: 2, truncated: false, window: 'latest' as const,
      messages: [
        { id: 'm1', senderName: '星河科技', senderAvatar: '', direction: 'inbound' as const, type: 1, content: { content: '需要报价' }, sentAt: '2026-07-05 10:00:00' },
        { id: 'm2', senderName: '张三', senderAvatar: '', direction: 'outbound' as const, type: 1, content: { content: '请确认报价' }, sentAt: '2026-07-05 11:00:00' },
      ],
    }));
    renderPage({ search, detail });

    fireEvent.click(await screen.findByRole('button', { name: /星河科技/ }));

    expect(await screen.findByText('需要报价')).toBeTruthy();
    expect(screen.getAllByText('请确认报价').length).toBeGreaterThanOrEqual(1);
    expect(detail).toHaveBeenCalledWith(conversation.id);
    expect(screen.getByRole('heading', { name: '会话轨迹' })).toBeTruthy();
  });
});
