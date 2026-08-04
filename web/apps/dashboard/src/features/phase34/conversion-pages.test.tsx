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
  it('renders the acquisition-link authorization boundary without inventing provider data', () => {
    const read = vi.fn();
    const write = vi.fn();
    view(<RedirectLinkPage api={{ read, write }} />);

    expect(screen.getByRole('heading', { name: '获客链接' })).toBeTruthy();
    expect(screen.getByText('企业授权尚未接入')).toBeTruthy();
    expect(screen.getByLabelText('链接名称')).toBeTruthy();
    expect(screen.getByText('访问人数')).toBeTruthy();
    expect(screen.getByRole('button', { name: '立即授权并使用' })).toHaveProperty('disabled', true);
    expect(read).not.toHaveBeenCalled();
    expect(write).not.toHaveBeenCalled();
  });

  it('renders the customer-service structure and blocks sync until its provider is configured', () => {
    const read = vi.fn();
    const write = vi.fn();
    view(<WechatCustomerServicePage api={{ read, write }} />);

    expect(screen.getByRole('heading', { name: '微信客服' })).toBeTruthy();
    expect(screen.getByLabelText('客服名称')).toBeTruthy();
    expect(screen.getByText('客服账号')).toBeTruthy();
    expect(screen.getByText('接待员工')).toBeTruthy();
    expect(screen.getByText('接待方式')).toBeTruthy();
    expect(screen.getByRole('button', { name: '同步客服账号' })).toHaveProperty('disabled', true);
    expect(screen.getByText('微信客服 Provider 未配置')).toBeTruthy();
    expect(read).not.toHaveBeenCalled();
    expect(write).not.toHaveBeenCalled();
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
