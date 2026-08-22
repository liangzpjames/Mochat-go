/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';
import {
  ChannelCodePage,
  GroupCodePage,
  LiveCodeShortChainPage,
} from './acquisition-pages';

const access: AccessContext = {
  session: { token: 'token', userId: '1', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set([
    '/acquisition/v2-channel-code',
    '/acquisition/group-code',
    '/acquisition/live-code-short-chain',
  ]),
  allowedActions: new Set(),
};

beforeEach(() => {
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} });
});
afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

function view(page: React.ReactNode, allowedActions = access.allowedActions) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const rendered = render(
    <MemoryRouter>
      <QueryClientProvider client={queryClient}>
        <DashboardAccessProvider value={{ ...access, allowedActions }}>
          {page}
        </DashboardAccessProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
  return { ...rendered, queryClient };
}

describe('Phase 3.4 acquisition pages', () => {
  it('loads channel codes from the existing dashboard provider and applies a name filter', async () => {
    const read = vi.fn().mockResolvedValue({
      list: [{ channelCodeId: 12, name: '官网咨询', employeeNames: ['销售一部'], contactNum: 8 }],
    });
    const api: BusinessWorkbenchApi = { read, write: vi.fn() };
    view(<ChannelCodePage api={api} />);

    expect(await screen.findByText('官网咨询')).toBeTruthy();
    expect(read).toHaveBeenCalledWith('/channelCode/index', expect.objectContaining({ page: 1, perPage: 20 }));
    fireEvent.change(screen.getByLabelText('活码名称'), { target: { value: '官网' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));

    await waitFor(() => expect(read).toHaveBeenLastCalledWith(
      '/channelCode/index',
      expect.objectContaining({ name: '官网', page: 1, perPage: 20 }),
    ));
  });

  it('shows channel code details using provider data', async () => {
    const api: BusinessWorkbenchApi = {
      read: vi.fn().mockResolvedValue({ list: [{ channelCodeId: 12, name: '官网咨询', contactNum: 8 }] }),
      write: vi.fn(),
    };
    view(<ChannelCodePage api={api} />);

    await screen.findByText('官网咨询');
    fireEvent.click(screen.getByRole('button', { name: '详情' }));

    expect(screen.getByLabelText('渠道活码详情').textContent).toContain('新增好友数');
    expect(screen.getByLabelText('渠道活码详情').textContent).toContain('8');
  });

  it('groups channel details into a scannable drawer and exposes one close action', async () => {
    const api: BusinessWorkbenchApi = {
      read: vi.fn().mockResolvedValue({ list: [{ channelCodeId: 12, name: '官网咨询', contactNum: 8, statisticsAvailable: false }] }),
      write: vi.fn(),
    };
    view(<ChannelCodePage api={api} />);

    await screen.findByText('官网咨询');
    fireEvent.click(screen.getByRole('button', { name: '详情' }));
    const drawer = screen.getByLabelText('渠道活码详情');

    expect(drawer.querySelectorAll('button[aria-label="关闭详情"]')).toHaveLength(1);
    expect(within(drawer).getByRole('heading', { name: '基础信息' })).toBeTruthy();
    expect(within(drawer).getByRole('heading', { name: '归因与效果' })).toBeTruthy();
    expect(within(drawer).getAllByText('暂无可验证数据')).toHaveLength(2);

    fireEvent.click(drawer.querySelector('[data-phase34-detail-backdrop]') as HTMLElement);
    expect(screen.queryByLabelText('渠道活码详情')).toBeNull();
  });

  it('shows group configuration as an explicit empty state in the detail drawer', async () => {
    const api: BusinessWorkbenchApi = {
      read: vi.fn().mockResolvedValue({ list: [{ id: 21, qrcodeName: '售后服务群', isVerified: 0, rooms: '' }] }),
      write: vi.fn(),
    };
    view(<GroupCodePage api={api} />);

    await screen.findByText('售后服务群');
    fireEvent.click(screen.getByRole('button', { name: '详情' }));
    const drawer = screen.getByLabelText('群活码详情');

    expect(within(drawer).getByRole('heading', { name: '基础信息' })).toBeTruthy();
    expect(within(drawer).getByRole('heading', { name: '配置与关系' })).toBeTruthy();
    expect(within(drawer).getByText('暂无关联群聊')).toBeTruthy();
    fireEvent.click(within(drawer).getByRole('button', { name: '返回列表' }));
    expect(screen.queryByLabelText('群活码详情')).toBeNull();
  });

  it('loads group code-compatible records from the real auto-pull endpoint', async () => {
    const read = vi.fn().mockResolvedValue({ list: [{ id: 21, name: '售后服务群', stateText: '进行中', roomNum: 3 }] });
    const api: BusinessWorkbenchApi = { read, write: vi.fn() };
    view(<GroupCodePage api={api} />);

    expect(await screen.findByText('售后服务群')).toBeTruthy();
    expect(read).toHaveBeenCalledWith('/workRoomAutoPull/index', expect.objectContaining({ page: 1, perPage: 20 }));
    expect(screen.getByText('关联群聊')).toBeTruthy();
  });

  it('filters group codes by verification state without changing the list tab', async () => {
    const api: BusinessWorkbenchApi = {
      read: vi.fn().mockResolvedValue({ list: [
        { id: 21, qrcodeName: '已验证群', isVerified: 1 },
        { id: 22, qrcodeName: '待配置群', isVerified: 0 },
      ] }),
      write: vi.fn(),
    };
    view(<GroupCodePage api={api} />);

    expect(await screen.findByText('已验证群')).toBeTruthy();
    expect(screen.getByText('待配置群')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '已验证' }));

    await waitFor(() => {
      expect(screen.getByText('已验证群')).toBeTruthy();
      expect(screen.queryByText('待配置群')).toBeNull();
    });
  });

  it('renders a retryable error returned by a connected provider', async () => {
    const read = vi.fn()
      .mockRejectedValueOnce(new Error('provider unavailable'))
      .mockResolvedValueOnce({ list: [{ channelCodeId: 12, name: '官网咨询' }] });
    const api: BusinessWorkbenchApi = { read, write: vi.fn() };
    view(<ChannelCodePage api={api} />);

    await screen.findByRole('heading', { name: '加载失败' });
    fireEvent.click(screen.getByRole('button', { name: '重新加载' }));

    expect(await screen.findByText('官网咨询')).toBeTruthy();
  });

  it('keeps page actions available until Phase 3.4 granular permissions are published', async () => {
    const api: BusinessWorkbenchApi = { read: vi.fn().mockResolvedValue({ list: [] }), write: vi.fn() };
    view(<ChannelCodePage api={api} />, new Set(['/channelCode/index@search']));

    await screen.findByRole('heading', { name: '暂无记录' });
    expect(screen.getByRole('button', { name: '查询' })).toBeTruthy();
    expect(screen.getByRole('button', { name: '重置' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: '刷新' })).toBeNull();
  });

  it('creates a channel code through the existing write Provider', async () => {
    const write = vi.fn().mockResolvedValue(undefined);
    const read = vi.fn().mockImplementation((path: string) => Promise.resolve(path === '/workEmployee/index'
      ? { page: { totalPage: 1 }, list: [{ id: 21, name: '李娜', status: 1 }] }
      : { list: [] }));
    view(<ChannelCodePage api={{ read, write }} />);

    await screen.findByRole('heading', { name: '暂无记录' });
    fireEvent.click(screen.getByRole('button', { name: '新建渠道活码' }));
    fireEvent.change(screen.getByLabelText('渠道活码名称'), { target: { value: '官网咨询' } });
    fireEvent.click(await screen.findByRole('button', { name: '选择成员 李娜' }));
    fireEvent.click(screen.getByRole('button', { name: '保存渠道活码' }));

    await waitFor(() => expect(write).toHaveBeenCalledWith(
      '/channelCode/store',
      expect.objectContaining({
        baseInfo: expect.objectContaining({ name: '官网咨询', autoAddFriend: 1 }),
        drainageEmployee: expect.objectContaining({ type: 1 }),
        welcomeMessage: expect.any(Object),
      }),
      'POST',
    ));
  });

  it('loads a channel code into the edit drawer, validates it, and updates through the Provider', async () => {
    const write = vi.fn().mockResolvedValue(undefined);
    const read = vi.fn().mockImplementation((path: string) => {
      if (path === '/channelCode/index') return Promise.resolve({ list: [{ channelCodeId: 12, name: '官网咨询' }] });
      if (path === '/channelCode/show') return Promise.resolve({
        baseInfo: { groupId: 0, name: '官网咨询', autoAddFriend: 1, tags: [] },
        drainageEmployee: {
          type: 1,
          specialPeriod: { status: 1, detail: [{ timeSlot: [{ employeeId: [21] }] }] },
          addMax: { status: 2, employees: [], spareEmployeeIds: [] },
        },
        welcomeMessage: { scanCodePush: 2, messageDetail: [] },
      });
      if (path === '/workEmployee/index') return Promise.resolve({ page: { totalPage: 1 }, list: [{ id: 21, name: '李娜', status: 1 }] });
      return Promise.resolve({ list: [] });
    });
    view(<ChannelCodePage api={{ read, write }} />);

    await screen.findByText('官网咨询');
    fireEvent.click(screen.getByRole('button', { name: '编辑' }));
    const drawer = await screen.findByRole('dialog', { name: '编辑渠道活码' });
    const nameInput = within(drawer).getByLabelText<HTMLInputElement>('渠道活码名称');
    expect(nameInput.value).toBe('官网咨询');
    expect(await within(drawer).findByRole('button', { name: '取消选择成员 李娜' })).toBeTruthy();

    fireEvent.change(nameInput, { target: { value: '' } });
    fireEvent.click(within(drawer).getByRole('button', { name: '保存渠道活码' }));
    expect(within(drawer).getByText('请输入活码名称')).toBeTruthy();
    expect(within(drawer).getByRole('alert').textContent).toContain('请检查填写是否合规');
    expect(write).not.toHaveBeenCalled();

    fireEvent.change(nameInput, { target: { value: '官网咨询更新' } });
    fireEvent.click(within(drawer).getByRole('button', { name: '保存渠道活码' }));
    await waitFor(() => expect(write).toHaveBeenCalledWith(
      '/channelCode/update',
      expect.objectContaining({
        channelCodeId: 12,
        baseInfo: expect.objectContaining({ name: '官网咨询更新' }),
      }),
      'PUT',
    ));
  });

  it('creates a group code through the existing auto-pull Provider', async () => {
    const write = vi.fn().mockResolvedValue(undefined);
    const read = vi.fn().mockImplementation((path: string) => {
      if (path === '/workEmployee/index') return Promise.resolve({ page: { totalPage: 1 }, list: [{ id: 21, name: '李娜', status: 1 }] });
      if (path === '/workRoom/roomIndex') return Promise.resolve({ list: [{ roomId: 41, roomName: '客户群一', currentNum: 18, roomMax: 200 }] });
      return Promise.resolve({ list: [] });
    });
    view(<GroupCodePage api={{ read, write }} />);

    await screen.findByRole('heading', { name: '暂无记录' });
    fireEvent.click(screen.getByRole('button', { name: '新建群活码' }));
    fireEvent.change(screen.getAllByLabelText('群活码名称')[1]!, { target: { value: '售后服务群' } });
    fireEvent.change(screen.getByLabelText('入群引导语'), { target: { value: '欢迎入群' } });
    fireEvent.click(await screen.findByRole('button', { name: '选择成员 李娜' }));
    fireEvent.change(screen.getByLabelText('客户标签 ID'), { target: { value: '31' } });
    fireEvent.click(screen.getByRole('button', { name: '选择群聊' }));
    const roomDialog = await screen.findByRole('dialog', { name: '选择群聊' });
    fireEvent.click(await within(roomDialog).findByRole('button', { name: '选择群聊 客户群一' }));
    fireEvent.click(within(roomDialog).getByRole('button', { name: '确认选择' }));
    fireEvent.click(screen.getByRole('button', { name: '保存群活码' }));

    await waitFor(() => expect(write).toHaveBeenCalledWith(
      '/workRoomAutoPull/store',
      expect.objectContaining({ corpId: 7, qrcodeName: '售后服务群', employees: [21], tags: [31] }),
      'POST',
    ));
  });

  it('keeps a failed group edit open with the loaded business choices', async () => {
    const write = vi.fn().mockRejectedValue(new Error('企微 Provider 暂不可用'));
    const read = vi.fn().mockImplementation((path: string) => {
      if (path === '/workRoomAutoPull/index') return Promise.resolve({ list: [{ workRoomAutoPullId: 21, qrcodeName: '售后服务群' }] });
      if (path === '/workRoomAutoPull/show') return Promise.resolve({
        workRoomAutoPullId: 21,
        qrcodeName: '售后服务群',
        leadingWords: '欢迎入群',
        employees: [21],
        tags: [31],
        rooms: [{ roomId: 41, maxNum: 200 }],
      });
      if (path === '/workEmployee/index') return Promise.resolve({ page: { totalPage: 1 }, list: [{ id: 21, name: '李娜', status: 1 }] });
      if (path === '/workRoom/roomIndex') return Promise.resolve({ list: [{ roomId: 41, roomName: '客户群一', currentNum: 18, roomMax: 200 }] });
      return Promise.resolve({ list: [] });
    });
    view(<GroupCodePage api={{ read, write }} />);

    await screen.findByText('售后服务群');
    fireEvent.click(screen.getByRole('button', { name: '编辑' }));
    const drawer = await screen.findByRole('dialog', { name: '编辑群活码' });
    expect(within(drawer).getByLabelText<HTMLInputElement>('群活码名称').value).toBe('售后服务群');
    expect(await within(drawer).findByRole('button', { name: '取消选择成员 李娜' })).toBeTruthy();
    expect(within(drawer).getByText('已选择 1 个群聊')).toBeTruthy();

    fireEvent.click(within(drawer).getByRole('button', { name: '保存群活码' }));
    await waitFor(() => expect(write).toHaveBeenCalledWith(
      '/workRoomAutoPull/update',
      expect.objectContaining({ workRoomAutoPullId: 21, qrcodeName: '售后服务群' }),
      'PUT',
    ));
    expect(await within(drawer).findByText('企微 Provider 暂不可用')).toBeTruthy();
    expect(screen.getByRole('dialog', { name: '编辑群活码' })).toBeTruthy();
  });

  it('loads real employees and rooms from scoped providers without exposing raw JSON', async () => {
    const read = vi.fn().mockImplementation((path: string) => {
      if (path === '/workEmployee/index') return Promise.resolve({ page: { totalPage: 1 }, list: [{ id: 21, name: '李娜', status: 1 }, { id: 22, name: '王强', status: 1 }] });
      if (path === '/workRoom/roomIndex') return Promise.resolve({ list: [{ roomId: 41, roomName: '客户群一', currentNum: 18, roomMax: 200 }] });
      return Promise.resolve({ list: [] });
    });
    const write = vi.fn().mockResolvedValue(undefined);
    view(<GroupCodePage api={{ read, write }} />);

    await screen.findByRole('heading', { name: '暂无记录' });
    fireEvent.click(screen.getByRole('button', { name: '新建群活码' }));
    expect(await screen.findByRole('button', { name: '选择成员 李娜' })).toBeTruthy();
    expect(screen.queryByLabelText('群聊配置 JSON')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '选择群聊' }));
    const roomDialog = await screen.findByRole('dialog', { name: '选择群聊' });
    expect(await within(roomDialog).findByRole('button', { name: '选择群聊 客户群一' })).toBeTruthy();
    expect(read).toHaveBeenCalledWith('/workEmployee/index', expect.objectContaining({ status: 1, page: 1, perPage: 100 }));
    expect(read).toHaveBeenCalledWith('/workRoom/roomIndex', {});
  });

  it('toggles employee choices with ordinary clicks without replacing earlier selections', async () => {
    const read = vi.fn().mockImplementation((path: string) => Promise.resolve(path === '/workEmployee/index'
      ? { page: { totalPage: 1 }, list: [{ id: 21, name: '李娜', status: 1 }, { id: 22, name: '王强', status: 1 }] }
      : { list: [] }));
    view(<ChannelCodePage api={{ read, write: vi.fn() }} />);

    await screen.findByRole('heading', { name: '暂无记录' });
    fireEvent.click(screen.getByRole('button', { name: '新建渠道活码' }));
    const liNa = await screen.findByRole('button', { name: '选择成员 李娜' });
    const wangQiang = screen.getByRole('button', { name: '选择成员 王强' });

    fireEvent.click(liNa);
    fireEvent.click(wangQiang);
    expect(screen.getByText('已选择 2 人')).toBeTruthy();
    expect(liNa.getAttribute('aria-pressed')).toBe('true');
    expect(wangQiang.getAttribute('aria-pressed')).toBe('true');

    fireEvent.click(screen.getByRole('button', { name: '取消选择成员 李娜' }));
    expect(screen.getByText('已选择 1 人')).toBeTruthy();
    expect(screen.getByRole('button', { name: '选择成员 李娜' }).getAttribute('aria-pressed')).toBe('false');
    expect(screen.getByRole('button', { name: '取消选择成员 王强' }).getAttribute('aria-pressed')).toBe('true');
  });

  it('selects real rooms in a draft and maps confirmed rooms without exposing JSON', async () => {
    const write = vi.fn().mockResolvedValue(undefined);
    const read = vi.fn().mockImplementation((path: string) => {
      if (path === '/workEmployee/index') return Promise.resolve({ page: { totalPage: 1 }, list: [{ id: 21, name: '李娜', status: 1 }] });
      if (path === '/workRoom/roomIndex') return Promise.resolve({ list: [
        { roomId: 41, roomName: '客户群一', currentNum: 18, roomMax: 200 },
        { roomId: 42, roomName: '客户群二', currentNum: 36, roomMax: 180 },
      ] });
      return Promise.resolve({ list: [] });
    });
    view(<GroupCodePage api={{ read, write }} />);

    await screen.findByRole('heading', { name: '暂无记录' });
    fireEvent.click(screen.getByRole('button', { name: '新建群活码' }));
    expect(screen.queryByLabelText('群聊配置 JSON')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '选择群聊' }));
    const roomDialog = await screen.findByRole('dialog', { name: '选择群聊' });
    fireEvent.click(await within(roomDialog).findByRole('button', { name: '选择群聊 客户群一' }));
    expect(within(roomDialog).getByText('已选择 1 个群聊')).toBeTruthy();
    fireEvent.click(within(roomDialog).getByRole('button', { name: '取消' }));
    expect(screen.getByText('尚未选择群聊')).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: '选择群聊' }));
    const reopenedDialog = await screen.findByRole('dialog', { name: '选择群聊' });
    fireEvent.click(await within(reopenedDialog).findByRole('button', { name: '选择群聊 客户群一' }));
    fireEvent.click(within(reopenedDialog).getByRole('button', { name: '选择群聊 客户群二' }));
    fireEvent.click(within(reopenedDialog).getByRole('button', { name: '确认选择' }));
    expect(screen.getByText('已选择 2 个群聊')).toBeTruthy();

    fireEvent.change(screen.getAllByLabelText('群活码名称')[1]!, { target: { value: '售后服务群' } });
    fireEvent.click(screen.getByRole('button', { name: '选择成员 李娜' }));
    fireEvent.change(screen.getByLabelText('入群引导语'), { target: { value: '欢迎入群' } });
    fireEvent.change(screen.getByLabelText('客户标签 ID'), { target: { value: '31' } });
    fireEvent.click(screen.getByRole('button', { name: '保存群活码' }));

    await waitFor(() => expect(write).toHaveBeenCalledWith(
      '/workRoomAutoPull/store',
      expect.objectContaining({ rooms: JSON.stringify([{ roomId: 41, maxNum: 200 }, { roomId: 42, maxNum: 180 }]) }),
      'POST',
    ));
  });

  it('caps selectable rooms at five and disables rooms without synced capacity', async () => {
    const rooms = Array.from({ length: 7 }, (_, index) => ({
      roomId: index + 1,
      roomName: `客户群${index + 1}`,
      currentNum: index,
      roomMax: index === 6 ? 0 : 200,
    }));
    const read = vi.fn().mockImplementation((path: string) => Promise.resolve(path === '/workRoom/roomIndex'
      ? { list: rooms }
      : path === '/workEmployee/index'
        ? { page: { totalPage: 1 }, list: [{ id: 21, name: '李娜', status: 1 }] }
        : { list: [] }));
    view(<GroupCodePage api={{ read, write: vi.fn() }} />);

    await screen.findByRole('heading', { name: '暂无记录' });
    fireEvent.click(screen.getByRole('button', { name: '新建群活码' }));
    fireEvent.click(screen.getByRole('button', { name: '选择群聊' }));
    const dialog = await screen.findByRole('dialog', { name: '选择群聊' });
    await within(dialog).findByRole('button', { name: '选择群聊 客户群1' });
    for (let index = 1; index <= 5; index += 1) {
      fireEvent.click(within(dialog).getByRole('button', { name: `选择群聊 客户群${index}` }));
    }
    expect(within(dialog).getByText('最多选择 5 个群聊')).toBeTruthy();
    expect(within(dialog).getByRole('button', { name: '选择群聊 客户群6' })).toHaveProperty('disabled', true);
    expect(within(dialog).getByRole('button', { name: '选择群聊 客户群7' })).toHaveProperty('disabled', true);
    expect(within(dialog).getByText('容量未同步，暂不可选')).toBeTruthy();
  });

  it('writes an explicitly cleared room draft back to the form', async () => {
    const read = vi.fn().mockImplementation((path: string) => Promise.resolve(path === '/workRoom/roomIndex'
      ? { list: [{ roomId: 41, roomName: '客户群一', currentNum: 18, roomMax: 200 }] }
      : path === '/workEmployee/index'
        ? { page: { totalPage: 1 }, list: [{ id: 21, name: '李娜', status: 1 }] }
        : { list: [] }));
    view(<GroupCodePage api={{ read, write: vi.fn() }} />);

    await screen.findByRole('heading', { name: '暂无记录' });
    fireEvent.click(screen.getByRole('button', { name: '新建群活码' }));
    fireEvent.click(screen.getByRole('button', { name: '选择群聊' }));
    let dialog = await screen.findByRole('dialog', { name: '选择群聊' });
    fireEvent.click(await within(dialog).findByRole('button', { name: '选择群聊 客户群一' }));
    fireEvent.click(within(dialog).getByRole('button', { name: '确认选择' }));
    expect(screen.getByText('已选择 1 个群聊')).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: '选择群聊' }));
    dialog = await screen.findByRole('dialog', { name: '选择群聊' });
    fireEvent.click(within(dialog).getByRole('button', { name: '清空选择' }));
    const confirm = within(dialog).getByRole('button', { name: '确认选择' });
    expect(confirm).toHaveProperty('disabled', false);
    fireEvent.click(confirm);
    expect(screen.getByText('尚未选择群聊')).toBeTruthy();
  });

  it('drops selections that disappear after scoped provider refresh and asks for reselection', async () => {
    let employeeRows = [{ id: 21, name: '李娜', status: 1 }];
    let roomRows = [{ roomId: 41, roomName: '客户群一', currentNum: 18, roomMax: 200 }];
    const read = vi.fn().mockImplementation((path: string) => {
      if (path === '/workEmployee/index') return Promise.resolve({ page: { totalPage: 1 }, list: employeeRows });
      if (path === '/workRoom/roomIndex') return Promise.resolve({ list: roomRows });
      return Promise.resolve({ list: [] });
    });
    const { queryClient } = view(<GroupCodePage api={{ read, write: vi.fn() }} />);

    await screen.findByRole('heading', { name: '暂无记录' });
    fireEvent.click(screen.getByRole('button', { name: '新建群活码' }));
    fireEvent.click(await screen.findByRole('button', { name: '选择成员 李娜' }));
    fireEvent.click(screen.getByRole('button', { name: '选择群聊' }));
    const dialog = await screen.findByRole('dialog', { name: '选择群聊' });
    fireEvent.click(await within(dialog).findByRole('button', { name: '选择群聊 客户群一' }));
    fireEvent.click(within(dialog).getByRole('button', { name: '确认选择' }));

    employeeRows = [];
    roomRows = [];
    await queryClient.invalidateQueries({ queryKey: ['live-code-employees', access.corp.id] });
    await queryClient.invalidateQueries({ queryKey: ['live-code-rooms', access.corp.id] });

    await waitFor(() => {
      expect(screen.getByText('已选择 0 人')).toBeTruthy();
      expect(screen.getByText('尚未选择群聊')).toBeTruthy();
      expect(screen.getByText('部分已选成员已失效，请重新选择。')).toBeTruthy();
      expect(screen.getByText('部分已选群聊已失效，请重新选择。')).toBeTruthy();
    });
  });

  it('starts with a clean create form after cancelling and reopening', async () => {
    const read = vi.fn().mockImplementation((path: string) => Promise.resolve(path === '/workEmployee/index'
      ? { page: { totalPage: 1 }, list: [{ id: 21, name: '李娜', status: 1 }] }
      : { list: [] }));
    view(<ChannelCodePage api={{ read, write: vi.fn() }} />);

    await screen.findByRole('heading', { name: '暂无记录' });
    fireEvent.click(screen.getByRole('button', { name: '新建渠道活码' }));
    fireEvent.change(screen.getByLabelText('渠道活码名称'), { target: { value: '官网咨询' } });
    fireEvent.click(await screen.findByRole('button', { name: '选择成员 李娜' }));
    fireEvent.click(screen.getByRole('button', { name: '取消' }));

    fireEvent.click(screen.getByRole('button', { name: '新建渠道活码' }));
    expect(screen.getByLabelText<HTMLInputElement>('渠道活码名称').value).toBe('');
    expect(screen.getByText('已选择 0 人')).toBeTruthy();
  });

  it('shows an explicit read-only state when granular permissions omit create', async () => {
    view(
      <ChannelCodePage api={{ read: vi.fn().mockResolvedValue({ list: [] }), write: vi.fn() }} />,
      new Set(['/acquisition/v2-channel-code@refresh']),
    );

    await screen.findByRole('heading', { name: '暂无记录' });
    expect(screen.queryByRole('button', { name: '新建渠道活码' })).toBeNull();
    expect(screen.getByText('当前账号仅可查看渠道活码')).toBeTruthy();
  });

  it('loads persisted short links, opens the create drawer, and creates a draft target', async () => {
    const read = vi.fn().mockResolvedValue({
      list: [{ id: 8, name: '群活码短链', token: 'abc123', targetUrl: '/acquisition/group-code', status: 'active', visitTotal: 4 }],
    });
    const write = vi.fn().mockResolvedValue(undefined);
    const api: BusinessWorkbenchApi = { read, write };
    view(<LiveCodeShortChainPage api={api} />);

    expect(await screen.findByText('群活码短链')).toBeTruthy();
    expect(read).toHaveBeenCalledWith('/liveCodeShortChain/index', expect.objectContaining({ page: 1, perPage: 20 }));
    fireEvent.click(screen.getByRole('button', { name: '创建短链' }));
    expect(screen.getByLabelText('创建短链')).toBeTruthy();
    fireEvent.change(screen.getAllByLabelText('短链名称')[1]!, { target: { value: '群活码短链' } });
    fireEvent.change(screen.getByLabelText('目标地址'), { target: { value: '/acquisition/group-code' } });
    fireEvent.click(screen.getByRole('button', { name: '保存短链' }));

    await waitFor(() => expect(write).toHaveBeenCalledWith(
      '/liveCodeShortChain/store',
      { name: '群活码短链', targetUrl: '/acquisition/group-code', targetType: 'url' },
      'POST',
    ));
  });

  it('disables a persisted short link through the provider', async () => {
    const read = vi.fn().mockResolvedValue({
      list: [{ id: 8, name: '群活码短链', token: 'abc123', targetUrl: '/acquisition/group-code', status: 'active', visitTotal: 4 }],
    });
    const write = vi.fn().mockResolvedValue(undefined);
    view(<LiveCodeShortChainPage api={{ read, write }} />);

    await screen.findByText('群活码短链');
    fireEvent.click(screen.getByRole('button', { name: '停用' }));
    expect(write).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));
    await waitFor(() => expect(write).toHaveBeenCalledWith('/liveCodeShortChain/disable', { id: 8 }, 'POST'));
  });
});
