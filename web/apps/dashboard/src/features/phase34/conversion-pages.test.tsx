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

  it('refreshes the persisted acquisition-link failure state after authorization errors', async () => {
    const read = vi.fn()
      .mockResolvedValueOnce({ list: [{ id: 4, name: '官网获客入口', targetUrl: '/acquisition/v2-channel-code', authorizationStatus: 'unauthorized', status: 'draft' }] })
      .mockResolvedValueOnce({ list: [{ id: 4, name: '官网获客入口', targetUrl: '/acquisition/v2-channel-code', authorizationStatus: 'failed', status: 'failed' }] });
    const write = vi.fn().mockRejectedValue(new Error('企业微信凭据未配置'));
    view(<RedirectLinkPage api={{ read, write }} />);

    expect(await screen.findByText('官网获客入口')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '立即授权并使用' }));

    expect((await screen.findByRole('alert')).textContent).toContain('企业微信凭据未配置');
    expect(await screen.findByText('失败 / 失败')).toBeTruthy();
    expect(read).toHaveBeenCalledTimes(2);
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

  it('creates a one-click group template through the existing auto-pull provider', async () => {
    const read = vi.fn().mockResolvedValue({ list: [] });
    const write = vi.fn().mockResolvedValue(undefined);
    view(<GroupTemplatePage api={{ read, write }} />);

    await screen.findByRole('heading', { name: '暂无模板' });
    fireEvent.click(screen.getByRole('button', { name: '新建加群模板' }));
    expect(screen.getByLabelText('一键加群模板')).toBeTruthy();
    fireEvent.change(screen.getAllByLabelText('模板名称')[1]!, { target: { value: '新品加群' } });
    fireEvent.change(screen.getByLabelText('入群引导语'), { target: { value: '欢迎加入新品群' } });
    fireEvent.change(screen.getByLabelText('使用成员ID'), { target: { value: '99' } });
    fireEvent.change(screen.getByLabelText('标签ID'), { target: { value: '900001' } });
    fireEvent.change(screen.getByLabelText('群聊配置JSON'), { target: { value: '[{"roomId":900001,"maxNum":50}]' } });
    fireEvent.click(screen.getByRole('button', { name: '保存加群模板' }));

    await waitFor(() => expect(write).toHaveBeenCalledWith(
      '/workRoomAutoPull/store',
      { corpId: 7, qrcodeName: '新品加群', isVerified: 2, leadingWords: '欢迎加入新品群', employees: [99], tags: [900001], rooms: '[{"roomId":900001,"maxNum":50}]' },
      'POST',
    ));
  });

  it('persists a selected material reference for the one-click group template', async () => {
    const read = vi.fn().mockImplementation((endpoint: string) => endpoint === '/materialSelector/index'
      ? Promise.resolve({ list: [{ id: 45, name: '入群话术', preview: '欢迎加入' }] })
      : Promise.resolve({ list: [] }));
    const write = vi.fn().mockResolvedValue(undefined);
    view(<GroupTemplatePage api={{ read, write }} />);

    await screen.findByRole('heading', { name: '暂无模板' });
    fireEvent.click(screen.getByRole('button', { name: '新建加群模板' }));
    fireEvent.change(screen.getAllByLabelText('模板名称')[1]!, { target: { value: '引用素材加群' } });
    fireEvent.change(screen.getByLabelText('入群引导语'), { target: { value: '手动内容' } });
    await screen.findByRole('option', { name: '入群话术' });
    fireEvent.change(screen.getByRole('combobox', { name: '引用素材' }), { target: { value: '45' } });
    fireEvent.change(screen.getByLabelText('使用成员ID'), { target: { value: '99' } });
    fireEvent.change(screen.getByLabelText('标签ID'), { target: { value: '900001' } });
    fireEvent.change(screen.getByLabelText('群聊配置JSON'), { target: { value: '[{"roomId":900001,"maxNum":50}]' } });
    fireEvent.click(screen.getByRole('button', { name: '保存加群模板' }));

    await waitFor(() => expect(write).toHaveBeenCalledWith(
      '/workRoomAutoPull/store',
      expect.objectContaining({ mediumId: 45, leadingWords: '欢迎加入' }),
      'POST',
    ));
  });
});
