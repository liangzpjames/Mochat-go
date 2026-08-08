/* eslint-disable @typescript-eslint/no-unsafe-assignment, @typescript-eslint/unbound-method */
import { ApiError } from '@mochat/api-client';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { SensitiveWordApi } from './sensitive-word-api';
import { SensitiveWordPage } from './sensitive-word-page';

vi.mock('../../app/access-context', () => ({
  useDashboardAccess: () => ({
	corp: { id: '7' },
	allowedActions: new Set([
	  '/ai-insight/v2/sensitive-word@add',
	  '/ai-insight/v2/sensitive-word@edit',
	  '/ai-insight/v2/sensitive-word@delete',
	]),
  }),
}));

beforeEach(() => {
  vi.stubGlobal('ResizeObserver', class {
    observe() {}
    unobserve() {}
    disconnect() {}
  });
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

function apiFixture(overrides: Partial<SensitiveWordApi> = {}): SensitiveWordApi {
  return {
	list: vi.fn().mockResolvedValue({ items: [{ id: 1, groupId: 2, groupName: '默认', name: '旧词', status: 1, version: 'word-v1' }], total: 1, page: 1, perPage: 10 }),
	groups: vi.fn().mockResolvedValue([{ id: 2, name: '默认', version: 'group-v1' }, { id: 3, name: '财务', version: 'group-v2' }]),
	create: vi.fn().mockResolvedValue({ version: '0', idempotent: false }),
	setEnabled: vi.fn().mockResolvedValue({ version: 'word-v2', idempotent: false }),
	move: vi.fn().mockResolvedValue({ version: 'word-v2', idempotent: false }),
	remove: vi.fn().mockResolvedValue({ version: 'word-v2', idempotent: false }),
	createGroup: vi.fn().mockResolvedValue({ version: '0', idempotent: false }),
	renameGroup: vi.fn().mockResolvedValue({ version: 'group-v2', idempotent: false }),
	matches: vi.fn().mockResolvedValue({ items: [{ id: 9, sensitiveWordName: '报价', source: 2, sourceText: '员工', triggerName: '小王', triggerScenario: '客户群', triggerTime: '2026-07-01 10:00:00' }], total: 1, page: 1, perPage: 10 }),
	matchDetail: vi.fn().mockResolvedValue([{ sender: '小王', msgContent: { content: '报价不可外发' } }]),
	...overrides,
  };
}

function renderPage(api: SensitiveWordApi) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  const invalidateQueries = vi.spyOn(queryClient, 'invalidateQueries');
  const view = render(
	<QueryClientProvider client={queryClient}>
	  <SensitiveWordPage api={api} />
	</QueryClientProvider>,
  );
  return { ...view, invalidateQueries };
}

describe('SensitiveWordPage', () => {
  it('shows honest record summaries, refreshes and paginates without losing filters', async () => {
	const matches = vi.fn().mockResolvedValue({
	  items: [{ id: 9, sensitiveWordName: '报价', source: 2, sourceText: '员工', triggerName: '小王', triggerScenario: '客户群', triggerTime: '2026-07-01 10:00:00' }],
	  total: 25, page: 1, perPage: 10,
	});
	const { container } = renderPage(apiFixture({ matches }));

	expect((await screen.findByRole('region', { name: '记录概览' })).textContent).toContain('25');
	expect(screen.getByText('命中记录总数')).toBeTruthy();
	expect(screen.getByText('当前页记录')).toBeTruthy();
	expect(container.querySelector('.sensitive-word-record-toolbar')).not.toBeNull();

	fireEvent.click(screen.getByRole('button', { name: '刷新记录' }));
	await waitFor(() => expect(matches).toHaveBeenCalledTimes(2));
	fireEvent.change(screen.getByRole('textbox', { name: '员工 ID' }), { target: { value: '3,5' } });
	fireEvent.click(screen.getByRole('button', { name: '查询记录' }));
	await waitFor(() => expect(matches).toHaveBeenLastCalledWith(expect.objectContaining({ employeeIds: [3, 5], page: 1 })));
	fireEvent.click(screen.getByRole('button', { name: '下一页' }));
	await waitFor(() => expect(matches).toHaveBeenLastCalledWith(expect.objectContaining({ employeeIds: [3, 5], page: 2 })));
  });

  it('opens a structured match detail drawer and closes it', async () => {
	renderPage(apiFixture());
	await screen.findByText('报价');

	fireEvent.click(screen.getByRole('button', { name: '查看详情' }));

	const dialog = await screen.findByRole('dialog', { name: '命中详情' });
	expect(dialog.textContent).toContain('报价不可外发');
	expect(dialog.querySelector('pre')).toBeNull();
	fireEvent.click(screen.getByRole('button', { name: '关闭详情' }));
	expect(screen.queryByRole('dialog', { name: '命中详情' })).toBeNull();
  });

  it('uses fixed record/config partitions and applies record filters', async () => {
	const api = apiFixture();
	renderPage(api);

	expect(await screen.findByRole('heading', { name: '敏感词管理' })).toBeTruthy();
	expect(screen.getByRole('button', { name: '敏感词记录' })).toBeTruthy();
	expect(screen.getByRole('button', { name: '敏感词配置' })).toBeTruthy();
	await waitFor(() => expect(api.matches).toHaveBeenCalledWith(expect.objectContaining({ page: 1, perPage: 10 })));

	fireEvent.change(screen.getByRole('textbox', { name: '员工 ID' }), { target: { value: '3,5' } });
	fireEvent.change(screen.getByRole('spinbutton', { name: '客户群 ID' }), { target: { value: '9' } });
	fireEvent.change(screen.getByRole('combobox', { name: '记录词组' }), { target: { value: '2' } });
	fireEvent.change(screen.getByLabelText('开始时间'), { target: { value: '2026-07-01T00:00' } });
	fireEvent.change(screen.getByLabelText('结束时间'), { target: { value: '2026-07-02T00:00' } });
	fireEvent.click(screen.getByRole('button', { name: '查询记录' }));

	await waitFor(() => expect(api.matches).toHaveBeenLastCalledWith({
	  employeeIds: [3, 5], workRoomId: 9, groupId: 2,
	  triggerStart: '2026-07-01 00:00:00', triggerEnd: '2026-07-02 00:00:00', page: 1, perPage: 10,
	}));
	expect(await screen.findByText('报价')).toBeTruthy();
  });

	it('creates groups and words, toggles, moves and confirms deletion with concurrency metadata', async () => {
	const api = apiFixture();
	const { invalidateQueries } = renderPage(api);
	fireEvent.click(await screen.findByRole('button', { name: '敏感词配置' }));

	fireEvent.change(await screen.findByRole('textbox', { name: '新词组名称' }), { target: { value: '合规' } });
	fireEvent.click(screen.getByRole('button', { name: '新增词组' }));
	await waitFor(() => expect(api.createGroup).toHaveBeenCalledWith(expect.objectContaining({ names: ['合规'], version: '0', idempotencyKey: expect.any(String) })));

	fireEvent.change(screen.getByRole('textbox', { name: '敏感词名称' }), { target: { value: '报价' } });
	fireEvent.change(screen.getByRole('combobox', { name: '敏感词分组' }), { target: { value: '2' } });
	fireEvent.click(screen.getByRole('button', { name: '新增敏感词' }));
	await waitFor(() => expect(api.create).toHaveBeenCalledWith(expect.objectContaining({ names: ['报价'], groupId: 2, version: '0', idempotencyKey: expect.any(String) })));

	fireEvent.click(screen.getByRole('button', { name: '停用 旧词' }));
	expect(api.setEnabled).not.toHaveBeenCalled();
	fireEvent.click(await screen.findByRole('button', { name: '确认' }));
	await waitFor(() => expect(api.setEnabled).toHaveBeenCalledWith(expect.objectContaining({ id: 1, enabled: false, version: 'word-v1', idempotencyKey: expect.any(String) })));
	fireEvent.change(screen.getByRole('combobox', { name: '移动 旧词' }), { target: { value: '3' } });
	fireEvent.click(screen.getByRole('button', { name: '确认移动 旧词' }));
	await waitFor(() => expect(api.move).toHaveBeenCalledWith(expect.objectContaining({ id: 1, groupId: 3, version: 'word-v1', idempotencyKey: expect.any(String) })));
	fireEvent.click(screen.getByRole('button', { name: '删除 旧词' }));
	expect(api.remove).not.toHaveBeenCalled();
	fireEvent.click(await screen.findByRole('button', { name: '确认' }));
	await waitFor(() => expect(api.remove).toHaveBeenCalledWith(expect.objectContaining({ id: 1, version: 'word-v1', confirmed: true, idempotencyKey: expect.any(String) })));

	expect((await screen.findByRole('status')).textContent).toContain('操作成功');
	expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: ['sensitive-word', '7', 'words'] });
  });

  it('renders query failures through PageState and gives a refresh-oriented 409 mutation message', async () => {
	const forbiddenApi = apiFixture({ matches: vi.fn().mockRejectedValue(new ApiError('forbidden', '无权限', { status: 403 })) });
	const forbiddenView = renderPage(forbiddenApi);
	await waitFor(() => expect(forbiddenView.container.querySelector('.page-state-forbidden')).not.toBeNull());
	forbiddenView.unmount();

	const conflictApi = apiFixture({ setEnabled: vi.fn().mockRejectedValue(new ApiError('validation', '版本冲突', { status: 409 })) });
	const conflictView = renderPage(conflictApi);
	fireEvent.click(await screen.findByRole('button', { name: '敏感词配置' }));
	fireEvent.click(await screen.findByRole('button', { name: '停用 旧词' }));
	fireEvent.click(await screen.findByRole('button', { name: '确认' }));
	expect((await screen.findByRole('alert')).textContent).toContain('数据已被其他人更新，请刷新后重试');
	conflictView.unmount();

	const quotaApi = apiFixture({ create: vi.fn().mockRejectedValue(new ApiError('validation', '套餐额度已达上限：敏感词词库数 20/20', { status: 400 })) });
	renderPage(quotaApi);
	fireEvent.click(await screen.findByRole('button', { name: '敏感词配置' }));
	fireEvent.change(await screen.findByRole('textbox', { name: '敏感词名称' }), { target: { value: '新词' } });
	fireEvent.change(screen.getByRole('combobox', { name: '敏感词分组' }), { target: { value: '2' } });
	fireEvent.click(screen.getByRole('button', { name: '新增敏感词' }));
	expect((await screen.findByRole('alert')).textContent).toContain('套餐额度已达上限');
  });

  it('shows an empty PageState inside the active partition without hiding its controls', async () => {
	const api = apiFixture({ matches: vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, perPage: 10 }) });
	const { container } = renderPage(api);
	await waitFor(() => expect(container.querySelector('.page-state-empty')).not.toBeNull());
	expect(screen.getByRole('button', { name: '查询记录' })).toBeTruthy();
  });
});
