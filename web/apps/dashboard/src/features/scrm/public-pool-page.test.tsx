import { ApiError } from '@mochat/api-client';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { PublicPoolPage } from './public-pool-page';

vi.mock('../../app/access-context', () => ({ useDashboardAccess: () => ({ corp: { id: '7' } }) }));
afterEach(cleanup);

describe('PublicPoolPage', () => {
  it('lists public customers and claims with the current version', async () => {
    const claim = vi.fn().mockResolvedValue({});
    const api = { listPublicPool: vi.fn().mockResolvedValue({ items: [{ id: 'a1', contactId: 'c1', ownerId: null, collaboratorIds: [], status: 'public_pool', version: 3 }], nextCursor: '' }), updateAssignment: vi.fn(), releaseToPublicPool: vi.fn(), claimFromPublicPool: claim };
    render(<QueryClientProvider client={new QueryClient()}><PublicPoolPage api={api} /></QueryClientProvider>);
    await screen.findByText('c1');
    fireEvent.click(screen.getByRole('button', { name: '领取' }));
    await waitFor(() => expect(claim).toHaveBeenCalledWith({ corpId: 7, contactId: 'c1', version: 3, idempotencyKey: 'claim-c1-3' }));
  });

  it('uses shared table surfaces and retries the PageState query failure', async () => {
    const api = {
      listPublicPool: vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce({ items: [{ id: 'a1', contactId: 'c1', ownerId: null, collaboratorIds: [], status: 'public_pool', version: 3 }], nextCursor: '' }),
      claimFromPublicPool: vi.fn(),
    };
    const { container } = render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><PublicPoolPage api={api} /></QueryClientProvider>);

    await waitFor(() => expect(container.querySelector('.page-state-error')).not.toBeNull());
    fireEvent.click(container.querySelector('.page-state-retry')!);
    await waitFor(() => expect(api.listPublicPool).toHaveBeenCalledTimes(2));

    expect(container.querySelector('.dashboard-page-header')).not.toBeNull();
    expect(container.querySelector('.dashboard-data-card')).not.toBeNull();
    expect(container.querySelector('.dashboard-table-scroll')).not.toBeNull();
  });

  it('renders empty and 403 query responses through PageState', async () => {
    const emptyApi = { listPublicPool: vi.fn().mockResolvedValue({ items: [], nextCursor: '' }), claimFromPublicPool: vi.fn() };
    const emptyView = render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><PublicPoolPage api={emptyApi} /></QueryClientProvider>);
    await waitFor(() => expect(emptyView.container.querySelector('.page-state-empty')).not.toBeNull());
    emptyView.unmount();

    const forbiddenApi = { listPublicPool: vi.fn().mockRejectedValue(new ApiError('forbidden', '无权限', { status: 403 })), claimFromPublicPool: vi.fn() };
    const forbiddenView = render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><PublicPoolPage api={forbiddenApi} /></QueryClientProvider>);
    await waitFor(() => expect(forbiddenView.container.querySelector('.page-state-forbidden')).not.toBeNull());
  });

  it('renders a 409 claim conflict through PageState while preserving the business message', async () => {
    const api = {
      listPublicPool: vi.fn().mockResolvedValue({ items: [{ id: 'a1', contactId: 'c1', ownerId: null, collaboratorIds: [], status: 'public_pool', version: 3 }], nextCursor: '' }),
      claimFromPublicPool: vi.fn().mockRejectedValue(new ApiError('validation', 'conflict', { status: 409 })),
    };
    const { container } = render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><PublicPoolPage api={api} /></QueryClientProvider>);
    fireEvent.click(await screen.findByRole('button', { name: '领取' }));
    await waitFor(() => expect(container.querySelector('.page-state-conflict')).not.toBeNull());
    expect(screen.getByText('领取冲突，请刷新后重试。')).toBeTruthy();
  });
});
