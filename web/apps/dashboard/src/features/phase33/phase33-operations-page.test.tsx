import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';
import { Phase33OperationsPage, phase33OperationConfigs } from './phase33-operations-page';

const access: AccessContext = {
  session: { token: 'token', userId: '1', corpId: '7', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set(['/chat/resign-staff', '/customer/inheritance']),
  allowedActions: new Set(),
};

afterEach(cleanup);

function view(path: keyof typeof phase33OperationConfigs, api: BusinessWorkbenchApi) {
  return render(
    <MemoryRouter>
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <DashboardAccessProvider value={access}>
          <Phase33OperationsPage api={api} config={phase33OperationConfigs[path]} />
        </DashboardAccessProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

describe('Phase33OperationsPage', () => {
  it('maps the four Task 1 routes to real providers or an explicit unavailable state', () => {
    expect(phase33OperationConfigs['/chat/resign-staff'].readEndpoint).toBe('/contactTransfer/info');
    expect(phase33OperationConfigs['/customer/inheritance'].readEndpoint).toBe('/contactTransfer/unassignedList');
    expect(phase33OperationConfigs['/chat/file-audio'].providerState).toBe('unavailable');
    expect(phase33OperationConfigs['/chat/refuse-archive'].providerState).toBe('unavailable');
  });

  it('loads inheritance records through the existing endpoint and applies a named filter', async () => {
    const api: BusinessWorkbenchApi = {
      read: vi.fn().mockResolvedValue({ list: [{ contactId: 'external-1', name: '李雷', employee: '张三' }], lastTime: '2026-08-02 09:00:00' }),
      write: vi.fn(),
    };
    view('/customer/inheritance', api);

    expect(await screen.findByText('李雷')).toBeTruthy();
    expect(api.read).toHaveBeenCalledWith('/contactTransfer/unassignedList', expect.objectContaining({ page: 1, perPage: 20 }));
    fireEvent.change(screen.getByLabelText('客户名称'), { target: { value: '李雷' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(api.read).toHaveBeenLastCalledWith(
      '/contactTransfer/unassignedList',
      expect.objectContaining({ contactName: '李雷', page: 1, perPage: 20 }),
    ));
  });

  it('renders the shared empty result state after a real provider returns no records', async () => {
    const api: BusinessWorkbenchApi = { read: vi.fn().mockResolvedValue({ list: [] }), write: vi.fn() };
    const result = view('/chat/resign-staff', api);

    await waitFor(() => expect(result.container.querySelector('.page-state-empty')).not.toBeNull());
  });

  it.each(['/chat/file-audio', '/chat/refuse-archive'] as const)(
    'does not invent provider data or downloads for %s',
    (path) => {
      const api: BusinessWorkbenchApi = { read: vi.fn(), write: vi.fn() };
      view(path, api);

      expect(screen.getByRole('heading', { name: '能力未接入' })).toBeTruthy();
      expect(screen.getByText('当前环境尚未接入可用的媒体或拒绝存档数据提供方。')).toBeTruthy();
      expect(api.read).not.toHaveBeenCalled();
      expect(screen.queryByRole('link', { name: /下载/ })).toBeNull();
    },
  );
});
