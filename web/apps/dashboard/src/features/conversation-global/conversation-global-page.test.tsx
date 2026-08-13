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
  session: { token: 'Bearer test', userId: '1', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set(['/chat/v2-all']),
  allowedActions: new Set(),
};

const page: ConversationPage = {
  list: [{
    id: 'msg:archive-31',
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
  id: 'msg:archive-31',
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

function renderPage(
  api: ConversationGlobalApi,
  entry = '/chat/v2-all',
  fixedConversationType?: 'employee' | 'customer' | 'room',
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <MemoryRouter initialEntries={[entry]}>
      <QueryClientProvider client={queryClient}>
        <DashboardAccessProvider value={access}>
          <ConversationGlobalPage
            api={api}
            {...(fixedConversationType === undefined ? {} : { fixedConversationType })}
          />
          <LocationProbe />
        </DashboardAccessProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

describe('ConversationGlobalPage', () => {
  it('locks a scoped page to its menu conversation type', async () => {
    const search = vi.fn(() => Promise.resolve({ ...page, page: 1 }));
    renderPage({ search, detail: vi.fn() }, '/chat/v2-staff?page=1&pageSize=20', 'employee');

    await screen.findByText('星河科技');

    expect(search).toHaveBeenCalledWith(expect.objectContaining({ conversationType: 'employee' }));
    expect(screen.getByRole('heading', { name: '员工会话' })).not.toBeNull();
    expect(screen.queryByRole('button', { name: '客户会话' })).toBeNull();
  });

  it('shows honest query summaries and switches conversation type from quick filters', async () => {
    const search = vi.fn(() => Promise.resolve({ ...page, page: 1 }));
    const { container } = renderPage({ search, detail: vi.fn() }, '/chat/v2-all?page=1&pageSize=20');

    expect((await screen.findByRole('region', { name: '查询概览' })).textContent).toContain('61');
    expect(screen.getByText('会话总量')).not.toBeNull();
    expect(screen.getByText('当前页会话')).not.toBeNull();
    expect(container.querySelector('.conversation-global-overview')).not.toBeNull();
    expect(container.querySelector('.conversation-global-type-tabs')).not.toBeNull();

    fireEvent.click(screen.getByRole('button', { name: '客户会话' }));

    await waitFor(() => {
      expect(screen.getByLabelText('当前地址').textContent).toContain('conversationType=customer');
      expect(screen.getByLabelText('当前地址').textContent).toContain('page=1');
    });
  });

  it('refreshes the current query without changing its URL filters', async () => {
    const search = vi.fn(() => Promise.resolve({ ...page, page: 1 }));
    renderPage(
      { search, detail: vi.fn() },
      '/chat/v2-all?keyword=%E6%8A%A5%E4%BB%B7&page=1&pageSize=20',
    );
    await screen.findByText('星河科技');

    fireEvent.click(screen.getByRole('button', { name: '刷新消息' }));

    await waitFor(() => expect(search).toHaveBeenCalledTimes(2));
    expect(screen.getByLabelText('当前地址').textContent).toContain('keyword=');
  });

  it('restores filters from the URL, renders real results, and paginates in the URL', async () => {
    const search = vi.fn(() => Promise.resolve(page));
    const { container } = renderPage({ search, detail: vi.fn() },
      '/chat/v2-all?keyword=%E6%8A%A5%E4%BB%B7&conversationType=customer&employeeIds=9&employeeIds=12&startAt=2026-07-01&endAt=2026-07-31&page=2&pageSize=20');

    expect(await screen.findByText('星河科技')).not.toBeNull();
    expect(screen.getByDisplayValue('报价')).not.toBeNull();
    expect(screen.getByText('请确认报价')).not.toBeNull();
    expect(search).toHaveBeenCalledWith({
      keyword: '报价',
      conversationType: 'customer',
      employeeIds: ['9', '12'],
      startAt: '2026-07-01',
      endAt: '2026-07-31',
      page: 2,
      pageSize: 20,
    });
    expect(container.querySelector('.dashboard-page-header')).not.toBeNull();
    expect(container.querySelector('.dashboard-filter-bar')).not.toBeNull();
    expect(container.querySelector('.dashboard-data-card')).not.toBeNull();
    expect(container.querySelector('.dashboard-table-scroll')).not.toBeNull();
    expect(screen.getByRole('navigation', { name: '分页' })).not.toBeNull();

    fireEvent.click(screen.getByRole('button', { name: '下一页' }));
    await waitFor(() => expect(screen.getByLabelText('当前地址').textContent).toContain('page=3'));
  });

  it('writes submitted filters to the URL and resets pagination', async () => {
    const search = vi.fn(() => Promise.resolve({ ...page, page: 1 }));
    renderPage({ search, detail: vi.fn() }, '/chat/v2-all?page=4&pageSize=20');
    await screen.findByText('星河科技');

    fireEvent.change(screen.getByLabelText('关键词'), { target: { value: '续约' } });
    fireEvent.change(screen.getByLabelText('会话对象类型'), { target: { value: 'room' } });
    fireEvent.change(screen.getByLabelText('员工 ID'), { target: { value: '12,15' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));

    await waitFor(() => {
      const url = screen.getByLabelText('当前地址').textContent ?? '';
      expect(url).toContain('keyword=%E7%BB%AD%E7%BA%A6');
      expect(url).toContain('conversationType=room');
      expect(url).toContain('employeeIds=12');
      expect(url).toContain('employeeIds=15');
      expect(url).toContain('page=1');
    });
  });

  it('resets every persisted filter and pagination value', async () => {
    renderPage({ search: vi.fn(() => Promise.resolve({ ...page, page: 4 })), detail: vi.fn() },
      '/chat/v2-all?keyword=%E6%8A%A5%E4%BB%B7&conversationType=room&employeeIds=9&startAt=2026-07-01&endAt=2026-07-31&page=4&pageSize=50');
    await screen.findByText('星河科技');

    fireEvent.click(screen.getByRole('button', { name: '重置' }));

    await waitFor(() => {
      expect(screen.getByLabelText('当前地址').textContent).toBe('/chat/v2-all?page=1&pageSize=20');
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

  it('distinguishes archive authorization from RBAC forbidden', async () => {
    renderPage({
      search: vi.fn(() => Promise.reject(
        new ApiError('forbidden', 'archive not authorized', { status: 403, code: 40301 }),
      )),
      detail: vi.fn(),
    });

    const state = (await screen.findByText('会话归档未开通')).closest('[role="status"]');
    expect(state?.textContent).toContain('会话归档未开通');
    expect(screen.getByRole('link', { name: '去配置会话归档' }).getAttribute('href')).toBe('/company-setting/website');
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

  it('shows the honest group participant identity limitation', async () => {
    const roomPage: ConversationPage = {
      ...page,
      page: 1,
      list: page.list.map((item) => ({
        ...item,
        targetType: 'room',
        targetName: '客户群',
      })),
    };
    renderPage({
      search: vi.fn(() => Promise.resolve(roomPage)),
      detail: vi.fn(),
    }, '/chat/v2-all?conversationType=room&page=1&pageSize=20');

    expect(await screen.findByText('群聊入站消息暂无法识别具体群成员，详情中统一显示“群成员”。')).not.toBeNull();
  });

  it('shows a non-retryable forbidden state when detail permission is revoked', async () => {
    renderPage({
      search: vi.fn(() => Promise.resolve({ ...page, page: 1 })),
      detail: vi.fn(() => Promise.reject(
        new ApiError('forbidden', 'forbidden', { status: 403 }),
      )),
    });
    await screen.findByText('星河科技');

    fireEvent.click(screen.getByRole('button', { name: '查看会话' }));

    expect(await screen.findByText('无权读取会话详情')).not.toBeNull();
    expect(screen.queryByRole('button', { name: '重试详情' })).toBeNull();
  });

  it('keeps the archive authorization state consistent for detail requests', async () => {
    renderPage({
      search: vi.fn(() => Promise.resolve({ ...page, page: 1 })),
      detail: vi.fn(() => Promise.reject(
        new ApiError('forbidden', 'archive not authorized', { status: 403, code: 40301 }),
      )),
    });
    await screen.findByText('星河科技');

    fireEvent.click(screen.getByRole('button', { name: '查看会话' }));

    expect(await screen.findByText('当前企业未开通会话内容存档')).not.toBeNull();
    expect(screen.queryByRole('button', { name: '重试详情' })).toBeNull();
  });
});
