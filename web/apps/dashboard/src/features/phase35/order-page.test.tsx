import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, expect, test, vi } from 'vitest';
import { OrderPage } from './order-page';
import { DashboardAccessProvider } from '../../app/access-context';

afterEach(cleanup);

test('uses typed business fields and blocks an incomplete order', () => {
  const api = { read: vi.fn(), write: vi.fn() };
  render(<QueryClientProvider client={new QueryClient()}><OrderPage api={api} /></QueryClientProvider>);
  expect(screen.getByLabelText('联系人')).not.toBeNull();
  expect(screen.getByLabelText('订单标题')).not.toBeNull();
  expect(screen.getByLabelText('金额（元）')).not.toBeNull();
  expect(screen.getByLabelText('备注')).not.toBeNull();
  fireEvent.change(screen.getByLabelText('金额（元）'), { target: { value: '12.345' } });
  expect(screen.getByRole('button', { name: '创建订单' }).hasAttribute('disabled')).toBe(true);
  expect(screen.queryByText(/\{\s*"/)).toBeNull();
});

test('quick creates a real contact and selects it for the order', async () => {
  const api={read:vi.fn().mockResolvedValue({items:[]}),write:vi.fn().mockResolvedValue({id:'contact-new',name:'新联系人'})};
  const access={corp:{id:'9'},session:{},menu:[],allowedRoutes:new Set(),allowedActions:new Set()} as never;
  render(<DashboardAccessProvider value={access}><QueryClientProvider client={new QueryClient()}><OrderPage api={api}/></QueryClientProvider></DashboardAccessProvider>);
  const quickName = await screen.findByLabelText('快速联系人姓名') as HTMLInputElement;
  const quickPhone = screen.getByLabelText('快速联系人手机') as HTMLInputElement;
  fireEvent.change(quickName,{target:{value:'新联系人'}});fireEvent.change(quickPhone,{target:{value:'13800138000'}});fireEvent.click(screen.getByRole('button',{name:'创建并选中'}));
  expect(await screen.findByText('联系人已创建并自动选中，可以继续填写订单。')).not.toBeNull();
  expect((screen.getByLabelText('联系人') as HTMLSelectElement).value).toBe('contact-new');
  expect(screen.queryByLabelText('快速联系人姓名')).toBeNull();
  expect(screen.queryByLabelText('快速联系人手机')).toBeNull();
  expect(screen.queryByText('联系人创建失败，请检查姓名、手机号或权限。')).toBeNull();
  expect(api.write).toHaveBeenCalledWith('/scrm/contacts',expect.objectContaining({corpId:9,name:'新联系人'}),'POST');
});
