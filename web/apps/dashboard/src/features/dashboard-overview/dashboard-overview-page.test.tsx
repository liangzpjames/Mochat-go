import { ApiError } from '@mochat/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type {
  DashboardOverview,
  DashboardOverviewApi,
} from './dashboard-overview-api';
import { createDashboardOverviewApi } from './dashboard-overview-api';
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
  summary: {
    weChatContactNum: 137,
    weChatRoomNum: 29,
    roomMemberNum: 86,
    corpMemberNum: 18,
    addContactNum: 12,
    lastAddContactNum: 9,
    addIntoRoomNum: 8,
    lastAddIntoRoomNum: 6,
    lossContactNum: 2,
    lastLossContactNum: 3,
    quitRoomNum: 1,
    lastQuitRoomNum: 1,
  },
  page: 1,
  pageSize: 20,
  total: 1,
};

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllEnvs();
});

function LocationProbe() {
  const location = useLocation();
  return <output aria-label="当前地址">{location.pathname}{location.search}</output>;
}

function renderPage(
  api: Pick<DashboardOverviewApi, 'load'> & Partial<Pick<DashboardOverviewApi, 'exportCsv'>>,
  entry = '/index',
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <MemoryRouter initialEntries={[entry]}>
      <QueryClientProvider client={queryClient}>
        <DashboardAccessProvider value={access}>
          <DashboardOverviewPage api={{ exportCsv: vi.fn(() => Promise.resolve(new Blob())), ...api }} initialRange={range} />
          <LocationProbe />
        </DashboardAccessProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

describe('DashboardOverviewPage', () => {
  it('builds the default range in the explicit enterprise timezone', async () => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(new Date('2026-08-01T16:30:00Z'));
    vi.stubEnv('TZ', 'UTC');
    const load = vi.fn(() => Promise.resolve(overview));

    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <MemoryRouter initialEntries={['/index']}>
        <QueryClientProvider client={queryClient}>
          <DashboardAccessProvider value={access}>
            <DashboardOverviewPage api={{ load, exportCsv: vi.fn(() => Promise.resolve(new Blob())) }} />
          </DashboardAccessProvider>
        </QueryClientProvider>
      </MemoryRouter>,
    );

    await screen.findByText('真实客户总数');
    expect(load).toHaveBeenCalledWith(expect.objectContaining({
      startDate: '2026-07-03',
      endDate: '2026-08-02',
    }));
  });
  it('shows loading, then renders cards and trend values from the API response', async () => {
    let resolve: ((value: DashboardOverview) => void) | undefined;
    const load = vi.fn(() => new Promise<DashboardOverview>((accept) => {
      resolve = accept;
    }));
    const { container } = renderPage({ load });

    expect(screen.getAllByRole('status')[0]?.textContent).toContain('正在加载数据概览');
    resolve?.(overview);

    expect(await screen.findByText('真实客户总数')).not.toBeNull();
    expect(screen.getByText('137')).not.toBeNull();
    expect(screen.getAllByText('2026-07-30')).not.toHaveLength(0);
    expect(screen.getByLabelText('新增客户 12')).not.toBeNull();
    expect(screen.getByText(/2026-07-31 09:30:00/)).not.toBeNull();
    expect(load).toHaveBeenCalledWith({
      corpId: '7',
      startDate: range.from,
      endDate: range.to,
      employeeIds: [],
      departmentIds: [],
      period: 'day',
      page: 1,
      pageSize: 20,
    });
    expect(container.querySelector('.dashboard-page-header')).not.toBeNull();
    expect(container.querySelector('.dashboard-filter-bar')).not.toBeNull();
    expect(container.querySelector('.dashboard-stat-grid')).not.toBeNull();
    expect(container.querySelector('.dashboard-data-card')).not.toBeNull();
  });

  it('keeps the complete business dashboard visible for a successful empty response', async () => {
    renderPage({
      load: vi.fn(() => Promise.resolve({
        cards: [],
        trend: [],
        updatedAt: '',
      })),
    });

    expect(await screen.findByRole('heading', { name: 'AI 洞察' })).not.toBeNull();
    expect(screen.getByRole('heading', { name: '数据概览' })).not.toBeNull();
    expect(screen.getByRole('heading', { name: '会话数据' })).not.toBeNull();
    expect(screen.getByRole('heading', { name: '质检数据' })).not.toBeNull();
    expect(screen.getByRole('heading', { name: '员工会话数据排行' })).not.toBeNull();
    expect(screen.getByRole('heading', { name: '员工会话轨迹一览' })).not.toBeNull();
    expect(screen.getAllByText('暂无数据').length).toBeGreaterThan(0);
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

  it('removes previously loaded statistics when refresh loses corp authorization', async () => {
    const load = vi.fn()
      .mockResolvedValueOnce(overview)
      .mockRejectedValueOnce(new ApiError('forbidden', 'forbidden', { status: 403 }));
    renderPage({ load });
    expect(await screen.findByText('137')).not.toBeNull();

    fireEvent.click(screen.getByRole('button', { name: '刷新' }));

    expect(await screen.findByText('无权访问当前企业数据')).not.toBeNull();
    expect(screen.queryByText('137')).toBeNull();
    expect(screen.queryByLabelText('新增客户 12')).toBeNull();
  });

  it('shows a retryable error state', async () => {
    const load = vi.fn()
      .mockRejectedValueOnce(new Error('网络异常'))
      .mockResolvedValueOnce(overview);
    const { container } = renderPage({ load });

    await waitFor(() => expect(container.querySelector('.page-state-error')).not.toBeNull());
    fireEvent.click(screen.getByRole('button', { name: '重试' }));
    expect(await screen.findByText('真实客户总数')).not.toBeNull();
    expect(load).toHaveBeenCalledTimes(2);
  });

  it('turns a legacy fallback-route response into a controlled error state', async () => {
    const request = vi.fn(() => Promise.resolve({
      weChatContactNum: 137,
      updateTime: '2026-07-31 09:30:00',
    }));
    renderPage(createDashboardOverviewApi({ request }));

    expect(await screen.findByText('数据概览接口尚未启用')).not.toBeNull();
    expect(screen.queryByText('137')).toBeNull();
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
      startDate: '2026-07-08',
      endDate: '2026-07-20',
      employeeIds: [],
      departmentIds: [],
      period: 'day',
      page: 1,
      pageSize: 20,
    }));
    const refresh = screen.getByRole('button', { name: '刷新' });
    await waitFor(() => expect(refresh.hasAttribute('disabled')).toBe(false));
    fireEvent.click(refresh);
    await waitFor(() => expect(load).toHaveBeenCalledTimes(3));
  });

  it('restores complete filters from the URL, changes the trend period, and paginates', async () => {
    const load = vi.fn(() => Promise.resolve({ ...overview, total: 41 }));
    renderPage({ load, exportCsv: vi.fn(() => Promise.resolve(new Blob())) },
      '/index?startDate=2026-07-01&endDate=2026-07-31&employeeIds=9&employeeIds=12&departmentIds=3&period=week&page=2&pageSize=20');

    await screen.findByText('真实客户总数');
    expect(load).toHaveBeenCalledWith({
      corpId: '7', startDate: '2026-07-01', endDate: '2026-07-31',
      employeeIds: ['9', '12'], departmentIds: ['3'], period: 'week', page: 2, pageSize: 20,
    });
    expect(screen.getByDisplayValue('9,12')).not.toBeNull();
    expect((screen.getByLabelText('趋势周期') as HTMLSelectElement).value).toBe('week');

    fireEvent.click(screen.getByRole('button', { name: '下一页' }));
    await waitFor(() => expect(screen.getByLabelText('当前地址').textContent).toContain('page=3'));
  });

  it('shows an export error without altering the active scoped query', async () => {
    const exportCsv = vi.fn(() => Promise.reject(new Error('导出失败')));
    renderPage({ load: vi.fn(() => Promise.resolve(overview)), exportCsv },
      '/index?startDate=2026-07-01&endDate=2026-07-31&employeeIds=9&departmentIds=3&period=month&page=1&pageSize=20');
    await screen.findByText('真实客户总数');

    fireEvent.click(screen.getByRole('button', { name: '导出 CSV' }));
    expect(await screen.findByText('导出失败')).not.toBeNull();
    expect(exportCsv).toHaveBeenCalledWith({
      corpId: '7', startDate: '2026-07-01', endDate: '2026-07-31',
      employeeIds: ['9'], departmentIds: ['3'], period: 'month', page: 1, pageSize: 20,
    });
  });

  it('rejects a range longer than the backend 31-day window', async () => {
    const load = vi.fn(() => Promise.resolve(overview));
    renderPage({ load });
    await screen.findByText('真实客户总数');

    fireEvent.change(screen.getByLabelText('开始日期'), {
      target: { value: '2026-06-01' },
    });
    fireEvent.change(screen.getByLabelText('结束日期'), {
      target: { value: '2026-07-31' },
    });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));

    expect(await screen.findByText('日期范围最多为 31 天')).not.toBeNull();
    expect(load).toHaveBeenCalledTimes(1);
  });
});
