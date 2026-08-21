import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import { RefuseArchivePage } from './refuse-archive-page';
import type { RefuseArchiveApi } from './refuse-archive-api';

afterEach(() => cleanup());

const row = { id: 3, subjectType: 'customer' as const, subjectId: 'c1', subjectName: '星河科技', employeeId: 9, employeeName: '张三', authorizationStatus: 'refused', source: 'wecom', refusedAt: '2026-08-20T10:00:00+08:00', authorizedAt: '', lastFollowUpAt: '', followUpStatus: 'unfollowed', followUpNote: '', updatedAt: '2026-08-20T10:00:00+08:00' };

function renderPage(api: RefuseArchiveApi) {
  const access = { corp: { id: '9', authorized: true }, session: {}, menu: [], allowedRoutes: new Set(), allowedActions: new Set(['/chat/refuse-archive#manage']) } as never;
  return render(<MemoryRouter initialEntries={['/chat/refuse-archive']}><DashboardAccessProvider value={access}><QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><RefuseArchivePage api={api} /></QueryClientProvider></DashboardAccessProvider></MemoryRouter>);
}

describe('RefuseArchivePage', () => {
  it('switches customer and room tabs with Chinese statuses', async () => {
    const list = vi.fn().mockResolvedValue({ items: [row], total: 1, page: 1, perPage: 20 });
    const api: RefuseArchiveApi = { list, followUp: vi.fn() };
    renderPage(api);
    expect(await screen.findByText('星河科技')).not.toBeNull();
    expect(screen.getByRole('cell', { name: '已拒绝' })).not.toBeNull();
    fireEvent.click(screen.getByRole('tab', { name: '群聊' }));
    await waitFor(() => expect(list).toHaveBeenLastCalledWith(expect.objectContaining({ subjectType: 'room', page: 1, perPage: 20 })));
  });

  it('opens and saves the follow-up drawer', async () => {
    const followUp = vi.fn().mockResolvedValue({ updated: true });
    const api: RefuseArchiveApi = { list: vi.fn().mockResolvedValue({ items: [row], total: 1, page: 1, perPage: 20 }), followUp };
    renderPage(api);
    fireEvent.click(await screen.findByRole('button', { name: '跟进' }));
    expect(screen.getByRole('dialog', { name: '拒绝存档跟进' })).not.toBeNull();
    fireEvent.change(screen.getByLabelText('跟进备注'), { target: { value: '已电话沟通' } });
    fireEvent.click(screen.getByRole('button', { name: '保存跟进' }));
    await waitFor(() => expect(followUp).toHaveBeenCalledWith({ id: 3, status: 'contacted', note: '已电话沟通' }));
  });
});
