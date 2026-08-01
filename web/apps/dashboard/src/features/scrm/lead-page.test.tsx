import { ApiError } from '@mochat/api-client';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { LeadApi } from './lead-api';
import { LeadPage } from './lead-page';

vi.mock('../../app/access-context', () => ({ useDashboardAccess: () => ({ corp: { id: '7' }, allowedActions: new Set(['/customer/clue/default@add', '/customer/clue/default@assign', '/customer/clue/default@edit']) }) }));
afterEach(cleanup);

const lead = { id: 'lead-0', businessKey: 'wx:existing', name: '已有线索', phone: '13800000000', source: 'wecom' as const, status: 'new' as const, ownerId: null, convertedContactId: '', discardReason: '', version: 1 };
function api(overrides: Partial<LeadApi> = {}): LeadApi { return { list: vi.fn().mockResolvedValue({ items: [lead], nextCursor: '' }), create: vi.fn().mockResolvedValue(lead), findDuplicates: vi.fn().mockResolvedValue({ items: [] }), assign: vi.fn().mockResolvedValue({ results: [] }), transition: vi.fn().mockResolvedValue({ ...lead, status: 'qualified', version: 2 }), ...overrides }; }
function view(value: LeadApi) { return render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })}><LeadPage api={value} /></QueryClientProvider>); }

describe('LeadPage', () => {
  it('checks duplicates and creates a corp-scoped lead', async () => {
    const value = api(); view(value);
    expect(await screen.findByRole('heading', { name: '线索池' })).toBeTruthy();
    fireEvent.change(screen.getByRole('textbox', { name: '客户名称' }), { target: { value: '客户甲' } });
    fireEvent.change(screen.getByRole('textbox', { name: '联系电话' }), { target: { value: '13900000000' } });
    fireEvent.change(screen.getByRole('textbox', { name: '业务标识' }), { target: { value: 'wx:customer-1' } });
    fireEvent.click(screen.getByRole('button', { name: '新增线索' }));
    await waitFor(() => expect(value.findDuplicates).toHaveBeenCalledWith({ corpId: 7, businessKey: 'wx:customer-1', phone: '13900000000' }));
    await waitFor(() => expect(value.create).toHaveBeenCalledWith({ corpId: 7, businessKey: 'wx:customer-1', name: '客户甲', phone: '13900000000', source: 'wecom' }));
  });

  it('submits combined filters and reset restores defaults', async () => {
    const value = api(); view(value); await screen.findByText('已有线索');
    fireEvent.change(screen.getByRole('textbox', { name: '搜索线索' }), { target: { value: 'Ada' } });
    fireEvent.change(screen.getByRole('combobox', { name: '线索状态' }), { target: { value: 'qualified' } });
    fireEvent.change(screen.getByRole('combobox', { name: '线索来源' }), { target: { value: 'manual' } });
    fireEvent.change(screen.getByRole('spinbutton', { name: '负责人筛选' }), { target: { value: '12' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(value.list).toHaveBeenLastCalledWith(expect.objectContaining({ corpId: 7, keyword: 'Ada', statuses: ['qualified'], sources: ['manual'], ownerIds: [12] })));
    fireEvent.click(screen.getByRole('button', { name: '重置' }));
    await waitFor(() => expect(value.list).toHaveBeenLastCalledWith({ corpId: 7, pageSize: 20 }));
  });

  it('reports per-target partial failures for batch assignment', async () => {
    const value = api({ assign: vi.fn().mockResolvedValue({ results: [{ id: 'lead-0', status: 'failed', errorCode: 'CONFLICT' }] }) }); view(value); await screen.findByText('已有线索');
    fireEvent.click(screen.getByRole('checkbox', { name: '选择已有线索' }));
    fireEvent.change(screen.getByRole('spinbutton', { name: '分配负责人' }), { target: { value: '12' } });
    fireEvent.click(screen.getByRole('button', { name: '批量分配' }));
    await waitFor(() => expect(value.assign).toHaveBeenCalledWith({ corpId: 7, ownerId: 12, targets: [{ id: 'lead-0', version: 1 }] }));
    expect((await screen.findByRole('alert')).textContent).toContain('CONFLICT');
  });

  it('maps a stale transition to a refreshable conflict message', async () => {
    const value = api({ transition: vi.fn().mockRejectedValue(new ApiError('validation', '冲突', { status: 409 })) }); view(value); await screen.findByText('已有线索');
    fireEvent.click(screen.getByRole('button', { name: '标记为已确认' }));
    expect((await screen.findByRole('alert')).textContent).toContain('数据已更新');
  });

  it('renders empty and 403 query responses through PageState inside the unified shell', async () => {
    const emptyView = view(api({ list: vi.fn().mockResolvedValue({ items: [], nextCursor: '' }) })); await waitFor(() => expect(emptyView.container.querySelector('.page-state-empty')).not.toBeNull()); expect(emptyView.container.querySelector('.dashboard-page-header')).not.toBeNull(); emptyView.unmount();
    const forbiddenView = view(api({ list: vi.fn().mockRejectedValue(new ApiError('forbidden', '无权限', { status: 403 })) })); await waitFor(() => expect(forbiddenView.container.querySelector('.page-state-forbidden')).not.toBeNull());
  });
});
