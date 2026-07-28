/* eslint-disable @typescript-eslint/unbound-method */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { App } from 'antd';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import { MenuAdminPage, type MenuAdminPageApi } from './menu-admin-page';

const access: AccessContext = {
  session: { token: 't', userId: '1', corpId: '7', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true }, menu: [],
  allowedRoutes: new Set(['/menu/index']),
  allowedActions: new Set(['/menu/index@search', '/menu/index@add', '/menu/index@edit']),
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
function api(): MenuAdminPageApi {
  return {
    list: vi.fn(() => Promise.resolve({ list: [{ menuId: 1, name: '客户管理', level: 1,
      parentId: 0, icon: 'team', status: 1, menuPath: '1', levelName: '一级菜单',
      operateName: 'admin', updatedAt: '2026-07-28', children: [] }],
      page: { page: 1, perPage: 10, total: 1, totalPage: 1 } })),
    options: vi.fn(() => Promise.resolve([{ menuId: 1, name: '客户管理', level: 1,
      parentId: 0, dataPermission: 2, children: [] }])),
    detail: vi.fn(() => Promise.resolve({ menuId: 1, level: 1, name: '客户管理',
      icon: 'team', linkUrl: '', linkType: 0, status: 1 })),
    usedIcons: vi.fn(() => Promise.resolve(['team'])), create: vi.fn(() => Promise.resolve()),
    update: vi.fn(() => Promise.resolve()), updateStatus: vi.fn(() => Promise.resolve()),
    remove: vi.fn(() => Promise.resolve()),
  };
}
function renderPage(client: MenuAdminPageApi) {
  render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
    <App><DashboardAccessProvider value={access}><MenuAdminPage api={client} /></DashboardAccessProvider></App>
  </QueryClientProvider>);
}
describe('MenuAdminPage', () => {
  it('loads and searches menus', async () => {
    const client = api(); renderPage(client); await screen.findByText('客户管理');
    fireEvent.change(screen.getByPlaceholderText('请输入菜单名称'), { target: { value: '客户' } });
    fireEvent.click(screen.getByRole('button', { name: /查\s*询/ }));
    await waitFor(() => expect(client.list).toHaveBeenLastCalledWith({ name: '客户', page: 1, perPage: 10 }));
  });
  it('creates a first-level menu', async () => {
    const client = api(); renderPage(client); await screen.findByText('客户管理');
    fireEvent.click(screen.getByRole('button', { name: /添\s*加/ }));
    fireEvent.change(screen.getByLabelText('名称'), { target: { value: '系统管理' } });
    fireEvent.click(screen.getByRole('button', { name: /确\s*定/ }));
    await waitFor(() => expect(client.create).toHaveBeenCalledWith(expect.objectContaining({
      level: 1, name: '系统管理',
    })));
  });
  it('edits and disables a menu', async () => {
    const client = api(); renderPage(client); await screen.findByText('客户管理');
    fireEvent.click(screen.getByRole('button', { name: '编辑' }));
    await screen.findByDisplayValue('客户管理');
    fireEvent.click(screen.getByRole('button', { name: /禁\s*用/ }));
    await waitFor(() => expect(client.updateStatus).toHaveBeenCalledWith(1, 2));
  });
});
