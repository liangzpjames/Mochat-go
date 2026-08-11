/* eslint-disable @typescript-eslint/unbound-method */
import { App } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import { ContactFieldPage, type ContactFieldPageApi } from './contact-field-page';

const allActions = new Set([
  '/contactField/index@advanced', '/contactField/index@all',
  '/contactField/index@add', '/contactField/index@batch',
  '/contactField/index@close', '/contactField/index@edit',
]);
const access: AccessContext = {
  session: { token: 'token', userId: '1', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true }, menu: [],
  allowedRoutes: new Set(['/contactField/index']), allowedActions: allActions,
};

beforeEach(() => {
  globalThis.ResizeObserver = class { observe() {} unobserve() {} disconnect() {} };
  Object.defineProperty(window, 'matchMedia', { configurable: true, value: vi.fn(() => ({
    matches: false, addListener: vi.fn(), removeListener: vi.fn(),
    addEventListener: vi.fn(), removeEventListener: vi.fn(),
  })) });
  Object.defineProperty(window, 'getComputedStyle', { configurable: true,
    value: () => ({ getPropertyValue: () => '' }) as unknown as CSSStyleDeclaration });
});
afterEach(cleanup);

function api(): ContactFieldPageApi {
  return {
    list: vi.fn(() => Promise.resolve({ list: [
      { id: 1, name: 'gender', label: '性别', type: 1, typeText: '单选',
        options: ['男', '女'], order: 10, status: 1, isSys: 1 },
      { id: 2, name: 'city', label: '城市', type: 0, typeText: '文本',
        options: [], order: 8, status: 1, isSys: 0 },
    ], page: { page: 1, perPage: 10, total: 2, totalPage: 1 } })),
    create: vi.fn(() => Promise.resolve()), update: vi.fn(() => Promise.resolve()),
    updateStatus: vi.fn(() => Promise.resolve()), remove: vi.fn(() => Promise.resolve()),
    batchUpdate: vi.fn(() => Promise.resolve()),
  };
}

function renderPage(client: ContactFieldPageApi, allowedActions = allActions) {
  render(<QueryClientProvider client={new QueryClient({ defaultOptions: {
    queries: { retry: false }, mutations: { retry: false },
  } })}><App><DashboardAccessProvider value={{ ...access, allowedActions }}>
    <ContactFieldPage api={client} />
  </DashboardAccessProvider></App></QueryClientProvider>);
}

describe('ContactFieldPage', () => {
  it('loads fields with the all-status filter', async () => {
    const client = api(); renderPage(client);
    expect(await screen.findByText('城市')).not.toBeNull();
    expect(client.list).toHaveBeenCalledWith({ status: 2, page: 1, perPage: 10 });
  });

  it('hides the advanced field surface when a legacy action contract omits it', () => {
    renderPage(api(), new Set(['/contactField/index@add']));
    expect(screen.queryByText('高级属性')).toBeNull();
  });

  it('creates a validated custom field and refreshes the list', async () => {
    const client = api(); renderPage(client); await screen.findByText('城市');
    fireEvent.click(screen.getByRole('button', { name: '新增属性' }));
    fireEvent.change(screen.getByLabelText('字段名称'), { target: { value: '行业' } });
    fireEvent.click(screen.getByRole('button', { name: /确\s*定/ }));
    await waitFor(() => expect(client.create).toHaveBeenCalledWith({
      label: '行业', type: 0, options: [], order: 0, status: 1,
    }));
    await waitFor(() => expect(client.list).toHaveBeenCalledTimes(2));
  });

  it('gates system restrictions and updates custom field status', async () => {
    const client = api(); renderPage(client); await screen.findByText('城市');
    expect(screen.getAllByRole('button', { name: '删除' })).toHaveLength(1);
    fireEvent.click(screen.getByRole('button', { name: '关闭' }));
    await waitFor(() => expect(client.updateStatus).toHaveBeenCalledWith(2, 0));
  });
});
