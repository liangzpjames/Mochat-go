import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router';
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

function LocationDisplay() {
  const location = useLocation();
  return <output aria-hidden="true" data-testid="location">{location.pathname}</output>;
}

function view(
  path: keyof typeof phase33OperationConfigs,
  api: BusinessWorkbenchApi,
  allowedActions = access.allowedActions,
) {
  return render(
    <MemoryRouter>
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <DashboardAccessProvider value={{ ...access, allowedActions }}>
          <Phase33OperationsPage api={api} config={phase33OperationConfigs[path]} />
          <LocationDisplay />
        </DashboardAccessProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

describe('Phase33OperationsPage', () => {
  it('maps the Task 1 routes to real providers or an explicit unavailable state', () => {
    expect(phase33OperationConfigs['/chat/resign-staff'].readEndpoint).toBe('/contactTransfer/info');
    expect(phase33OperationConfigs['/customer/inheritance'].readEndpoint).toBe('/contactTransfer/unassignedList');
    expect(phase33OperationConfigs['/chat/refuse-archive'].providerState).toBe('unavailable');
  });

  it('loads inheritance records through the existing endpoint and applies a named filter', async () => {
    const read = vi.fn().mockResolvedValue({ list: [{ contactId: 'external-1', name: '李雷', employee: '张三' }], lastTime: '2026-08-02 09:00:00' });
    const api: BusinessWorkbenchApi = {
      read,
      write: vi.fn(),
    };
    view('/customer/inheritance', api);

    expect(await screen.findByText('李雷')).toBeTruthy();
    expect(read).toHaveBeenCalledWith('/contactTransfer/unassignedList', expect.objectContaining({ page: 1, perPage: 20 }));
    fireEvent.change(screen.getByLabelText('客户名称'), { target: { value: '李雷' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(read).toHaveBeenLastCalledWith(
      '/contactTransfer/unassignedList',
      expect.objectContaining({ contactName: '李雷', page: 1, perPage: 20 }),
    ));
  });

  it('renders the loading state while a connected provider request is pending', () => {
    const api: BusinessWorkbenchApi = { read: vi.fn(() => new Promise(() => undefined)), write: vi.fn() };
    view('/chat/resign-staff', api);

    expect(screen.getByRole('status').getAttribute('aria-busy')).toBe('true');
  });

  it('retries a provider request after an API error', async () => {
    const read = vi.fn()
      .mockRejectedValueOnce(new Error('network unavailable'))
      .mockResolvedValueOnce({ list: [{ contactId: 'external-1', name: '李雷' }] });
    const api: BusinessWorkbenchApi = { read, write: vi.fn() };
    view('/customer/inheritance', api);

    await screen.findByRole('heading', { name: '加载失败' });
    fireEvent.click(screen.getByRole('button', { name: '重新加载' }));

    expect(await screen.findByText('李雷')).toBeTruthy();
    expect(read).toHaveBeenCalledTimes(2);
  });

  it('does not offer an API retry when refresh is not granted in strict mode', async () => {
    const api: BusinessWorkbenchApi = { read: vi.fn().mockRejectedValue(new Error('network unavailable')), write: vi.fn() };
    view('/customer/inheritance', api, new Set(['/customer/inheritance@search']));

    await screen.findByRole('heading', { name: '加载失败' });
    expect(screen.queryByRole('button', { name: '重新加载' })).toBeNull();
  });

  it('refreshes the connected provider records on demand', async () => {
    const read = vi.fn().mockResolvedValue({ list: [{ contactId: 'external-1', name: '李雷' }] });
    const api: BusinessWorkbenchApi = { read, write: vi.fn() };
    view('/customer/inheritance', api);

    await screen.findByText('李雷');
    fireEvent.click(screen.getByRole('button', { name: '刷新' }));

    await waitFor(() => expect(read).toHaveBeenCalledTimes(2));
  });

  it('resets the applied name filter and closes the selected detail', async () => {
    const read = vi.fn().mockResolvedValue({ list: [{ contactId: 'external-1', name: '李雷', employee: '张三' }] });
    const api: BusinessWorkbenchApi = { read, write: vi.fn() };
    view('/customer/inheritance', api);

    await screen.findByText('李雷');
    fireEvent.change(screen.getByLabelText('客户名称'), { target: { value: '李雷' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(read).toHaveBeenCalledTimes(2));
    fireEvent.click(await screen.findByRole('button', { name: '详情' }));
    expect(screen.getByLabelText('记录详情')).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: '重置' }));

    expect(screen.getByLabelText('客户名称').getAttribute('value')).toBe('');
    expect(screen.queryByLabelText('记录详情')).toBeNull();
  });

  it('opens record details and navigates to the configured handoff operation', async () => {
    const api: BusinessWorkbenchApi = {
      read: vi.fn().mockResolvedValue({ list: [{ contactId: 'external-1', name: '李雷', employee: '张三' }] }),
      write: vi.fn(),
    };
    view('/customer/inheritance', api);

    await screen.findByText('李雷');
    fireEvent.click(screen.getByRole('button', { name: '详情' }));
    expect(screen.getByLabelText('记录详情').textContent).toContain('张三');
    fireEvent.click(screen.getByRole('button', { name: '进入交接操作' }));

    expect(screen.getByTestId('location').textContent).toBe('/contactTransfer/resignIndex');
  });

  it('treats an empty action set as compatibility mode but hides every ungranted action in strict mode', async () => {
    const api: BusinessWorkbenchApi = {
      read: vi.fn().mockResolvedValue({ list: [{ contactId: 'external-1', name: '李雷' }] }),
      write: vi.fn(),
    };
    view('/customer/inheritance', api, new Set(['/customer/inheritance@refresh']));

    await screen.findByText('李雷');
    expect(screen.getByRole('button', { name: '刷新' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: '查询' })).toBeNull();
    expect(screen.queryByRole('button', { name: '重置' })).toBeNull();
    expect(screen.queryByRole('button', { name: '详情' })).toBeNull();
    expect(screen.queryByRole('button', { name: '进入交接操作' })).toBeNull();
  });

  it('renders the shared empty result state after a real provider returns no records', async () => {
    const api: BusinessWorkbenchApi = { read: vi.fn().mockResolvedValue({ list: [] }), write: vi.fn() };
    const { container } = view('/chat/resign-staff', api);

    await waitFor(() => expect(container.querySelector('.page-state-empty')).not.toBeNull());
  });

  it.each(['/chat/refuse-archive'] as const)(
    'does not invent provider data or downloads for %s',
    (path) => {
      const read = vi.fn();
      const api: BusinessWorkbenchApi = { read, write: vi.fn() };
      view(path, api);

      expect(screen.getByRole('heading', { name: '能力未接入' })).toBeTruthy();
      expect(screen.getByText('当前环境尚未接入可用的媒体或拒绝存档数据提供方。')).toBeTruthy();
      expect(read).not.toHaveBeenCalled();
      expect(screen.queryByRole('link', { name: /下载/ })).toBeNull();
    },
  );
});
