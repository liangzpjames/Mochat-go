import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { WeComBridge } from '../wecom/wecom-bridge';
import { BatchAddPage, ContactSopPage, MediumPage, RoomSopPage } from './business-pages';

const sendChatMessage = vi.fn().mockResolvedValue(undefined);
const navigateToAddCustomer = vi.fn().mockResolvedValue(undefined);
const bridge: WeComBridge = {
  available: () => true,
  sendChatMessage,
  navigateToAddCustomer,
};

afterEach(() => { cleanup(); vi.clearAllMocks(); });

describe('Sidebar business pages', () => {
  it('loads persisted media and sends a selected text through WeCom', async () => {
    const request = vi.fn()
      .mockResolvedValueOnce([{ id: 0, name: '未分组' }])
      .mockResolvedValueOnce({ page: { perPage: 20, total: 1, totalPage: 1 }, list: [{ id: 8, type: '文本', mediaId: '', content: { content: '真实活动文案' } }] });
    render(<MemoryRouter><MediumPage bridge={bridge} onReauthenticate={vi.fn()} request={request} /></MemoryRouter>);
    expect(await screen.findByText('真实活动文案')).not.toBeNull();
    fireEvent.click(screen.getByRole('checkbox', { name: /真实活动文案/ }));
    fireEvent.click(screen.getByRole('button', { name: '发送已选素材' }));
    await waitFor(() => expect(sendChatMessage).toHaveBeenCalledWith({ type: 'text', content: '真实活动文案' }));
    expect(await screen.findByText('已发送 1 项')).not.toBeNull();
  });

  it('shows an honest unavailable message outside WeCom', async () => {
    const unavailable = { ...bridge, available: () => false };
    const request = vi.fn().mockResolvedValueOnce([]).mockResolvedValueOnce({ page: { perPage: 20, total: 0, totalPage: 0 }, list: [] });
    render(<MemoryRouter><MediumPage bridge={unavailable} onReauthenticate={vi.fn()} request={request} /></MemoryRouter>);
    expect(await screen.findByText('暂无可用素材')).not.toBeNull();
    expect(screen.getByText(/需在企业微信客户端中打开/)).not.toBeNull();
  });

  it('loads a personal SOP and copies text without claiming it was sent', async () => {
    const request = vi.fn().mockResolvedValue({ id: 4, contactSopId: 12, creator: '员工甲', time: '09:00', tipTime: '2026-08-22', task: { content: [{ type: 0, value: '请今日回访' }] }, contact: { id: 11, name: '林晓', avatar: null } });
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } });
    render(<MemoryRouter initialEntries={['/contactSop?id=4']}><ContactSopPage onReauthenticate={vi.fn()} request={request} /></MemoryRouter>);
    expect(await screen.findByText('林晓')).not.toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '复制文本' }));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith('请今日回访'));
    expect(screen.getByText('文本已复制，尚未发送。')).not.toBeNull();
  });

  it('persists room completion once and disables repeated submission', async () => {
    const request = vi.fn()
      .mockResolvedValueOnce({ id: 5, roomSopId: 13, creator: '员工乙', time: '10:00', state: 0, task: { content: [] }, room: { id: 21, name: '客户群' } })
      .mockResolvedValueOnce([]);
    render(<MemoryRouter initialEntries={['/roomSop?id=5']}><RoomSopPage onReauthenticate={vi.fn()} request={request} /></MemoryRouter>);
    const button = await screen.findByRole('button', { name: '标记为已完成' });
    fireEvent.click(button); fireEvent.click(button);
    await waitFor(() => expect(request).toHaveBeenCalledTimes(2));
    expect(screen.getByRole('button', { name: '已完成' }).hasAttribute('disabled')).toBe(true);
  });

  it('filters persisted batch-add contacts and invokes the authorized add action', async () => {
    const request = vi.fn().mockResolvedValue({ employeeName: '员工甲', list: [{ id: 1, phone: '13800000000', status: '待添加' }] });
    render(<MemoryRouter initialEntries={['/contactBatchAdd?batchId=9']}><BatchAddPage bridge={bridge} onReauthenticate={vi.fn()} request={request} /></MemoryRouter>);
    expect(await screen.findByText('13800000000')).not.toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '打开企微加客户' }));
    await waitFor(() => expect(navigateToAddCustomer).toHaveBeenCalledTimes(1));
  });
});
