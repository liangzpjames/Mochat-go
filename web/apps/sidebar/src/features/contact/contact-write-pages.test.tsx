import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { ContactEditPage } from './contact-edit-page';
import { ContactRemarkPage } from './contact-remark-page';
import { ContactTagPage } from './contact-tag-page';
import type { ContactRequest } from './contact-api';

afterEach(cleanup);

const summary = { id: 11, name: '林晓', avatar: null, corpId: 3 };
const workspace = {
  name: '林晓', avatar: null, gender: 2, genderText: '女', businessNo: 'C-11',
  remark: '旧备注', description: '', tag: [{ tagId: 7, tagName: '已有标签' }],
  roomName: [], employeeName: [],
};

function frame(node: React.ReactNode) {
  return render(<MemoryRouter initialEntries={['/contact/remark?wxExternalUserid=external-1&agentId=7']}>{node}</MemoryRouter>);
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, reject, resolve };
}

describe('customer write pages', () => {
  it('offers an explicit cancel path without persisting on every write page', async () => {
    const remarkDone = vi.fn();
    frame(<ContactRemarkPage request={vi.fn().mockResolvedValueOnce(summary).mockResolvedValueOnce(workspace)} onDone={remarkDone} onReauthenticate={vi.fn()} />);
    fireEvent.click(await screen.findByRole('button', { name: '取消' }));
    expect(remarkDone).toHaveBeenCalledTimes(1);
    cleanup();

    const tagDone = vi.fn();
    frame(<ContactTagPage request={vi.fn().mockResolvedValueOnce(summary).mockResolvedValueOnce(workspace).mockResolvedValueOnce([]).mockResolvedValueOnce([])} onDone={tagDone} onReauthenticate={vi.fn()} />);
    fireEvent.click(await screen.findByRole('button', { name: '取消' }));
    expect(tagDone).toHaveBeenCalledTimes(1);
    cleanup();

    const editDone = vi.fn();
    frame(<ContactEditPage request={vi.fn().mockResolvedValueOnce(summary).mockResolvedValueOnce([{ contactFieldId: 31, contactFieldPivotId: '', name: '城市', type: 0, typeText: '文本', options: [], value: '上海' }])} onDone={editDone} onReauthenticate={vi.fn()} />);
    fireEvent.click(await screen.findByRole('button', { name: '取消' }));
    expect(editDone).toHaveBeenCalledTimes(1);
  });

  it('retries a failed remark load without leaving the page', async () => {
    const request = vi.fn()
      .mockRejectedValueOnce(new Error('网络暂不可用'))
      .mockResolvedValueOnce(summary)
      .mockResolvedValueOnce(workspace);
    frame(<ContactRemarkPage request={request} onDone={vi.fn()} onReauthenticate={vi.fn()} />);

    fireEvent.click(await screen.findByRole('button', { name: '重试' }));
    expect(await screen.findByDisplayValue('旧备注')).not.toBeNull();
  });

  it('loads, validates and persists a remark once', async () => {
    const request = vi.fn()
      .mockResolvedValueOnce(summary)
      .mockResolvedValueOnce(workspace)
      .mockResolvedValue([]);
    const onDone = vi.fn();
    frame(<ContactRemarkPage request={request} onDone={onDone} onReauthenticate={vi.fn()} />);

    const input = await screen.findByLabelText('备注名');
    expect((input as HTMLInputElement).value).toBe('旧备注');
    fireEvent.change(input, { target: { value: '' } });
    fireEvent.click(screen.getByRole('button', { name: '保存备注' }));
    expect(screen.getByRole('alert').textContent).toContain('请输入 1 至 10 个字符');
    fireEvent.change(input, { target: { value: '新备注' } });
    fireEvent.click(screen.getByRole('button', { name: '保存备注' }));
    fireEvent.click(screen.getByRole('button', { name: '保存中' }));

    await waitFor(() => expect(onDone).toHaveBeenCalledTimes(1));
    expect(request.mock.calls.filter(([path]) => path === '/workContact/update')).toHaveLength(1);
  });

  it('keeps a locally saved remark on screen until WeCom retry succeeds', async () => {
    const request = vi.fn()
      .mockResolvedValueOnce(summary)
      .mockResolvedValueOnce(workspace)
      .mockResolvedValueOnce({ savedLocally: true, wecomSynced: false, retryable: true })
      .mockResolvedValueOnce({ savedLocally: true, wecomSynced: true, retryable: false });
    const onDone = vi.fn();
    frame(<ContactRemarkPage request={request} onDone={onDone} onReauthenticate={vi.fn()} />);

    const input = await screen.findByLabelText('备注名');
    fireEvent.change(input, { target: { value: '新备注' } });
    fireEvent.click(screen.getByRole('button', { name: '保存备注' }));

    expect((await screen.findByRole('alert')).textContent).toContain('本地已保存');
    expect(onDone).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: '重试同步' }));

    await waitFor(() => expect(onDone).toHaveBeenCalledTimes(1));
    expect(request.mock.calls.filter(([path]) => path === '/workContact/update')).toHaveLength(2);
  });

  it('locks existing tags and appends selected persisted tags', async () => {
    const request = vi.fn()
      .mockResolvedValueOnce(summary)
      .mockResolvedValueOnce(workspace)
      .mockResolvedValueOnce([{ groupId: 3, groupName: '阶段' }])
      .mockResolvedValueOnce([{ id: 7, name: '已有标签' }, { id: 9, name: '新标签' }])
      .mockResolvedValue([]);
    const onDone = vi.fn();
    frame(<ContactTagPage request={request} onDone={onDone} onReauthenticate={vi.fn()} />);

    expect(await screen.findByText('已有标签（已存在）')).not.toBeNull();
    fireEvent.click(screen.getByRole('checkbox', { name: '新标签' }));
    fireEvent.click(screen.getByRole('button', { name: '保存标签' }));

    await waitFor(() => expect(onDone).toHaveBeenCalledTimes(1));
    const update = request.mock.calls.find(([path]) => path === '/workContact/update');
    expect(update?.[1]).toMatchObject({ body: JSON.stringify({ contactId: 11, tag: [9] }) });
  });

  it('retries WeCom tag sync without losing the locally saved selection', async () => {
    const request = vi.fn()
      .mockResolvedValueOnce(summary)
      .mockResolvedValueOnce(workspace)
      .mockResolvedValueOnce([{ groupId: 3, groupName: '阶段' }])
      .mockResolvedValueOnce([{ id: 7, name: '已有标签' }, { id: 9, name: '新标签' }])
      .mockResolvedValueOnce({ savedLocally: true, wecomSynced: false, retryable: true })
      .mockResolvedValueOnce({ savedLocally: true, wecomSynced: true, retryable: false });
    const onDone = vi.fn();
    frame(<ContactTagPage request={request} onDone={onDone} onReauthenticate={vi.fn()} />);

    fireEvent.click(await screen.findByRole('checkbox', { name: '新标签' }));
    fireEvent.click(screen.getByRole('button', { name: '保存标签' }));
    expect((await screen.findByRole('alert')).textContent).toContain('本地已保存');
    expect(onDone).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: '重试同步' }));
    await waitFor(() => expect(onDone).toHaveBeenCalledTimes(1));
    expect(request.mock.calls.filter(([path]) => path === '/workContact/update')).toHaveLength(2);
  });

  it('keeps only the latest tag-group response during rapid switching', async () => {
    const slow = deferred<Array<{ id: number; name: string }>>();
    const latest = deferred<Array<{ id: number; name: string }>>();
    const request = vi.fn((path: string): Promise<unknown> => {
      if (path.startsWith('/workContact/detail?')) return Promise.resolve(summary);
      if (path.startsWith('/workContact/show?')) return Promise.resolve(workspace);
      if (path === '/workContactTagGroup/index') return Promise.resolve([
        { groupId: 3, groupName: '阶段' },
        { groupId: 4, groupName: '来源' },
      ]);
      if (path === '/workContactTag/allTag') return Promise.resolve([{ id: 7, name: '已有标签' }]);
      if (path === '/workContactTag/allTag?groupId=3') return slow.promise;
      if (path === '/workContactTag/allTag?groupId=4') return latest.promise;
      return Promise.reject(new Error(`unexpected request ${path}`));
    }) as unknown as ContactRequest;
    frame(<ContactTagPage request={request} onDone={vi.fn()} onReauthenticate={vi.fn()} />);

    expect(await screen.findByText('已有标签（已存在）')).not.toBeNull();
    fireEvent.change(screen.getByLabelText('标签分组'), { target: { value: '3' } });
    fireEvent.change(screen.getByLabelText('标签分组'), { target: { value: '4' } });
    latest.resolve([{ id: 12, name: '最新分组标签' }]);
    expect(await screen.findByRole('checkbox', { name: '最新分组标签' })).not.toBeNull();
    slow.resolve([{ id: 13, name: '过期分组标签' }]);

    await waitFor(() => expect(screen.queryByRole('checkbox', { name: '过期分组标签' })).toBeNull());
    expect(screen.getByRole('checkbox', { name: '最新分组标签' })).not.toBeNull();
  });

  it('clears stale tags and disables save when the latest group load fails', async () => {
    const failed = deferred<Array<{ id: number; name: string }>>();
    const request = vi.fn((path: string): Promise<unknown> => {
      if (path.startsWith('/workContact/detail?')) return Promise.resolve(summary);
      if (path.startsWith('/workContact/show?')) return Promise.resolve(workspace);
      if (path === '/workContactTagGroup/index') return Promise.resolve([{ groupId: 3, groupName: '阶段' }]);
      if (path === '/workContactTag/allTag') return Promise.resolve([{ id: 7, name: '已有标签' }, { id: 9, name: '新标签' }]);
      if (path === '/workContactTag/allTag?groupId=3') return failed.promise;
      return Promise.reject(new Error(`unexpected request ${path}`));
    }) as unknown as ContactRequest;
    frame(<ContactTagPage request={request} onDone={vi.fn()} onReauthenticate={vi.fn()} />);

    fireEvent.click(await screen.findByRole('checkbox', { name: '新标签' }));
    expect(screen.getByRole('button', { name: '保存标签' }).hasAttribute('disabled')).toBe(false);
    fireEvent.change(screen.getByLabelText('标签分组'), { target: { value: '3' } });
    failed.reject(new Error('分组标签加载失败'));

    expect((await screen.findByRole('alert')).textContent).toContain('分组标签加载失败');
    expect(screen.queryByRole('checkbox', { name: '新标签' })).toBeNull();
    expect(screen.getByRole('button', { name: '保存标签' }).hasAttribute('disabled')).toBe(true);
  });

  it('renders portrait field types and persists edited values', async () => {
    const request = vi.fn()
      .mockResolvedValueOnce(summary)
      .mockResolvedValueOnce([
        { contactFieldId: 31, contactFieldPivotId: 901, name: '城市', type: 0, typeText: '文本', options: [], value: '上海' },
        { contactFieldId: 32, contactFieldPivotId: '', name: '爱好', type: 2, typeText: '多选', options: ['跑步', '读书'], value: ['跑步'] },
        { contactFieldId: 33, contactFieldPivotId: '', name: '等级', type: 3, typeText: '单选', options: ['A', 'B'], value: '' },
      ])
      .mockResolvedValue([]);
    const onDone = vi.fn();
    frame(<ContactEditPage request={request} onDone={onDone} onReauthenticate={vi.fn()} />);

    const city = await screen.findByLabelText('城市');
    fireEvent.change(city, { target: { value: '杭州' } });
    fireEvent.click(screen.getByRole('checkbox', { name: '读书' }));
    fireEvent.click(screen.getByRole('radio', { name: 'A' }));
    fireEvent.click(screen.getByRole('button', { name: '保存画像' }));

    await waitFor(() => expect(onDone).toHaveBeenCalledTimes(1));
    const update = request.mock.calls.find(([path]) => path === '/contactFieldPivot/update');
    const init = update?.[1] as RequestInit | undefined;
    expect(init?.body).toContain('杭州');
    expect(init?.body).toContain('读书');
  });
});
