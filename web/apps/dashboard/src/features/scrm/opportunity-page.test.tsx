import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, expect, it, vi } from 'vitest';
import { OpportunityPage } from './opportunity-page';

vi.mock('../../app/access-context', () => ({ useDashboardAccess: () => ({ corp: { id: '7' } }) }));

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
});
