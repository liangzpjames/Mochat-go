/* eslint-disable @typescript-eslint/unbound-method */
import { App } from 'antd';
import { ApiError } from '@mochat/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import { CorpPage, type CorpPageApi } from './corp-page';

const access: AccessContext = {
  session: { token: 'Bearer token', userId: '1', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set(['/corp/index']),
  allowedActions: new Set([
    '/corp/index@search',
    '/corp/index@addwx',
    '/corp/index@check',
    '/corp/index@edit',
  ]),
};

afterEach(cleanup);
beforeEach(() => {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    value: vi.fn(() => ({
      addEventListener: vi.fn(),
      addListener: vi.fn(),
      matches: false,
      removeEventListener: vi.fn(),
      removeListener: vi.fn(),
    })),
  });
  Object.defineProperty(window, 'getComputedStyle', {
    configurable: true,
    value: () => ({ getPropertyValue: () => '' }) as unknown as CSSStyleDeclaration,
  });
});

function renderPage(api: CorpPageApi, allowedActions = access.allowedActions) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const invalidateQueries = vi.spyOn(queryClient, 'invalidateQueries');
  const rendered = render(
    <QueryClientProvider client={queryClient}>
      <App>
        <DashboardAccessProvider value={{ ...access, allowedActions }}>
          <CorpPage api={api} />
        </DashboardAccessProvider>
      </App>
    </QueryClientProvider>,
  );
  return { invalidateQueries, unmount: rendered.unmount };
}

function api(overrides: Partial<CorpPageApi> = {}): CorpPageApi {
  return {
    create: vi.fn(() => Promise.resolve()),
    list: vi.fn(() => Promise.resolve({
      list: [{ corpId: 7, corpName: '测试企业', wxCorpId: 'wx-7', createdAt: '2026-07-28' }],
      page: { perPage: 10, total: 1, totalPage: 1 },
    })),
    show: vi.fn(() => Promise.resolve({
      corpId: 7,
      corpName: '测试企业',
      wxCorpId: 'wx-7',
      employeeSecret: 'employee',
      contactSecret: 'contact',
      eventCallback: 'https://example.test/callback',
      token: 'callback-token',
      encodingAesKey: 'aes-key',
    })),
    update: vi.fn(() => Promise.resolve()),
    ...overrides,
  };
}

describe('CorpPage', () => {
  it('loads the active corp query and preserves search query parameters', async () => {
    const client = api();
    renderPage(client);

    expect(await screen.findByText('wx-7')).not.toBeNull();
    expect(client.list).toHaveBeenCalledWith({
      corpId: '7',
      corpName: '',
      page: 1,
      perPage: 10,
    });

    fireEvent.change(screen.getByPlaceholderText('搜索企业微信名称'), {
      target: { value: '测试' },
    });
    fireEvent.click(screen.getByRole('button', { name: /查\s*找/ }));
    await waitFor(() => expect(client.list).toHaveBeenLastCalledWith({
      corpId: '7',
      corpName: '测试',
      page: 1,
      perPage: 10,
    }));
  });

  it('hides write and detail actions without action permissions', async () => {
    renderPage(api(), new Set(['/corp/index@search']));

    expect(await screen.findByText('wx-7')).not.toBeNull();
    expect(screen.queryByRole('button', { name: '添加企业微信' })).toBeNull();
    expect(screen.queryByRole('button', { name: '查看' })).toBeNull();
    expect(screen.queryByRole('button', { name: '修改' })).toBeNull();
  });

  it('keeps callback credentials visible in read-only detail mode', async () => {
    renderPage(api());
    await screen.findByText('wx-7');

    fireEvent.click(screen.getByRole('button', { name: '查看' }));

    expect(await screen.findByDisplayValue('callback-token')).not.toBeNull();
    expect(screen.getByDisplayValue('aes-key')).not.toBeNull();
  });

  it('shows empty and business error states', async () => {
    const { unmount } = renderPage(api({
      list: vi.fn(() => Promise.resolve({
        list: [],
        page: { perPage: 10, total: 0, totalPage: 0 },
      })),
    }));
    expect(await screen.findByText('暂无企业微信授权')).not.toBeNull();
    unmount();

    renderPage(api({ list: vi.fn(() => Promise.reject(new Error('加载失败'))) }));
    expect((await screen.findByRole('alert')).textContent).toContain('加载失败');
  });

  it.each([
    new ApiError('unauthorized', '登录已失效', { status: 401 }),
    new ApiError('forbidden', '无权访问企业配置', { status: 403 }),
  ])('renders access failures without exposing stale data', async (error) => {
    renderPage(api({ list: vi.fn(() => Promise.reject(error)) }));

    expect((await screen.findByRole('alert')).textContent).toContain(error.message);
    expect(screen.queryByText('wx-7')).toBeNull();
  });

  it('submits the audited request body and invalidates the active corp cache', async () => {
    const client = api();
    const { invalidateQueries } = renderPage(client);
    await screen.findByText('wx-7');

    fireEvent.click(screen.getByRole('button', { name: '修改' }));
    await screen.findByDisplayValue('employee');
    fireEvent.change(screen.getByLabelText('企业名称'), {
      target: { value: '更新企业' },
    });
    fireEvent.click(screen.getByRole('button', { name: '保存配置' }));

    await waitFor(() => expect(client.update).toHaveBeenCalledWith({
      corpId: 7,
      corpName: '更新企业',
      wxCorpId: 'wx-7',
      employeeSecret: 'employee',
      contactSecret: 'contact',
    }));
    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: ['corp', '7', 'admin-list'],
    });
    expect(await screen.findByText('保存成功')).not.toBeNull();
  });
});
