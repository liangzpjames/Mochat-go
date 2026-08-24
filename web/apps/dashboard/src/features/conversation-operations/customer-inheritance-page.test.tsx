import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import { CustomerInheritancePage } from './customer-inheritance-page';
import type { ContactTransferApi } from './contact-transfer-api';

afterEach(() => cleanup());

const customer = (id: number, name: string) => ({
  contactId: id, employeeId: 9, contactWxId: `wx-${id}`, employeeWxId: 'former-9', contactName: name,
  nickName: name, corpName: '星河科技', employeeName: '原员工', tags: [], transferState: '等待接替',
  addTime: '2026-08-20 10:00:00', addWay: '客户添加',
});

function makeApi(overrides: Partial<ContactTransferApi> = {}): ContactTransferApi {
  return {
    assigned: vi.fn().mockResolvedValue([customer(31, '重点客户')]),
    unassigned: vi.fn().mockResolvedValue({ list: [customer(31, '重点客户')], lastTime: '2026-08-20 10:00:00' }),
    rooms: vi.fn().mockResolvedValue([{ roomId: 8, chatId: 'room-8', roomName: '产品交流群', owner: '李四', userNum: 8, addNum: 2, quitNum: 1, createTime: '2026-08-20 09:00:00' }]),
    logs: vi.fn().mockResolvedValue([]),
    employees: vi.fn().mockResolvedValue([{ id: 9, name: '原员工', wxUserId: 'former-9', status: 1 }, { id: 10, name: '接替员工', wxUserId: 'takeover-10', status: 1 }]),
    syncUnassigned: vi.fn().mockResolvedValue(undefined),
    transferCustomers: vi.fn().mockResolvedValue([]),
    transferRooms: vi.fn().mockResolvedValue([]),
    ...overrides,
  };
}

function renderPage(api: ContactTransferApi, path = '/customer/inheritance') {
  const access = { corp: { id: '9', authorized: true }, session: {}, menu: [], allowedRoutes: new Set(['/customer/inheritance']), allowedActions: new Set<string>() } as never;
  return render(<MemoryRouter initialEntries={[path]}><DashboardAccessProvider value={access}><QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><CustomerInheritancePage api={api} /></QueryClientProvider></DashboardAccessProvider></MemoryRouter>);
}

describe('CustomerInheritancePage', () => {
  it('loads resigned customers and rooms through their real endpoints', async () => {
    const unassigned = vi.fn<ContactTransferApi['unassigned']>().mockResolvedValue({ list: [customer(31, '重点客户')], lastTime: '2026-08-20 10:00:00' });
    const rooms = vi.fn<ContactTransferApi['rooms']>().mockResolvedValue([{ roomId: 8, chatId: 'room-8', roomName: '产品交流群', owner: '李四', userNum: 8, addNum: 2, quitNum: 1, createTime: '2026-08-20 09:00:00' }]);
    const api = makeApi({ unassigned, rooms });
    renderPage(api, '/customer/inheritance?inheritanceTab=resigned&assetTab=customer');
    expect(await screen.findByText('重点客户')).not.toBeNull();
    expect(unassigned).toHaveBeenCalled();
    fireEvent.click(screen.getByRole('tab', { name: '待分配群聊' }));
    expect(await screen.findByText('产品交流群')).not.toBeNull();
    expect(rooms).toHaveBeenCalled();
  });

  it('keeps the sync action available while browsing unassigned rooms', async () => {
    const api = makeApi();
    renderPage(api, '/customer/inheritance?inheritanceTab=resigned&assetTab=room');
    expect(await screen.findByText('产品交流群')).not.toBeNull();
    expect(screen.getByRole('button', { name: '同步离职数据' })).not.toBeNull();
  });

  it('shows active customer transfer without deferred product entries', async () => {
    const api = makeApi();
    renderPage(api, '/customer/inheritance');
    fireEvent.click(screen.getByRole('tab', { name: '在职继承' }));
    expect(await screen.findByText('请选择需要被继承的员工')).not.toBeNull();
    expect(screen.queryByRole('tab', { name: '侧边栏配置' })).toBeNull();
    expect(screen.queryByRole('tab', { name: /在职群聊/ })).toBeNull();
    expect(screen.queryByText(/能力未接入|功能未接入/)).toBeNull();
  });

  it('guards assignment until a different successor is selected', async () => {
    const assigned = vi.fn<ContactTransferApi['assigned']>().mockResolvedValue([customer(31, '重点客户')]);
    const api = makeApi({ assigned });
    renderPage(api, '/customer/inheritance?inheritanceTab=active');
    await screen.findAllByRole('option', { name: '原员工' });
    fireEvent.change(screen.getByLabelText('原跟进员工'), { target: { value: '9' } });
    await waitFor(() => expect(assigned).toHaveBeenCalledWith(expect.objectContaining({ employeeIds: [9] })));
    fireEvent.click(await screen.findByLabelText('选择客户 31'));
    fireEvent.change(screen.getByLabelText('接替员工'), { target: { value: '9' } });
    expect(screen.getByRole<HTMLButtonElement>('button', { name: '分配给其他员工' }).disabled).toBe(true);
    expect(screen.getByText('接替员工不能与原跟进员工相同')).not.toBeNull();
  });

  it('summarizes partial provider failures and retains failed rows', async () => {
    const api = makeApi({
      unassigned: vi.fn().mockResolvedValue({ list: [customer(31, '客户一'), customer(32, '客户二')], lastTime: '' }),
      transferCustomers: vi.fn().mockResolvedValue([{ errcode: 0 }, { errcode: 40096, errmsg: '客户拒绝' }]),
    });
    renderPage(api);
    fireEvent.click(await screen.findByLabelText('选择客户 31'));
    fireEvent.click(screen.getByLabelText('选择客户 32'));
    fireEvent.change(screen.getByLabelText('接替员工'), { target: { value: '10' } });
    fireEvent.click(screen.getByRole('button', { name: '分配给其他员工' }));
    fireEvent.click(await screen.findByRole('button', { name: '确认分配' }));
    expect((await screen.findByRole('status')).textContent).toContain('成功 1 条，失败 1 条');
    expect(screen.queryByLabelText('选择客户 31')).toBeNull();
    expect(screen.getByLabelText<HTMLInputElement>('选择客户 32').checked).toBe(true);
  });

  it('reveals the refreshed snapshot again after a successful synchronization', async () => {
    const api = makeApi({ transferCustomers: vi.fn().mockResolvedValue([{ errcode: 0 }]) });
    renderPage(api);
    fireEvent.click(await screen.findByLabelText('选择客户 31'));
    fireEvent.change(screen.getByLabelText('接替员工'), { target: { value: '10' } });
    fireEvent.click(screen.getByRole('button', { name: '分配给其他员工' }));
    fireEvent.click(await screen.findByRole('button', { name: '确认分配' }));
    await waitFor(() => expect(screen.queryByLabelText('选择客户 31')).toBeNull());

    fireEvent.click(screen.getByRole('button', { name: '同步离职数据' }));
    fireEvent.click(await screen.findByRole('button', { name: '开始同步' }));
    expect(await screen.findByLabelText('选择客户 31')).not.toBeNull();
  });

  it('confirms synchronization completion after the dialog closes', async () => {
    const api = makeApi();
    renderPage(api);
    fireEvent.click(screen.getByRole('button', { name: '同步离职数据' }));
    fireEvent.click(await screen.findByRole('button', { name: '开始同步' }));
    expect((await screen.findByRole('status')).textContent).toContain('离职数据已同步');
  });

  it('opens inheritance records and keeps the main filter after closing', async () => {
    const logs = vi.fn<ContactTransferApi['logs']>().mockResolvedValue([{ mode: 1, name: '重点客户', employee: '接替员工', state: '已完成', corpName: '星河科技', roomNum: 0, createTime: '2026-08-20 10:00:00' }]);
    const api = makeApi({ logs });
    renderPage(api, '/customer/inheritance?contactName=%E9%87%8D%E7%82%B9');
    expect(screen.getByLabelText<HTMLInputElement>('客户名称').value).toBe('重点');
    fireEvent.click(screen.getByRole('button', { name: '继承记录' }));
    expect((await screen.findAllByText('重点客户')).length).toBeGreaterThan(0);
    expect(logs).toHaveBeenCalledWith(expect.objectContaining({ mode: 1 }));
    fireEvent.click(screen.getByRole('button', { name: '关闭继承记录' }));
    await waitFor(() => expect(screen.getByLabelText<HTMLInputElement>('客户名称').value).toBe('重点'));
  });
});
