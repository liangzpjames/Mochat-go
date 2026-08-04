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

afterEach(() => { cleanup(); vi.restoreAllMocks(); });

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
    expect(screen.getByRole('button', { name: '新建群发' })).toHaveProperty('disabled', false);
    fireEvent.change(screen.getByLabelText('任务名称'), { target: { value: '夏日' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(read).toHaveBeenLastCalledWith('/contactMessageBatchSend/index', expect.objectContaining({ batchTitle: '夏日' })));

    fireEvent.click(screen.getByRole('tab', { name: '群聊群发' }));
    await waitFor(() => expect(read).toHaveBeenLastCalledWith('/roomMessageBatchSend/index', expect.objectContaining({ page: 1, perPage: 20 })));
  });

  it('creates an immediate customer precise-send task through the real provider', async () => {
    const read = vi.fn().mockResolvedValue({ list: [] });
    const write = vi.fn().mockResolvedValue(undefined);
    view(<PreciseGroupSendPage api={{ read, write }} />);

    await screen.findByRole('heading', { name: '暂无群发任务' });
    fireEvent.click(screen.getByRole('button', { name: '新建群发' }));
    expect(screen.getByLabelText('新建群发任务')).toBeTruthy();
    fireEvent.change(screen.getAllByLabelText('任务名称')[1]!, { target: { value: '夏日客户触达' } });
    fireEvent.change(screen.getByLabelText('发送成员ID'), { target: { value: '99' } });
    fireEvent.change(screen.getByLabelText('群发内容'), { target: { value: '欢迎参与夏日活动' } });
    fireEvent.click(screen.getByRole('button', { name: '保存并发送' }));

    await waitFor(() => expect(write).toHaveBeenCalledWith(
      '/contactMessageBatchSend/store',
      { batchTitle: '夏日客户触达', employeeIds: [99], sendWay: 1, filterParams: {}, content: [{ msgType: 'text', content: '欢迎参与夏日活动' }] },
      'POST',
    ));
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

  it('queries real friends-circle tasks and materials and persists a draft', async () => {
    const read = vi.fn()
      .mockResolvedValueOnce({ list: [{ id: 21, taskName: '夏日朋友圈', sendWay: 'manual', status: 'draft', completedTotal: 0, targetTotal: 8, creatorName: '运营员', createdAt: '2026-08-04 14:00' }] })
      .mockResolvedValueOnce({ list: [{ id: 31, name: '新品海报', content: '{"text":"新品正文摘要"}', type: 'image', status: 'available', creatorName: '运营员', createdAt: '2026-08-04 14:10' }] })
      .mockResolvedValue({ list: [] });
    const write = vi.fn().mockResolvedValue(undefined);
    view(<FriendsCirclePage api={{ read, write }} />);

    expect(screen.getByRole('tab', { name: '朋友圈' })).toBeTruthy();
    expect(screen.getByRole('tab', { name: '朋友圈素材' })).toBeTruthy();
    expect(await screen.findByText('夏日朋友圈')).toBeTruthy();
    expect(read).toHaveBeenCalledWith('/friendsCircle/taskIndex', expect.objectContaining({ page: 1, perPage: 20 }));
    expect(screen.getByLabelText('任务名称')).toBeTruthy();
    expect(screen.getByText('发送方式')).toBeTruthy();
    expect(screen.getByText('完成情况')).toBeTruthy();
    fireEvent.click(screen.getByRole('tab', { name: '朋友圈素材' }));
    expect(await screen.findByText('新品海报')).toBeTruthy();
    expect(screen.getByText('新品正文摘要')).toBeTruthy();
    expect(read).toHaveBeenLastCalledWith('/friendsCircle/materialIndex', expect.objectContaining({ page: 1, perPage: 20 }));
    fireEvent.click(screen.getByRole('tab', { name: '朋友圈' }));
    fireEvent.click(screen.getByRole('button', { name: '添加朋友圈' }));
    fireEvent.change(screen.getByLabelText('草稿名称'), { target: { value: '秋日活动' } });
    fireEvent.change(screen.getByLabelText('草稿内容'), { target: { value: '欢迎参与' } });
    fireEvent.click(screen.getByRole('button', { name: '保存草稿' }));
    await waitFor(() => expect(write).toHaveBeenCalledWith('/friendsCircle/taskStore', expect.objectContaining({ taskName: '秋日活动', content: '欢迎参与', sendWay: 'manual' })));
    expect(screen.getByRole('button', { name: '导出' })).toHaveProperty('disabled', true);
    expect(screen.getByText('发布 Provider 未配置')).toBeTruthy();
  });

  it('reuses a real material provider in the friends-circle composer', async () => {
    const read = vi.fn().mockImplementation((endpoint: string) => endpoint === '/materialSelector/index'
      ? Promise.resolve({ list: [{ id: 45, name: '新品文案', preview: '新品今天上线' }] })
      : Promise.resolve({ list: [] }));
    view(<FriendsCirclePage api={{ read, write: vi.fn() }} />);

    await screen.findByRole('heading', { name: '还没有朋友圈任务' });
    fireEvent.click(screen.getByRole('button', { name: '添加朋友圈' }));
    expect(await screen.findByRole('option', { name: '新品文案' })).toBeTruthy();
    fireEvent.change(screen.getByLabelText('引用素材'), { target: { value: '45' } });
    expect(screen.getByLabelText('草稿内容')).toHaveProperty('value', '新品今天上线');
    expect(read).toHaveBeenCalledWith('/materialSelector/index', { scene: 'friends_circle' });
  });

  it('shows persistent publish failure, loads target failures, and exports scoped results', async () => {
    const read = vi.fn().mockImplementation((endpoint: string) => {
      if (endpoint === '/friendsCircle/taskResultIndex') return Promise.resolve({ list: [{ id: 41, taskId: 21, targetEmployeeId: 99, status: 'failed', failureCode: 'E_TIMEOUT', failureReason: '发送超时', occurredAt: '2026-08-04 15:00' }] });
      if (endpoint === '/friendsCircle/exportData') return Promise.resolve({ list: [{ taskId: 21, targetEmployeeId: 99, status: 'failed', failureCode: 'E_TIMEOUT', failureReason: '发送超时', occurredAt: '2026-08-04 15:00' }] });
      return Promise.resolve({ list: [{ id: 21, taskName: '夏日朋友圈', sendWay: 'manual', status: 'draft', completedTotal: 0, targetTotal: 1, creatorName: '运营员', createdAt: '2026-08-04 14:00' }] });
    });
    const write = vi.fn().mockRejectedValue(new Error('朋友圈发布 Provider 未配置'));
    view(<FriendsCirclePage api={{ read, write }} />);

    expect(await screen.findByText('夏日朋友圈')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '发起发布' }));
    expect((await screen.findByRole('alert')).textContent).toContain('朋友圈发布 Provider 未配置');
    fireEvent.click(screen.getByRole('button', { name: '查看进度' }));
    expect(await screen.findByLabelText('朋友圈任务进度')).toBeTruthy();
    expect(await screen.findByText('E_TIMEOUT')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '导出失败明细' }));
    await waitFor(() => expect(read).toHaveBeenCalledWith('/friendsCircle/exportData', { taskId: 21, status: 'failed' }));
  });

  it('isolates friends-circle query cache when the selected corp changes', async () => {
    const read = vi.fn().mockResolvedValueOnce({ list: [{ id: 1, taskName: 'corp-seven-draft' }] }).mockResolvedValueOnce({ list: [{ id: 2, taskName: 'corp-eight-draft' }] });
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const page = (value: AccessContext) => (
      <MemoryRouter>
        <QueryClientProvider client={client}>
          <DashboardAccessProvider value={value}><FriendsCirclePage api={{ read, write: vi.fn() }} /></DashboardAccessProvider>
        </QueryClientProvider>
      </MemoryRouter>
    );
    const rendered = render(page(access));
    expect(await screen.findByText('corp-seven-draft')).toBeTruthy();
    rendered.rerender(page({ ...access, session: { ...access.session, corpId: '8' }, corp: { ...access.corp, id: '8', name: 'corp-eight' } }));
    expect(await screen.findByText('corp-eight-draft')).toBeTruthy();
    expect(read).toHaveBeenCalledTimes(2);
  });

  it('supports keyboard filtering and protects unsaved drawer content', async () => {
    const read = vi.fn().mockResolvedValue({ list: [] });
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
    view(<FriendsCirclePage api={{ read, write: vi.fn() }} />);

    await screen.findByRole('heading', { name: '还没有朋友圈任务' });
    const reset = screen.getByRole('button', { name: '重置' });
    expect(reset).toHaveProperty('disabled', true);
    fireEvent.change(screen.getByLabelText('任务名称'), { target: { value: '秋日' } });
    fireEvent.keyDown(screen.getByLabelText('任务名称'), { key: 'Enter' });
    await waitFor(() => expect(read).toHaveBeenLastCalledWith('/friendsCircle/taskIndex', expect.objectContaining({ taskName: '秋日' })));

    const addButton = screen.getByRole('button', { name: '添加朋友圈' });
    addButton.focus();
    fireEvent.click(addButton);
    expect(screen.getByLabelText('朋友圈草稿').getAttribute('data-variant')).toBe('task');
    expect(document.activeElement).toBe(screen.getByLabelText('草稿名称'));
    expect(screen.getByLabelText('草稿名称').hasAttribute('required')).toBe(true);
    expect(screen.getByText('0 / 500')).toBeTruthy();
    fireEvent.change(screen.getByLabelText('草稿名称'), { target: { value: '未保存草稿' } });
    fireEvent.click(screen.getByRole('button', { name: '关闭新增面板' }));
    expect(confirm).toHaveBeenCalledOnce();
    expect(screen.getByLabelText('朋友圈草稿')).toBeTruthy();
    confirm.mockReturnValue(true);
    fireEvent.click(screen.getByRole('button', { name: '关闭新增面板' }));
    expect(screen.queryByLabelText('朋友圈草稿')).toBeNull();
    expect(document.activeElement).toBe(addButton);

    fireEvent.click(screen.getByRole('button', { name: '添加朋友圈' }));
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByLabelText('朋友圈草稿')).toBeNull();
  });

  it('locks composer fields while a draft save is in flight', async () => {
    let finishSave: (() => void) | undefined;
    const write = vi.fn().mockImplementation(() => new Promise<void>((resolve) => { finishSave = resolve; }));
    view(<FriendsCirclePage api={{ read: vi.fn().mockResolvedValue({ list: [] }), write }} />);

    await screen.findByRole('heading', { name: '还没有朋友圈任务' });
    fireEvent.click(screen.getByRole('button', { name: '添加朋友圈' }));
    fireEvent.change(screen.getByLabelText('草稿名称'), { target: { value: '保存中草稿' } });
    fireEvent.change(screen.getByLabelText('草稿内容'), { target: { value: '保存期间不可编辑' } });
    fireEvent.click(screen.getByRole('button', { name: '保存草稿' }));

    await waitFor(() => expect(write).toHaveBeenCalledOnce());
    expect(screen.getByLabelText('草稿名称')).toHaveProperty('disabled', true);
    expect(screen.getByLabelText('草稿内容')).toHaveProperty('disabled', true);
    expect(fireEvent.keyDown(document, { key: 'Tab' })).toBe(false);
    expect(document.activeElement).toBe(screen.getByRole('dialog', { name: '添加朋友圈草稿' }));
    finishSave?.();
    await waitFor(() => expect(screen.queryByLabelText('朋友圈草稿')).toBeNull());
  });
});
