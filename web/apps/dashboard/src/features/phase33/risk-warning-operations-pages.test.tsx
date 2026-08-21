import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type { MessageInterceptApi } from './message-intercept-api';
import type { SilentCustomerApi } from './silent-customer-api';
import { KeywordLibraryPage } from './keyword-library-page';
import { MessageInterceptPage } from './message-intercept-page';
import { SilentCustomerPage } from './silent-customer-page';

const access: AccessContext = { session: { token: 'token', userId: '1', expiresAt: null }, corp: { id: '7', name: '测试企业', authorized: true }, menu: [], allowedRoutes: new Set(), allowedActions: new Set() };
afterEach(cleanup);

function renderPage(node: ReactNode) {
  return render(<MemoryRouter><QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })}><DashboardAccessProvider value={access}>{node}</DashboardAccessProvider></QueryClientProvider></MemoryRouter>);
}

function messageApi(overrides: Partial<MessageInterceptApi> = {}): MessageInterceptApi {
  return { libraries: vi.fn().mockResolvedValue({ items: [{ id: 1, name: '违禁词库', description: '', matchMode: 'contains', status: 'enabled', draftVersion: 2, publishedVersion: 1, entryCount: 2, updatedAt: '2026-08-21' }], total: 1, page: 1, perPage: 20 }), entries: vi.fn().mockResolvedValue({ items: [{ id: 2, libraryId: 1, keyword: '报价', status: 'enabled', updatedAt: '2026-08-21' }], total: 1, page: 1, perPage: 20 }), rules: vi.fn().mockResolvedValue({ items: [{ id: 3, name: '自动拦截', libraryId: 1, libraryName: '违禁词库', libraryVersion: 1, conversationScopes: ['single'], decision: 'blocked', status: 'enabled', triggerCount: 2, updatedAt: '2026-08-21' }], total: 1, page: 1, perPage: 20 }), records: vi.fn().mockResolvedValue({ items: [{ id: 4, ruleId: 3, ruleName: '自动拦截', libraryId: 1, libraryName: '违禁词库', libraryVersion: 1, conversationType: 'customer', conversationId: 'c1', messageId: 'm1', senderId: 's1', senderName: '小王', messageContent: '报价不可外发', matchedKeywords: ['报价'], decision: 'blocked', explanation: '命中已发布词库', auditStatus: 'pending', occurredAt: '2026-08-21' }], total: 1, page: 1, perPage: 20 }), write: vi.fn().mockResolvedValue({}), ...overrides };
}

function silentApi(overrides: Partial<SilentCustomerApi> = {}): SilentCustomerApi {
  return { records: vi.fn().mockResolvedValue({ items: [{ id: 5, ruleId: 1, ruleName: '30天未互动', customerId: 'c1', customerName: '客户A', employeeId: 8, employeeName: '小王', lastInteractionAt: '2026-07-01', silentDays: 51, status: 'pending', assignedEmployeeId: 0, assignedEmployeeName: '', followUpNote: '', updatedAt: '2026-08-21' }], total: 1, page: 1, perPage: 20 }), rules: vi.fn().mockResolvedValue({ items: [{ id: 1, name: '30天未互动', silentDays: 30, status: 'enabled', triggerCount: 1, updatedAt: '2026-08-21' }], total: 1, page: 1, perPage: 20 }), staffOptions: vi.fn().mockResolvedValue([{ id: 8, name: '小王', departments: ['销售部'] }]), write: vi.fn().mockResolvedValue({}), ...overrides };
}

describe('risk warning operation pages', () => {
  it('uses a compact library workspace and typed entry labels', async () => {
    const api = messageApi();
    const { container } = renderPage(<KeywordLibraryPage api={api} />);
    expect(await screen.findByText('违禁词库')).toBeTruthy();
    expect(container.querySelector('.sensitive-word-config-workspace')).not.toBeNull();
    expect(screen.queryByText('AI 洞察')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: /违禁词库/ }));
    expect(await screen.findByText('报价')).toBeTruthy();
    expect(api.libraries).toHaveBeenCalledWith(expect.objectContaining({ page: 1, perPage: 20 }));
  });

  it('keeps intercept filters explicit and opens a human-readable detail drawer', async () => {
    const api = messageApi();
    renderPage(<MessageInterceptPage api={api} />);
    expect(await screen.findByText('报价不可外发')).toBeTruthy();
    expect(api.records).toHaveBeenCalledWith(expect.objectContaining({ page: 1, perPage: 20 }));
    fireEvent.change(screen.getByRole('textbox', { name: '内容或关键词' }), { target: { value: '合同' } });
    const calls = (api.records as ReturnType<typeof vi.fn>).mock.calls.length;
    expect(calls).toBe(1);
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(api.records).toHaveBeenLastCalledWith(expect.objectContaining({ keyword: '合同', page: 1 })));
    fireEvent.click(await screen.findByText('报价不可外发'));
    expect(await screen.findByRole('dialog', { name: '消息拦截详情' })).toBeTruthy();
    expect(screen.queryByText('conversationId')).toBeNull();
  });

  it('uses a staff selector for silent-customer actions instead of an ID input', async () => {
    const api = silentApi();
    renderPage(<SilentCustomerPage api={api} />);
    expect(await screen.findByText('客户A')).toBeTruthy();
    fireEvent.click(screen.getByRole('checkbox', { name: '选择记录 5' }));
    expect(await screen.findByRole('combobox', { name: '分派员工' })).toBeTruthy();
    expect(screen.queryByLabelText('分派员工 ID')).toBeNull();
    expect(api.staffOptions).toHaveBeenCalled();
  });
});
