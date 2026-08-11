/* eslint-disable @typescript-eslint/unbound-method */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import { BusinessWorkbenchPage, type BusinessWorkbenchApi } from './business-workbench-page';
import type { BusinessRouteConfig } from './catalog';

const access: AccessContext = {
  session: { token: 'token', userId: '1', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set(['/example/index', '/example/store']),
  allowedActions: new Set([
    '/example/index@search',
    '/example/index@add',
    '/example/index@sync',
    '/example/store@save',
  ]),
};

beforeEach(() => {
  globalThis.ResizeObserver = class { observe() {} unobserve() {} disconnect() {} };
  Object.defineProperty(window, 'matchMedia', { configurable: true, value: vi.fn(() => ({
    matches: false,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  })) });
  Object.defineProperty(window, 'getComputedStyle', {
    configurable: true,
    value: () => ({ getPropertyValue: () => '' }) as unknown as CSSStyleDeclaration,
  });
});
afterEach(cleanup);

function renderPage(config: BusinessRouteConfig, api: BusinessWorkbenchApi, navigate = vi.fn()) {
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    } })}>
      <App>
        <DashboardAccessProvider value={access}>
          <BusinessWorkbenchPage config={config} api={api} navigate={navigate} />
        </DashboardAccessProvider>
      </App>
    </QueryClientProvider>,
  );
  return navigate;
}

describe('BusinessWorkbenchPage', () => {
  it('loads normalized records and applies explicit filters', async () => {
    const api: BusinessWorkbenchApi = {
      read: vi.fn(() => Promise.resolve({
        list: [{ id: 1, name: '示例活动', statusText: '启用' }],
        page: { total: 1 },
      })),
      write: vi.fn(() => Promise.resolve()),
    };
    renderPage({
      path: '/example/index',
      title: '示例管理',
      description: '示例业务管理说明',
      mode: 'list',
      readEndpoint: '/example/index',
      fields: [{ key: 'keyword', label: '关键词', kind: 'text' }],
      actions: ['search', 'reset', 'create', 'sync'],
    }, api);

    expect(await screen.findByText('示例活动')).not.toBeNull();
    fireEvent.change(screen.getByLabelText('关键词'), { target: { value: '活动' } });
    fireEvent.click(screen.getByRole('button', { name: /查\s*询/ }));
    await waitFor(() => expect(api.read).toHaveBeenLastCalledWith(
      '/example/index',
      expect.objectContaining({ keyword: '活动', page: 1, perPage: 10 }),
    ));
  });

  it('submits route-specific form fields and navigates back', async () => {
    const api: BusinessWorkbenchApi = {
      read: vi.fn(() => Promise.resolve({})),
      write: vi.fn(() => Promise.resolve()),
    };
    const navigate = renderPage({
      path: '/example/store',
      title: '新建示例',
      description: '新建示例业务',
      mode: 'form',
      readEndpoint: '/example/show',
      writeEndpoint: '/example/store',
      fields: [
        { key: 'name', label: '名称', kind: 'text' },
        { key: 'content', label: '内容', kind: 'textarea' },
      ],
      actions: ['save', 'back'],
    }, api);

    fireEvent.change(screen.getByLabelText('名称'), { target: { value: '七月活动' } });
    fireEvent.change(screen.getByLabelText('内容'), { target: { value: '活动说明' } });
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));

    await waitFor(() => expect(api.write).toHaveBeenCalledWith('/example/store', {
      name: '七月活动',
      content: '活动说明',
    }));
    expect(navigate).toHaveBeenCalledWith('/example/index');
  });
});
