import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { vi, test, expect } from 'vitest';
import { FriendsPage } from './friends-page';

test('filters friends and opens real detail', async () => {
  const api = { read: vi.fn().mockResolvedValueOnce({ list: [{ id: 7, name: 'Ada', employeeName: 'Lina', tags: ['VIP'] }], total: 1 }).mockResolvedValueOnce({ id: 7, name: 'Ada', tracks: [] }), write: vi.fn() };
  render(<QueryClientProvider client={new QueryClient()}><FriendsPage api={api} /></QueryClientProvider>);
  await screen.findByText('Ada');
  fireEvent.click(screen.getByRole('button', { name: '查看 Ada' }));
  await waitFor(() => expect(api.read).toHaveBeenLastCalledWith('/workContact/show', expect.objectContaining({ id: 7 })));
  expect(await screen.findByRole('dialog', { name: '好友详情' })).not.toBeNull();
});
