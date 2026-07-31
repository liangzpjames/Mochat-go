import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { LeadPage } from './lead-page';

vi.mock('../../app/access-context', () => ({ useDashboardAccess: () => ({ corp: { id: '7' }, allowedActions: new Set(['/customer/clue/default@add']) }) }));
afterEach(cleanup);

describe('LeadPage', () => {
  it('lists real leads and submits a new lead', async () => {
    const api = {
      list: vi.fn().mockResolvedValue({ items: [], nextCursor: '' }),
      create: vi.fn().mockResolvedValue({ id: 'lead-1', name: '客户甲', status: 'new' }),
    };
    render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><LeadPage api={api} /></QueryClientProvider>);
    expect(await screen.findByRole('heading', { name: '线索池' })).toBeTruthy();
    fireEvent.change(screen.getByRole('textbox', { name: '客户名称' }), { target: { value: '客户甲' } });
    fireEvent.change(screen.getByRole('textbox', { name: '业务标识' }), { target: { value: 'wx:customer-1' } });
    fireEvent.click(screen.getByRole('button', { name: '新增线索' }));
    await waitFor(() => expect(api.create).toHaveBeenCalledWith({ businessKey: 'wx:customer-1', name: '客户甲', source: 'wecom' }));
  });
});
