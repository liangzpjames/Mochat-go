/* eslint-disable @typescript-eslint/unbound-method */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { MemoryRouter } from 'react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import { RolePermissionPage, type RolePermissionPageApi } from './role-permission-page';
import type { PermissionNode } from './role-api';

const access: AccessContext = {
  session: { token: 'token', userId: '1', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set(['/role/index', '/role/permissionShow']),
  allowedActions: new Set(['/role/permissionShow@save']),
};

beforeEach(() => {
  globalThis.ResizeObserver = class { observe() {} unobserve() {} disconnect() {} };
});
afterEach(cleanup);

function renderPage(api: RolePermissionPageApi, navigate = vi.fn()) {
  render(
    <MemoryRouter initialEntries={['/role/permissionShow?roleId=7']}>
      <QueryClientProvider client={new QueryClient({ defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      } })}>
        <App>
          <DashboardAccessProvider value={access}>
            <RolePermissionPage api={api} navigate={navigate} />
          </DashboardAccessProvider>
        </App>
      </QueryClientProvider>
    </MemoryRouter>,
  );
  return navigate;
}

describe('RolePermissionPage', () => {
  it('loads the role tree, cascades selection and saves checked ids', async () => {
    const api: RolePermissionPageApi = {
      permissions: vi.fn(() => Promise.resolve<PermissionNode[]>([
        {
          id: 1,
          name: '客户管理',
          checked: '3',
          children: [
            { id: 2, name: '客户列表', checked: '2', children: [] },
            { id: 3, name: '删除客户', checked: '1', children: [] },
          ],
        },
      ])),
      savePermissions: vi.fn(() => Promise.resolve()),
    };
    const navigate = renderPage(api);

    expect(await screen.findByText('客户管理')).not.toBeNull();
    fireEvent.click(screen.getByRole('checkbox', { name: '删除客户' }));
    fireEvent.click(screen.getByRole('button', { name: '保存权限' }));

    await waitFor(() => expect(api.savePermissions).toHaveBeenCalledWith(7, [1, 2, 3]));
    expect(navigate).toHaveBeenCalledWith('/role/index');
  });

  it('rejects a missing role id before loading', async () => {
    const api: RolePermissionPageApi = {
      permissions: vi.fn(() => Promise.resolve([])),
      savePermissions: vi.fn(() => Promise.resolve()),
    };
    render(
      <MemoryRouter initialEntries={['/role/permissionShow']}>
        <QueryClientProvider client={new QueryClient()}>
          <DashboardAccessProvider value={access}>
            <RolePermissionPage api={api} navigate={vi.fn()} />
          </DashboardAccessProvider>
        </QueryClientProvider>
      </MemoryRouter>,
    );

    expect((await screen.findByRole('alert')).textContent).toContain('缺少角色编号');
    expect(api.permissions).not.toHaveBeenCalled();
  });
});
