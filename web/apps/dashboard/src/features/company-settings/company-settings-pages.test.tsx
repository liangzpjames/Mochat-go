import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { CompanyRolePage } from './role-page';
import { CompanyStaffPage } from './staff-page';
import { CompanyWebsitePage } from './website-page';
import { CompanyAdditionalPage } from './additional-page';
import { CompanyAuthorizationPage } from './authorization-page';
import { createRoleApi } from '../role/role-api';
import { createUserAdminApi } from '../user-admin/user-admin-api';
import { createCorpAdminApi } from '../corp/corp-admin-api';
import { createMenuAdminApi } from '../menu-admin/menu-admin-api';

type RoleApi = ReturnType<typeof createRoleApi>;
type UserAdminApi = ReturnType<typeof createUserAdminApi>;
type CorpAdminApi = ReturnType<typeof createCorpAdminApi>;
type MenuAdminApi = ReturnType<typeof createMenuAdminApi>;

afterEach(cleanup);
beforeAll(() => { globalThis.ResizeObserver = class { observe() {} unobserve() {} disconnect() {} }; });

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
    expect(screen.getByRole('button', { name: '停用 张三' })).toBeTruthy();
  });

  it('员工权限：确认前不停用员工', async () => {
    const updateStatus = vi.fn().mockResolvedValue({});
    const api = {
      list: vi.fn().mockResolvedValue({ list: [{ userId: 3, userName: '张三', phone: '18600000000', gender: 1, roleId: 1, roleName: '管理员', status: 1, statusText: '启用', createdAt: '', department: [] }], normalNum: 1, notEnabledNum: 0, disableNum: 0, page: { perPage: 20, total: 1, totalPage: 1 } }),
      roles: vi.fn().mockResolvedValue([{ roleId: 1, name: '管理员' }]),
      updateStatus,
    } as unknown as UserAdminApi;
    renderPage(<CompanyStaffPage api={api} />);

    fireEvent.click(await screen.findByRole('button', { name: '停用 张三' }));
    expect(updateStatus).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));
    await waitFor(() => expect(updateStatus).toHaveBeenCalledTimes(1));
  });

  it('员工权限：取消编辑不覆盖手机号筛选', async () => {
    const api = {
      list: vi.fn().mockResolvedValue({ list: [{ userId: 3, userName: '张三', phone: '18600000000', gender: 1, roleId: 1, roleName: '管理员', status: 1, statusText: '启用', createdAt: '', department: [] }], normalNum: 1, notEnabledNum: 0, disableNum: 0, page: { perPage: 20, total: 1, totalPage: 1 } }),
      roles: vi.fn().mockResolvedValue([{ roleId: 1, name: '管理员' }]),
      updateStatus: vi.fn(), update: vi.fn(), create: vi.fn(), resetPassword: vi.fn(),
    } as unknown as UserAdminApi;
    renderPage(<CompanyStaffPage api={api} />);

    const phoneFilter = await screen.findByLabelText('手机号筛选');
    fireEvent.change(phoneFilter, { target: { value: '138' } });
    fireEvent.click(screen.getByRole('button', { name: '编辑' }));
    fireEvent.click(screen.getByRole('button', { name: '取消' }));

    expect(phoneFilter).toHaveProperty('value', '138');
  });

  it('企业信息：加载并渲染行', async () => {
    const api = {
      list: vi.fn().mockResolvedValue({ list: [{ corpId: 1, corpName: '演示企业', wxCorpId: 'wx-1', createdAt: '2026-08-07T00:00:00Z' }], page: { perPage: 20, total: 1, totalPage: 1 } }),
      show: vi.fn(), create: vi.fn(), update: vi.fn(),
    } as unknown as CorpAdminApi;
    renderPage(<CompanyWebsitePage api={api} />);
    expect(await screen.findByText('演示企业')).toBeTruthy();
  });

  it('企业信息：编辑时不回填明文密钥', async () => {
    const api = {
      list: vi.fn().mockResolvedValue({ list: [{ corpId: 1, corpName: '演示企业', wxCorpId: 'wx-1', createdAt: '2026-08-07T00:00:00Z' }], page: { perPage: 20, total: 1, totalPage: 1 } }),
      show: vi.fn().mockResolvedValue({ corpId: 1, corpName: '演示企业', wxCorpId: 'wx-1', employeeSecret: 'employee-plaintext', contactSecret: 'contact-plaintext', eventCallback: '', token: '', encodingAesKey: '' }),
      create: vi.fn(), update: vi.fn(),
    } as unknown as CorpAdminApi;
    renderPage(<CompanyWebsitePage api={api} />);

    fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
    const employeeSecret = await screen.findByLabelText('员工密钥');
    expect(employeeSecret).toHaveProperty('value', '');
    expect(employeeSecret.getAttribute('type')).toBe('password');
    expect(screen.getByText('留空表示不修改')).toBeTruthy();
  });

  it('附加权限：状态切换和删除都需确认', async () => {
    const updateStatus = vi.fn().mockResolvedValue({});
    const remove = vi.fn().mockResolvedValue({});
    const api = {
      list: vi.fn().mockResolvedValue({ list: [{ menuId: 5, parentId: 0, name: '朋友圈', level: 1, levelName: '一级', menuPath: '/moments', icon: '', status: 1, updatedAt: '' }], page: { total: 1, totalPage: 1 } }),
      updateStatus, remove,
    } as unknown as MenuAdminApi;
    renderPage(<CompanyAdditionalPage api={api} />);

    fireEvent.click(await screen.findByRole('button', { name: '停用 朋友圈' }));
    expect(updateStatus).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole('button', { name: '取消' }));
    expect(updateStatus).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: '删除 朋友圈' }));
    expect(remove).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));
    await waitFor(() => expect(remove).toHaveBeenCalledTimes(1));
  });

  it('授权管理：确认前不停用节点', async () => {
    const updateStatus = vi.fn().mockResolvedValue({});
    const api = {
      list: vi.fn().mockResolvedValue({ list: [{ menuId: 5, parentId: 0, name: '朋友圈', level: 1, levelName: '一级', menuPath: '/moments', icon: '', status: 1, updatedAt: '' }], page: { total: 1, totalPage: 1 } }),
      updateStatus,
    } as unknown as MenuAdminApi;
    renderPage(<CompanyAuthorizationPage api={api} />);

    fireEvent.click(await screen.findByRole('button', { name: '停用 朋友圈' }));
    expect(updateStatus).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));
    await waitFor(() => expect(updateStatus).toHaveBeenCalledTimes(1));
  });

  it('员工权限：共享 Modal 支持取消并恢复触发焦点', async () => {
    const api = { list: vi.fn().mockResolvedValue({ list: [], page: { total: 0, totalPage: 1 } }), roles: vi.fn().mockResolvedValue([{ roleId: 1, name: '管理员' }]) } as unknown as UserAdminApi;
    renderPage(<CompanyStaffPage api={api} />);
    const trigger = await screen.findByRole('button', { name: '新增员工' });
    fireEvent.click(trigger);
    const dialog = await screen.findByRole('dialog', { name: '新增员工' });
    expect(dialog.closest('.dashboard-dialog--modal')).not.toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '取消' }));
    await waitFor(() => expect(screen.queryByRole('dialog', { name: '新增员工' })).toBeNull());
    await waitFor(() => expect(document.activeElement).toBe(trigger));
  });

  it('角色授权树使用中文映射并安全回退未知权限名', async () => {
    const api = { list: vi.fn().mockResolvedValue({ list: [{ roleId: 1, name: '管理员', employeeNum: 2, remarks: '', status: 1, updatedAt: '' }], page: { total: 1, totalPage: 1 } }), permissions: vi.fn().mockResolvedValue([{ id: 10, name: 'Friends circle', checked: '1', children: [] }, { id: 11, name: '??? Provider ??', checked: '3', children: [] }]), savePermissions: vi.fn() } as unknown as RoleApi;
    renderPage(<CompanyRolePage api={api} />);
    fireEvent.click(await screen.findByRole('button', { name: '权限' }));
    expect(await screen.findByText('朋友圈')).toBeTruthy();
    expect(screen.getByText('未命名权限（11）')).toBeTruthy();
    expect(screen.queryByText('Friends circle')).toBeNull();
    expect(screen.queryByText('??? Provider ??')).toBeNull();
  });

  it('授权管理树使用中文业务名称并安全回退未知 Provider 权限', async () => {
    const api = {
      list: vi.fn().mockResolvedValue({
        list: [
          { menuId: 114001, parentId: 0, name: 'Friends circle task query', level: 4, levelName: '四级菜单', menuPath: '3', icon: '', status: 1, updatedAt: '' },
          { menuId: 114002, parentId: 0, name: 'Friends circle material query', level: 4, levelName: '四级菜单', menuPath: '4', icon: '', status: 1, updatedAt: '' },
          { menuId: 114003, parentId: 0, name: 'Friends circle task draft', level: 4, levelName: '四级菜单', menuPath: '5', icon: '', status: 1, updatedAt: '' },
          { menuId: 114004, parentId: 0, name: 'Friends circle material create', level: 4, levelName: '四级菜单', menuPath: '6', icon: '', status: 1, updatedAt: '' },
          { menuId: 114005, parentId: 0, name: 'Friends circle publish', level: 4, levelName: '四级菜单', menuPath: '7', icon: '', status: 1, updatedAt: '' },
          { menuId: 117003, parentId: 0, name: '??? Provider ??', level: 4, levelName: '四级菜单', menuPath: '8', icon: '', status: 1, updatedAt: '' },
        ],
        page: { total: 6, totalPage: 1 },
      }),
      updateStatus: vi.fn(),
    } as unknown as MenuAdminApi;

    renderPage(<CompanyAuthorizationPage api={api} />);

    expect(await screen.findByText('朋友圈任务查询')).toBeTruthy();
    expect(screen.getByText('朋友圈素材查询')).toBeTruthy();
    expect(screen.getByText('朋友圈任务草稿')).toBeTruthy();
    expect(screen.getByText('朋友圈素材创建')).toBeTruthy();
    expect(screen.getByText('朋友圈发布')).toBeTruthy();
    expect(screen.getByText('未命名权限（117003）')).toBeTruthy();
    expect(screen.queryByText(/Friends circle/i)).toBeNull();
    expect(screen.queryByText('??? Provider ??')).toBeNull();
  });
});
