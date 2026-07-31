import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
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
});
