import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';
import { CustomerLossPage } from './customer-loss-page';

const access: AccessContext = {
  session: { token: 'token', userId: '1', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set(['/ai-insight/v2/customer-loss']),
  allowedActions: new Set(),
};

afterEach(cleanup);

function view(accessValue: AccessContext, api: BusinessWorkbenchApi) {
  return render(
    <MemoryRouter>
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <DashboardAccessProvider value={accessValue}>
          <CustomerLossPage api={api} />
        </DashboardAccessProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

describe('CustomerLossPage', () => {
  it('renders forbidden state when the corp is not authorized (受限)', () => {
    const read = vi.fn();
    const result = view({ ...access, corp: { ...access.corp, authorized: false } }, { read, write: vi.fn() });

    expect(screen.getByRole('heading', { name: '无权访问当前企业数据' })).toBeTruthy();
    expect(read).not.toHaveBeenCalled();
    expect(result.container.querySelector('.page-state-forbidden')).not.toBeNull();
  });

  it('loads loss records and applies the employee filter', async () => {
    const read = vi.fn().mockResolvedValue({
      list: [{ id: 3, name: '李雷', employeeName: '张三', deletedAt: '2026-08-02 10:00:00' }],
    });
    const api = { read, write: vi.fn() };
    view(access, api);

    expect(await screen.findByText('李雷')).toBeTruthy();
    expect(read).toHaveBeenCalledWith('/workContact/lossContact', { page: 1, perPage: 20 });

    fireEvent.change(screen.getByLabelText('员工 ID'), { target: { value: '99' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(read).toHaveBeenLastCalledWith(
      '/workContact/lossContact',
      { employeeId: '99', page: 1, perPage: 20 },
    ));
  });

  it('renders empty state', async () => {
    const read = vi.fn().mockResolvedValue({ list: [] });
    view(access, { read, write: vi.fn() });

    await waitFor(() => expect(screen.getByRole('heading', { name: '暂无数据' })).toBeTruthy());
  });

  it('renders error state and retries', async () => {
    const read = vi.fn().mockRejectedValue(new Error('network down'));
    const api = { read, write: vi.fn() };
    const result = view(access, api);

    await waitFor(() => expect(result.container.querySelector('.page-state-error')).not.toBeNull());
    fireEvent.click(screen.getByRole('button', { name: '重新加载' }));
    await waitFor(() => expect(read).toHaveBeenCalledTimes(2));
  });
});
