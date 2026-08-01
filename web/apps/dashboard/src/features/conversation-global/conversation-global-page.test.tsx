import { ApiError } from '@mochat/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type {
  ConversationDetail,
  ConversationGlobalApi,
  ConversationPage,
} from './conversation-global-api';
import { ConversationGlobalPage } from './conversation-global-page';

const access: AccessContext = {
  session: { token: 'Bearer test', userId: '1', corpId: '7', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set(['/chat/v2-all']),
  allowedActions: new Set(),
};

const page: ConversationPage = {
  list: [{
    id: '9:1:31',
    employeeId: 9,
    employeeName: '张三',
    employeeAvatar: '',
    targetType: 'customer',
    targetId: 31,
    targetName: '星河科技',
    targetAvatar: '',
    lastMessage: '请确认报价',
    sentAt: '2026-07-05 11:00:00',
  }],
  total: 61,
  page: 2,
  pageSize: 20,
};

const detail: ConversationDetail = {
  id: '9:1:31',
  employeeId: 9,
  employeeName: '张三',
  targetType: 'customer',
  targetId: 31,
  targetName: '星河科技',
  messageTotal: 1,
  truncated: false,
  window: 'latest',
  messages: [{
    id: 'table:1:17',
    senderName: '张三',
    senderAvatar: '',
    direction: 'outbound',
    type: 1,
    content: { content: '你好' },
    sentAt: '2026-07-05 11:00:00',
  }],
};

afterEach(cleanup);

function LocationProbe() {
  const location = useLocation();
  return <output aria-label="当前地址">{location.pathname}{location.search}</output>;
}

function renderPage(api: ConversationGlobalApi, entry = '/chat/v2-all') {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <MemoryRouter initialEntries={[entry]}>
      <QueryClientProvider client={queryClient}>
        <DashboardAccessProvider value={access}>
          <ConversationGlobalPage api={api} />
          <LocationProbe />
        </DashboardAccessProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

describe('ConversationGlobalPage', () => {
  it('restores filters from the URL, renders real results, and paginates in the URL', async () => {
    const search = vi.fn(() => Promise.resolve(page));
    const { container } = renderPage({ search, detail: vi.fn() },
      '/chat/v2-all?keyword=%E6%8A%A5%E4%BB%B7&employeeId=9&customerId=31&from=2026-07-01&to=2026-07-31&page=2&pageSize=20');

    expect(await screen.findByText('星河科技')).not.toBeNull();
    expect(screen.getByDisplayValue('报价')).not.toBeNull();
    expect(screen.getByText('请确认报价')).not.toBeNull();
    expect(search).toHaveBeenCalledWith({
      corpId: '7',
      keyword: '报价',
      employeeId: '9',
      customerId: '31',
      roomId: '',
      from: '2026-07-01',
      to: '2026-07-31',
      page: 2,
      pageSize: 20,
    });
    expect(container.querySelector('.dashboard-page-header')).not.toBeNull();
    expect(container.querySelector('.dashboard-filter-bar')).not.toBeNull();
    expect(container.querySelector('.dashboard-data-card')).not.toBeNull();
    expect(container.querySelector('.dashboard-table-scroll')).not.toBeNull();
    expect(container.querySelector('.dashboard-table-actions')).not.toBeNull();

    fireEvent.click(screen.getByRole('button', { name: '下一页' }));
    await waitFor(() => expect(screen.getByLabelText('当前地址').textContent).toContain('page=3'));
  });

  it('writes submitted filters to the URL and resets pagination', async () => {
    const search = vi.fn(() => Promise.resolve({ ...page, page: 1 }));
    renderPage({ search, detail: vi.fn() }, '/chat/v2-all?page=4&pageSize=20');
    await screen.findByText('星河科技');

    fireEvent.change(screen.getByLabelText('关键词'), { target: { value: '续约' } });
    fireEvent.change(screen.getByLabelText('员工 ID'), { target: { value: '12' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));

    await waitFor(() => {
      const url = screen.getByLabelText('当前地址').textContent ?? '';
      expect(url).toContain('keyword=%E7%BB%AD%E7%BA%A6');
      expect(url).toContain('employeeId=12');
      expect(url).toContain('page=1');
    });
  });

  it('caps restored page size to the server limit', async () => {
    const search = vi.fn(() => Promise.resolve({ ...page, page: 1, pageSize: 100 }));
    renderPage({ search, detail: vi.fn() }, '/chat/v2-all?page=1&pageSize=1000000');

    await screen.findByText('星河科技');

    expect(search).toHaveBeenCalledWith(expect.objectContaining({ pageSize: 100 }));
  });

  it('lets a stale out-of-range URL return to the first page', async () => {
    renderPage({
      search: vi.fn(() => Promise.resolve({
        list: [], total: 61, page: 99, pageSize: 20,
      })),
      detail: vi.fn(),
    }, '/chat/v2-all?page=99&pageSize=20');

    fireEvent.click(await screen.findByRole('button', { name: '返回第一页' }));

    await waitFor(() => expect(screen.getByLabelText('当前地址').textContent).toContain('page=1'));
  });

  it('shows empty and retryable list states', async () => {
    const search = vi.fn()
      .mockRejectedValueOnce(new Error('网络异常'))
      .mockResolvedValueOnce({ list: [], total: 0, page: 1, pageSize: 20 });
    const { container } = renderPage({ search, detail: vi.fn() });

    await waitFor(() => expect(container.querySelector('.page-state-error')).not.toBeNull());
    fireEvent.click(screen.getByRole('button', { name: '重试' }));
    expect(await screen.findByText('当前筛选条件下暂无会话')).not.toBeNull();
  });

  it('opens the detail drawer and retries without changing list filters', async () => {
    const loadDetail = vi.fn()
      .mockRejectedValueOnce(new Error('详情加载失败'))
      .mockResolvedValueOnce(detail);
    renderPage(
      { search: vi.fn(() => Promise.resolve({ ...page, page: 1 })), detail: loadDetail },
      '/chat/v2-all?keyword=%E6%8A%A5%E4%BB%B7&page=1&pageSize=20',
    );
    await screen.findByText('星河科技');

    fireEvent.click(screen.getByRole('button', { name: '查看会话' }));
    expect(await screen.findByText('详情加载失败')).not.toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '重试详情' }));

    expect(await screen.findByRole('dialog', { name: '会话详情' })).not.toBeNull();
    expect(screen.getByText('你好')).not.toBeNull();
    expect(screen.getByLabelText('当前地址').textContent).toContain('keyword=');
    expect(loadDetail).toHaveBeenCalledTimes(2);
  });

  it('shows a dedicated forbidden state', async () => {
    renderPage({
      search: vi.fn(() => Promise.reject(
        new ApiError('forbidden', 'forbidden', { status: 403 }),
      )),
      detail: vi.fn(),
    });

    expect(await screen.findByText('无权查看当前企业会话')).not.toBeNull();
  });

  it('shows a dedicated non-retryable state when a conversation no longer exists', async () => {
    renderPage({
      search: vi.fn(() => Promise.resolve({ ...page, page: 1 })),
      detail: vi.fn(() => Promise.reject(
        new ApiError('validation', 'conversation not found', { status: 404 }),
      )),
    });
    await screen.findByText('星河科技');

    fireEvent.click(screen.getByRole('button', { name: '查看会话' }));

    expect(await screen.findByText('会话不存在或已无权访问')).not.toBeNull();
    expect(screen.queryByRole('button', { name: '重试详情' })).toBeNull();
  });

  it('labels a truncated detail as the latest message window', async () => {
    renderPage({
      search: vi.fn(() => Promise.resolve({ ...page, page: 1 })),
      detail: vi.fn(() => Promise.resolve({
        ...detail,
        messageTotal: 550,
        truncated: true,
      })),
    });
    await screen.findByText('星河科技');

    fireEvent.click(screen.getByRole('button', { name: '查看会话' }));

    expect(await screen.findByText('当前显示最近 200 条，共 550 条消息')).not.toBeNull();
  });
});
