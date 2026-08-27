import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, vi, test, expect } from 'vitest';
import { FriendsPage } from './friends-page';

afterEach(cleanup);

test('filters friends and opens real detail', async () => {
  const api = { read: vi.fn().mockResolvedValueOnce({ list: [{ id: 7, contactId: 7, employeeId: 3, name: 'Ada', employeeName: 'Lina', tags: ['VIP'], addWayText: '扫码' }], total: 1 }).mockResolvedValueOnce({ id: 7, name: 'Ada', employeeName: 'Lina', tag: [{ tagId: 1, tagName: 'VIP' }], addWayText: '扫码' }), write: vi.fn() };
  render(<QueryClientProvider client={new QueryClient()}><FriendsPage api={api} /></QueryClientProvider>);
  await screen.findByText('Ada');
  fireEvent.click(screen.getByRole('button', { name: '查看 Ada' }));
  await waitFor(() => expect(api.read).toHaveBeenLastCalledWith('/workContact/show', expect.objectContaining({ contactId: 7, employeeId: 3 })));
  expect(await screen.findByRole('dialog', { name: '好友详情' })).not.toBeNull();
});
test('centers plain-language next steps when friend sync has no data',async()=>{const api={read:vi.fn().mockResolvedValue({list:[],total:0}),write:vi.fn()};render(<QueryClientProvider client={new QueryClient()}><FriendsPage api={api}/></QueryClientProvider>);const emptyState=await screen.findByRole('region',{name:'好友数据接入说明'});expect(emptyState.classList.contains('phase35-empty-state')).toBe(true);expect(screen.getByText(/完成企业微信联系人同步后/)).not.toBeNull();expect(screen.queryByText(/Provider/i)).toBeNull()});
