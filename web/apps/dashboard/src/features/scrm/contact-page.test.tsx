import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { ContactPage } from './contact-page';

vi.mock('../../app/access-context', () => ({ useDashboardAccess: () => ({ corp: { id: '7' } }) }));

describe('ContactPage', () => {
  it('lists contacts inside the selected corp scope', async () => {
    const api = { listContacts: vi.fn().mockResolvedValue({ items: [{ id: 'c1', name: '张三', phone: '13800000000' }] }) };
    render(<QueryClientProvider client={new QueryClient()}><ContactPage api={api} /></QueryClientProvider>);
    expect(await screen.findByText('张三')).toBeTruthy();
    expect(api.listContacts).toHaveBeenCalledWith({ corpId: 7 });
  });

  it('uses shared surfaces and retries the PageState query failure', async () => {
    const api = {
      listContacts: vi.fn()
        .mockRejectedValueOnce(new Error('offline'))
        .mockResolvedValueOnce({ items: [{ id: 'c1', name: '张三', phone: null }] }),
    };
    const { container } = render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><ContactPage api={api} /></QueryClientProvider>);

    await waitFor(() => expect(container.querySelector('.page-state-error')).not.toBeNull());
    fireEvent.click(container.querySelector('.page-state-retry')!);
    await screen.findByText('张三');

    expect(api.listContacts).toHaveBeenCalledTimes(2);
    expect(container.querySelector('.dashboard-page-header')).not.toBeNull();
    expect(container.querySelector('.dashboard-data-card')).not.toBeNull();
    expect(container.querySelector('.dashboard-table-scroll')).not.toBeNull();
  });
});
