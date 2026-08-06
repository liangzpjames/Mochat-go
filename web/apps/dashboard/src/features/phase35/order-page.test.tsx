import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
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

test('renders contact name, order fields, and a localized audit timeline', async () => {
  const order={id:'o1',title:'年度续费',note:'客户确认',contactId:'contact-uuid',contactName:'张三',opportunityId:'opp1',amountCents:1200,status:'pending',version:1};
  const api={
    read:vi.fn((endpoint:string)=>Promise.resolve(endpoint==='/scrm/orders'?{data:[order]}:endpoint==='/scrm/orders/o1'?{order,audit:[{action:'created',actorId:7,fromVersion:0,toVersion:1,createdAt:'2026-08-06T10:00:00Z'}]}:{items:[]})),
    write:vi.fn(),
  };
  const access={corp:{id:'9'},session:{},menu:[],allowedRoutes:new Set(),allowedActions:new Set()} as never;
  render(<DashboardAccessProvider value={access}><QueryClientProvider client={new QueryClient()}><OrderPage api={api}/></QueryClientProvider></DashboardAccessProvider>);
  expect(await screen.findByText('张三')).not.toBeNull();
  fireEvent.click(screen.getByRole('button',{name:'查看详情'}));
  expect(await screen.findByText('客户确认')).not.toBeNull();
  expect(screen.getByText(/创建订单 · 操作人 7/)).not.toBeNull();
  expect(screen.getByText(/版本 0 → 1/)).not.toBeNull();
  expect(screen.queryByText('contact-uuid')).toBeNull();
});

test('disables duplicate submission while pending and preserves fields on failure', async () => {
  let rejectCreate!: (reason: Error) => void;
  const pending=new Promise<unknown>((_,reject)=>{rejectCreate=reject;});
  const contact={id:'c1',name:'张三'};
  const api={read:vi.fn((endpoint:string)=>Promise.resolve(endpoint==='/scrm/contacts'?{items:[contact]}:{items:[]})),write:vi.fn(()=>pending)};
  const access={corp:{id:'9'},session:{},menu:[],allowedRoutes:new Set(),allowedActions:new Set()} as never;
  render(<DashboardAccessProvider value={access}><QueryClientProvider client={new QueryClient()}><OrderPage api={api}/></QueryClientProvider></DashboardAccessProvider>);
  await screen.findByRole('option',{name:'张三'});
  fireEvent.change(screen.getByLabelText('联系人'),{target:{value:'c1'}});
  fireEvent.change(screen.getByLabelText('订单标题'),{target:{value:'续费订单'}});
  fireEvent.change(screen.getByLabelText('金额（元）'),{target:{value:'12.00'}});
  const submit=screen.getByRole('button',{name:'创建订单'});
  fireEvent.click(submit);
  expect((await screen.findByRole('button',{name:'创建中…'})).hasAttribute('disabled')).toBe(true);
  fireEvent.click(screen.getByRole('button',{name:'创建中…'}));
  expect(api.write).toHaveBeenCalledTimes(1);
  rejectCreate(new Error('failed'));
  await waitFor(()=>expect(screen.getByRole('alert')).not.toBeNull());
  expect((screen.getByLabelText('订单标题') as HTMLInputElement).value).toBe('续费订单');
  expect((screen.getByLabelText('金额（元）') as HTMLInputElement).value).toBe('12.00');
});
