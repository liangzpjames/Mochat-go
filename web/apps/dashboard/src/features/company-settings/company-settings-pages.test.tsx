import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ApiError } from '@mochat/api-client';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { CompanyRolePage } from './role-page';
import { CompanyStaffPage } from './staff-page';
import { CompanyWebsitePage } from './website-page';
import { CompanyAdditionalPage } from './additional-page';
import { CompanyAuthorizationPage } from './authorization-page';
import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
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
  const view = render(<QueryClientProvider client={client}>{node}</QueryClientProvider>);
  return { ...view, client };
}

const companyProfile: CompanyProfile = {
  tenantId: 7,
  corpId: 11,
  displayName: '演示企业',
  authoritativeCorpName: '权威企业名称',
  wxCorpId: 'wx-corp-11',
  applicationAgentId: '1000099',
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
    configureApplication: vi.fn().mockResolvedValue(companyProfile),
    rotateArchiveCredentials: vi.fn().mockResolvedValue(companyProfile),
    getCallbackConfiguration: vi.fn().mockResolvedValue({
      corpId: 11,
      callbackUrl: 'http://localhost:18080/weWork/callback?cid=11',
      token: 'callback-token',
      encodingAESKey: 'a'.repeat(43),
      configured: true,
      bindingVersion: 4,
    }),
    regenerateCallbackConfiguration: vi.fn().mockResolvedValue({
      corpId: 11,
      callbackUrl: 'http://localhost:18080/weWork/callback?cid=11',
      token: 'next-callback-token',
      encodingAESKey: 'b'.repeat(43),
      configured: true,
      bindingVersion: 5,
    }),
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

  it('role pagination clamps an empty response to one page', async () => {
    const api = { list: vi.fn().mockResolvedValue({ list: [], page: { page: 1, perPage: 20, total: 0, totalPage: 0 } }) } as unknown as RoleApi;
    renderPage(<CompanyRolePage api={api} />);
    await screen.findByText('暂无角色');
    expect(screen.queryByText(/1\/0/)).toBeNull();
    expect(await screen.findByText(/1\/1/)).toBeTruthy();
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
    renderPage(<CompanyWebsitePage api={api} isSuperAdmin />);

    expect(await screen.findByText('权威企业名称')).toBeTruthy();
    expect(screen.getByDisplayValue('1000099')).toBeTruthy();
    expect(screen.getAllByText('已验证').length).toBeGreaterThanOrEqual(1);
    expect(await screen.findByDisplayValue('http://localhost:18080/weWork/callback?cid=11')).toBeTruthy();
    expect(screen.getAllByText('已配置（加密保存）').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('未配置').length).toBeGreaterThanOrEqual(1);
    expect(screen.queryByRole('button', { name: '新建企业' })).toBeNull();
    expect(screen.queryByText('企业列表')).toBeNull();
    expect(screen.queryByText('分页')).toBeNull();
    expect(screen.queryByText('employee-plaintext')).toBeNull();
    expect(screen.queryByText('ciphertext')).toBeNull();
  });

  it('企业信息：保存后的 AgentID 回显，Secret 以已加密保存状态代替明文回显', async () => {
    const configuredProfile: CompanyProfile = {
      ...companyProfile,
      credentials: {
        ...companyProfile.credentials,
        agent: { configured: true },
      },
    };
    const api = companyApi({ getProfile: vi.fn().mockResolvedValue(configuredProfile) });
    renderPage(<CompanyWebsitePage api={api} isSuperAdmin />);

    expect(await screen.findByDisplayValue('1000099')).toBeTruthy();
    const secretInput = screen.getByLabelText('应用 Secret');
    expect(secretInput).toHaveProperty('value', '');
    expect(secretInput).toHaveProperty('placeholder', '已加密保存；输入新 Secret 可替换');
    expect(screen.getByText('应用 Secret 已加密保存，出于安全原因不会回显。')).toBeTruthy();
  });

  it('企业信息：第三方代开发模式只展示 SaaS 托管说明且不读取或修改企微凭据', async () => {
    const getCallbackConfiguration = vi.fn();
    const getSyncStatus = vi.fn();
    const api = companyApi({
      getProfile: vi.fn().mockResolvedValue({ ...companyProfile, wecomIntegrationMode: 'third_party_delegated' }),
      getCallbackConfiguration,
      getSyncStatus,
    });
    renderPage(<CompanyWebsitePage api={api} isSuperAdmin />);

    expect(await screen.findByText('企微配置由 SaaS 平台维护')).toBeTruthy();
    expect(screen.getAllByText('第三方代开发应用').length).toBeGreaterThanOrEqual(2);
    expect(screen.queryByLabelText('应用 Secret')).toBeNull();
    expect(screen.queryByRole('button', { name: '验证企业微信' })).toBeNull();
    expect(screen.queryByRole('button', { name: '开始员工同步' })).toBeNull();
    expect(getCallbackConfiguration).not.toHaveBeenCalled();
    expect(getSyncStatus).not.toHaveBeenCalled();
  });

  it('企业信息：待配置和暂停状态均 fail closed，不提供错误的同步或验证入口', async () => {
    const pendingApi = companyApi({ getProfile: vi.fn().mockResolvedValue({ ...companyProfile, bindingStatus: 'pending', wxCorpId: undefined, authoritativeCorpName: undefined }) });
    renderPage(<CompanyWebsitePage api={pendingApi} isSuperAdmin />);
    expect((await screen.findAllByText('待配置')).length).toBeGreaterThanOrEqual(1);
    expect(screen.getByRole('button', { name: '验证企业微信' })).toHaveProperty('disabled', true);
    expect(screen.getByRole('button', { name: '开始员工同步' })).toHaveProperty('disabled', true);
    cleanup();

    const suspendedApi = companyApi({ getProfile: vi.fn().mockResolvedValue({ ...companyProfile, bindingStatus: 'suspended' }) });
    renderPage(<CompanyWebsitePage api={suspendedApi} isSuperAdmin />);
    expect((await screen.findAllByText('已暂停')).length).toBeGreaterThanOrEqual(1);
    expect(screen.getByRole('button', { name: '验证企业微信' })).toHaveProperty('disabled', true);
    expect(screen.getByRole('button', { name: '开始员工同步' })).toHaveProperty('disabled', true);
  });

  it('企业信息：待配置同步状态显示中性提示而非故障重试', async () => {
    const getSyncStatus = vi.fn().mockRejectedValue(
      new ApiError('forbidden', 'configuration required', {
        status: 403,
        code: 403,
        machineCode: 'CORP_CONFIGURATION_REQUIRED',
      }),
    );
    const api = companyApi({
      getProfile: vi.fn().mockResolvedValue({ ...companyProfile, bindingStatus: 'pending', wxCorpId: undefined, authoritativeCorpName: undefined }),
      getSyncStatus,
    });
    renderPage(<CompanyWebsitePage api={api} isSuperAdmin />);

    expect(await screen.findByText('完成企业微信验证后即可同步员工')).toBeTruthy();
    expect(screen.queryByText('同步状态读取失败。')).toBeNull();
    expect(screen.queryByRole('button', { name: '重试' })).toBeNull();
    expect(screen.getByRole('button', { name: '开始员工同步' })).toHaveProperty('disabled', true);
  });

  it('企业信息：同步状态网络错误仍显示故障重试', async () => {
    const api = companyApi({
      getSyncStatus: vi.fn().mockRejectedValue(new ApiError('server', 'internal', { status: 500, code: 500, machineCode: 'INTERNAL_ERROR' })),
    });
    renderPage(<CompanyWebsitePage api={api} isSuperAdmin />);

    expect(await screen.findByText('同步状态读取失败。')).toBeTruthy();
    expect(screen.getByRole('button', { name: '重试' })).toBeTruthy();
  });

  it('企业信息：普通用户稳定显示403且不加载企业数据', async () => {
    const getProfile = vi.fn();
    const api = companyApi({ getProfile });
    renderPage(<CompanyWebsitePage api={api} isSuperAdmin={false} />);

    expect(await screen.findByText('暂无权限查看企业资料')).toBeTruthy();
    expect(getProfile).not.toHaveBeenCalled();
  });

  it('企业信息：拥有页面授权的普通用户会加载唯一企业资料', async () => {
    const getProfile = vi.fn().mockResolvedValue(companyProfile);
    const api = companyApi({ getProfile });
    const access: AccessContext = {
      session: { token: 'token', userId: '7', expiresAt: Date.now() + 60_000 },
      corp: { id: '11', name: '演示企业', authorized: true },
      profile: {
        userId: 7,
        userName: '普通用户',
        tenantId: 7,
        corpId: 11,
        corpName: '演示企业',
        workEmployeeId: 0,
        departmentIds: [],
        departmentEmployeeIds: [],
        isSuperAdmin: false,
        corpBindingStatus: 'verified',
        catalog: [],
        effectivePermissions: [{
          code: 'dashboard.company_setting.website',
          path: '/company-setting/website',
          name: '唯一企业资料',
          scope: 'tenant',
          sources: [],
        }],
        allowedRoutes: ['/company-setting/website'],
      },
      allowedRoutes: new Set(['/company-setting/website']),
      allowedActions: new Set(),
    };
    renderPage(
      <DashboardAccessProvider value={access}>
        <CompanyWebsitePage api={api} />
      </DashboardAccessProvider>,
    );

    expect(await screen.findByText('权威企业名称')).toBeTruthy();
    expect(getProfile).toHaveBeenCalledOnce();
  });

  it('企业信息：拥有页面授权的普通用户仍看不到 Provider 超管诊断', async () => {
    const api = companyApi();
    const providerStatusApi = {
      getStatus: vi.fn().mockResolvedValue({
        providers: [{
          kind: 'wecom_archive' as const,
          state: 'limited' as const,
          code: 'archive.credentials_missing',
          source: 'external' as const,
          action: '请联系管理员配置 Provider',
          reason: 'sensitive diagnostic reason',
          missing: ['MOCHAT_ARCHIVE_SECRET'],
          capabilities: ['archive_sync'],
          capabilityStatuses: [],
        }],
        freshAt: '2026-08-24T00:00:00Z',
      }),
    };
    const access: AccessContext = {
      session: { token: 'token', userId: '7', expiresAt: Date.now() + 60_000 },
      corp: { id: '11', name: '演示企业', authorized: true },
      profile: {
        userId: 7,
        userName: '普通用户',
        tenantId: 7,
        corpId: 11,
        corpName: '演示企业',
        workEmployeeId: 0,
        departmentIds: [],
        departmentEmployeeIds: [],
        isSuperAdmin: false,
        corpBindingStatus: 'verified',
        catalog: [],
        effectivePermissions: [],
        allowedRoutes: ['/company-setting/website'],
      },
      allowedRoutes: new Set(['/company-setting/website']),
      allowedActions: new Set(),
    };
    renderPage(
      <DashboardAccessProvider value={access}>
        <CompanyWebsitePage api={api} isSuperAdmin providerStatusApi={providerStatusApi} />
      </DashboardAccessProvider>,
    );

    expect(await screen.findByText('Provider 运行状态')).toBeTruthy();
    expect(screen.getByText('下一步：请联系管理员配置 Provider')).toBeTruthy();
    expect(screen.queryByText('sensitive diagnostic reason')).toBeNull();
    expect(screen.queryByText('MOCHAT_ARCHIVE_SECRET')).toBeNull();
  });

  it('企业信息：旧测试访问上下文缺少 profile 时仍失败关闭 Provider 诊断', async () => {
    const providerStatusApi = {
      getStatus: vi.fn().mockResolvedValue({
        providers: [{
          kind: 'wecom_archive' as const,
          state: 'limited' as const,
          code: 'archive.credentials_missing',
          source: 'external' as const,
          action: '请联系管理员配置 Provider',
          reason: 'legacy fixture secret reason',
          missing: ['MOCHAT_ARCHIVE_SECRET'],
          capabilities: ['archive_sync'],
          capabilityStatuses: [],
        }],
        freshAt: '2026-08-24T00:00:00Z',
      }),
    };
    const access: AccessContext = {
      session: { token: 'token', userId: '7', expiresAt: Date.now() + 60_000 },
      corp: { id: '11', name: '演示企业', authorized: true },
      allowedRoutes: new Set(['/company-setting/website']),
      allowedActions: new Set(),
    };
    renderPage(
      <DashboardAccessProvider value={access}>
        <CompanyWebsitePage api={companyApi()} providerStatusApi={providerStatusApi} />
      </DashboardAccessProvider>,
    );

    expect(await screen.findByText('Provider 运行状态')).toBeTruthy();
    expect(screen.queryByText('legacy fixture secret reason')).toBeNull();
    expect(screen.queryByText('MOCHAT_ARCHIVE_SECRET')).toBeNull();
  });

  it('企业信息：缺少页面授权的普通用户不加载企业数据', async () => {
    const getProfile = vi.fn();
    const api = companyApi({ getProfile });
    const access: AccessContext = {
      session: { token: 'token', userId: '7', expiresAt: Date.now() + 60_000 },
      corp: { id: '11', name: '演示企业', authorized: true },
      profile: {
        userId: 7,
        userName: '普通用户',
        tenantId: 7,
        corpId: 11,
        corpName: '演示企业',
        workEmployeeId: 0,
        departmentIds: [],
        departmentEmployeeIds: [],
        isSuperAdmin: false,
        corpBindingStatus: 'verified',
        catalog: [],
        effectivePermissions: [],
        allowedRoutes: [],
      },
      allowedRoutes: new Set(),
      allowedActions: new Set(),
    };
    renderPage(
      <DashboardAccessProvider value={access}>
        <CompanyWebsitePage api={api} />
      </DashboardAccessProvider>,
    );

    expect(await screen.findByText('暂无权限查看企业资料')).toBeTruthy();
    expect(screen.getByText('当前账号未获企业资料页面权限，请联系管理员授权。')).toBeTruthy();
    expect(getProfile).not.toHaveBeenCalled();
  });

  it('fails closed when no dashboard access result is provided', async () => {
    const getProfile = vi.fn();
    const api = companyApi({ getProfile });
    renderPage(<CompanyWebsitePage api={api} />);

    expect(await screen.findByText('暂无权限查看企业资料')).toBeTruthy();
    expect(getProfile).not.toHaveBeenCalled();
  });

  it('disables profile save for a whitespace-only display name', async () => {
    const updateProfile = vi.fn();
    const api = companyApi({ updateProfile });
    renderPage(<CompanyWebsitePage api={api} isSuperAdmin />);

    const displayInput = await screen.findByLabelText('展示名称');
    fireEvent.change(displayInput, { target: { value: '   ' } });
    const saveButton = screen.getByRole('button', { name: '保存企业资料' });
    expect(saveButton).toHaveProperty('disabled', true);
    expect(updateProfile).not.toHaveBeenCalled();
  });

  it('应用配置只保留 AgentID 和一个 Secret，确认后统一提交', async () => {
    const configureApplication = vi.fn().mockResolvedValue(companyProfile);
    const api = companyApi({ configureApplication });
    renderPage(<CompanyWebsitePage api={api} isSuperAdmin />);

    const agentIDInput = await screen.findByLabelText('应用 AgentID');
    const secretInput = screen.getByLabelText('应用 Secret');
    fireEvent.change(agentIDInput, { target: { value: '1000010' } });
    fireEvent.change(secretInput, { target: { value: 'one-secret' } });
    fireEvent.click(screen.getByRole('button', { name: '保存应用配置' }));
    expect(configureApplication).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));

    await waitFor(() => expect(configureApplication).toHaveBeenCalledWith(expect.objectContaining({
      wxAgentId: '1000010', secret: 'one-secret', expectedVersion: 4,
    })));
    expect(screen.queryByLabelText('员工密钥')).toBeNull();
    expect(screen.queryByLabelText('客户联系密钥')).toBeNull();
    expect(screen.queryByLabelText('AgentID', { exact: true })).toBeNull();
  });

  it('does not retain secrets in React Query mutation state after success or failure', async () => {
    const agentSecret = 'agent-test-secret';
    const archiveSecret = 'archive-test-secret';
    const configureApplication = vi.fn().mockResolvedValue(companyProfile);
    const rotateArchiveCredentials = vi.fn().mockRejectedValue(new ApiError('validation', 'conflict', { status: 409, machineCode: 'VERSION_CONFLICT' }));
    const api = companyApi({ configureApplication, rotateArchiveCredentials });
    const { client } = renderPage(<CompanyWebsitePage api={api} isSuperAdmin />);

    const agentIDInput = await screen.findByLabelText('应用 AgentID');
    const agentSecretInput = screen.getByLabelText('应用 Secret');
    fireEvent.change(agentIDInput, { target: { value: '1000010' } });
    fireEvent.change(agentSecretInput, { target: { value: agentSecret } });
    fireEvent.click(screen.getByRole('button', { name: '保存应用配置' }));
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));
    await waitFor(() => expect(configureApplication).toHaveBeenCalled());

    const archiveInput = screen.getByLabelText('会话存档 Secret');
    const publicKeyInput = screen.getByLabelText('会话存档 RSA 公钥');
    const privateKeyInput = screen.getByLabelText('会话存档 RSA 私钥');
    fireEvent.change(archiveInput, { target: { value: archiveSecret } });
    fireEvent.change(publicKeyInput, { target: { value: 'public-pem' } });
    fireEvent.change(privateKeyInput, { target: { value: 'private-pem' } });
    fireEvent.click(screen.getByRole('button', { name: '保存会话存档' }));
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));
    await waitFor(() => expect(rotateArchiveCredentials).toHaveBeenCalled());
    expect(archiveInput).toHaveProperty('value', archiveSecret);

    const mutationCacheSnapshot = JSON.stringify(client.getMutationCache().getAll());
    expect(mutationCacheSnapshot).not.toContain(agentSecret);
    expect(mutationCacheSnapshot).not.toContain(archiveSecret);
    expect(mutationCacheSnapshot).not.toContain('private-pem');
    expect(mutationCacheSnapshot).not.toContain('ciphertext');
  });

  it('企业信息：加载失败可重试并显示空绑定状态', async () => {
    const getProfile = vi.fn()
      .mockRejectedValueOnce(new ApiError('server', 'internal', { status: 500, machineCode: 'INTERNAL_ERROR' }))
      .mockResolvedValue(companyProfile);
    const api = companyApi({ getProfile });
    renderPage(<CompanyWebsitePage api={api} isSuperAdmin />);

    expect(await screen.findByText('服务暂时不可用，请稍后重试。')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '重新加载' }));
    expect(await screen.findByText('权威企业名称')).toBeTruthy();
    expect(getProfile).toHaveBeenCalledTimes(2);
  });

  it('企业信息：变更摘要确认前不写入，409保留输入并提示刷新', async () => {
    const updateProfile = vi.fn().mockRejectedValue(new ApiError('validation', 'conflict', { status: 409, code: 409, machineCode: 'VERSION_CONFLICT' }));
    const api = companyApi({ updateProfile });
    renderPage(<CompanyWebsitePage api={api} isSuperAdmin />);

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

  it('企业信息：应用 Secret 和 AgentID 都必填，成功后清空 Secret', async () => {
    const configureApplication = vi.fn().mockResolvedValue(companyProfile);
    const api = companyApi({ configureApplication });
    renderPage(<CompanyWebsitePage api={api} isSuperAdmin />);

    const agentIDInput = await screen.findByLabelText('应用 AgentID');
    const secretInput = screen.getByLabelText('应用 Secret');
    fireEvent.change(agentIDInput, { target: { value: '1000010' } });
    expect(screen.getByRole('button', { name: '保存应用配置' })).toHaveProperty('disabled', true);
    fireEvent.change(secretInput, { target: { value: 'new-secret' } });
    fireEvent.click(screen.getByRole('button', { name: '保存应用配置' }));
    expect(configureApplication).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: '确认' }));
    await waitFor(() => expect(configureApplication).toHaveBeenCalledWith(expect.objectContaining({ secret: 'new-secret', wxAgentId: '1000010', expectedVersion: 4 })));
    expect(configureApplication.mock.calls[0]?.[0]).not.toHaveProperty('tenantId');
    expect(configureApplication.mock.calls[0]?.[0]).not.toHaveProperty('corpId');
    await waitFor(() => expect(secretInput).toHaveProperty('value', ''));
  });

  it('会话存档同时提交 Secret、RSA 公钥和 RSA 私钥', async () => {
    const rotateArchive = vi.fn().mockResolvedValue(companyProfile);
    const api = companyApi({ rotateArchiveCredentials: rotateArchive });
    renderPage(<CompanyWebsitePage api={api} isSuperAdmin />);

    const archiveInputs = await screen.findAllByLabelText('会话存档 Secret');
    expect(archiveInputs).toHaveLength(1);
    const archiveInput = archiveInputs[0];
    if (archiveInput === undefined) throw new Error('archive secret input is missing');
    fireEvent.change(archiveInput, { target: { value: 'archive-secret' } });
    fireEvent.change(screen.getByLabelText('会话存档 RSA 公钥'), { target: { value: 'public-pem' } });
    fireEvent.change(screen.getByLabelText('会话存档 RSA 私钥'), { target: { value: 'private-pem' } });
    fireEvent.click(screen.getByRole('button', { name: '保存会话存档' }));
    expect(rotateArchive).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: '确认' }));
    await waitFor(() => expect(rotateArchive).toHaveBeenCalledWith(expect.objectContaining({
      chatSecret: 'archive-secret', rsaPublicKey: 'public-pem', rsaPrivateKey: 'private-pem', expectedVersion: 4,
    })));
  });

  it('回调配置只读展示并确认后重新生成', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } });
    const regenerate = vi.fn().mockResolvedValue({
      corpId: 11,
      callbackUrl: 'http://localhost:18080/weWork/callback?cid=11',
      token: 'next-token',
      encodingAESKey: 'b'.repeat(43),
      configured: true,
      bindingVersion: 5,
    });
    const api = companyApi({ regenerateCallbackConfiguration: regenerate });
    renderPage(<CompanyWebsitePage api={api} isSuperAdmin />);

    expect(await screen.findByDisplayValue('http://localhost:18080/weWork/callback?cid=11')).toHaveProperty('readOnly', true);
    expect(screen.getByDisplayValue('http://localhost:18080/wecom/archive/callback?cid=11')).toHaveProperty('readOnly', true);
    expect(screen.getByDisplayValue('callback-token')).toHaveProperty('readOnly', true);
    expect(screen.getByDisplayValue('a'.repeat(43))).toHaveProperty('readOnly', true);
    fireEvent.click(screen.getByRole('button', { name: '复制Token' }));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith('callback-token'));
    fireEvent.click(screen.getByRole('button', { name: '重新生成回调配置' }));
    expect(regenerate).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));
    await waitFor(() => expect(regenerate).toHaveBeenCalledWith(expect.objectContaining({ expectedVersion: 4 })));
    expect(await screen.findByDisplayValue('next-token')).toBeTruthy();
  });

  it('企业信息：同步状态明确且同步触发需确认，390px仍为单列可达控件', async () => {
    const startEmployeeSync = vi.fn().mockResolvedValue({ status: 'queued', departmentsCreated: 0, departmentsUpdated: 0, employeesCreated: 0, employeesUpdated: 0 });
    const api = companyApi({ startEmployeeSync, getSyncStatus: vi.fn().mockResolvedValue({ status: 'syncing', departments: 2, employees: 5 }) });
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 });
    renderPage(<CompanyWebsitePage api={api} isSuperAdmin />);

    expect(await screen.findByText('同步中')).toBeTruthy();
    const syncButton = screen.getByRole('button', { name: '开始员工同步' });
    expect(syncButton).toHaveProperty('disabled', true);
    expect(document.querySelector('.company-profile-form-grid')).not.toBeNull();
    expect(document.querySelector('table')).toBeNull();
  });

  it('企业信息：同步失败状态展示脱敏错误码并保留重试入口', async () => {
    const api = companyApi({ getSyncStatus: vi.fn().mockResolvedValue({ status: 'failed', departments: 2, employees: 5, errorCode: 'SYNC_FAILED' }) });
    renderPage(<CompanyWebsitePage api={api} isSuperAdmin />);

    expect(await screen.findByText('同步失败')).toBeTruthy();
    expect(screen.getByText('错误：SYNC_FAILED')).toBeTruthy();
    expect(screen.getByRole('button', { name: '刷新同步状态' })).toBeTruthy();
  });

  it('企业信息：员工同步失败提示只显示在员工同步卡片内', async () => {
    const startEmployeeSync = vi.fn().mockRejectedValue(
      new ApiError('validation', 'invalid request', {
        status: 400,
        code: 400,
        machineCode: 'INVALID_REQUEST',
      }),
    );
    const api = companyApi({
      startEmployeeSync,
      getSyncStatus: vi.fn().mockResolvedValue({ status: 'idle', departments: 0, employees: 0 }),
    });
    const { container } = renderPage(<CompanyWebsitePage api={api} isSuperAdmin />);

    fireEvent.click(await screen.findByRole('button', { name: '开始员工同步' }));
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));
    await waitFor(() => expect(startEmployeeSync).toHaveBeenCalledTimes(1));

    const alert = await screen.findByRole('alert');
    const syncSection = screen.getByRole('heading', { name: '从企业微信同步员工' }).closest('section');
    expect(syncSection?.contains(alert)).toBe(true);
    expect(alert.textContent).toContain('同步请求格式不正确');
    expect(container.querySelector('.company-profile-page > .phase35-notice-error')).toBeNull();
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
