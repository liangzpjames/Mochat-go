import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ApiError } from '@mochat/api-client';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { CompanyRolePage } from './role-page';
import { CompanyStaffPage } from './staff-page';
import { CompanyWebsitePage } from './website-page';
import { CompanyAdditionalPage } from './additional-page';
import { CompanyAuthorizationPage } from './authorization-page';
import { createRoleApi } from '../role/role-api';
import { createUserAdminApi } from '../user-admin/user-admin-api';
import { createMenuAdminApi } from '../menu-admin/menu-admin-api';
import type { CompanyProfile, CompanyProfileApi } from './company-profile-api';

type RoleApi = ReturnType<typeof createRoleApi>;
type UserAdminApi = ReturnType<typeof createUserAdminApi>;
type MenuAdminApi = ReturnType<typeof createMenuAdminApi>;

afterEach(cleanup);
beforeAll(() => { globalThis.ResizeObserver = class { observe() {} unobserve() {} disconnect() {} }; });

function renderPage(node: React.ReactNode) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={client}>{node}</QueryClientProvider>);
}

const companyProfile: CompanyProfile = {
  tenantId: 7,
  corpId: 11,
  displayName: '演示企业',
  authoritativeCorpName: '权威企业名称',
  wxCorpId: 'wx-corp-11',
  bindingStatus: 'verified',
  bindingVersion: 4,
  credentials: {
    wecom: { configured: true },
    agent: { configured: false },
    archive: { configured: false },
  },
  updatedAt: '2026-08-12T00:00:00Z',
};

function companyApi(overrides: Partial<CompanyProfileApi> = {}): CompanyProfileApi {
  return {
    getProfile: vi.fn().mockResolvedValue(companyProfile),
    updateProfile: vi.fn().mockResolvedValue(companyProfile),
    rotateWeComCredentials: vi.fn().mockResolvedValue(companyProfile),
    rotateAgentCredentials: vi.fn().mockResolvedValue(companyProfile),
    rotateArchiveCredentials: vi.fn().mockResolvedValue(companyProfile),
    verify: vi.fn().mockResolvedValue(companyProfile),
    startEmployeeSync: vi.fn().mockResolvedValue({ status: 'queued', departmentsCreated: 0, departmentsUpdated: 0, employeesCreated: 0, employeesUpdated: 0 }),
    getSyncStatus: vi.fn().mockResolvedValue({ status: 'completed', departments: 3, employees: 8 }),
    listAudits: vi.fn().mockResolvedValue({ items: [], page: 1, perPage: 20, total: 0 }),
    ...overrides,
  };
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

  it('企业信息：只展示唯一企业资料、绑定状态和脱敏凭据状态', async () => {
    const api = companyApi();
    renderPage(<CompanyWebsitePage api={api} />);

    expect(await screen.findByText('权威企业名称')).toBeTruthy();
    expect(screen.getAllByText('已验证').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('已配置（加密保存）')).toBeTruthy();
    expect(screen.getAllByText('未配置').length).toBe(2);
    expect(screen.queryByRole('button', { name: '新建企业' })).toBeNull();
    expect(screen.queryByText('企业列表')).toBeNull();
    expect(screen.queryByText('分页')).toBeNull();
    expect(screen.queryByText('employee-plaintext')).toBeNull();
    expect(screen.queryByText('ciphertext')).toBeNull();
  });

  it('企业信息：待配置和暂停状态均 fail closed，不提供错误的同步或验证入口', async () => {
    const pendingApi = companyApi({ getProfile: vi.fn().mockResolvedValue({ ...companyProfile, bindingStatus: 'pending', wxCorpId: undefined, authoritativeCorpName: undefined }) });
    renderPage(<CompanyWebsitePage api={pendingApi} />);
    expect((await screen.findAllByText('待配置')).length).toBeGreaterThanOrEqual(1);
    expect(screen.getByRole('button', { name: '验证企业微信' })).toHaveProperty('disabled', true);
    expect(screen.getByRole('button', { name: '开始员工同步' })).toHaveProperty('disabled', true);
    cleanup();

    const suspendedApi = companyApi({ getProfile: vi.fn().mockResolvedValue({ ...companyProfile, bindingStatus: 'suspended' }) });
    renderPage(<CompanyWebsitePage api={suspendedApi} />);
    expect((await screen.findAllByText('已暂停')).length).toBeGreaterThanOrEqual(1);
    expect(screen.getByRole('button', { name: '验证企业微信' })).toHaveProperty('disabled', true);
    expect(screen.getByRole('button', { name: '开始员工同步' })).toHaveProperty('disabled', true);
  });

  it('企业信息：普通用户稳定显示403且不加载企业数据', async () => {
    const getProfile = vi.fn();
    const api = companyApi({ getProfile });
    renderPage(<CompanyWebsitePage api={api} isSuperAdmin={false} />);

    expect(await screen.findByText('暂无权限查看企业资料')).toBeTruthy();
    expect(getProfile).not.toHaveBeenCalled();
  });

  it('企业信息：加载失败可重试并显示空绑定状态', async () => {
    const getProfile = vi.fn()
      .mockRejectedValueOnce(new ApiError('server', 'internal', { status: 500, machineCode: 'INTERNAL_ERROR' }))
      .mockResolvedValue(companyProfile);
    const api = companyApi({ getProfile });
    renderPage(<CompanyWebsitePage api={api} />);

    expect(await screen.findByText('服务暂时不可用，请稍后重试。')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '重新加载' }));
    expect(await screen.findByText('权威企业名称')).toBeTruthy();
    expect(getProfile).toHaveBeenCalledTimes(2);
  });

  it('企业信息：变更摘要确认前不写入，409保留输入并提示刷新', async () => {
    const updateProfile = vi.fn().mockRejectedValue(new ApiError('validation', 'conflict', { status: 409, code: 409, machineCode: 'VERSION_CONFLICT' }));
    const api = companyApi({ updateProfile });
    renderPage(<CompanyWebsitePage api={api} />);

    const displayInput = await screen.findByLabelText('展示名称');
    fireEvent.change(displayInput, { target: { value: '新展示名' } });
    fireEvent.click(screen.getByRole('button', { name: '保存企业资料' }));
    expect(updateProfile).not.toHaveBeenCalled();
    expect(await screen.findByText(/变更摘要：展示名称改为/)).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '确认' }));
    await waitFor(() => expect(updateProfile).toHaveBeenCalledWith(expect.objectContaining({ displayName: '新展示名', expectedVersion: 4 })));
    expect(updateProfile.mock.calls[0]?.[0]).not.toHaveProperty('tenantId');
    expect(updateProfile.mock.calls[0]?.[0]).not.toHaveProperty('corpId');
    expect(await screen.findByText('资料已被其他操作更新，请刷新后重试；当前填写内容已保留。')).toBeTruthy();
    expect(displayInput).toHaveProperty('value', '新展示名');
  });

  it('企业信息：Secret留空不修改，凭据写入也必须确认且成功后清空', async () => {
    const rotate = vi.fn().mockResolvedValue(companyProfile);
    const api = companyApi({ rotateWeComCredentials: rotate });
    renderPage(<CompanyWebsitePage api={api} />);

    const employeeInput = await screen.findByLabelText('员工密钥');
    fireEvent.change(employeeInput, { target: { value: 'new-secret' } });
    fireEvent.click(screen.getByRole('button', { name: '保存企业微信凭据' }));
    expect(rotate).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: '确认' }));
    await waitFor(() => expect(rotate).toHaveBeenCalledWith(expect.objectContaining({ employeeSecret: 'new-secret', expectedVersion: 4 })));
    expect(rotate.mock.calls[0]?.[0]).not.toHaveProperty('tenantId');
    expect(rotate.mock.calls[0]?.[0]).not.toHaveProperty('corpId');
    await waitFor(() => expect(employeeInput).toHaveProperty('value', ''));
  });

  it('将会话存档 Secret 只提交到会话存档接口，不与企业微信凭据重复展示', async () => {
    const rotateArchive = vi.fn().mockResolvedValue(companyProfile);
    const api = companyApi({ rotateArchiveCredentials: rotateArchive });
    renderPage(<CompanyWebsitePage api={api} />);

    const archiveInputs = await screen.findAllByLabelText('会话存档 Secret');
    expect(archiveInputs).toHaveLength(1);
    const archiveInput = archiveInputs[0];
    if (archiveInput === undefined) throw new Error('archive secret input is missing');
    fireEvent.change(archiveInput, { target: { value: 'archive-secret' } });
    fireEvent.click(screen.getByRole('button', { name: '保存会话存档' }));
    expect(rotateArchive).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: '确认' }));
    await waitFor(() => expect(rotateArchive).toHaveBeenCalledWith(expect.objectContaining({ chatSecret: 'archive-secret', expectedVersion: 4 })));
  });

  it('企业信息：同步状态明确且同步触发需确认，390px仍为单列可达控件', async () => {
    const startEmployeeSync = vi.fn().mockResolvedValue({ status: 'queued', departmentsCreated: 0, departmentsUpdated: 0, employeesCreated: 0, employeesUpdated: 0 });
    const api = companyApi({ startEmployeeSync, getSyncStatus: vi.fn().mockResolvedValue({ status: 'syncing', departments: 2, employees: 5 }) });
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 });
    renderPage(<CompanyWebsitePage api={api} />);

    expect(await screen.findByText('同步中')).toBeTruthy();
    const syncButton = screen.getByRole('button', { name: '开始员工同步' });
    expect(syncButton).toHaveProperty('disabled', true);
    expect(document.querySelector('.company-profile-form-grid')).not.toBeNull();
    expect(document.querySelector('table')).toBeNull();
  });

  it('企业信息：同步失败状态展示脱敏错误码并保留重试入口', async () => {
    const api = companyApi({ getSyncStatus: vi.fn().mockResolvedValue({ status: 'failed', departments: 2, employees: 5, errorCode: 'SYNC_FAILED' }) });
    renderPage(<CompanyWebsitePage api={api} />);

    expect(await screen.findByText('同步失败')).toBeTruthy();
    expect(screen.getByText('错误：SYNC_FAILED')).toBeTruthy();
    expect(screen.getByRole('button', { name: '刷新同步状态' })).toBeTruthy();
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
