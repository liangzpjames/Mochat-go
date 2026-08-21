import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type { ConversationGlobalApi } from './conversation-global-api';
import { ConversationExportPage, shouldPollExportTasks } from './conversation-export-page';

const access: AccessContext = {
  session: { token: 'Bearer test', userId: '1', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true }, menu: [],
  allowedRoutes: new Set(['/chat/export']), allowedActions: new Set(),
};

const candidate = { id: 31, name: '星河科技', avatar: '', subtitle: 'customer', conversationCount: 2, messageCount: 18, lastMessageAt: '2026-07-05 11:00:00', selectable: true };

function renderPage(api: ConversationGlobalApi) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<MemoryRouter initialEntries={['/chat/export']}><QueryClientProvider client={queryClient}><DashboardAccessProvider value={access}><ConversationExportPage api={api} /></DashboardAccessProvider></QueryClientProvider></MemoryRouter>);
}

afterEach(cleanup);

describe('ConversationExportPage', () => {
  it('polls only while the current task page contains active tasks', () => {
    const baseTask = {
      id: 1, exportType: 'employee' as const, objectCount: 1,
      startAt: '2026-08-15T00:00:00+08:00', endAt: '2026-08-21T10:00:00+08:00',
      fileMode: 'split' as const, format: 'zip' as const, estimatedMessageCount: 3,
      messageCount: 3, fileCount: 1, artifactName: 'export.zip', artifactSize: 1,
      expiresAt: '2026-08-28T10:00:00+08:00', createdAt: '2026-08-21T10:00:00+08:00',
    };
    expect(shouldPollExportTasks(undefined)).toBe(false);
    expect(shouldPollExportTasks({ items: [{ ...baseTask, status: 'completed' }], total: 1, page: 1, pageSize: 20 })).toBe(false);
    expect(shouldPollExportTasks({ items: [{ ...baseTask, status: 'pending' }], total: 1, page: 1, pageSize: 20 })).toBe(true);
    expect(shouldPollExportTasks({ items: [{ ...baseTask, status: 'running' }], total: 1, page: 1, pageSize: 20 })).toBe(true);
  });

  it('uses the approved two-step flow and queries candidates only after entering step two', async () => {
    const exportCandidates = vi.fn().mockResolvedValue({ items: [candidate], total: 1, page: 1, pageSize: 20, limitations: [], capabilities: [] });
    renderPage({ search: vi.fn(), exportCandidates, detail: vi.fn() });
    expect(screen.getByRole('heading', { name: '会话导出' })).toBeTruthy();
    expect(screen.getByRole('button', { name: '按员工导出' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '按客户导出' }));
    fireEvent.click(screen.getByRole('button', { name: '下一步' }));
    expect(await screen.findByText('星河科技')).toBeTruthy();
    expect(exportCandidates).toHaveBeenCalledWith({ type: 'customer', keyword: '', departmentId: null, page: 1, pageSize: 20 });
  });

  it('keeps keyword edits local until the query button is submitted', async () => {
    const exportCandidates = vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 20, limitations: [], capabilities: [] });
    renderPage({ search: vi.fn(), exportCandidates, detail: vi.fn() });
    fireEvent.click(screen.getByRole('button', { name: '下一步' }));
    await waitFor(() => expect(exportCandidates).toHaveBeenCalledTimes(1));
    const input = screen.getByLabelText('搜索员工');
    fireEvent.change(input, { target: { value: '张三' } });
    expect(exportCandidates).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(exportCandidates).toHaveBeenCalledWith({ type: 'employee', keyword: '张三', departmentId: null, page: 1, pageSize: 20 }));
  });

  it('keeps the dialog open and displays the server error inside it when task creation fails', async () => {
    const exportCandidates = vi.fn().mockResolvedValue({ items: [candidate], total: 1, page: 1, pageSize: 20, limitations: [], capabilities: [] });
    const createExportTask = vi.fn().mockRejectedValue(new Error('结束时间不能晚于当前时间'));
    renderPage({ search: vi.fn(), exportCandidates, createExportTask, detail: vi.fn() });

    fireEvent.click(screen.getByRole('button', { name: '下一步' }));
    await screen.findByText(candidate.name);
    fireEvent.click(screen.getByLabelText(`选择${candidate.name}`));
    fireEvent.click(screen.getByRole('button', { name: '下一步' }));
    const dialog = screen.getByRole('dialog', { name: '确认导出员工数据' });
    fireEvent.click(within(dialog).getByRole('button', { name: '确认创建任务' }));

    expect((await within(dialog).findByRole('alert')).textContent).toContain('结束时间不能晚于当前时间');
    expect(screen.getByRole('dialog', { name: '确认导出员工数据' })).toBeTruthy();
  });

});
