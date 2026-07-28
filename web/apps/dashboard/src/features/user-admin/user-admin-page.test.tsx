/* eslint-disable @typescript-eslint/unbound-method */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { App } from 'antd';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import { UserAdminPage, type UserAdminPageApi } from './user-admin-page';

const access: AccessContext = {
  session: { token: 't', userId: '1', corpId: '7', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true }, menu: [],
  allowedRoutes: new Set(['/user/index']),
  allowedActions: new Set(['/user/index@search', '/user/index@add']),
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
function api(): UserAdminPageApi {
  return {
    list: vi.fn(() => Promise.resolve({ list: [{ userId: 4, userName: '张三',
      phone: '13800000000', gender: 1, roleId: 2, roleName: '管理员', status: 0,
      statusText: '未启用', createdAt: '2026-07-29',
      department: [{ departmentId: 1, departmentName: '销售部' }] }],
      normalNum: 0, notEnabledNum: 1, disableNum: 0,
      page: { perPage: 10, total: 1, totalPage: 1 } })),
    detail: vi.fn(() => Promise.resolve({ userId: 4, userName: '张三',
      phone: '13800000000', gender: 1, roleId: 2, roleName: '管理员', status: 0,
      statusText: '未启用', createdAt: '2026-07-29', department: [] })),
    departments: vi.fn(() => Promise.resolve([])),
    roles: vi.fn(() => Promise.resolve([{ roleId: 2, name: '管理员' }])),
    create: vi.fn(() => Promise.resolve()), update: vi.fn(() => Promise.resolve()),
    updateStatus: vi.fn(() => Promise.resolve()), resetPassword: vi.fn(() => Promise.resolve()),
  };
}
function renderPage(client: UserAdminPageApi) {
  render(<QueryClientProvider client={new QueryClient({ defaultOptions: {
    queries: { retry: false }, mutations: { retry: false },
  } })}><App><DashboardAccessProvider value={access}>
    <UserAdminPage api={client} />
  </DashboardAccessProvider></App></QueryClientProvider>);
}
describe('UserAdminPage', () => {
  it('loads, filters, and batch-enables accounts', async () => {
    const client = api(); renderPage(client); await screen.findByText('张三');
    fireEvent.change(screen.getByPlaceholderText('按手机号码筛选'), { target: { value: '13800000000' } });
    fireEvent.click(screen.getByRole('button', { name: /查\s*询/ }));
    await waitFor(() => expect(client.list).toHaveBeenLastCalledWith({
      phone: '13800000000', status: '', page: 1, perPage: 10,
    }));
    await screen.findByText('张三');
    fireEvent.click(screen.getByRole('checkbox', { name: /Select row/ }));
    fireEvent.click(screen.getByRole('button', { name: '启用选中账户' }));
    await waitFor(() => expect(client.updateStatus).toHaveBeenCalledWith([4], 1));
  });
  it('rejects a malformed phone before account creation', async () => {
    const client = api(); renderPage(client); await screen.findByText('张三');
    fireEvent.click(screen.getByRole('button', { name: /添\s*加/ }));
    const dialog = screen.getAllByRole('dialog').find(element => element.style.display !== 'none')!;
    fireEvent.change(within(dialog).getByLabelText('员工姓名'), { target: { value: '李四' } });
    fireEvent.change(within(dialog).getByLabelText('手机号码'), { target: { value: '123' } });
    fireEvent.change(within(dialog).getByLabelText('密码'), { target: { value: 'abc123' } });
    fireEvent.change(within(dialog).getByLabelText('确认密码'), { target: { value: 'abc123' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'OK' }));
    expect(await screen.findByText('手机号码格式错误')).not.toBeNull();
    expect(client.create).not.toHaveBeenCalled();
  });
  it('resets an account password', async () => {
    const client = api(); renderPage(client); await screen.findByText('张三');
    fireEvent.click(screen.getByRole('button', { name: '重置密码' }));
    const dialog = screen.getAllByRole('dialog').find(element => element.style.display !== 'none')!;
    fireEvent.change(within(dialog).getByLabelText('新密码'), { target: { value: 'new123' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'OK' }));
    await waitFor(() => expect(client.resetPassword).toHaveBeenCalledWith(4, 'new123'));
  });
});
