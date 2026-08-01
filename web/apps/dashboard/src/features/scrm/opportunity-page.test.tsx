import { ApiError } from '@mochat/api-client';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { OpportunityPage } from './opportunity-page';

vi.mock('../../app/access-context', () => ({ useDashboardAccess: () => ({ corp: { id: '7' } }) }));
afterEach(cleanup);

describe('OpportunityPage', () => {
  it('filters by stage and changes an opportunity stage with the version', async () => {
    const change = vi.fn().mockResolvedValue({});
    const api = {
      listOpportunities: vi.fn().mockResolvedValue({ items: [{ id: 'o1', contactId: 'c1', stage: 'proposal', amount: 100, startDate: '2026-08-01', endDate: '2026-08-02', ownerId: 9, status: 'open', version: 2 }], nextCursor: '' }),
      createOpportunity: vi.fn(), changeOpportunityStage: change,
    };
    render(<QueryClientProvider client={new QueryClient()}><OpportunityPage api={api} /></QueryClientProvider>);
    await screen.findByText('c1');
    fireEvent.change(screen.getByLabelText('阶段'), { target: { value: 'proposal' } });
    fireEvent.click(await screen.findByRole('button', { name: '赢单' }));
    await waitFor(() => expect(change).toHaveBeenCalledWith({ corpId: 7, opportunityId: 'o1', toStage: 'won', reason: '', version: 2, idempotencyKey: 'opportunity-o1-won-2' }));
  });

  it('uses shared filters and retries the PageState query failure', async () => {
    const api = {
      listOpportunities: vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce({ items: [{ id: 'o1', contactId: 'c1', stage: 'proposal', amount: 100, startDate: '', endDate: '', ownerId: null, status: 'open', version: 2 }], nextCursor: '' }),
      changeOpportunityStage: vi.fn(),
    };
    const { container } = render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><OpportunityPage api={api} /></QueryClientProvider>);

    await waitFor(() => expect(container.querySelector('.page-state-error')).not.toBeNull());
    fireEvent.click(container.querySelector('.page-state-retry')!);
    await waitFor(() => expect(api.listOpportunities).toHaveBeenCalledTimes(2));

    expect(container.querySelector('.dashboard-page-header')).not.toBeNull();
    expect(container.querySelector('.dashboard-filter-bar')).not.toBeNull();
    expect(container.querySelector('.dashboard-data-card')).not.toBeNull();
    expect(container.querySelector('.dashboard-table-scroll')).not.toBeNull();
  });

  it('renders empty and 403 query responses through PageState', async () => {
    const emptyApi = { listOpportunities: vi.fn().mockResolvedValue({ items: [], nextCursor: '' }), changeOpportunityStage: vi.fn() };
    const emptyView = render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><OpportunityPage api={emptyApi} /></QueryClientProvider>);
    await waitFor(() => expect(emptyView.container.querySelector('.page-state-empty')).not.toBeNull());
    emptyView.unmount();

    const forbiddenApi = { listOpportunities: vi.fn().mockRejectedValue(new ApiError('forbidden', '无权限', { status: 403 })), changeOpportunityStage: vi.fn() };
    const forbiddenView = render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><OpportunityPage api={forbiddenApi} /></QueryClientProvider>);
    await waitFor(() => expect(forbiddenView.container.querySelector('.page-state-forbidden')).not.toBeNull());
  });

  it('renders a 409 stage conflict through PageState while preserving the business message', async () => {
    const api = {
      listOpportunities: vi.fn().mockResolvedValue({ items: [{ id: 'o1', contactId: 'c1', stage: 'proposal', amount: 100, startDate: '', endDate: '', ownerId: null, status: 'open', version: 2 }], nextCursor: '' }),
      changeOpportunityStage: vi.fn().mockRejectedValue(new ApiError('validation', 'conflict', { status: 409 })),
    };
    const { container } = render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><OpportunityPage api={api} /></QueryClientProvider>);
    fireEvent.click(await screen.findByRole('button', { name: '赢单' }));
    await waitFor(() => expect(container.querySelector('.page-state-conflict')).not.toBeNull());
    expect(screen.getByText('机会状态冲突，请刷新后重试')).toBeTruthy();
  });
});
