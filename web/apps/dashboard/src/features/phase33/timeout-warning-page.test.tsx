import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';
import { TimeoutWarningPage } from './timeout-warning-page';

const access: AccessContext = { session: { token: 'token', userId: '1', expiresAt: null }, corp: { id: '7', name: '测试企业', authorized: true }, menu: [], allowedRoutes: new Set(['/ai-insight/v2/timeout']), allowedActions: new Set() };
afterEach(cleanup);
beforeAll(() => { globalThis.ResizeObserver = class { observe() {} unobserve() {} disconnect() {} }; });

function view(api: BusinessWorkbenchApi, initial = '/ai-insight/v2/timeout') {
  return render(<MemoryRouter initialEntries={[initial]}><QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><DashboardAccessProvider value={access}><TimeoutWarningPage api={api} /></DashboardAccessProvider></QueryClientProvider></MemoryRouter>);
}

describe('TimeoutWarningPage', () => {
  it('renders compact records workspace with fixed Chinese columns', async () => {
    const read = vi.fn().mockResolvedValue({ items: [{ id: 8, timeoutSeconds: 620, customerName: '李雷', employeeName: '张三', conversationType: 'single', riskLevel: 'high', ruleName: '十分钟未回复', auditStatus: 'pending', occurredAt: '2026-08-08T08:00:00Z', triggerMessage: '请问还在吗？' }], total: 1, page: 1, perPage: 20 });
    view({ read, write: vi.fn() });
    expect(screen.getByRole('button', { name: '超时记录' })).toBeTruthy();
    expect(screen.getByRole('button', { name: '规则配置' })).toBeTruthy();
    expect(screen.getByRole('button', { name: '高级设置' })).toBeTruthy();
    expect(screen.queryByText('AI 洞察 / 风险预警')).toBeNull();
    expect(await screen.findByText('十分钟未回复')).toBeTruthy();
    expect(screen.getByText('10 分 20 秒')).toBeTruthy();
    expect(screen.queryByText('aiSummary')).toBeNull();
  });

  it('keeps filters in URL and confirms batch audit before writing', async () => {
    const read = vi.fn().mockResolvedValue({ items: [{ id: 8, timeoutSeconds: 620, customerName: '李雷', employeeName: '张三', conversationType: 'single', riskLevel: 'high', ruleName: '十分钟未回复', auditStatus: 'pending', occurredAt: '2026-08-08T08:00:00Z', triggerMessage: '请问还在吗？' }], total: 1, page: 1, perPage: 20 });
    const write = vi.fn().mockResolvedValue({});
    view({ read, write });
    await screen.findByText('十分钟未回复');
    fireEvent.change(screen.getByLabelText('客户'), { target: { value: '李雷' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(read).toHaveBeenLastCalledWith('/timeout-warning/records', { customer: '李雷', page: 1, perPage: 20 }));
    await screen.findByText('请问还在吗？');
    fireEvent.click(screen.getByRole('checkbox', { name: '选择记录 8' }));
    fireEvent.click(screen.getByRole('button', { name: '确认超时' }));
    expect(write).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: /^确认$/ }));
    await waitFor(() => expect(write).toHaveBeenCalledWith('/timeout-warning/records/audit', { ids: [8], action: 'confirmed', remark: '' }, 'POST'));
  });

  it('opens rule editor and protects destructive rule actions', async () => {
    const read = vi.fn().mockResolvedValue({ items: [{ id: 8, name: '十分钟未回复', status: 'enabled', monitorTarget: 'all', conversationScopes: ['single'], strategies: [{ timeoutMinutes: 10, riskLevel: 'medium', notifyType: 'none' }], triggerCount: 2, createdAt: '2026-08-08' }] });
    const write = vi.fn().mockResolvedValue({});
    view({ read, write });
    fireEvent.click(screen.getByRole('button', { name: '规则配置' }));
    expect(await screen.findByText('十分钟未回复')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '停用' }));
    expect(write).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: /^确认$/ }));
    await waitFor(() => expect(write).toHaveBeenCalledWith('/timeout-warning/rules/status', { id: 8, status: 'disabled' }, 'PUT'));
  });
});
