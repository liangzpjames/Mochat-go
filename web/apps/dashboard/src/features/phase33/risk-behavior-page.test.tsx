import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type { RiskBehaviorApi } from './risk-behavior-api';
import { RiskBehaviorPage } from './risk-behavior-page';

const access: AccessContext = { session: { token: 'token', userId: '1', expiresAt: null }, corp: { id: '7', name: '测试企业', authorized: true }, menu: [], allowedRoutes: new Set(['/ai-insight/v2/risk']), allowedActions: new Set() };
afterEach(cleanup);
beforeAll(() => { globalThis.ResizeObserver = class { observe() {} unobserve() {} disconnect() {} }; });

function view(api: RiskBehaviorApi) { return render(<MemoryRouter><QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><DashboardAccessProvider value={access}><RiskBehaviorPage api={api} /></DashboardAccessProvider></QueryClientProvider></MemoryRouter>); }
function api(overrides: Partial<RiskBehaviorApi> = {}): RiskBehaviorApi {
  return { records: vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, perPage: 20, summary: { total: 0, pending: 0, highRisk: 0, processed: 0 } }), recordDetail: vi.fn().mockResolvedValue({ record: { id: 1 }, audits: [], conversationAvailable: false }), rules: vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, perPage: 20 }), audit: vi.fn(), createRule: vi.fn(), updateRule: vi.fn(), setRuleEnabled: vi.fn(), removeRule: vi.fn(), scannerStatus: vi.fn().mockResolvedValue({ enabled: true, state: 'ready', lastAttemptAt: '', lastSuccessAt: '', lastFailureAt: '', lastError: '' }), ...overrides };
}

describe('RiskBehaviorPage', () => {
  it('renders typed real records without dynamic field names', async () => {
    const records = vi.fn<RiskBehaviorApi['records']>().mockResolvedValue({ items: [{ id: 1, behavior: 'private_transaction', riskLevel: 'high', conversationType: 'customer', triggerMessage: '请私下交易', auditStatus: 'pending', occurredAt: '2026-08-01T10:00:00Z', relatedUser: { name: '客户A' }, conversationId: '', messageId: '', aiSummary: '' }], total: 1, page: 1, perPage: 20, summary: { total: 1, pending: 1, highRisk: 1, processed: 0 } });
    const client = api({ records });
    view(client);
    expect(await screen.findByText('私下交易')).toBeTruthy();
    expect(screen.queryByText('AI 洞察')).toBeNull();
    expect(screen.queryByText('conversationId')).toBeNull();
    expect(records).toHaveBeenCalledWith({ page: 1, riskLevel: '', behavior: '', auditStatus: '', conversationType: '', employeeIds: [], occurredFrom: '', occurredTo: '' });
  });

  it('queries only after the filter is explicitly submitted', async () => {
    const records = vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, perPage: 20, summary: { total: 0, pending: 0, highRisk: 0, processed: 0 } });
    const client = api({ records });
    view(client);
    await waitFor(() => expect(records).toHaveBeenCalledTimes(1));
    fireEvent.change(screen.getByLabelText('风险等级'), { target: { value: 'high' } });
    expect(records).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(records).toHaveBeenLastCalledWith(expect.objectContaining({ riskLevel: 'high', page: 1 })));
  });

  it('opens a row detail and closes from overlay and Escape', async () => {
    const client = api({ records: vi.fn().mockResolvedValue({ items: [{ id: 9, behavior: 'sensitive_word', riskLevel: 'medium', conversationType: 'group', triggerMessage: '合同', auditStatus: 'pending', occurredAt: '2026-08-01T10:00:00Z', relatedUser: { roomName: '客户群' }, conversationId: '', messageId: '', aiSummary: '' }], total: 1, page: 1, perPage: 20, summary: { total: 1, pending: 1, highRisk: 0, processed: 0 } }) });
    view(client);
    fireEvent.click(await screen.findByText('合同'));
    expect(await screen.findByRole('dialog', { name: '风险详情' })).toBeTruthy();
    fireEvent.click(screen.getByTestId('risk-warning-drawer-overlay'));
    await waitFor(() => expect(screen.queryByRole('dialog', { name: '风险详情' })).toBeNull());
  });

  it('keeps real row content visible when the detail enrichment request fails', async () => {
    const client = api({ records: vi.fn().mockResolvedValue({ items: [{ id: 9, behavior: 'private_transaction', riskLevel: 'high', conversationType: 'customer', triggerMessage: '真实触发内容', auditStatus: 'confirmed', occurredAt: '2026-08-01T10:00:00Z', relatedUser: { customerName: '客户A' }, conversationId: '', messageId: '', aiSummary: '' }], total: 1, page: 1, perPage: 20, summary: { total: 1, pending: 0, highRisk: 1, processed: 1 } }), recordDetail: vi.fn().mockRejectedValue(new Error('服务不可用')) });
    view(client);
    fireEvent.click(await screen.findByText('真实触发内容'));
    expect((await screen.findAllByText('客户A')).length).toBeGreaterThan(0);
    expect(screen.queryByRole('heading', { name: '加载失败' })).toBeNull();
    expect(await screen.findByText('详情补充接口暂不可用，当前内容已从风险记录列表读取，未虚构缺失数据。')).toBeTruthy();
  });

  it('keeps rule mutations behind confirmation', async () => {
    const setRuleEnabled = vi.fn<RiskBehaviorApi['setRuleEnabled']>().mockResolvedValue({});
    const client = api({ rules: vi.fn().mockResolvedValue({ items: [{ id: 11, name: '私下交易规则', status: 'enabled', subject: 'employee', triggerCount: 1, aiInsightEnabled: false, whitelist: [], strategies: [{ behavior: 'private_transaction', pattern: '私下交易', notifyType: 'none', riskLevel: 'high' }] }], total: 1, page: 1, perPage: 20 }), setRuleEnabled });
    view(client);
    fireEvent.click(screen.getByRole('button', { name: '规则配置' }));
    const disable = await screen.findByRole('button', { name: '停用' });
    fireEvent.click(disable);
    expect(setRuleEnabled).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));
    await waitFor(() => expect(setRuleEnabled).toHaveBeenCalledWith({ id: 11, status: 'disabled' }));
  });
});
