import { cleanup, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { CompanyRolePage } from './role-page';
import { CompanyStaffPage } from './staff-page';
import { CompanyWebsitePage } from './website-page';
import { createRoleApi } from '../role/role-api';
import { createUserAdminApi } from '../user-admin/user-admin-api';
import { createCorpAdminApi } from '../corp/corp-admin-api';

type RoleApi = ReturnType<typeof createRoleApi>;
type UserAdminApi = ReturnType<typeof createUserAdminApi>;
type CorpAdminApi = ReturnType<typeof createCorpAdminApi>;

afterEach(cleanup);

function renderPage(node: React.ReactNode) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={client}>{node}</QueryClientProvider>);
}

describe('企业设置页面', () => {
  it('角色管理：数据加载并渲染行', async () => {
    const api = {
      list: vi.fn().mockResolvedValue({ list: [{ roleId: 1, name: '管理员', employeeNum: 2, remarks: 'x', status: 1, updatedAt: '2026-08-07T00:00:00Z' }], page: { page: 1, perPage: 20, total: 1, totalPage: 1 } }),
      permissions: vi.fn().mockResolvedValue([{ id: 10, name: '数据概览', checked: '1', children: [] }]),
      savePermissions: vi.fn().mockResolvedValue({}),
      create: vi.fn(), update: vi.fn(), remove: vi.fn(), updateStatus: vi.fn(),
    } as unknown as RoleApi;
    renderPage(<CompanyRolePage api={api} />);
    expect(await screen.findByText('管理员')).toBeTruthy();
  });

  it('角色管理：错误态与空态', async () => {
    const api = { list: vi.fn().mockRejectedValue(new Error('boom')) } as unknown as RoleApi;
    renderPage(<CompanyRolePage api={api} />);
    expect(await screen.findByText(/数据加载失败/)).toBeTruthy();
    const emptyApi = { list: vi.fn().mockResolvedValue({ list: [], page: { page: 1, perPage: 20, total: 0, totalPage: 1 } }) } as unknown as RoleApi;
    const { unmount } = renderPage(<CompanyRolePage api={emptyApi} />);
    expect(await screen.findByText('暂无角色')).toBeTruthy();
    unmount();
  });

  it('员工权限：加载并渲染行，含状态切换按钮', async () => {
    const api = {
      list: vi.fn().mockResolvedValue({ list: [{ userId: 3, userName: '张三', phone: '18600000000', gender: 1, roleId: 1, roleName: '管理员', status: 1, statusText: '启用', createdAt: '', department: [] }], normalNum: 1, notEnabledNum: 0, disableNum: 0, page: { perPage: 20, total: 1, totalPage: 1 } }),
      roles: vi.fn().mockResolvedValue([{ roleId: 1, name: '管理员' }]),
      updateStatus: vi.fn().mockResolvedValue({}),
      create: vi.fn(), update: vi.fn(), resetPassword: vi.fn(),
    } as unknown as UserAdminApi;
    renderPage(<CompanyStaffPage api={api} />);
    expect(await screen.findByText('张三')).toBeTruthy();
    expect(screen.getByRole('button', { name: '停用' })).toBeTruthy();
  });

  it('企业信息：加载并渲染行', async () => {
    const api = {
      list: vi.fn().mockResolvedValue({ list: [{ corpId: 1, corpName: '演示企业', wxCorpId: 'wx-1', createdAt: '2026-08-07T00:00:00Z' }], page: { perPage: 20, total: 1, totalPage: 1 } }),
      show: vi.fn(), create: vi.fn(), update: vi.fn(),
    } as unknown as CorpAdminApi;
    renderPage(<CompanyWebsitePage api={api} />);
    expect(await screen.findByText('演示企业')).toBeTruthy();
  });
});
