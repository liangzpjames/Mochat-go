/* eslint-disable @typescript-eslint/unbound-method */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { App } from 'antd';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import { RolePage, type RolePageApi } from './role-page';

const allActions = new Set([
  '/role/index@search', '/role/index@add', '/role/index@checkMember',
  '/role/index@edit', '/role/index@copy', '/role/index@delete',
  '/role/index@use', '/role/permissionShow',
]);
const access: AccessContext = {
  session: { token: 'token', userId: '1', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true }, menu: [],
  allowedRoutes: new Set(['/role/index', '/role/permissionShow']), allowedActions: allActions,
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

function api(): RolePageApi {
  return {
    list: vi.fn(() => Promise.resolve({ list: [
      { roleId: 1, name: '管理员', employeeNum: 2, remarks: '系统管理', updatedAt: '2026-07-28', status: 1, dataPermission: 1 },
      { roleId: 2, name: '访客', employeeNum: 0, remarks: '', updatedAt: '2026-07-28', status: 2 },
    ], page: { page: 1, perPage: 10, total: 2, totalPage: 1 } })),
    detail: vi.fn(() => Promise.resolve({ roleId: 1, name: '管理员', remarks: '系统管理', dataPermission: 1 })),
    members: vi.fn(() => Promise.resolve({ list: [
      { employeeId: 8, employeeName: '张三', phone: '13800000000', email: 'a@example.com', department: '销售部' },
    ], page: { page: 1, perPage: 10, total: 1, totalPage: 1 } })),
    create: vi.fn(() => Promise.resolve()), copy: vi.fn(() => Promise.resolve()),
    update: vi.fn(() => Promise.resolve()), updateStatus: vi.fn(() => Promise.resolve()),
    remove: vi.fn(() => Promise.resolve()),
  };
}

function renderPage(client: RolePageApi, navigate = vi.fn()) {
  render(<QueryClientProvider client={new QueryClient({ defaultOptions: {
    queries: { retry: false }, mutations: { retry: false },
  } })}><App><DashboardAccessProvider value={access}>
    <RolePage api={client} navigate={navigate} />
  </DashboardAccessProvider></App></QueryClientProvider>);
  return navigate;
}

describe('RolePage', () => {
  it('loads and searches roles', async () => {
    const client = api(); renderPage(client);
    expect(await screen.findByText('管理员')).not.toBeNull();
    fireEvent.change(screen.getAllByPlaceholderText('请输入角色名称')[0]!, { target: { value: '主管' } });
    fireEvent.click(screen.getByRole('button', { name: /查\s*询/ }));
    await waitFor(() => expect(client.list).toHaveBeenLastCalledWith({ name: '主管', page: 1, perPage: 10 }));
  });

  it('creates and copies roles with validated values', async () => {
    const client = api(); renderPage(client); await screen.findByText('管理员');
    fireEvent.click(screen.getByRole('button', { name: /添\s*加/ }));
    fireEvent.change(screen.getByLabelText('角色名称'), { target: { value: '主管' } });
    fireEvent.click(screen.getByRole('button', { name: /确\s*定/ }));
    await waitFor(() => expect(client.create).toHaveBeenCalledWith({
      name: '主管', remarks: '', dataPermission: 2,
    }));

    fireEvent.click(screen.getAllByRole('button', { name: '复制权限' })[0]!);
    fireEvent.change(screen.getByLabelText('角色名称'), { target: { value: '管理员副本' } });
    fireEvent.click(screen.getByRole('button', { name: /确\s*定/ }));
    await waitFor(() => expect(client.copy).toHaveBeenCalledWith(1, {
      name: '管理员副本', remarks: '系统管理', dataPermission: 1,
    }));
  });

  it('shows members, protects assigned roles, and navigates to permission setup', async () => {
    const client = api(); const navigate = renderPage(client); await screen.findByText('管理员');
    fireEvent.click(screen.getByRole('button', { name: '2' }));
    expect(await screen.findByText('张三')).not.toBeNull();
    expect(client.members).toHaveBeenCalledWith({ roleId: 1, page: 1, perPage: 10 });

    fireEvent.click(screen.getAllByRole('button', { name: '设置权限' })[0]!);
    expect(navigate).toHaveBeenCalledWith('/role/permissionShow?roleId=1');
    expect(screen.getAllByRole('button', { name: '删除' })).toHaveLength(1);
    expect((screen.getAllByRole('switch')[0] as HTMLButtonElement).disabled).toBe(true);
  });
});
