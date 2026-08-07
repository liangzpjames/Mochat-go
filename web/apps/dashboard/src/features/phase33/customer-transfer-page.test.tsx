import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';
import { CustomerTransferPage } from './customer-transfer-page';

const access: AccessContext = {
  session: { token: 'token', userId: '1', corpId: '7', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set(['/chat/resign-staff', '/customer/inheritance']),
  allowedActions: new Set(),
};

afterEach(cleanup);

function view(mode: 'inheritance' | 'resign', accessValue: AccessContext, api: BusinessWorkbenchApi) {
  return render(
    <MemoryRouter>
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <DashboardAccessProvider value={accessValue}>
          <CustomerTransferPage api={api} mode={mode} />
        </DashboardAccessProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

describe('CustomerTransferPage', () => {
  it.each([
    ['inheritance', '/contactTransfer/unassignedList', '客户继承'],
    ['resign', '/contactTransfer/info', '离职员工'],
  ] as const)('loads %s records from %s', async (mode, endpoint, title) => {
    const read = vi.fn().mockResolvedValue({
      list: [{ contactId: 1, contactName: '李雷', handoverEmployeeName: '张三', status: 'pending' }],
    });
    view(mode, access, { read, write: vi.fn() });

    expect(await screen.findByText('李雷')).toBeTruthy();
    expect(screen.getByRole('heading', { name: title })).toBeTruthy();
    expect(read).toHaveBeenCalledWith(endpoint, { page: 1, perPage: 20 });
  });

  it('renders forbidden state when the corp is not authorized (受限)', () => {
    const read = vi.fn();
    const result = view('resign', { ...access, corp: { ...access.corp, authorized: false } }, { read, write: vi.fn() });

    expect(screen.getByRole('heading', { name: '无权访问当前企业数据' })).toBeTruthy();
    expect(read).not.toHaveBeenCalled();
    expect(result.container.querySelector('.page-state-forbidden')).not.toBeNull();
  });

  it('renders empty state', async () => {
    const read = vi.fn().mockResolvedValue({ list: [] });
    view('inheritance', access, { read, write: vi.fn() });

    await waitFor(() => expect(screen.getByRole('heading', { name: '暂无待分配客户' })).toBeTruthy());
    expect(screen.getByRole('link', { name: '去分配' })).toBeTruthy();
  });

  it('renders error state and retries', async () => {
    const read = vi.fn().mockRejectedValue(new Error('network down'));
    const api = { read, write: vi.fn() };
    const result = view('resign', access, api);

    await waitFor(() => expect(result.container.querySelector('.page-state-error')).not.toBeNull());
    fireEvent.click(screen.getByRole('button', { name: '重新加载' }));
    await waitFor(() => expect(read).toHaveBeenCalledTimes(2));
  });

  it('opens record detail', async () => {
    const read = vi.fn().mockResolvedValue({
      list: [{ contactId: 1, contactName: '李雷', status: 'pending' }],
    });
    view('resign', access, { read, write: vi.fn() });

    fireEvent.click(await screen.findByRole('button', { name: '详情' }));
    expect(screen.getByRole('heading', { name: '记录详情' })).toBeTruthy();
  });
});
