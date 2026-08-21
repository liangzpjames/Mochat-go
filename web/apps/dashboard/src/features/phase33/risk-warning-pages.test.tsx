import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';
import { RiskWarningPage, riskWarningConfigs } from './risk-warning-pages';

const riskPaths = [
  '/ai-insight/v2/risk',
  '/ai-insight/v2/timeout',
  '/ai-insight/v2/customer-loss',
  '/ai-insight/v2/message-intercept',
  '/ai-insight/v2/keyword-library',
  '/ai-insight/v2/silent-customer',
] as const;

const access: AccessContext = {
  session: { token: 'token', userId: '1', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set(riskPaths),
  allowedActions: new Set(),
};

afterEach(cleanup);

function view(path: (typeof riskPaths)[number], api: BusinessWorkbenchApi) {
  return render(
    <MemoryRouter>
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <DashboardAccessProvider value={access}>
          <RiskWarningPage api={api} config={riskWarningConfigs[path]} />
        </DashboardAccessProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

describe('RiskWarningPage', () => {
  it('defines six native risk routes with only customer loss connected to a real provider', () => {
    expect(Object.keys(riskWarningConfigs).sort()).toEqual([...riskPaths].sort());
    expect(riskWarningConfigs['/ai-insight/v2/customer-loss'].readEndpoint).toBe('/workContact/lossContact');
    for (const path of riskPaths.filter((path) => path !== '/ai-insight/v2/customer-loss' && path !== '/ai-insight/v2/timeout')) {
      expect(riskWarningConfigs[path].providerState).toBe('unavailable');
    }
  });

  it('loads customer loss records through the supported real endpoint and employee filter', async () => {
    const read = vi.fn().mockResolvedValue({ list: [{ id: 3, name: '李雷', employeeName: '张三', deletedAt: '2026-08-02 10:00:00' }] });
    const api: BusinessWorkbenchApi = {
      read,
      write: vi.fn(),
    };
    view('/ai-insight/v2/customer-loss', api);

    expect(await screen.findByText('李雷')).toBeTruthy();
    expect(read).toHaveBeenCalledWith('/workContact/lossContact', { page: 1, perPage: 20 });
    fireEvent.change(screen.getByLabelText('员工 ID'), { target: { value: '99' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(read).toHaveBeenLastCalledWith(
      '/workContact/lossContact',
      { employeeId: '99', page: 1, perPage: 20 },
    ));
  });

  it('renders query failure through the shared PageState and can retry the real provider', async () => {
    const read = vi.fn().mockRejectedValue(new Error('network down'));
    const api: BusinessWorkbenchApi = { read, write: vi.fn() };
    const result = view('/ai-insight/v2/customer-loss', api);

    await waitFor(() => expect(result.container.querySelector('.page-state-error')).not.toBeNull());
    fireEvent.click(screen.getByRole('button', { name: '重新加载' }));
    await waitFor(() => expect(read).toHaveBeenCalledTimes(2));
  });

  it.each(riskPaths.filter((path) => path !== '/ai-insight/v2/customer-loss' && path !== '/ai-insight/v2/timeout'))(
    'does not fabricate risk data when %s has no provider',
    (path) => {
      const read = vi.fn();
      const api: BusinessWorkbenchApi = { read, write: vi.fn() };
      view(path, api);

      expect(screen.getByRole('heading', { name: '暂无可用记录' })).toBeTruthy();
      expect(screen.getByRole('button', { name: '查询' }).hasAttribute('disabled')).toBe(true);
      expect(read).not.toHaveBeenCalled();
      expect(screen.queryByRole('button', { name: '详情' })).toBeNull();
    },
  );
});
