import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../../app/access-context';
import type { AccessContext } from '../../../app/access-loader';
import type { BusinessWorkbenchApi } from '../../business-workbench/business-workbench-page';
import { MaterialManagementPage } from './material-management-page';

const access: AccessContext = {
  session: { token: 'token', userId: '1', corpId: '7', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set(['/acquisition/material-management']),
  allowedActions: new Set(),
};

afterEach(() => { cleanup(); vi.restoreAllMocks(); });

function view(api: BusinessWorkbenchApi) {
  return render(
    <MemoryRouter>
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <DashboardAccessProvider value={access}><MaterialManagementPage api={api} /></DashboardAccessProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

function readProvider() {
  return vi.fn().mockImplementation((endpoint: string) => {
    if (endpoint === '/mediumGroup/index') return Promise.resolve([{ id: 0, name: '全部分组' }, { id: 9, name: '活动素材' }]);
    if (endpoint === '/workDepartment/pageIndex') return Promise.resolve({ list: [{ departmentId: 10, name: '销售部', children: [{ departmentId: 11, name: '华东销售组' }] }] });
    return Promise.resolve({ list: [{ id: 21, type: '文本', content: { title: '欢迎文案', content: '你好，欢迎咨询' }, mediumGroupId: 9, mediumGroupName: '活动素材', scopeType: 'public', status: 'available', sidebarVisible: true, userName: '管理员', createdAt: '2026-08-04 16:00' }], page: { total: 1 } });
  });
}

describe('Phase 3.4 material management', () => {
  it('queries real materials by scope, group, type and keyword', async () => {
    const read = readProvider();
    view({ read, write: vi.fn() });

    expect(await screen.findByText('欢迎文案')).toBeTruthy();
    expect(read).toHaveBeenCalledWith('/medium/index', expect.objectContaining({ scopeType: 'public', page: 1, perPage: 20 }));
    fireEvent.click(screen.getByRole('button', { name: '活动素材' }));
    fireEvent.change(screen.getByLabelText('素材类型'), { target: { value: '1' } });
    fireEvent.change(screen.getByLabelText('搜索素材'), { target: { value: '欢迎' } });
    fireEvent.keyDown(screen.getByLabelText('搜索素材'), { key: 'Enter' });
    await waitFor(() => expect(read).toHaveBeenLastCalledWith('/medium/index', expect.objectContaining({ mediumGroupId: 9, type: 1, searchStr: '欢迎' })));
  });

  it('creates a scoped text material from the drawer', async () => {
    const write = vi.fn().mockResolvedValue(undefined);
    view({ read: readProvider(), write });
    await screen.findByText('欢迎文案');

    fireEvent.click(screen.getByRole('button', { name: '添加素材' }));
    fireEvent.change(screen.getByLabelText('素材名称'), { target: { value: '新品介绍' } });
    fireEvent.change(screen.getByLabelText('素材正文'), { target: { value: '新品现已上线' } });
    fireEvent.click(screen.getByRole('button', { name: '保存素材' }));

    await waitFor(() => expect(write).toHaveBeenCalledWith('/medium/store', expect.objectContaining({ type: 1, scopeType: 'public', content: { title: '新品介绍', content: '新品现已上线' } }), 'POST'));
    expect(screen.queryByRole('dialog', { name: '添加素材' })).toBeNull();
  });

  it('requires and persists a real department when creating department material', async () => {
    const write = vi.fn().mockResolvedValue(undefined);
    view({ read: readProvider(), write });
    await screen.findByText('欢迎文案');

    fireEvent.click(screen.getByRole('tab', { name: /部门素材/ }));
    fireEvent.click(screen.getByRole('button', { name: '添加素材' }));
    fireEvent.change(screen.getByLabelText('素材名称'), { target: { value: '部门话术' } });
    fireEvent.change(screen.getByLabelText('素材正文'), { target: { value: '仅销售部可用' } });
    expect(screen.getByRole('button', { name: '保存素材' }).disabled).toBe(true);
    await screen.findByRole('option', { name: '华东销售组' });
    fireEvent.change(screen.getByLabelText('所属部门'), { target: { value: '11' } });
    await waitFor(() => expect(screen.getByRole('button', { name: '保存素材' }).disabled).toBe(false));
    fireEvent.click(screen.getByRole('button', { name: '保存素材' }));

    await waitFor(() => expect(write).toHaveBeenCalledWith('/medium/store', expect.objectContaining({ scopeType: 'department', scopeId: 11 }), 'POST'));
  });

  it('creates a real material group and refreshes the group provider', async () => {
    vi.spyOn(window, 'prompt').mockReturnValue('新品分组');
    const read = readProvider();
    const write = vi.fn().mockResolvedValue(undefined);
    view({ read, write });
    await screen.findByText('欢迎文案');

    fireEvent.click(screen.getByRole('button', { name: '创建分组' }));
    await waitFor(() => expect(write).toHaveBeenCalledWith('/mediumGroup/store', { name: '新品分组' }, 'POST'));
    expect(read.mock.calls.filter(([endpoint]) => endpoint === '/mediumGroup/index').length).toBeGreaterThan(1);
  });

  it('batch moves selected materials and surfaces reference conflicts', async () => {
    const write = vi.fn().mockResolvedValueOnce(undefined).mockRejectedValueOnce(new Error('素材正在被欢迎语引用'));
    view({ read: readProvider(), write });
    await screen.findByText('欢迎文案');

    fireEvent.click(screen.getByRole('checkbox', { name: '选择欢迎文案' }));
    fireEvent.change(screen.getByLabelText('移动到分组'), { target: { value: '9' } });
    fireEvent.click(screen.getByRole('button', { name: '批量移动' }));
    await waitFor(() => expect(write).toHaveBeenCalledWith('/medium/batchGroupUpdate', { ids: [21], mediumGroupId: 9 }, 'POST'));
    fireEvent.click(screen.getByRole('button', { name: '批量删除' }));
    expect((await screen.findByRole('alert')).textContent).toContain('素材正在被欢迎语引用');
  });
});
