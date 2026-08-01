import { ApiError } from '@mochat/api-client';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { LeadPage } from './lead-page';

vi.mock('../../app/access-context', () => ({ useDashboardAccess: () => ({ corp: { id: '7' }, allowedActions: new Set(['/customer/clue/default@add']) }) }));
afterEach(cleanup);

describe('LeadPage', () => {
  it('lists real leads and submits a new lead', async () => {
    const api = {
      list: vi.fn().mockResolvedValue({ items: [{ id: 'lead-0', businessKey: 'wx:existing', name: '已有线索', source: 'wecom', status: 'new', version: 1 }], nextCursor: '' }),
      create: vi.fn().mockResolvedValue({ id: 'lead-1', name: '客户甲', status: 'new' }),
    };
    render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><LeadPage api={api} /></QueryClientProvider>);
    expect(await screen.findByRole('heading', { name: '线索池' })).toBeTruthy();
    fireEvent.change(screen.getByRole('textbox', { name: '客户名称' }), { target: { value: '客户甲' } });
    fireEvent.change(screen.getByRole('textbox', { name: '业务标识' }), { target: { value: 'wx:customer-1' } });
    fireEvent.click(screen.getByRole('button', { name: '新增线索' }));
    await waitFor(() => expect(api.create).toHaveBeenCalledWith({ businessKey: 'wx:customer-1', name: '客户甲', source: 'wecom' }));
  });

  it('uses shared filters and retries the PageState query failure', async () => {
    const api = {
      list: vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce({ items: [{ id: 'lead-0', businessKey: 'wx:existing', name: '已有线索', source: 'wecom', status: 'new', version: 1 }], nextCursor: '' }),
      create: vi.fn(),
    };
    const { container } = render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><LeadPage api={api} /></QueryClientProvider>);

    await waitFor(() => expect(container.querySelector('.page-state-error')).not.toBeNull());
    fireEvent.click(container.querySelector('.page-state-retry')!);
    await waitFor(() => expect(api.list).toHaveBeenCalledTimes(2));

    expect(container.querySelector('.dashboard-page-header')).not.toBeNull();
    expect(container.querySelector('.dashboard-filter-bar')).not.toBeNull();
    expect(container.querySelector('.dashboard-data-card')).not.toBeNull();
    expect(container.querySelector('.dashboard-table-scroll')).not.toBeNull();
  });

  it('renders empty and 403 query responses through PageState', async () => {
    const emptyApi = { list: vi.fn().mockResolvedValue({ items: [], nextCursor: '' }), create: vi.fn() };
    const emptyView = render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><LeadPage api={emptyApi} /></QueryClientProvider>);
    await waitFor(() => expect(emptyView.container.querySelector('.page-state-empty')).not.toBeNull());
    emptyView.unmount();

    const forbiddenApi = { list: vi.fn().mockRejectedValue(new ApiError('forbidden', '无权限', { status: 403 })), create: vi.fn() };
    const forbiddenView = render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><LeadPage api={forbiddenApi} /></QueryClientProvider>);
    await waitFor(() => expect(forbiddenView.container.querySelector('.page-state-forbidden')).not.toBeNull());
  });
});
