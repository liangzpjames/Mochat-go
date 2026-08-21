import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';
import { CustomerLossPage } from './customer-loss-page';

const access: AccessContext = { session: { token: 'token', userId: '1', expiresAt: null }, corp: { id: '7', name: '测试企业', authorized: true }, menu: [], allowedRoutes: new Set(['/ai-insight/v2/customer-loss']), allowedActions: new Set() };
afterEach(cleanup);

function view(accessValue: AccessContext, api: BusinessWorkbenchApi, initial = '/ai-insight/v2/customer-loss') {
  return render(<MemoryRouter initialEntries={[initial]}><QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><DashboardAccessProvider value={accessValue}><CustomerLossPage api={api} /></DashboardAccessProvider></QueryClientProvider></MemoryRouter>);
}

describe('CustomerLossPage', () => {
  it('uses fixed Chinese columns and a compact tabbed workspace', async () => {
    const read = vi.fn().mockResolvedValue({ list: [{ id: 3, contactId: 9, name: '李雷', employeeName: '张三', deletedAt: '2026-08-02T10:00:00Z' }], page: { total: 1, page: 1, perPage: 20 } });
    view(access, { read, write: vi.fn() });
    expect(screen.getByRole('button', { name: '客户流失' })).toBeTruthy();
    expect(screen.getByRole('button', { name: '流失规则' })).toBeTruthy();
    expect(await screen.findByText('李雷')).toBeTruthy();
    expect(screen.getAllByText('员工删除客户').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByRole('columnheader', { name: '客户' })).toBeTruthy();
    expect(screen.queryByText('contactId')).toBeNull();
  });

  it('applies employee filter only after query and opens a detail drawer', async () => {
    const read = vi.fn().mockResolvedValue({ list: [{ id: 3, contactId: 9, name: '李雷', employeeName: '张三', deletedAt: '2026-08-02T10:00:00Z' }], page: { total: 1, page: 1, perPage: 20 } });
    view(access, { read, write: vi.fn() });
    await screen.findByText('李雷');
    expect(read).toHaveBeenCalledWith('/workContact/lossContact', { page: 1, perPage: 20 });
    fireEvent.change(screen.getByLabelText('关联员工'), { target: { value: '99' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(read).toHaveBeenLastCalledWith('/workContact/lossContact', { employeeId: '99', page: 1, perPage: 20 }));
    await screen.findByText('李雷');
    fireEvent.click(screen.getByRole('button', { name: '查看详情' }));
    expect(screen.getByRole('dialog', { name: '客户流失详情' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '关闭' }));
    expect(screen.queryByRole('dialog', { name: '客户流失详情' })).toBeNull();
  });

  it('renders forbidden, empty and retry states', async () => {
    const read = vi.fn().mockResolvedValue({ list: [] });
    view({ ...access, corp: { ...access.corp, authorized: false } }, { read, write: vi.fn() });
    expect(screen.getByRole('heading', { name: '无权访问当前企业数据' })).toBeTruthy();
    cleanup();
    const failing = vi.fn().mockRejectedValue(new Error('network down'));
    const result = view(access, { read: failing, write: vi.fn() });
    await waitFor(() => expect(result.container.querySelector('.page-state-error')).not.toBeNull());
    fireEvent.click(screen.getByRole('button', { name: '重新加载' }));
    await waitFor(() => expect(failing).toHaveBeenCalledTimes(2));
  });
});
