import { ApiError } from '@mochat/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type {
  DashboardOverview,
  DashboardOverviewApi,
} from './dashboard-overview-api';
import { DashboardOverviewPage } from './dashboard-overview-page';

const access: AccessContext = {
  session: { token: 'Bearer test', userId: '1', corpId: '7', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set(['/index']),
  allowedActions: new Set(),
};

const range = { from: '2026-07-01', to: '2026-07-31' };

const overview: DashboardOverview = {
  cards: [
    { key: 'contacts', label: '真实客户总数', value: 137 },
    { key: 'rooms', label: '真实客户群', value: 29 },
  ],
  trend: [
    {
      date: '2026-07-30',
      addContactNum: 12,
      addIntoRoomNum: 8,
      lossContactNum: 2,
      quitRoomNum: 1,
    },
  ],
  updatedAt: '2026-07-31 09:30:00',
};

afterEach(cleanup);

function renderPage(api: DashboardOverviewApi) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <DashboardAccessProvider value={access}>
        <DashboardOverviewPage api={api} initialRange={range} />
      </DashboardAccessProvider>
    </QueryClientProvider>,
  );
}

describe('DashboardOverviewPage', () => {
  it('shows loading, then renders cards and trend values from the API response', async () => {
    let resolve: ((value: DashboardOverview) => void) | undefined;
    const load = vi.fn(() => new Promise<DashboardOverview>((accept) => {
      resolve = accept;
    }));
    renderPage({ load });

    expect(screen.getByRole('status').textContent).toContain('正在加载数据概览');
    resolve?.(overview);

    expect(await screen.findByText('真实客户总数')).not.toBeNull();
    expect(screen.getByText('137')).not.toBeNull();
    expect(screen.getByText('2026-07-30')).not.toBeNull();
    expect(screen.getByLabelText('新增客户 12')).not.toBeNull();
    expect(screen.getByText(/2026-07-31 09:30:00/)).not.toBeNull();
    expect(load).toHaveBeenCalledWith({ corpId: '7', ...range });
  });

  it('shows an empty state for a successful empty response', async () => {
    renderPage({
      load: vi.fn(() => Promise.resolve({
        cards: [],
        trend: [],
        updatedAt: '',
      })),
    });

    expect(await screen.findByText('当前日期范围暂无数据')).not.toBeNull();
  });

  it('shows a dedicated forbidden state without stale statistics', async () => {
    renderPage({
      load: vi.fn(() => Promise.reject(
        new ApiError('forbidden', 'forbidden', { status: 403 }),
      )),
    });

    expect(await screen.findByText('无权访问当前企业数据')).not.toBeNull();
    expect(screen.queryByText('137')).toBeNull();
  });

  it('shows a retryable error state', async () => {
    const load = vi.fn()
      .mockRejectedValueOnce(new Error('网络异常'))
      .mockResolvedValueOnce(overview);
    renderPage({ load });

    expect((await screen.findByRole('alert')).textContent).toContain('网络异常');
    fireEvent.click(screen.getByRole('button', { name: '重试' }));
    expect(await screen.findByText('真实客户总数')).not.toBeNull();
    expect(load).toHaveBeenCalledTimes(2);
  });

  it('applies a new date range and refreshes through the injected API', async () => {
    const load = vi.fn(() => Promise.resolve(overview));
    renderPage({ load });
    await screen.findByText('真实客户总数');

    fireEvent.change(screen.getByLabelText('开始日期'), {
      target: { value: '2026-07-08' },
    });
    fireEvent.change(screen.getByLabelText('结束日期'), {
      target: { value: '2026-07-20' },
    });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));

    await waitFor(() => expect(load).toHaveBeenLastCalledWith({
      corpId: '7',
      from: '2026-07-08',
      to: '2026-07-20',
    }));
    const refresh = screen.getByRole('button', { name: '刷新' });
    await waitFor(() => expect(refresh.hasAttribute('disabled')).toBe(false));
    fireEvent.click(refresh);
    await waitFor(() => expect(load).toHaveBeenCalledTimes(3));
  });
});
