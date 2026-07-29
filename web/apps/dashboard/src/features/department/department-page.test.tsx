/* eslint-disable @typescript-eslint/unbound-method */
import { App } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import { DepartmentPage, type DepartmentPageApi } from './department-page';

const access: AccessContext = {
  session: { token: 'token', userId: '1', corpId: '7', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [], allowedRoutes: new Set(['/department/index']),
  allowedActions: new Set(['/department/index@search', '/department/index@sync', '/department/index@check']),
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

function api(): DepartmentPageApi {
  return {
    list: vi.fn(() => Promise.resolve({ list: [{
      departmentId: 10, departmentPath: '1', name: '总部', level: '一级部门',
      children: [{ departmentId: 11, departmentPath: '1-1', name: '销售部', level: '二级部门' }],
    }], page: { page: 1, perPage: 10, total: 1, totalPage: 1 } })),
    members: vi.fn(() => Promise.resolve({ list: [{
      employeeId: 21, employeeName: '张三', phone: '13800000000', roleName: '管理员',
    }], page: { page: 1, perPage: 10, total: 1, totalPage: 1 } })),
    conditions: vi.fn(() => Promise.resolve({ status: [], contactAuth: [], syncTime: '2026-07-28 10:00:00' })),
    sync: vi.fn(() => Promise.resolve()),
  };
}

function renderPage(client: DepartmentPageApi, allowedActions = access.allowedActions) {
  render(<QueryClientProvider client={new QueryClient({ defaultOptions: {
    queries: { retry: false }, mutations: { retry: false },
  } })}><App><DashboardAccessProvider value={{ ...access, allowedActions }}>
    <DepartmentPage api={client} />
  </DashboardAccessProvider></App></QueryClientProvider>);
}

describe('DepartmentPage', () => {
  it('loads the tree and sync time', async () => {
    const client = api(); renderPage(client);
    expect(await screen.findByText('总部')).not.toBeNull();
    expect(screen.getByText(/2026-07-28 10:00:00/)).not.toBeNull();
    expect(client.list).toHaveBeenCalledWith({ name: '', parentName: '', page: 1, perPage: 10 });
  });

  it('gates search, sync, and member actions', async () => {
    renderPage(api(), new Set()); await screen.findByText('总部');
    expect(screen.queryByRole('button', { name: '查询' })).toBeNull();
    expect(screen.queryByRole('button', { name: '同步企业微信通讯录' })).toBeNull();
    expect(screen.queryByRole('button', { name: '查看成员' })).toBeNull();
  });

  it('applies filters and opens the paged member dialog', async () => {
    const client = api(); renderPage(client); await screen.findByText('总部');
    fireEvent.change(screen.getByLabelText('组织名称'), { target: { value: '销售' } });
    fireEvent.click(screen.getByRole('button', { name: /查\s*询/ }));
    await waitFor(() => expect(client.list).toHaveBeenLastCalledWith({
      name: '销售', parentName: '', page: 1, perPage: 10,
    }));
    await screen.findByText('总部');
    fireEvent.click(screen.getAllByRole('button', { name: '查看成员' })[0]!);
    expect(await screen.findByText('13800000000')).not.toBeNull();
    expect(client.members).toHaveBeenCalledWith({ departmentId: 10, page: 1, perPage: 10 });
  });

  it('refreshes the tree and sync time after synchronization', async () => {
    const client = api(); renderPage(client); await screen.findByText('总部');
    fireEvent.click(screen.getByRole('button', { name: '同步企业微信通讯录' }));
    await waitFor(() => expect(client.list).toHaveBeenCalledTimes(2));
    expect(client.conditions).toHaveBeenCalledTimes(2);
  });
});
