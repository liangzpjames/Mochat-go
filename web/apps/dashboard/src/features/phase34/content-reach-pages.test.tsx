import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import { FriendsCirclePage, PreciseGroupSendPage } from './content-reach-pages';

const access: AccessContext = {
  session: { token: 'token', userId: '1', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set(['/acquisition/precise-group-send', '/acquisition/friends-circle']),
  allowedActions: new Set(),
};

afterEach(() => { cleanup(); vi.restoreAllMocks(); });

beforeAll(() => { vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} }); });

function view(page: React.ReactNode) {
  return render(
    <MemoryRouter>
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <DashboardAccessProvider value={access}>{page}</DashboardAccessProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

// The drawer's multi-select onChange reads event.target.selectedOptions.
// fireEvent.change assigns `value` as a plain data property (bypassing the
// native setter), which does not update option selectedness, so we set each
// option's selected flag directly before dispatching the change event.
function changeMultiSelectValue(select: HTMLElement, value: string): void {
  for (const option of Array.from(select.querySelectorAll('option'))) {
    option.selected = option.getAttribute('value') === value;
  }
  fireEvent.change(select);
}

// Multi-value variant of changeMultiSelectValue: selects every option whose
// value is listed (and deselects the rest) before dispatching a single change.
function changeMultiSelectValues(select: HTMLElement, values: string[]): void {
  for (const option of Array.from(select.querySelectorAll('option'))) {
    option.selected = values.includes(option.getAttribute('value') ?? '');
  }
  fireEvent.change(select);
}

describe('Phase 3.4 content-reach pages', () => {
  it('queries both real precise-send providers and applies the task-title filter', async () => {
    const read = vi.fn().mockResolvedValue({ list: [{ id: 8, batchTitle: '夏日活动', content: [{ type: 'text', content: '欢迎参与' }], sendStatus: 1, sendTotal: 20, receivedTotal: 12, createdAt: '2026-08-04 10:00' }] });
    view(<PreciseGroupSendPage api={{ read, write: vi.fn() }} />);

    expect(await screen.findByText('欢迎参与')).toBeTruthy();
    const contactTab = screen.getByRole('tab', { name: '客户群发' });
    expect(contactTab.getAttribute('aria-controls')).toBe('precise-send-panel');
    expect(screen.getByRole('tabpanel').getAttribute('id')).toBe('precise-send-panel');
    expect(read).toHaveBeenCalledWith('/contactMessageBatchSend/index', expect.objectContaining({ page: 1, perPage: 20 }));
    expect(screen.getByRole('button', { name: '新建群发' })).toHaveProperty('disabled', false);
    fireEvent.change(screen.getByLabelText('任务名称'), { target: { value: '夏日' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(read).toHaveBeenLastCalledWith('/contactMessageBatchSend/index', expect.objectContaining({ batchTitle: '夏日' })));

    fireEvent.click(screen.getByRole('tab', { name: '群聊群发' }));
    await waitFor(() => expect(read).toHaveBeenLastCalledWith('/roomMessageBatchSend/index', expect.objectContaining({ page: 1, perPage: 20 })));
  });

  it('requires member IDs for customer sends and keeps external provider status explicit', async () => {
    const read = vi.fn().mockImplementation((path: string) => Promise.resolve(path === '/workEmployee/index'
      ? { list: [{ id: 999999, name: '测试员工', departmentName: '销售部' }] }
      : path === '/workContact/index' ? { list: [{ id: 501, contactId: 501, employeeId: 999999, name: '测试客户', employeeName: '测试员工' }] } : { list: [] }));
    const write = vi.fn().mockRejectedValue(new Error('客户群发 Provider 返回 422：员工无效'));
    view(<PreciseGroupSendPage api={{ read, write }} />);

    await screen.findByRole('heading', { name: '暂无群发任务' });
    expect(screen.getByText('本地任务 Provider 已连接 · 外部发送待配置')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '新建群发' }));
    fireEvent.change(screen.getAllByLabelText('任务名称')[1]!, { target: { value: '客户空成员校验' } });
    fireEvent.change(screen.getByLabelText('群发内容'), { target: { value: '客户触达内容' } });

    const submit = screen.getByRole('button', { name: '保存并发送' });
    expect(submit).toHaveProperty('disabled', true);
    fireEvent.click(submit);
    expect(write).not.toHaveBeenCalled();

    await screen.findByRole('option', { name: /测试员工/ });
    changeMultiSelectValue(screen.getByLabelText('发送成员选择'), '999999');
    fireEvent.change(screen.getByLabelText('搜索客户'), { target: { value: '测试' } });
    await waitFor(() => expect(read).toHaveBeenCalledWith('/workContact/index', expect.objectContaining({ keyWords: '测试', employeeId: '999999' })));
    await screen.findByRole('option', { name: /测试客户/ });
    changeMultiSelectValue(screen.getByLabelText('客户选择'), '999999:501');
    await waitFor(() => expect(submit).toHaveProperty('disabled', false));
    fireEvent.click(submit);
    expect(write).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));
    expect((await screen.findByRole('alert')).textContent).toContain('客户群发 Provider 返回 422');
  });

  it('requires a real group owner ID for room sends and renders the room provider error in the drawer', async () => {
    const read = vi.fn().mockImplementation((path: string) => Promise.resolve(path === '/workEmployee/index'
      ? { list: [{ id: 999999, name: '测试群主', departmentName: '销售部' }] }
      : path === '/workRoom/index' ? { list: [{ id: 601, roomId: 601, ownerId: 999999, name: '测试群' }] } : { list: [] }));
    const write = vi.fn().mockRejectedValue(new Error('群聊群发 Provider 返回 422：群主无效'));
    view(<PreciseGroupSendPage api={{ read, write }} />);

    await screen.findByRole('heading', { name: '暂无群发任务' });
    fireEvent.click(screen.getByRole('tab', { name: '群聊群发' }));
    await screen.findByRole('heading', { name: '暂无群发任务' });
    fireEvent.click(screen.getByRole('button', { name: '新建群发' }));
    fireEvent.change(screen.getAllByLabelText('任务名称')[1]!, { target: { value: '群聊空群主校验' } });
    fireEvent.change(screen.getByLabelText('群发内容'), { target: { value: '群聊触达内容' } });

    const submit = screen.getByRole('button', { name: '保存并发送' });
    expect(screen.getByLabelText('群主选择')).toBeTruthy();
    expect(submit).toHaveProperty('disabled', true);
    await screen.findByRole('option', { name: /测试群主/ });
    changeMultiSelectValue(screen.getByLabelText('群主选择'), '999999');
    await waitFor(() => expect(read).toHaveBeenCalledWith('/workRoom/index', expect.objectContaining({ workRoomOwnerId: '999999' })));
    await screen.findByRole('option', { name: '测试群' });
    changeMultiSelectValue(screen.getByLabelText('群聊选择'), '999999:601');
    await waitFor(() => expect(submit).toHaveProperty('disabled', false));
    fireEvent.click(submit);
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));
    expect((await screen.findByRole('alert')).textContent).toContain('群聊群发 Provider 返回 422');
    const roomCreatePayload: Record<string, unknown> = {
      batchTitle: '群聊空群主校验', employeeIds: [999999],
      roomTargets: [{ ownerEmployeeId: 999999, roomId: 601 }],
      idempotencyKey: expect.any(String), sendWay: 1,
      content: [{ msgType: 'text', content: '群聊触达内容' }],
    };
    await waitFor(() => expect(write).toHaveBeenCalledWith(
      '/roomMessageBatchSend/store',
      expect.objectContaining(roomCreatePayload),
      'POST',
    ));
  });

  it('loads rooms per selected owner and keys each room target with its owning employee id', async () => {
    const read = vi.fn().mockImplementation((path: string, query: Record<string, string | number> = {}) => {
      if (path === '/workEmployee/index') {
        return Promise.resolve({ list: [
          { id: 999999, name: '测试群主', departmentName: '销售部' },
          { id: 888888, name: '二群主', departmentName: '销售部' },
        ] });
      }
      if (path === '/workRoom/index') {
        return Promise.resolve({ list: String(query.workRoomOwnerId) === '888888'
          ? [{ id: 602, roomId: 602, name: '二群' }]
          : [{ id: 601, roomId: 601, name: '测试群' }] });
      }
      return Promise.resolve({ list: [] });
    });
    const write = vi.fn().mockResolvedValue(undefined);
    view(<PreciseGroupSendPage api={{ read, write }} />);

    await screen.findByRole('heading', { name: '暂无群发任务' });
    fireEvent.click(screen.getByRole('tab', { name: '群聊群发' }));
    await screen.findByRole('heading', { name: '暂无群发任务' });
    fireEvent.click(screen.getByRole('button', { name: '新建群发' }));
    fireEvent.change(screen.getAllByLabelText('任务名称')[1]!, { target: { value: '双群主群发' } });
    fireEvent.change(screen.getByLabelText('群发内容'), { target: { value: '双群主触达内容' } });

    const submit = screen.getByRole('button', { name: '保存并发送' });
    expect(submit).toHaveProperty('disabled', true);
    await screen.findByRole('option', { name: /测试群主/ });
    changeMultiSelectValues(screen.getByLabelText('群主选择'), ['999999', '888888']);
    await waitFor(() => expect(read).toHaveBeenCalledWith('/workRoom/index', expect.objectContaining({ workRoomOwnerId: '999999' })));
    await waitFor(() => expect(read).toHaveBeenCalledWith('/workRoom/index', expect.objectContaining({ workRoomOwnerId: '888888' })));
    await screen.findByRole('option', { name: '测试群' });
    await screen.findByRole('option', { name: '二群' });
    changeMultiSelectValues(screen.getByLabelText('群聊选择'), ['999999:601', '888888:602']);
    await waitFor(() => expect(submit).toHaveProperty('disabled', false));
    fireEvent.click(submit);
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));

    const multiOwnerPayload: Record<string, unknown> = {
      batchTitle: '双群主群发', employeeIds: [999999, 888888],
      roomTargets: [
        { ownerEmployeeId: 999999, roomId: 601 },
        { ownerEmployeeId: 888888, roomId: 602 },
      ],
      idempotencyKey: expect.any(String), sendWay: 1,
      content: [{ msgType: 'text', content: '双群主触达内容' }],
    };
    await waitFor(() => expect(write).toHaveBeenCalledWith(
      '/roomMessageBatchSend/store',
      expect.objectContaining(multiOwnerPayload),
      'POST',
    ));
  });

  it('creates an immediate customer precise-send task through the real provider', async () => {
    const read = vi.fn().mockImplementation((path: string) => Promise.resolve(path === '/workEmployee/index'
      ? { list: [{ id: 99, name: '测试员工' }] }
      : path === '/workContact/index' ? { list: [{ id: 301, contactId: 301, employeeId: 99, name: '测试客户' }] } : { list: [] }));
    const write = vi.fn().mockResolvedValue(undefined);
    view(<PreciseGroupSendPage api={{ read, write }} />);

    await screen.findByRole('heading', { name: '暂无群发任务' });
    fireEvent.click(screen.getByRole('button', { name: '新建群发' }));
    expect(screen.getByLabelText('新建群发任务')).toBeTruthy();
    fireEvent.change(screen.getAllByLabelText('任务名称')[1]!, { target: { value: '夏日客户触达' } });
    await screen.findByRole('option', { name: /测试员工/ });
    changeMultiSelectValue(screen.getByLabelText('发送成员选择'), '99');
    await screen.findByRole('option', { name: /测试客户/ });
    changeMultiSelectValue(screen.getByLabelText('客户选择'), '99:301');
    fireEvent.change(screen.getByLabelText('群发内容'), { target: { value: '欢迎参与夏日活动' } });
    fireEvent.click(screen.getByRole('button', { name: '保存并发送' }));
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));

    const createPayload: Record<string, unknown> = {
      batchTitle: '夏日客户触达', employeeIds: [99], sendWay: 1, filterParams: {},
      contactTargets: [{ employeeId: 99, contactId: 301 }], idempotencyKey: expect.any(String),
      content: [{ msgType: 'text', content: '欢迎参与夏日活动' }],
    };
    await waitFor(() => expect(write).toHaveBeenCalledWith(
      '/contactMessageBatchSend/store',
      expect.objectContaining(createPayload),
      'POST',
    ));
  });

  it('persists a selected material reference when creating a precise-send task', async () => {
    const read = vi.fn().mockImplementation((endpoint: string) => endpoint === '/workEmployee/index'
      ? Promise.resolve({ list: [{ id: 99, name: '测试员工' }] })
      : endpoint === '/workContact/index' ? Promise.resolve({ list: [{ id: 301, contactId: 301, employeeId: 99, name: '测试客户' }] })
      : endpoint === '/materialSelector/index'
      ? Promise.resolve({ list: [{ id: 45, name: '群发话术', preview: '欢迎咨询' }] })
      : Promise.resolve({ list: [] }));
    const write = vi.fn().mockResolvedValue(undefined);
    view(<PreciseGroupSendPage api={{ read, write }} />);

    await screen.findByRole('heading', { name: '暂无群发任务' });
    fireEvent.click(screen.getByRole('button', { name: '新建群发' }));
    fireEvent.change(screen.getAllByLabelText('任务名称')[1]!, { target: { value: '引用素材群发' } });
    await screen.findByRole('option', { name: /测试员工/ });
    changeMultiSelectValue(screen.getByLabelText('发送成员选择'), '99');
    await screen.findByRole('option', { name: /测试客户/ });
    changeMultiSelectValue(screen.getByLabelText('客户选择'), '99:301');
    await screen.findByRole('option', { name: '群发话术' });
    fireEvent.change(screen.getByRole('combobox', { name: '引用素材' }), { target: { value: '45' } });
    fireEvent.click(screen.getByRole('button', { name: '保存并发送' }));
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));

    await waitFor(() => expect(write).toHaveBeenCalledWith(
      '/contactMessageBatchSend/store',
      expect.objectContaining({ mediumId: 45, content: [{ msgType: 'text', content: '欢迎咨询' }] }),
      'POST',
    ));
  });

  it('shows execution data, details, and retries a failed precise-send query', async () => {
    const read = vi.fn().mockRejectedValueOnce(new Error('provider unavailable')).mockResolvedValueOnce({ list: [{ id: 8, content: [{ content: '恢复后的任务' }], sendTotal: 20, receivedTotal: 12 }] });
    view(<PreciseGroupSendPage api={{ read, write: vi.fn() }} />);

    await screen.findByRole('heading', { name: '加载失败' });
    fireEvent.click(screen.getByRole('button', { name: '重新加载' }));
    expect(await screen.findByText('恢复后的任务')).toBeTruthy();
    expect(screen.getByText('20 / 12')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '详情' }));
    await waitFor(() => expect(screen.getByLabelText('群发任务详情').textContent).toContain('执行数据'));
  });

  it('keeps key controls and table reachable at a 390px viewport with local scroll', async () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 });
    const read = vi.fn().mockResolvedValue({ list: [{ id: 8, batchTitle: '夏日活动', content: [{ type: 'text', content: '欢迎参与' }], sendStatus: 1, sendTotal: 20, receivedTotal: 12, createdAt: '2026-08-04 10:00' }] });
    view(<PreciseGroupSendPage api={{ read, write: vi.fn() }} />);

    await screen.findByText('欢迎参与');
    expect(screen.getByRole('button', { name: '新建群发' })).toBeTruthy();
    expect(screen.getByRole('button', { name: '详情' })).toBeTruthy();
    expect(screen.getByLabelText('任务名称')).toBeTruthy();
    // The results table must be inside a local scroll container so the
    // 720px-min table never forces document-level horizontal overflow.
    expect(screen.getByRole('table').closest('.dashboard-table-scroll')).toBeTruthy();
    expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(390);

    fireEvent.click(screen.getByRole('button', { name: '新建群发' }));
    expect(screen.getByLabelText('发送成员选择')).toBeTruthy();
    expect(screen.getByLabelText('群发内容')).toBeTruthy();
    expect(screen.getByRole('button', { name: '保存并发送' })).toBeTruthy();
  });

  it('queries real friends-circle tasks and materials and persists a draft', async () => {
    const read = vi.fn()
      .mockResolvedValueOnce({ list: [{ id: 21, taskName: '夏日朋友圈', sendWay: 'manual', status: 'draft', completedTotal: 0, targetTotal: 8, creatorName: '运营员', createdAt: '2026-08-04 14:00' }] })
      .mockResolvedValueOnce({ list: [{ id: 31, name: '新品海报', content: '{"text":"新品正文摘要"}', type: 'image', status: 'available', creatorName: '运营员', createdAt: '2026-08-04 14:10' }] })
      .mockResolvedValue({ list: [] });
    const write = vi.fn().mockResolvedValue(undefined);
    view(<FriendsCirclePage api={{ read, write }} />);

    expect(screen.getByRole('tab', { name: '朋友圈' }).getAttribute('aria-controls')).toBe('friends-circle-panel');
    expect(screen.getByRole('tab', { name: '朋友圈素材' })).toBeTruthy();
    expect(screen.getByRole('tabpanel').getAttribute('id')).toBe('friends-circle-panel');
    expect(await screen.findByText('夏日朋友圈')).toBeTruthy();
    expect(read).toHaveBeenCalledWith('/friendsCircle/taskIndex', expect.objectContaining({ page: 1, perPage: 20 }));
    expect(screen.getByLabelText('任务名称')).toBeTruthy();
    expect(screen.getByText('发送方式')).toBeTruthy();
    expect(screen.getByText('完成情况')).toBeTruthy();
    fireEvent.click(screen.getByRole('tab', { name: '朋友圈素材' }));
    expect(await screen.findByText('新品海报')).toBeTruthy();
    expect(screen.getByText('新品正文摘要')).toBeTruthy();
    expect(read).toHaveBeenLastCalledWith('/friendsCircle/materialIndex', expect.objectContaining({ page: 1, perPage: 20 }));
    fireEvent.click(screen.getByRole('tab', { name: '朋友圈' }));
    fireEvent.click(screen.getByRole('button', { name: '添加朋友圈' }));
    fireEvent.change(screen.getByLabelText('草稿名称'), { target: { value: '秋日活动' } });
    fireEvent.change(screen.getByLabelText('草稿内容'), { target: { value: '欢迎参与' } });
    fireEvent.click(screen.getByRole('button', { name: '保存草稿' }));
    await waitFor(() => expect(write).toHaveBeenCalledWith('/friendsCircle/taskStore', expect.objectContaining({ taskName: '秋日活动', content: '欢迎参与', sendWay: 'manual' })));
    expect(screen.getByRole('button', { name: '导出' })).toHaveProperty('disabled', true);
    expect(screen.getByText('发布 Provider 未配置')).toBeTruthy();
  });

  it('reuses a real material provider in the friends-circle composer', async () => {
    const read = vi.fn().mockImplementation((endpoint: string) => endpoint === '/materialSelector/index'
      ? Promise.resolve({ list: [{ id: 45, name: '新品文案', preview: '新品今天上线' }] })
      : Promise.resolve({ list: [] }));
    view(<FriendsCirclePage api={{ read, write: vi.fn() }} />);

    await screen.findByRole('heading', { name: '还没有朋友圈任务' });
    fireEvent.click(screen.getByRole('button', { name: '添加朋友圈' }));
    expect(await screen.findByRole('option', { name: '新品文案' })).toBeTruthy();
    fireEvent.change(screen.getByLabelText('引用素材'), { target: { value: '45' } });
    expect(screen.getByLabelText('草稿内容')).toHaveProperty('value', '新品今天上线');
    expect(read).toHaveBeenCalledWith('/materialSelector/index', { scene: 'friends_circle' });
  });

  it('persists the selected material ID in a friends-circle task draft', async () => {
    const read = vi.fn().mockImplementation((endpoint: string) => endpoint === '/materialSelector/index'
      ? Promise.resolve({ list: [{ id: 45, name: '朋友圈话术', preview: '本周新品已上线' }] })
      : Promise.resolve({ list: [] }));
    const write = vi.fn().mockResolvedValue(undefined);
    view(<FriendsCirclePage api={{ read, write }} />);

    await screen.findByRole('heading', { name: '还没有朋友圈任务' });
    fireEvent.click(screen.getByRole('button', { name: '添加朋友圈' }));
    await screen.findByRole('option', { name: '朋友圈话术' });
    fireEvent.change(screen.getByRole('combobox', { name: '引用素材' }), { target: { value: '45' } });
    fireEvent.change(screen.getByLabelText('草稿名称'), { target: { value: '引用素材朋友圈' } });
    fireEvent.click(screen.getByRole('button', { name: '保存草稿' }));

    await waitFor(() => expect(write).toHaveBeenCalledWith(
      '/friendsCircle/taskStore',
      expect.objectContaining({ mediumId: 45, taskName: '引用素材朋友圈', content: '本周新品已上线', sendWay: 'manual' }),
    ));
  });

  it('shows persistent publish failure, loads target failures, and exports scoped results', async () => {
    const read = vi.fn().mockImplementation((endpoint: string) => {
      if (endpoint === '/friendsCircle/taskResultIndex') return Promise.resolve({ list: [{ id: 41, taskId: 21, targetEmployeeId: 99, status: 'failed', failureCode: 'E_TIMEOUT', failureReason: '发送超时', occurredAt: '2026-08-04 15:00' }] });
      if (endpoint === '/friendsCircle/exportData') return Promise.resolve({ list: [{ taskId: 21, targetEmployeeId: 99, status: 'failed', failureCode: 'E_TIMEOUT', failureReason: '发送超时', occurredAt: '2026-08-04 15:00' }] });
      return Promise.resolve({ list: [{ id: 21, taskName: '夏日朋友圈', sendWay: 'manual', status: 'draft', completedTotal: 0, targetTotal: 1, creatorName: '运营员', createdAt: '2026-08-04 14:00' }] });
    });
    const write = vi.fn().mockRejectedValue(new Error('朋友圈发布 Provider 未配置'));
    view(<FriendsCirclePage api={{ read, write }} />);

    expect(await screen.findByText('夏日朋友圈')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '发起发布' }));
    expect((await screen.findByRole('alert')).textContent).toContain('朋友圈发布 Provider 未配置');
    fireEvent.click(screen.getByRole('button', { name: '查看进度' }));
    expect(await screen.findByLabelText('朋友圈任务进度')).toBeTruthy();
    expect(await screen.findByText('E_TIMEOUT')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '导出失败明细' }));
    await waitFor(() => expect(read).toHaveBeenCalledWith('/friendsCircle/exportData', { taskId: 21, status: 'failed' }));
  });

  it('keeps friends-circle actions inside their task rows and binds each action to its task ID', async () => {
    const tasks = [
      { id: 21, taskName: '草稿任务', sendWay: 'manual', status: 'draft', completedTotal: 0, targetTotal: 1, creatorName: '运营员', createdAt: '2026-08-04 14:00' },
      { id: 22, taskName: '失败任务', sendWay: 'manual', status: 'failed', completedTotal: 0, targetTotal: 2, creatorName: '运营员', createdAt: '2026-08-04 14:05' },
    ];
    const read = vi.fn().mockImplementation((endpoint: string) => {
      if (endpoint === '/friendsCircle/taskResultIndex') return Promise.resolve({ list: [] });
      return Promise.resolve({ list: tasks });
    });
    const write = vi.fn().mockResolvedValue(undefined);
    view(<FriendsCirclePage api={{ read, write }} />);

    expect(await screen.findByText('草稿任务')).toBeTruthy();
    const draftRow = screen.getByRole('row', { name: /草稿任务/ });
    const failedRow = screen.getByRole('row', { name: /失败任务/ });
    expect(within(draftRow).getByRole('button', { name: '查看进度' })).toBeTruthy();
    expect(within(draftRow).getByRole('button', { name: '发起发布' })).toBeTruthy();
    expect(within(failedRow).getByRole('button', { name: '查看进度' })).toBeTruthy();
    expect(within(failedRow).queryByRole('button', { name: '发起发布' })).toBeNull();

    fireEvent.click(within(failedRow).getByRole('button', { name: '查看进度' }));
    expect(await screen.findByLabelText('朋友圈任务进度')).toBeTruthy();
    await waitFor(() => expect(read).toHaveBeenCalledWith('/friendsCircle/taskResultIndex', expect.objectContaining({ taskId: 22 })));
    fireEvent.click(screen.getByRole('button', { name: '关闭朋友圈任务进度' }));

    fireEvent.click(within(draftRow).getByRole('button', { name: '发起发布' }));
    await waitFor(() => expect(write).toHaveBeenCalledWith('/friendsCircle/publish', { taskId: 21 }, 'POST'));
    expect(screen.queryAllByRole('button', { name: '查看进度' })).toHaveLength(2);
  });

  it('isolates friends-circle query cache when the selected corp changes', async () => {
    const read = vi.fn().mockResolvedValueOnce({ list: [{ id: 1, taskName: 'corp-seven-draft' }] }).mockResolvedValueOnce({ list: [{ id: 2, taskName: 'corp-eight-draft' }] });
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const page = (value: AccessContext) => (
      <MemoryRouter>
        <QueryClientProvider client={client}>
          <DashboardAccessProvider value={value}><FriendsCirclePage api={{ read, write: vi.fn() }} /></DashboardAccessProvider>
        </QueryClientProvider>
      </MemoryRouter>
    );
    const rendered = render(page(access));
    expect(await screen.findByText('corp-seven-draft')).toBeTruthy();
    rendered.rerender(page({ ...access, session: { ...access.session }, corp: { ...access.corp, id: '8', name: 'corp-eight' } }));
    expect(await screen.findByText('corp-eight-draft')).toBeTruthy();
    expect(read).toHaveBeenCalledTimes(2);
  });

  it('supports keyboard filtering and protects unsaved drawer content', async () => {
    const read = vi.fn().mockResolvedValue({ list: [] });
    view(<FriendsCirclePage api={{ read, write: vi.fn() }} />);

    await screen.findByRole('heading', { name: '还没有朋友圈任务' });
    const reset = screen.getByRole('button', { name: '重置' });
    expect(reset).toHaveProperty('disabled', true);
    fireEvent.change(screen.getByLabelText('任务名称'), { target: { value: '秋日' } });
    fireEvent.keyDown(screen.getByLabelText('任务名称'), { key: 'Enter' });
    await waitFor(() => expect(read).toHaveBeenLastCalledWith('/friendsCircle/taskIndex', expect.objectContaining({ taskName: '秋日' })));

    const addButton = screen.getByRole('button', { name: '添加朋友圈' });
    addButton.focus();
    fireEvent.click(addButton);
    expect(screen.getByLabelText('朋友圈草稿').getAttribute('data-variant')).toBe('task');
    expect(document.activeElement).toBe(screen.getByLabelText('草稿名称'));
    expect(screen.getByLabelText('草稿名称').hasAttribute('required')).toBe(true);
    expect(screen.getByText('0 / 500')).toBeTruthy();
    fireEvent.change(screen.getByLabelText('草稿名称'), { target: { value: '未保存草稿' } });
    fireEvent.click(screen.getByRole('button', { name: '关闭新增面板' }));
    const discardDialog = await screen.findByRole('dialog', { name: '放弃未保存内容？' });
    expect(screen.getByLabelText('朋友圈草稿')).toBeTruthy();
    fireEvent.click(within(discardDialog).getByRole('button', { name: '取消' }));
    expect(screen.getByLabelText('朋友圈草稿')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '关闭新增面板' }));
    fireEvent.click(await screen.findByRole('button', { name: '放弃更改' }));
    expect(screen.queryByLabelText('朋友圈草稿')).toBeNull();
    expect(document.activeElement).toBe(addButton);

    fireEvent.click(screen.getByRole('button', { name: '添加朋友圈' }));
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByLabelText('朋友圈草稿')).toBeNull();
  });

  it('locks composer fields while a draft save is in flight', async () => {
    let finishSave: (() => void) | undefined;
    const write = vi.fn().mockImplementation(() => new Promise<void>((resolve) => { finishSave = resolve; }));
    view(<FriendsCirclePage api={{ read: vi.fn().mockResolvedValue({ list: [] }), write }} />);

    await screen.findByRole('heading', { name: '还没有朋友圈任务' });
    fireEvent.click(screen.getByRole('button', { name: '添加朋友圈' }));
    fireEvent.change(screen.getByLabelText('草稿名称'), { target: { value: '保存中草稿' } });
    fireEvent.change(screen.getByLabelText('草稿内容'), { target: { value: '保存期间不可编辑' } });
    fireEvent.click(screen.getByRole('button', { name: '保存草稿' }));

    await waitFor(() => expect(write).toHaveBeenCalledOnce());
    expect(screen.getByLabelText('草稿名称')).toHaveProperty('disabled', true);
    expect(screen.getByLabelText('草稿内容')).toHaveProperty('disabled', true);
    expect(fireEvent.keyDown(document, { key: 'Tab' })).toBe(false);
    expect(document.activeElement).toBe(screen.getByRole('dialog', { name: '添加朋友圈草稿' }));
    finishSave?.();
    await waitFor(() => expect(screen.queryByLabelText('朋友圈草稿')).toBeNull());
  });
});
