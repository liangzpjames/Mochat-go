/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';
import {
  ChannelCodePage,
  GroupCodePage,
  LiveCodeShortChainPage,
} from './acquisition-pages';

const access: AccessContext = {
  session: { token: 'token', userId: '1', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set([
    '/acquisition/v2-channel-code',
    '/acquisition/group-code',
    '/acquisition/live-code-short-chain',
  ]),
  allowedActions: new Set(),
};

beforeEach(() => {
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} });
});
afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

function view(page: React.ReactNode, allowedActions = access.allowedActions) {
  return render(
    <MemoryRouter>
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <DashboardAccessProvider value={{ ...access, allowedActions }}>
          {page}
        </DashboardAccessProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

describe('Phase 3.4 acquisition pages', () => {
  it('loads channel codes from the existing dashboard provider and applies a name filter', async () => {
    const read = vi.fn().mockResolvedValue({
      list: [{ channelCodeId: 12, name: '官网咨询', employeeNames: ['销售一部'], contactNum: 8 }],
    });
    const api: BusinessWorkbenchApi = { read, write: vi.fn() };
    view(<ChannelCodePage api={api} />);

    expect(await screen.findByText('官网咨询')).toBeTruthy();
    expect(read).toHaveBeenCalledWith('/channelCode/index', expect.objectContaining({ page: 1, perPage: 20 }));
    fireEvent.change(screen.getByLabelText('活码名称'), { target: { value: '官网' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));

    await waitFor(() => expect(read).toHaveBeenLastCalledWith(
      '/channelCode/index',
      expect.objectContaining({ name: '官网', page: 1, perPage: 20 }),
    ));
  });

  it('shows channel code details using provider data', async () => {
    const api: BusinessWorkbenchApi = {
      read: vi.fn().mockResolvedValue({ list: [{ channelCodeId: 12, name: '官网咨询', contactNum: 8 }] }),
      write: vi.fn(),
    };
    view(<ChannelCodePage api={api} />);

    await screen.findByText('官网咨询');
    fireEvent.click(screen.getByRole('button', { name: '详情' }));

    expect(screen.getByLabelText('渠道活码详情').textContent).toContain('新增好友数');
    expect(screen.getByLabelText('渠道活码详情').textContent).toContain('8');
  });

  it('loads group code-compatible records from the real auto-pull endpoint', async () => {
    const read = vi.fn().mockResolvedValue({ list: [{ id: 21, name: '售后服务群', stateText: '进行中', roomNum: 3 }] });
    const api: BusinessWorkbenchApi = { read, write: vi.fn() };
    view(<GroupCodePage api={api} />);

    expect(await screen.findByText('售后服务群')).toBeTruthy();
    expect(read).toHaveBeenCalledWith('/workRoomAutoPull/index', expect.objectContaining({ page: 1, perPage: 20 }));
    expect(screen.getByText('关联群聊')).toBeTruthy();
  });

  it('renders a retryable error returned by a connected provider', async () => {
    const read = vi.fn()
      .mockRejectedValueOnce(new Error('provider unavailable'))
      .mockResolvedValueOnce({ list: [{ channelCodeId: 12, name: '官网咨询' }] });
    const api: BusinessWorkbenchApi = { read, write: vi.fn() };
    view(<ChannelCodePage api={api} />);

    await screen.findByRole('heading', { name: '加载失败' });
    fireEvent.click(screen.getByRole('button', { name: '重新加载' }));

    expect(await screen.findByText('官网咨询')).toBeTruthy();
  });

  it('keeps page actions available until Phase 3.4 granular permissions are published', async () => {
    const api: BusinessWorkbenchApi = { read: vi.fn().mockResolvedValue({ list: [] }), write: vi.fn() };
    view(<ChannelCodePage api={api} />, new Set(['/channelCode/index@search']));

    await screen.findByRole('heading', { name: '暂无记录' });
    expect(screen.getByRole('button', { name: '查询' })).toBeTruthy();
    expect(screen.getByRole('button', { name: '重置' })).toBeTruthy();
    expect(screen.getByRole('button', { name: '刷新' })).toBeTruthy();
  });

  it('creates a channel code through the existing write Provider', async () => {
    const write = vi.fn().mockResolvedValue(undefined);
    view(<ChannelCodePage api={{ read: vi.fn().mockResolvedValue({ list: [] }), write }} />);

    await screen.findByRole('heading', { name: '暂无记录' });
    fireEvent.click(screen.getByRole('button', { name: '新建渠道活码' }));
    fireEvent.change(screen.getByLabelText('渠道活码名称'), { target: { value: '官网咨询' } });
    fireEvent.change(screen.getByLabelText('使用成员 ID'), { target: { value: '21' } });
    fireEvent.click(screen.getByRole('button', { name: '保存渠道活码' }));

    await waitFor(() => expect(write).toHaveBeenCalledWith(
      '/channelCode/store',
      expect.objectContaining({
        baseInfo: expect.objectContaining({ name: '官网咨询', autoAddFriend: 1 }),
        drainageEmployee: expect.objectContaining({ type: 1 }),
        welcomeMessage: expect.any(Object),
      }),
      'POST',
    ));
  });

  it('creates a group code through the existing auto-pull Provider', async () => {
    const write = vi.fn().mockResolvedValue(undefined);
    view(<GroupCodePage api={{ read: vi.fn().mockResolvedValue({ list: [] }), write }} />);

    await screen.findByRole('heading', { name: '暂无记录' });
    fireEvent.click(screen.getByRole('button', { name: '新建群活码' }));
    fireEvent.change(screen.getAllByLabelText('群活码名称')[1]!, { target: { value: '售后服务群' } });
    fireEvent.change(screen.getByLabelText('入群引导语'), { target: { value: '欢迎入群' } });
    fireEvent.change(screen.getByLabelText('使用成员 ID'), { target: { value: '21' } });
    fireEvent.change(screen.getByLabelText('客户标签 ID'), { target: { value: '31' } });
    fireEvent.change(screen.getByLabelText('群聊配置 JSON'), { target: { value: '[{"roomId":41,"maxNum":50}]' } });
    fireEvent.click(screen.getByRole('button', { name: '保存群活码' }));

    await waitFor(() => expect(write).toHaveBeenCalledWith(
      '/workRoomAutoPull/store',
      expect.objectContaining({ corpId: 7, qrcodeName: '售后服务群', employees: [21], tags: [31] }),
      'POST',
    ));
  });

  it('shows an explicit read-only state when granular permissions omit create', async () => {
    view(
      <ChannelCodePage api={{ read: vi.fn().mockResolvedValue({ list: [] }), write: vi.fn() }} />,
      new Set(['/acquisition/v2-channel-code@refresh']),
    );

    await screen.findByRole('heading', { name: '暂无记录' });
    expect(screen.queryByRole('button', { name: '新建渠道活码' })).toBeNull();
    expect(screen.getByText('当前账号仅可查看渠道活码')).toBeTruthy();
  });

  it('loads persisted short links, opens the create drawer, and creates a draft target', async () => {
    const read = vi.fn().mockResolvedValue({
      list: [{ id: 8, name: '群活码短链', token: 'abc123', targetUrl: '/acquisition/group-code', status: 'active', visitTotal: 4 }],
    });
    const write = vi.fn().mockResolvedValue(undefined);
    const api: BusinessWorkbenchApi = { read, write };
    view(<LiveCodeShortChainPage api={api} />);

    expect(await screen.findByText('群活码短链')).toBeTruthy();
    expect(read).toHaveBeenCalledWith('/liveCodeShortChain/index', expect.objectContaining({ page: 1, perPage: 20 }));
    fireEvent.click(screen.getByRole('button', { name: '创建短链' }));
    expect(screen.getByLabelText('创建短链')).toBeTruthy();
    fireEvent.change(screen.getAllByLabelText('短链名称')[1]!, { target: { value: '群活码短链' } });
    fireEvent.change(screen.getByLabelText('目标地址'), { target: { value: '/acquisition/group-code' } });
    fireEvent.click(screen.getByRole('button', { name: '保存短链' }));

    await waitFor(() => expect(write).toHaveBeenCalledWith(
      '/liveCodeShortChain/store',
      { name: '群活码短链', targetUrl: '/acquisition/group-code', targetType: 'url' },
      'POST',
    ));
  });

  it('disables a persisted short link through the provider', async () => {
    const read = vi.fn().mockResolvedValue({
      list: [{ id: 8, name: '群活码短链', token: 'abc123', targetUrl: '/acquisition/group-code', status: 'active', visitTotal: 4 }],
    });
    const write = vi.fn().mockResolvedValue(undefined);
    view(<LiveCodeShortChainPage api={{ read, write }} />);

    await screen.findByText('群活码短链');
    fireEvent.click(screen.getByRole('button', { name: '停用' }));
    expect(write).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));
    await waitFor(() => expect(write).toHaveBeenCalledWith('/liveCodeShortChain/disable', { id: 8 }, 'POST'));
  });
});
