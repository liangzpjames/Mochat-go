import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { vi, test, expect } from 'vitest';
import { GroupPage } from './group-page';

test('loads groups and retrieves members for selected group', async () => {
  const api = { read: vi.fn().mockResolvedValueOnce({ list: [{ id: 9, name: '客户群', ownerName: 'Lina', memberCount: 2 }], total: 1 }).mockResolvedValueOnce({ id: 9, name: '客户群', members: [{ id: 1, name: 'Ada' }] }), write: vi.fn() };
  render(<QueryClientProvider client={new QueryClient()}><GroupPage api={api} /></QueryClientProvider>);
  fireEvent.click(await screen.findByRole('button', { name: '查看 客户群' }));
  await waitFor(() => expect(api.read).toHaveBeenLastCalledWith('/workRoom/roomIndex', expect.objectContaining({ roomId: 9 })));
  expect(await screen.findByText('Ada')).not.toBeNull();
});
test('explains provider setup when no groups are synchronized',async()=>{const api={read:vi.fn().mockResolvedValue({list:[],total:0}),write:vi.fn()};render(<QueryClientProvider client={new QueryClient()}><GroupPage api={api}/></QueryClientProvider>);expect(await screen.findByRole('region',{name:'客户群数据接入说明'})).not.toBeNull();expect(screen.getByText(/群同步 Provider/)).not.toBeNull()});
