import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, expect, it, vi } from 'vitest';
import { PublicPoolPage } from './public-pool-page';

vi.mock('../../app/access-context', () => ({ useDashboardAccess: () => ({ corp: { id: '7' } }) }));

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
      listPublicPool: vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce({ items: [], nextCursor: '' }),
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
});
