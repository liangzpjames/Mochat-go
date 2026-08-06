import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { vi, test, expect } from 'vitest';
import { FriendsPage } from './friends-page';

test('filters friends and opens real detail', async () => {
  const api = { read: vi.fn().mockResolvedValueOnce({ list: [{ id: 7, contactId: 7, employeeId: 3, name: 'Ada', employeeName: 'Lina', tags: ['VIP'], addWayText: '扫码' }], total: 1 }).mockResolvedValueOnce({ id: 7, name: 'Ada', employeeName: 'Lina', tag: [{ tagId: 1, tagName: 'VIP' }], addWayText: '扫码' }), write: vi.fn() };
  render(<QueryClientProvider client={new QueryClient()}><FriendsPage api={api} /></QueryClientProvider>);
  await screen.findByText('Ada');
  fireEvent.click(screen.getByRole('button', { name: '查看 Ada' }));
  await waitFor(() => expect(api.read).toHaveBeenLastCalledWith('/workContact/show', expect.objectContaining({ contactId: 7, employeeId: 3 })));
  expect(await screen.findByRole('dialog', { name: '好友详情' })).not.toBeNull();
});
test('explains the next step when friend sync has no data',async()=>{const api={read:vi.fn().mockResolvedValue({list:[],total:0}),write:vi.fn()};render(<QueryClientProvider client={new QueryClient()}><FriendsPage api={api}/></QueryClientProvider>);expect(await screen.findByRole('region',{name:'好友数据接入说明'})).not.toBeNull();expect(screen.getByText(/企业微信通讯录同步/)).not.toBeNull()});
