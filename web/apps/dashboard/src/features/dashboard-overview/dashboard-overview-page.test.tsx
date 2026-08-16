import { ApiError } from '@mochat/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type {
  DashboardOverview,
  DashboardOverviewApi,
  DashboardOverviewQuery,
} from './dashboard-overview-api';
import { createDashboardOverviewApi } from './dashboard-overview-api';
import { DashboardOverviewPage, parseAISummary } from './dashboard-overview-page';
import type { Phase35Api } from '../phase35/api';

const access: AccessContext = {
  session: { token: 'Bearer test', userId: '1', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set(['/index']),
  allowedActions: new Set(),
};

const range = { from: '2026-07-01', to: '2026-07-31' };

function trendRangeForTest(): { trendStartDate: string; trendEndDate: string } {
  const now = new Date();
  const format = (date: Date) => {
    const parts = new Intl.DateTimeFormat('en-US', {
      timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit',
    }).formatToParts(date);
    const values = Object.fromEntries(parts.map((part) => [part.type, part.value]));
    return `${values.year}-${values.month}-${values.day}`;
  };
  const from = new Date(now);
  from.setDate(from.getDate() - 6);
  const to = new Date(now);
  to.setDate(to.getDate() + 1);
  return { trendStartDate: format(from), trendEndDate: format(to) };
}

const overview: DashboardOverview = {
  cards: [
    { key: 'customer', label: '客户总数', value: 137 },
    { key: 'lead', label: '线索总数', value: 58 },
    { key: 'order', label: '订单总数', value: 12 },
    { key: 'behavior', label: '行为事件', value: 41 },
  ],
  trend: [{ date: '2026-07-30', addCustomerNum: 12 }],
  summary: {
    customer: 137, lead: 58, contact: 46, opportunity: 30, won: 15, order: 12, behavior: 41, employee: 0,
  },
  limitations: [],
  updatedAt: '2026-07-31 09:30:00',
  aiInsight: {
    capability: 'ready',
    provider: 'dashscope',
    summary: '✅ **核心客户意图识别：**\n1. **商务洽谈与价格协商**（高频出现）：\n   - 明确议价（第7条）\n   - 索要报价单（第15条）\n\n---\n\n✅ **跟进建议：**\n1. **立即响应并闭环技术问题**：针对第17条\n2. **分层推进商务决策**：今日内发送报价单\n\n（所有建议均严格依据归档文本）',
    generatedAt: '2026-08-15T23:59:59+08:00',
  },
  conversation: {
    customer: { sessions: 11, employeeMessages: 22, customerMessages: 16 },
    room: { sessions: 4, employeeMessages: 6, customerMessages: 6 },
    trend: [
      { date: '2026-08-10', customerSessions: 1, customerEmployeeMessages: 1, customerCustomerMessages: 1, roomSessions: 0, roomEmployeeMessages: 0, roomCustomerMessages: 0 },
      { date: '2026-08-11', customerSessions: 1, customerEmployeeMessages: 1, customerCustomerMessages: 1, roomSessions: 0, roomEmployeeMessages: 0, roomCustomerMessages: 0 },
      { date: '2026-08-12', customerSessions: 1, customerEmployeeMessages: 1, customerCustomerMessages: 0, roomSessions: 0, roomEmployeeMessages: 0, roomCustomerMessages: 0 },
      { date: '2026-08-13', customerSessions: 1, customerEmployeeMessages: 1, customerCustomerMessages: 1, roomSessions: 0, roomEmployeeMessages: 0, roomCustomerMessages: 0 },
      { date: '2026-08-14', customerSessions: 1, customerEmployeeMessages: 1, customerCustomerMessages: 1, roomSessions: 0, roomEmployeeMessages: 0, roomCustomerMessages: 0 },
      { date: '2026-08-15', customerSessions: 1, customerEmployeeMessages: 1, customerCustomerMessages: 1, roomSessions: 2, roomEmployeeMessages: 2, roomCustomerMessages: 2 },
      { date: '2026-08-16', customerSessions: 1, customerEmployeeMessages: 3, customerCustomerMessages: 0, roomSessions: 0, roomEmployeeMessages: 0, roomCustomerMessages: 0 },
    ],
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
  optionsApi?: Phase35Api,
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <MemoryRouter initialEntries={[entry]}>
      <QueryClientProvider client={queryClient}>
        <DashboardAccessProvider value={access}>
          <DashboardOverviewPage api={{ exportCsv: vi.fn(() => Promise.resolve(new Blob())), ...api }} initialRange={range} optionsApi={optionsApi} />
          <LocationProbe />
        </DashboardAccessProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

describe('DashboardOverviewPage', () => {
  it('uses the shared accessible dashboard pagination contract', async () => {
    const api = {
      load: vi.fn().mockResolvedValue({
        ...overview,
        total: 25,
        trend: [{ date: '2026-08-13', addCustomerNum: 2 }],
      }),
      exportCsv: vi.fn(() => Promise.resolve(new Blob())),
    };
    renderPage(api);
    const pagination = await screen.findByRole('navigation', { name: '分页' });
    expect(within(pagination).getByText('共 25 条')).toBeTruthy();
    expect(within(pagination).getByRole('button', { name: '第 1 页' }).getAttribute('aria-current')).toBe('page');
  });
  it('removes the advanced range filter and relies on the principal scope', async () => {
    const optionsApi = { read: vi.fn(), write: vi.fn() };
    const load = vi.fn().mockResolvedValue(overview);
    renderPage({ load }, '/index?employeeIds=9&departmentIds=3', optionsApi);
    await screen.findByText('客户总数');
    expect(screen.queryByText('高级范围筛选')).toBeNull();
    expect(screen.queryByLabelText('员工范围')).toBeNull();
    expect(screen.queryByLabelText('部门范围')).toBeNull();
    expect(optionsApi.read).not.toHaveBeenCalled();
    expect(load).toHaveBeenCalledWith(expect.objectContaining({
      employeeIds: [],
      departmentIds: [],
    }));
  });

  it('builds the default range in the explicit enterprise timezone', async () => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(new Date('2026-08-01T16:30:00Z'));
    vi.stubEnv('TZ', 'UTC');
    const load = vi.fn((_input: DashboardOverviewQuery) => Promise.resolve(overview));

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

    await screen.findByText('客户总数');
    expect(load).toHaveBeenCalledWith(expect.objectContaining({
      startDate: '2026-08-01',
      endDate: '2026-09-01',
    }));
    const firstCall = load.mock.calls[0]?.[0];
    expect(firstCall).not.toHaveProperty('period');
  });

  it('shows loading, then renders cards and trend values from the API response', async () => {
    let resolve: ((value: DashboardOverview) => void) | undefined;
    const load = vi.fn(() => new Promise<DashboardOverview>((accept) => {
      resolve = accept;
    }));
    const { container } = renderPage({ load });

    expect(screen.getAllByRole('status')[0]?.textContent).toContain('正在加载数据概览');
    resolve?.(overview);

    expect(await screen.findByText('客户总数')).not.toBeNull();
    expect(screen.getByText('137')).not.toBeNull();
    expect(screen.getAllByText('2026-07-30')).not.toHaveLength(0);
    expect(screen.getByLabelText('新增客户 12')).not.toBeNull();
    expect(screen.getByText(/2026-07-31 09:30:00/)).not.toBeNull();
    expect(load).toHaveBeenCalledWith({
      corpId: '7',
      startDate: range.from,
      endDate: range.to,
      ...trendRangeForTest(),
      employeeIds: [],
      departmentIds: [],
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
        summary: {
          customer: 0, lead: 0, contact: 0, opportunity: 0, won: 0, order: 0, behavior: 0, employee: 0,
        },
        limitations: [],
        updatedAt: '',
        page: 1,
        pageSize: 20,
        total: 0,
      })),
    });

    expect(await screen.findByRole('heading', { name: '经营概览' })).not.toBeNull();
    expect(screen.getByRole('heading', { name: '数据概览' })).not.toBeNull();
    expect(screen.getByRole('heading', { name: 'AI 洞察' })).not.toBeNull();
    expect(screen.getByRole('heading', { name: '经营趋势明细' })).not.toBeNull();
    expect(screen.getByRole('heading', { name: '会话数据' })).not.toBeNull();
    expect(screen.queryByRole('heading', { name: '质检数据' })).toBeNull();
    expect(screen.queryByRole('heading', { name: '员工会话数据排行' })).toBeNull();
    expect(screen.queryByRole('heading', { name: '员工会话轨迹一览' })).toBeNull();
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
    expect(await screen.findByText('客户总数')).not.toBeNull();
    expect(load).toHaveBeenCalledTimes(2);
  });

  it('turns a legacy fallback-route response into a controlled error state', async () => {
    const request = vi.fn(() => Promise.resolve({
      weChatContactNum: 137,
      updateTime: '2026-07-31 09:30:00',
    }));
    renderPage(createDashboardOverviewApi({ request }));

    expect(await screen.findByText('数据概览接口返回了无效数据')).not.toBeNull();
    expect(screen.queryByText('137')).toBeNull();
  });

  it('applies a new date range and refreshes through the injected API', async () => {
    const load = vi.fn(() => Promise.resolve(overview));
    renderPage({ load });
    await screen.findByText('客户总数');

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
      ...trendRangeForTest(),
      employeeIds: [],
      departmentIds: [],
      page: 1,
      pageSize: 20,
    }));
    const refresh = screen.getByRole('button', { name: '刷新' });
    await waitFor(() => expect(refresh.hasAttribute('disabled')).toBe(false));
    fireEvent.click(refresh);
    await waitFor(() => expect(load).toHaveBeenCalledTimes(3));
  });

  it('keeps the date range from the URL, ignores legacy employee filters and paginates', async () => {
    const load = vi.fn(() => Promise.resolve({ ...overview, total: 41 }));
    renderPage({ load, exportCsv: vi.fn(() => Promise.resolve(new Blob())) },
      '/index?startDate=2026-07-01&endDate=2026-08-01&employeeIds=9&employeeIds=12&departmentIds=3&page=2&pageSize=20');

    await screen.findByText('客户总数');
    expect(load).toHaveBeenCalledWith({
      corpId: '7', startDate: '2026-07-01', endDate: '2026-08-01',
      ...trendRangeForTest(),
      employeeIds: [], departmentIds: [], page: 2, pageSize: 20,
    });
    expect(screen.queryByLabelText('员工范围')).toBeNull();
    expect(screen.queryByLabelText('趋势周期')).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: '下一页' }));
    await waitFor(() => expect(screen.getByLabelText('当前地址').textContent).toContain('page=3'));
  });

  it('shows an export error without altering the active scoped query', async () => {
    const exportCsv = vi.fn(() => Promise.reject(new Error('导出失败')));
    renderPage({ load: vi.fn(() => Promise.resolve(overview)), exportCsv },
      '/index?startDate=2026-07-01&endDate=2026-08-01&employeeIds=9&departmentIds=3&page=1&pageSize=20');
    await screen.findByText('客户总数');

    fireEvent.click(screen.getByRole('button', { name: '导出 CSV' }));
    expect(await screen.findByText('导出失败')).not.toBeNull();
    expect(exportCsv).toHaveBeenCalledWith({
      corpId: '7', startDate: '2026-07-01', endDate: '2026-08-01',
      ...trendRangeForTest(),
      employeeIds: [], departmentIds: [], page: 1, pageSize: 20,
    });
  });

  it('rejects a range longer than the backend 31-day window', async () => {
    const load = vi.fn(() => Promise.resolve(overview));
    renderPage({ load });
    await screen.findByText('客户总数');

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

  it('links the conversation selection to the right trend chart and detail table', async () => {
    renderPage({ load: vi.fn(() => Promise.resolve(overview)) });
    await screen.findByText('客户总数');

    expect(screen.getByRole('img', { name: '近七日客户会话趋势' })).not.toBeNull();
    expect(screen.getByRole('heading', { name: '客户会话 · 近七日趋势明细' })).not.toBeNull();
    expect(screen.queryByLabelText('会话数 2')).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: /客户群/ }));

    expect(screen.getByRole('img', { name: '近七日客户群趋势' })).not.toBeNull();
    expect(screen.getByRole('heading', { name: '客户群 · 近七日趋势明细' })).not.toBeNull();
    expect(screen.getByLabelText('会话数 2')).not.toBeNull();
    const roomRow = screen.getByText('2026-08-15').closest('tr');
    const roomCells = [...(roomRow?.querySelectorAll('td') ?? [])].map((cell) => cell.textContent);
    expect(roomCells).toEqual(['2026-08-15', '2', '2', '2']);
  });

  it('renders the AI insight summary with structured sections', async () => {
    renderPage({ load: vi.fn(() => Promise.resolve(overview)) });
    await screen.findByText('客户总数');

    expect(screen.getByText('核心客户意图识别：')).not.toBeNull();
    expect(screen.getByText('商务洽谈与价格协商')).not.toBeNull();
    expect(screen.getByText('明确议价（第7条）')).not.toBeNull();
    expect(screen.getByText('立即响应并闭环技术问题')).not.toBeNull();
  });

  it('parses AI summary markdown-ish lines into structured blocks', () => {
    expect(parseAISummary('---\n✅ **标题**\n1. 第一项\n   - 子项\n正文')).toEqual([
      { kind: 'divider' },
      { kind: 'section', text: '✅ **标题**' },
      { kind: 'numbered', index: '1', text: '第一项' },
      { kind: 'bullet', text: '子项' },
      { kind: 'paragraph', text: '正文' },
    ]);
  });
});
