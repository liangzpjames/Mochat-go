import { ApiError } from '@mochat/api-client';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useLocation, MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Opportunity, ScrmApi } from './scrm-api';
import { OpportunityPage } from './opportunity-page';

vi.mock('../../app/access-context', () => ({ useDashboardAccess: () => ({ corp: { id: '7' }, allowedActions: new Set(['/customer/opportunity@edit']) }) }));
afterEach(cleanup);

const openOpportunity: Opportunity = { id: 'o1', contactId: 'c1', stage: 'proposal', amount: 100, startDate: '2026-08-01', endDate: '2026-08-31', ownerId: 9, status: 'open', lostReason: '', version: 2 };

function LocationProbe() {
  const location = useLocation();
  return <output aria-label="current-search">{location.search}</output>;
}

function renderPage(api: ScrmApi, initialEntry = '/customer/opportunity') {
  return render(<MemoryRouter initialEntries={[initialEntry]}><QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })}><OpportunityPage api={api} /><LocationProbe /></QueryClientProvider></MemoryRouter>);
}

function apiWith(overrides: Partial<ScrmApi> = {}): ScrmApi {
  return {
    listOpportunities: vi.fn().mockResolvedValue({ items: [openOpportunity], nextCursor: '' }),
    createOpportunity: vi.fn().mockResolvedValue(openOpportunity),
    changeOpportunityStage: vi.fn().mockResolvedValue({ ...openOpportunity, status: 'won', stage: 'won', version: 3 }),
    appendFollowUp: vi.fn().mockResolvedValue({ id: 'f1', contactId: 'c1', content: '已发送方案', createdAt: '2026-08-01T00:00:00Z', createdBy: 3 }),
    listFollowUps: vi.fn().mockResolvedValue({ items: [], nextCursor: '' }),
    listPublicPool: vi.fn(), updateAssignment: vi.fn(), releaseToPublicPool: vi.fn(), claimFromPublicPool: vi.fn(), batchClaimFromPublicPool: vi.fn(),
    listTags: vi.fn(), createTag: vi.fn(), renameTag: vi.fn(),
    ...overrides,
  };
}

describe('OpportunityPage', () => {
  it('renders the unified SCRM header, overview and refresh toolbar', async () => {
    const value = apiWith();
    renderPage(value);
    await screen.findByText('c1');
    expect(screen.getByText('SCRM · 销售协同')).toBeTruthy();
    expect(screen.getByRole('region', { name: '商机数据概览' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '刷新商机' }));
    await waitFor(() => expect(value.listOpportunities).toHaveBeenCalledTimes(2));
  });

  it('restores combined filters and cursor from URL, then clears cursor on a new search', async () => {
    const api = apiWith();
    renderPage(api, '/customer/opportunity?stage=proposal&status=open&ownerId=9&cursor=o9');
    await screen.findByText('c1');
    expect(api.listOpportunities).toHaveBeenCalledWith({ corpId: 7, stage: 'proposal', status: 'open', ownerId: 9, cursor: 'o9', pageSize: 20 });

    fireEvent.change(screen.getByLabelText('商机阶段'), { target: { value: 'won' } });
    fireEvent.change(screen.getByLabelText('商机状态'), { target: { value: '' } });
    fireEvent.change(screen.getByLabelText('负责人筛选'), { target: { value: '' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(screen.getByLabelText('current-search').textContent).toBe('?stage=won'));
    await waitFor(() => expect(api.listOpportunities).toHaveBeenLastCalledWith({ corpId: 7, stage: 'won', pageSize: 20 }));
  });

  it('creates an opportunity with complete validated fields and refreshes the list', async () => {
    const createOpportunity = vi.fn().mockResolvedValue(openOpportunity);
    const api = apiWith({ createOpportunity });
    renderPage(api);
    await screen.findByText('c1');
    fireEvent.change(screen.getByLabelText('联系人 ID'), { target: { value: 'c2' } });
    fireEvent.change(screen.getByLabelText('商机金额'), { target: { value: '1280.50' } });
    fireEvent.change(screen.getByLabelText('开始日期'), { target: { value: '2026-08-02' } });
    fireEvent.change(screen.getByLabelText('结束日期'), { target: { value: '2026-08-20' } });
    fireEvent.change(screen.getByLabelText('商机负责人'), { target: { value: '12' } });
    fireEvent.click(screen.getByRole('button', { name: '创建商机' }));
    await waitFor(() => expect(createOpportunity).toHaveBeenCalledWith(expect.objectContaining({ corpId: 7, contactId: 'c2', stage: 'proposal', amount: 1280.5, startDate: '2026-08-02', endDate: '2026-08-20', ownerId: 12, idempotencyKey: expect.stringMatching(/^opportunity-create-/) })));
    expect(await screen.findByText('商机创建成功。')).toBeTruthy();
  });

  it('wins, loses with a reason, appends follow-up, and hides transitions for terminal items', async () => {
    const changeOpportunityStage = vi.fn().mockResolvedValue({ ...openOpportunity, version: 3 });
    const appendFollowUp = vi.fn().mockResolvedValue({ id: 'f1', contactId: 'c1', content: '已发送方案', createdAt: '2026-08-01T00:00:00Z', createdBy: 3 });
    const api = apiWith({ changeOpportunityStage, appendFollowUp });
    renderPage(api);
    await screen.findByText('c1');
    fireEvent.click(screen.getByRole('button', { name: '赢单 o1' }));
    await waitFor(() => expect(changeOpportunityStage).toHaveBeenCalledWith({ corpId: 7, opportunityId: 'o1', stageId: 'won', lostReason: '', version: 2, idempotencyKey: 'opportunity-stage-o1-won-2' }));

    fireEvent.change(screen.getByLabelText('输单原因 o1'), { target: { value: '客户预算取消' } });
    fireEvent.click(screen.getByRole('button', { name: '输单 o1' }));
    await waitFor(() => expect(changeOpportunityStage).toHaveBeenCalledWith({ corpId: 7, opportunityId: 'o1', stageId: 'lost', lostReason: '客户预算取消', version: 2, idempotencyKey: 'opportunity-stage-o1-lost-2' }));

    fireEvent.change(screen.getByLabelText('跟进内容 o1'), { target: { value: '已发送方案' } });
    fireEvent.click(screen.getByRole('button', { name: '追加跟进 o1' }));
    await waitFor(() => expect(appendFollowUp).toHaveBeenCalledWith(expect.objectContaining({ corpId: 7, contactId: 'c1', content: '已发送方案', idempotencyKey: expect.stringMatching(/^opportunity-follow-/) })));

    cleanup();
    renderPage(apiWith({ listOpportunities: vi.fn().mockResolvedValue({ items: [{ ...openOpportunity, stage: 'won', status: 'won' }], nextCursor: '' }) }));
    await screen.findByText('c1');
    expect(screen.queryByRole('button', { name: '赢单 o1' })).toBeNull();
    expect(screen.queryByRole('button', { name: '输单 o1' })).toBeNull();
  });

  it('uses a new idempotency key for each successful create or follow-up operation', async () => {
    const createOpportunity = vi.fn().mockResolvedValue(openOpportunity);
    const appendFollowUp = vi.fn().mockResolvedValue({ id: 'f1', contactId: 'c1', content: '重复内容', createdAt: '2026-08-01T00:00:00Z', createdBy: 3 });
    renderPage(apiWith({ createOpportunity, appendFollowUp }));
    await screen.findByText('c1');

    const fillCreate = () => {
      fireEvent.change(screen.getByLabelText('联系人 ID'), { target: { value: 'c2' } });
      fireEvent.change(screen.getByLabelText('商机金额'), { target: { value: '100' } });
      fireEvent.change(screen.getByLabelText('开始日期'), { target: { value: '2026-08-02' } });
      fireEvent.change(screen.getByLabelText('结束日期'), { target: { value: '2026-08-20' } });
    };
    fillCreate(); fireEvent.click(screen.getByRole('button', { name: '创建商机' }));
    await waitFor(() => expect(createOpportunity).toHaveBeenCalledTimes(1));
    const firstCreateKey = createOpportunity.mock.calls[0]?.[0].idempotencyKey;
    fillCreate(); fireEvent.click(screen.getByRole('button', { name: '创建商机' }));
    await waitFor(() => expect(createOpportunity).toHaveBeenCalledTimes(2));
    expect(createOpportunity.mock.calls[1]?.[0].idempotencyKey).not.toBe(firstCreateKey);

    const followInput = screen.getByLabelText('跟进内容 o1');
    fireEvent.change(followInput, { target: { value: '重复内容' } }); fireEvent.click(screen.getByRole('button', { name: '追加跟进 o1' }));
    await waitFor(() => expect(appendFollowUp).toHaveBeenCalledTimes(1));
    const firstFollowKey = appendFollowUp.mock.calls[0]?.[0].idempotencyKey;
    fireEvent.change(followInput, { target: { value: '重复内容' } }); fireEvent.click(screen.getByRole('button', { name: '追加跟进 o1' }));
    await waitFor(() => expect(appendFollowUp).toHaveBeenCalledTimes(2));
    expect(appendFollowUp.mock.calls[1]?.[0].idempotencyKey).not.toBe(firstFollowKey);
  });

  it.each([
    [409, 'page-state-conflict'],
    [422, 'page-state-invalid-transition'],
    [403, 'page-state-forbidden'],
    [404, 'page-state-not-found'],
  ])('renders mutation status %s through the shared PageState', async (status, className) => {
    const api = apiWith({ changeOpportunityStage: vi.fn().mockRejectedValue(new ApiError('validation', 'failed', { status })) });
    const { container } = renderPage(api);
    fireEvent.click(await screen.findByRole('button', { name: '赢单 o1' }));
    await waitFor(() => expect(container.querySelector(`.${className}`)).not.toBeNull());
  });

  it('renders loading failures, empty data, and retry through PageState', async () => {
    const listOpportunities = vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce({ items: [], nextCursor: '' });
    const { container } = renderPage(apiWith({ listOpportunities }));
    await waitFor(() => expect(container.querySelector('.page-state-error')).not.toBeNull());
    fireEvent.click(container.querySelector('.page-state-retry')!);
    await waitFor(() => expect(container.querySelector('.page-state-empty')).not.toBeNull());
    expect(listOpportunities).toHaveBeenCalledTimes(2);
  });
});
