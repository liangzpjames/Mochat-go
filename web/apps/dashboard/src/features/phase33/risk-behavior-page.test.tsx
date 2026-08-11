import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';
import { RiskBehaviorPage } from './risk-behavior-page';

const authorizedAccess: AccessContext = {
  session: { token: 'token', userId: '1', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set(['/ai-insight/v2/risk']),
  allowedActions: new Set(),
};

const forbiddenAccess: AccessContext = {
  ...authorizedAccess,
  corp: { id: '7', name: '测试企业', authorized: false },
};

afterEach(cleanup);
beforeAll(() => { globalThis.ResizeObserver = class { observe() {} unobserve() {} disconnect() {} }; });

function view(access: AccessContext, api: BusinessWorkbenchApi) {
  return render(
    <MemoryRouter>
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <DashboardAccessProvider value={access}>
          <RiskBehaviorPage api={api} />
        </DashboardAccessProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

describe('RiskBehaviorPage', () => {
  it('loads risk records and renders rows', async () => {
    const read = vi.fn().mockResolvedValue({
      items: [{
        id: 1,
        name: '转账话术',
        behavior: '关键词命中',
        riskLevel: 'high',
        auditStatus: 'pending',
        occurredAt: '2026-08-01 10:00:00',
      }],
    });
    view(authorizedAccess, { read, write: vi.fn() });

    expect(await screen.findByText('转账话术')).toBeTruthy();
    expect(read).toHaveBeenCalledWith('/risk/records', { page: 1, perPage: 20 });
  });

  it('renders empty state when there are no records', async () => {
    const read = vi.fn().mockResolvedValue({ items: [] });
    view(authorizedAccess, { read, write: vi.fn() });

    await waitFor(() => expect(screen.getByRole('heading', { name: '暂无数据' })).toBeTruthy());
  });

  it('renders error state and retries', async () => {
    const read = vi.fn().mockRejectedValue(new Error('network down'));
    const api = { read, write: vi.fn() };
    const result = view(authorizedAccess, api);

    await waitFor(() => expect(result.container.querySelector('.page-state-error')).not.toBeNull());
    fireEvent.click(screen.getByRole('button', { name: '重新加载' }));
    await waitFor(() => expect(read).toHaveBeenCalledTimes(2));
  });

  it('does not call the API when the corp is not authorized (受限)', () => {
    const read = vi.fn();
    view(forbiddenAccess, { read, write: vi.fn() });

    expect(read).not.toHaveBeenCalled();
  });

  it('switches to rules tab and saves a new rule through the real contract', async () => {
    const read = vi.fn().mockResolvedValue({ items: [] });
    const write = vi.fn().mockResolvedValue({});
    view(authorizedAccess, { read, write });

    fireEvent.click(screen.getByRole('button', { name: '规则配置' }));
    await waitFor(() => expect(read).toHaveBeenLastCalledWith('/risk/rules', { page: 1, perPage: 20 }));

    fireEvent.click(screen.getByRole('button', { name: '新增规则' }));
    fireEvent.change(screen.getByLabelText('规则名称'), { target: { value: '转账拦截' } });
    fireEvent.change(screen.getByLabelText('匹配关键词'), { target: { value: '转账' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(write).toHaveBeenCalledWith(
      '/risk/rules',
      expect.objectContaining({ name: '转账拦截' }),
      'POST',
    ));
  });

  it('does not toggle or delete a named rule before confirmation', async () => {
    const read = vi.fn().mockResolvedValue({
      items: [{ id: 11, name: '转账拦截', status: 'enabled', subject: 'employee' }],
    });
    const write = vi.fn().mockResolvedValue({});
    view(authorizedAccess, { read, write });

    fireEvent.click(screen.getByRole('button', { name: '规则配置' }));
    const disable = await screen.findByRole('button', { name: '停用 转账拦截' });
    fireEvent.click(disable);
    expect(write).not.toHaveBeenCalledWith('/risk/rules/status', expect.anything(), 'PUT');
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));
    await waitFor(() => expect(write).toHaveBeenCalledTimes(1));
  });
});
