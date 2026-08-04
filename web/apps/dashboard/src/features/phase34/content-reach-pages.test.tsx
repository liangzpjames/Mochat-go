import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import { FriendsCirclePage, PreciseGroupSendPage } from './content-reach-pages';

const access: AccessContext = {
  session: { token: 'token', userId: '1', corpId: '7', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set(['/acquisition/precise-group-send', '/acquisition/friends-circle']),
  allowedActions: new Set(),
};

afterEach(cleanup);

function view(page: React.ReactNode) {
  return render(
    <MemoryRouter>
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <DashboardAccessProvider value={access}>{page}</DashboardAccessProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

describe('Phase 3.4 content-reach pages', () => {
  it('queries both real precise-send providers and applies the task-title filter', async () => {
    const read = vi.fn().mockResolvedValue({ list: [{ id: 8, batchTitle: '夏日活动', content: [{ type: 'text', content: '欢迎参与' }], sendStatus: 1, sendTotal: 20, receivedTotal: 12, createdAt: '2026-08-04 10:00' }] });
    view(<PreciseGroupSendPage api={{ read, write: vi.fn() }} />);

    expect(await screen.findByText('欢迎参与')).toBeTruthy();
    expect(read).toHaveBeenCalledWith('/contactMessageBatchSend/index', expect.objectContaining({ page: 1, perPage: 20 }));
    expect(screen.getByRole('button', { name: '新建群发' })).toHaveProperty('disabled', true);
    fireEvent.change(screen.getByLabelText('任务名称'), { target: { value: '夏日' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(read).toHaveBeenLastCalledWith('/contactMessageBatchSend/index', expect.objectContaining({ batchTitle: '夏日' })));

    fireEvent.click(screen.getByRole('tab', { name: '群聊群发' }));
    await waitFor(() => expect(read).toHaveBeenLastCalledWith('/roomMessageBatchSend/index', expect.objectContaining({ page: 1, perPage: 20 })));
  });

  it('shows execution data, details, and retries a failed precise-send query', async () => {
    const read = vi.fn().mockRejectedValueOnce(new Error('provider unavailable')).mockResolvedValueOnce({ list: [{ id: 8, content: [{ content: '恢复后的任务' }], sendTotal: 20, receivedTotal: 12 }] });
    view(<PreciseGroupSendPage api={{ read, write: vi.fn() }} />);

    await screen.findByRole('heading', { name: '加载失败' });
    fireEvent.click(screen.getByRole('button', { name: '重新加载' }));
    expect(await screen.findByText('恢复后的任务')).toBeTruthy();
    expect(screen.getByText('20 / 12')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '详情' }));
    expect(screen.getByLabelText('群发任务详情').textContent).toContain('执行数据');
  });

  it('renders the observed friends-circle structure without calling a missing provider', () => {
    const read = vi.fn();
    const write = vi.fn();
    view(<FriendsCirclePage api={{ read, write }} />);

    expect(screen.getByRole('tab', { name: '朋友圈' })).toBeTruthy();
    expect(screen.getByRole('tab', { name: '朋友圈素材' })).toBeTruthy();
    expect(screen.getByLabelText('任务名称')).toBeTruthy();
    expect(screen.getByLabelText('发送方式')).toBeTruthy();
    expect(screen.getByText('完成情况')).toBeTruthy();
    expect(screen.getByRole('button', { name: '添加朋友圈' })).toHaveProperty('disabled', true);
    expect(screen.getByRole('button', { name: '导出' })).toHaveProperty('disabled', true);
    expect(screen.getByText('朋友圈 Provider 未配置')).toBeTruthy();
    expect(read).not.toHaveBeenCalled();
    expect(write).not.toHaveBeenCalled();
  });
});
