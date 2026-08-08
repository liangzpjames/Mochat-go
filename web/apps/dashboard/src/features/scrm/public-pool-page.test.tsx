import { ApiError } from '@mochat/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { ScrmApi } from './scrm-api';
import { PublicPoolPage } from './public-pool-page';

vi.mock('../../app/access-context', () => ({
  useDashboardAccess: () => ({
    corp: { id: '7' },
    session: { userId: '42' },
    allowedActions: new Set(['/customer/contact@edit']),
  }),
}));
afterEach(cleanup);

const item = {
  id: 'a1', contactId: 'c1', contactName: 'Ada', ownerId: null, collaboratorIds: [], status: 'public_pool', version: 3,
  source: 'wecom', businessType: 'retail', tagNames: ['VIP'], region: 'Shanghai', recycleCount: 2,
  poolAction: 'return', poolReason: '长期未跟进', previousOwnerId: 18, lastFollowUpAt: '2026-07-31T08:00:00Z',
};

function api(overrides: Partial<ScrmApi> = {}): ScrmApi {
  return {
    listPublicPool: vi.fn().mockResolvedValue({ items: [item], nextCursor: '20' }),
    updateAssignment: vi.fn(),
    releaseToPublicPool: vi.fn().mockResolvedValue(item),
    claimFromPublicPool: vi.fn().mockResolvedValue({ ...item, ownerId: 42, status: 'owned', version: 4 }),
    batchClaimFromPublicPool: vi.fn().mockResolvedValue({ results: [] }),
    listOpportunities: vi.fn(), createOpportunity: vi.fn(), changeOpportunityStage: vi.fn(),
    listFollowUps: vi.fn(), appendFollowUp: vi.fn(), listTags: vi.fn(), createTag: vi.fn(), renameTag: vi.fn(),
    listContactOptions: vi.fn().mockResolvedValue([{ id: 'contact-owned', name: '归属客户', version: 8 }]),
    ...overrides,
  };
}

function LocationProbe() {
  const location = useLocation();
  return <output aria-label="current-url">{location.pathname}{location.search}</output>;
}

function view(value: ScrmApi, entry = '/customer/public-sea') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return render(<MemoryRouter initialEntries={[entry]}><QueryClientProvider client={client}><PublicPoolPage api={value} /><LocationProbe /></QueryClientProvider></MemoryRouter>);
}

describe('PublicPoolPage', () => {
  it('renders the unified SCRM header, overview and refresh toolbar', async () => {
    const value = api();
    view(value);
    await screen.findByText('Ada');
    expect(screen.getByText('SCRM · 客户流转')).toBeTruthy();
    expect(screen.getByRole('region', { name: '公海数据概览' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '刷新公海' }));
    await waitFor(() => expect(value.listPublicPool).toHaveBeenCalledTimes(2));
  });

  it('restores all filters from URL and renders auditable pool history', async () => {
    const value = api();
    view(value, '/customer/public-sea?keyword=Ada&source=wecom&businessType=retail&tagId=tag-1&region=Shanghai&reason=expired&previousOwnerId=18&cursor=20');

    await screen.findByText('Ada');
    expect(value.listPublicPool).toHaveBeenCalledWith({ corpId: 7, keyword: 'Ada', sources: ['wecom'], businessTypes: ['retail'], tagIds: ['tag-1'], regions: ['Shanghai'], reasons: ['expired'], previousOwnerIds: [18], cursor: '20', pageSize: 20 });
    expect(screen.getByText('VIP')).toBeTruthy();
    expect(screen.getByText('长期未跟进')).toBeTruthy();
    expect(screen.getByText('18')).toBeTruthy();
    expect(screen.getByText('2 次')).toBeTruthy();
  });

  it('syncs filters, reset and pagination through the URL', async () => {
    const value = api();
    view(value);
    await screen.findByText('Ada');
    fireEvent.change(screen.getByLabelText('公海关键词'), { target: { value: ' Grace ' } });
    fireEvent.change(screen.getByLabelText('来源筛选'), { target: { value: 'manual' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(screen.getByLabelText('current-url').textContent).toContain('?keyword=Grace&source=manual'));
    fireEvent.click(await screen.findByRole('button', { name: '下一页' }));
    await waitFor(() => expect(screen.getByLabelText('current-url').textContent).toContain('cursor=20'));
    fireEvent.click(screen.getByRole('button', { name: '重置' }));
    await waitFor(() => expect(screen.getByLabelText('current-url').textContent).toBe('/customer/public-sea'));
  });

  it('claims with the complete command and reports per-item batch partial failure', async () => {
    const claim = vi.fn().mockResolvedValue({ ...item, ownerId: 42 });
    const batch = vi.fn().mockResolvedValue({ results: [{ id: 'c1', status: 'succeeded', errorCode: '' }, { id: 'c2', status: 'failed', errorCode: 'CONFLICT' }] });
    const value = api({ listPublicPool: vi.fn().mockResolvedValue({ items: [item, { ...item, id: 'a2', contactId: 'c2', contactName: 'Grace', version: 5 }], nextCursor: '' }), claimFromPublicPool: claim, batchClaimFromPublicPool: batch });
    view(value);

    fireEvent.click(await screen.findByRole('button', { name: '领取 Ada' }));
    await waitFor(() => expect(claim).toHaveBeenCalledWith({ corpId: 7, contactId: 'c1', userId: 42, version: 3, idempotencyKey: 'claim-c1-3-42' }));

    fireEvent.click(screen.getByRole('checkbox', { name: '选择 Ada' }));
    fireEvent.click(screen.getByRole('checkbox', { name: '选择 Grace' }));
    fireEvent.click(screen.getByRole('button', { name: '批量领取' }));
    await waitFor(() => expect(batch).toHaveBeenCalledWith({ corpId: 7, userId: 42, targets: [{ contactId: 'c1', version: 3, idempotencyKey: 'batch-claim-c1-3-42' }, { contactId: 'c2', version: 5, idempotencyKey: 'batch-claim-c2-5-42' }] }));
    expect(await screen.findByText('批量领取完成：1 成功，1 失败（CONFLICT）。')).toBeTruthy();
  });

  it('retries a failed claim with the exact saved command', async () => {
    const claim = vi.fn()
      .mockRejectedValueOnce(new ApiError('server', 'temporary', { status: 503 }))
      .mockResolvedValueOnce({ ...item, ownerId: 42, status: 'owned', version: 4 });
    view(api({ claimFromPublicPool: claim }));

    fireEvent.click(await screen.findByRole('button', { name: /Ada/ }));
    const retry = await screen.findByRole('button', { name: '重试原操作' });
    fireEvent.click(retry);

    const command = { corpId: 7, contactId: 'c1', userId: 42, version: 3, idempotencyKey: 'claim-c1-3-42' };
    await waitFor(() => expect(claim).toHaveBeenCalledTimes(2));
    expect(claim).toHaveBeenNthCalledWith(1, command);
    expect(claim).toHaveBeenNthCalledWith(2, command);
  });

  it('retries failed batch and move mutations with their complete saved parameters', async () => {
    const batch = vi.fn()
      .mockRejectedValueOnce(new ApiError('server', 'temporary', { status: 503 }))
      .mockResolvedValueOnce({ results: [{ id: 'c1', status: 'succeeded', errorCode: '' }] });
    const batchView = view(api({ batchClaimFromPublicPool: batch }));
    fireEvent.click(await screen.findByRole('checkbox', { name: '选择 Ada' }));
    fireEvent.click(screen.getByRole('button', { name: '批量领取' }));
    fireEvent.click(await screen.findByRole('button', { name: '重试原操作' }));
    const batchCommand = { corpId: 7, userId: 42, targets: [{ contactId: 'c1', version: 3, idempotencyKey: 'batch-claim-c1-3-42' }] };
    await waitFor(() => expect(batch).toHaveBeenCalledTimes(2));
    expect(batch).toHaveBeenNthCalledWith(1, batchCommand);
    expect(batch).toHaveBeenNthCalledWith(2, batchCommand);
    batchView.unmount();

    const move = vi.fn()
      .mockRejectedValueOnce(new ApiError('server', 'temporary', { status: 503 }))
      .mockResolvedValueOnce(item);
    view(api({ releaseToPublicPool: move }));
    await screen.findByText('Ada');
    fireEvent.change(screen.getByRole('combobox', { name: '操作联系人' }), { target: { value: 'contact-owned' } });
    fireEvent.change(screen.getByLabelText('操作入海原因'), { target: { value: '超时未跟进' } });
    fireEvent.click(screen.getByRole('button', { name: '退回公海' }));
    fireEvent.click(await screen.findByRole('button', { name: '重试原操作' }));
    const moveCommand = { corpId: 7, contactId: 'contact-owned', version: 8, action: 'return', reason: '超时未跟进', idempotencyKey: 'pool-return-contact-owned-8-42' };
    await waitFor(() => expect(move).toHaveBeenCalledTimes(2));
    expect(move).toHaveBeenNthCalledWith(1, moveCommand);
    expect(move).toHaveBeenNthCalledWith(2, moveCommand);
  });

  it('supports entering, returning and reclaiming with a persisted reason', async () => {
    const release = vi.fn().mockResolvedValue(item);
    const value = api({ releaseToPublicPool: release });
    view(value);
    await screen.findByText('Ada');
    fireEvent.change(screen.getByRole('combobox', { name: '操作联系人' }), { target: { value: 'contact-owned' } });
    fireEvent.change(screen.getByLabelText('操作入海原因'), { target: { value: '超时未跟进' } });
    fireEvent.click(screen.getByRole('button', { name: '退回公海' }));
    await waitFor(() => expect(release).toHaveBeenCalledWith({ corpId: 7, contactId: 'contact-owned', version: 8, action: 'return', reason: '超时未跟进', idempotencyKey: 'pool-return-contact-owned-8-42' }));
    expect(screen.getByRole('button', { name: '进入公海' })).toBeTruthy();
    expect(screen.getByRole('button', { name: '管理员回收' })).toBeTruthy();
  });

  it('uses PageState for loading, empty, forbidden, retry and conflict', async () => {
    const retrying = api({ listPublicPool: vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce({ items: [item], nextCursor: '' }) });
    const first = view(retrying);
    await waitFor(() => expect(first.container.querySelector('.page-state-error')).not.toBeNull());
    fireEvent.click(first.container.querySelector('.page-state-retry')!);
    await screen.findByText('Ada');
    first.unmount();

    const empty = view(api({ listPublicPool: vi.fn().mockResolvedValue({ items: [], nextCursor: '' }) }));
    await waitFor(() => expect(empty.container.querySelector('.page-state-empty')).not.toBeNull());
    empty.unmount();

    const forbidden = view(api({ listPublicPool: vi.fn().mockRejectedValue(new ApiError('forbidden', '无权限', { status: 403 })) }));
    await waitFor(() => expect(forbidden.container.querySelector('.page-state-forbidden')).not.toBeNull());
    forbidden.unmount();

    const conflict = view(api({ claimFromPublicPool: vi.fn().mockRejectedValue(new ApiError('validation', 'conflict', { status: 409 })) }));
    fireEvent.click(await screen.findByRole('button', { name: '领取 Ada' }));
    await waitFor(() => expect(conflict.container.querySelector('.page-state-conflict')).not.toBeNull());
    const refresh = screen.getByRole('button', { name: '刷新数据' });
    fireEvent.click(refresh);
    await waitFor(() => expect(conflict.container.querySelector('.page-state-conflict')).toBeNull());
  });
});
