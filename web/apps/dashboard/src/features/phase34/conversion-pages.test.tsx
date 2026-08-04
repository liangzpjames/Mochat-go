import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';
import { GroupTemplatePage, RedirectLinkPage, WechatCustomerServicePage } from './conversion-pages';

const access: AccessContext = {
  session: { token: 'token', userId: '1', corpId: '7', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set([
    '/acquisition/redirect-link',
    '/acquisition/wechat-customer-service',
    '/acquisition/group-template',
  ]),
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

describe('Phase 3.4 conversion pages', () => {
  it('loads persisted acquisition links and creates one through the provider', async () => {
    const read = vi.fn().mockResolvedValue({
      list: [{ id: 4, name: '官网获客入口', targetUrl: '/acquisition/v2-channel-code', authorizationStatus: 'authorized', status: 'active', visitTotal: 12, conversionTotal: 3 }],
    });
    const write = vi.fn().mockResolvedValue(undefined);
    view(<RedirectLinkPage api={{ read, write }} />);

    expect(await screen.findByText('官网获客入口')).toBeTruthy();
    expect(read).toHaveBeenCalledWith('/acquisitionLink/index', expect.objectContaining({ page: 1, perPage: 20 }));
    fireEvent.click(screen.getByRole('button', { name: '创建获客链接' }));
    expect(screen.getByLabelText('创建获客链接')).toBeTruthy();
    fireEvent.change(screen.getAllByLabelText('链接名称')[1]!, { target: { value: '新品入口' } });
    fireEvent.change(screen.getByLabelText('目标地址'), { target: { value: '/acquisition/v2-channel-code' } });
    fireEvent.click(screen.getByRole('button', { name: '保存获客链接' }));

    await waitFor(() => expect(write).toHaveBeenCalledWith(
      '/acquisitionLink/store',
      { name: '新品入口', targetUrl: '/acquisition/v2-channel-code' },
      'POST',
    ));
  });

  it('loads customer-service accounts, persists a draft, and exposes sync failure', async () => {
    const read = vi.fn().mockResolvedValue({
      list: [{ id: 9, name: '售前客服', account: 'kf_001', employeeIds: '[3,4]', receiveMode: 'round_robin', status: 'pending_sync' }],
    });
    const write = vi.fn()
      .mockResolvedValueOnce(undefined)
      .mockRejectedValueOnce(new Error('WeCom customer-service provider unavailable'));
    view(<WechatCustomerServicePage api={{ read, write }} />);

    expect(await screen.findByText('售前客服')).toBeTruthy();
    expect(read).toHaveBeenCalledWith('/customerService/index', expect.objectContaining({ page: 1, perPage: 20 }));
    fireEvent.click(screen.getByRole('button', { name: '创建客服' }));
    fireEvent.change(screen.getAllByLabelText('客服名称')[1]!, { target: { value: '售后客服' } });
    fireEvent.change(screen.getByLabelText('客服账号'), { target: { value: 'kf_002' } });
    fireEvent.click(screen.getByRole('button', { name: '保存客服' }));
    await waitFor(() => expect(write).toHaveBeenCalledWith(
      '/customerService/store',
      { name: '售后客服', account: 'kf_002', employeeIds: [], receiveMode: 'round_robin' },
      'POST',
    ));

    fireEvent.click(screen.getByRole('button', { name: '同步客服账号' }));
    expect((await screen.findByRole('alert')).textContent).toContain('WeCom customer-service provider unavailable');
  });

  it('loads group templates from the real auto-pull provider and applies a name filter', async () => {
    const read = vi.fn().mockResolvedValue({
      list: [{ workRoomAutoPullId: 31, qrcodeName: '新客福利群', stateText: '启用', roomNum: 4, todayContactNum: 6, createName: '运营员', createdAt: '2026-08-04 09:00' }],
    });
    const api: BusinessWorkbenchApi = { read, write: vi.fn() };
    view(<GroupTemplatePage api={api} />);

    expect(await screen.findByText('新客福利群')).toBeTruthy();
    expect(read).toHaveBeenCalledWith('/workRoomAutoPull/index', expect.objectContaining({ page: 1, perPage: 20 }));
    expect(screen.getByText('今日新增')).toBeTruthy();
    fireEvent.change(screen.getByLabelText('模板名称'), { target: { value: '福利' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));

    await waitFor(() => expect(read).toHaveBeenLastCalledWith(
      '/workRoomAutoPull/index',
      expect.objectContaining({ qrcodeName: '福利', page: 1, perPage: 20 }),
    ));
  });

  it('shows group-template provider details and supports retry', async () => {
    const read = vi.fn()
      .mockRejectedValueOnce(new Error('provider unavailable'))
      .mockResolvedValueOnce({ list: [{ workRoomAutoPullId: 31, qrcodeName: '新客福利群', roomNum: 4 }] });
    const api: BusinessWorkbenchApi = { read, write: vi.fn() };
    view(<GroupTemplatePage api={api} />);

    await screen.findByRole('heading', { name: '加载失败' });
    fireEvent.click(screen.getByRole('button', { name: '重新加载' }));
    expect(await screen.findByText('新客福利群')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '详情' }));
    expect(screen.getByLabelText('一键加群详情').textContent).toContain('群总数');
    expect(screen.getByLabelText('一键加群详情').textContent).toContain('4');
  });
});
