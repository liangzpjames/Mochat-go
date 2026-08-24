import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { SensitiveWordApi } from './sensitive-word-api';
import { SensitiveWordPage } from './sensitive-word-page';

vi.mock('../../app/access-context', () => ({ useDashboardAccess: () => ({ corp: { id: '7' }, allowedActions: new Set() }) }));
vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} });
afterEach(() => { cleanup(); vi.restoreAllMocks(); });

function apiFixture(overrides: Partial<SensitiveWordApi> = {}): SensitiveWordApi {
  return {
    list: vi.fn().mockResolvedValue({ items: [{ id: 1, groupId: 2, groupName: '默认', name: '旧词', status: 1, version: 'word-v1', employeeHitCount: 3, customerHitCount: 5, createdAt: '2026-08-21 10:00:00' }], total: 1, page: 1, perPage: 20 }),
    groups: vi.fn().mockResolvedValue([{ id: 2, name: '默认', version: 'group-v1', wordCount: 1, enabledCount: 1 }, { id: 3, name: '财务', version: 'group-v2', wordCount: 0, enabledCount: 0 }]),
    filterOptions: vi.fn().mockResolvedValue({ employees: [{ id: 3, name: '小王', count: 1 }], rooms: [{ id: 9, name: '客户群 A', count: 1 }] }),
    monitorStatus: vi.fn().mockResolvedValue({ enabled: false, state: 'disabled' }),
    create: vi.fn().mockResolvedValue({ version: 'word-v2', idempotent: false }),
    setEnabled: vi.fn().mockResolvedValue({ version: 'word-v2', idempotent: false }),
    move: vi.fn().mockResolvedValue({ version: 'word-v2', idempotent: false }),
    remove: vi.fn().mockResolvedValue({ version: 'word-v2', idempotent: false }),
    createGroup: vi.fn().mockResolvedValue({ version: 'group-v3', idempotent: false }),
    renameGroup: vi.fn().mockResolvedValue({ version: 'group-v3', idempotent: false }),
    matches: vi.fn().mockResolvedValue({ items: [{ id: 9, sensitiveWordID: 11, sensitiveWordName: '报价', source: 2, sourceText: '员工', triggerName: '小王', triggerScenario: '客户群', triggerTime: '2026-07-01 10:00:00', contentPreview: '报价不可外发', workRoomID: 9 }], total: 25, page: 1, perPage: 20 }),
    matchDetail: vi.fn().mockResolvedValue([{ sender: '小王', messageType: '文本', sendTime: '2026-07-01 10:00:00', isTrigger: true, content: '报价不可外发' }]),
    ...overrides,
  };
}

function renderPage(api: SensitiveWordApi) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return render(<MemoryRouter><QueryClientProvider client={queryClient}><SensitiveWordPage api={api} /></QueryClientProvider></MemoryRouter>);
}

describe('SensitiveWordPage', () => {
  it('uses an explicit query button and fixed 20-row pagination', async () => {
    const matches = vi.fn<SensitiveWordApi['matches']>().mockResolvedValue({ items: [{ id: 9, sensitiveWordID: 11, sensitiveWordName: '报价', source: 2, sourceText: '员工', triggerName: '小王', triggerScenario: '客户群', triggerTime: '2026-07-01 10:00:00', contentPreview: '报价不可外发', workRoomID: 9 }], total: 25, page: 1, perPage: 20 });
    const api = apiFixture({ matches });
    renderPage(api);
    await screen.findByText('报价');
    expect(screen.queryByText('AI 洞察')).toBeNull();
    expect(matches).toHaveBeenCalledWith(expect.objectContaining({ page: 1, perPage: 20 }));
    const callsBeforeTyping = matches.mock.calls.length;
    fireEvent.change(screen.getByRole('combobox', { name: '触发员工' }), { target: { value: '3' } });
    expect(matches.mock.calls.length).toBe(callsBeforeTyping);
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(matches).toHaveBeenLastCalledWith(expect.objectContaining({ employeeIds: [3], page: 1, perPage: 20 })));
    await screen.findByRole('button', { name: '第 2 页' });
    fireEvent.click(screen.getByRole('button', { name: '第 2 页' }));
    await waitFor(() => expect(matches).toHaveBeenLastCalledWith(expect.objectContaining({ employeeIds: [3], page: 2, perPage: 20 })));
  });

  it('opens detail from the whole row and closes through the shared overlay', async () => {
    const api = apiFixture();
    renderPage(api);
    const row = (await screen.findByText('报价')).closest('tr');
    expect(row).not.toBeNull();
    fireEvent.click(row!);
    await waitFor(() => {
      const text = screen.getByRole('dialog', { name: '敏感词命中详情' }).textContent ?? '';
      expect(text).toContain('报价不可外发');
      expect(text).toContain('触发消息');
      expect(text).toContain('发送人');
      expect(text).not.toContain('isTrigger');
      expect(text).not.toContain('msgContent');
    });
    fireEvent.click(screen.getByTestId('risk-warning-drawer-overlay'));
    expect(screen.queryByRole('dialog', { name: '敏感词命中详情' })).toBeNull();
  });

  it('keeps configuration in a fixed left-group/right-table workspace', async () => {
    const list = vi.fn<SensitiveWordApi['list']>().mockResolvedValue({ items: [{ id: 1, groupId: 2, groupName: '默认', name: '旧词', status: 1, version: 'word-v1', employeeHitCount: 3, customerHitCount: 5, createdAt: '2026-08-21 10:00:00' }], total: 1, page: 1, perPage: 20 });
    const api = apiFixture({ list });
    const { container } = renderPage(api);
    fireEvent.click(await screen.findByRole('button', { name: '敏感词配置' }));
    expect(container.querySelector('.sensitive-word-config-workspace')).not.toBeNull();
    expect(screen.getByRole('button', { name: /默认/ }).getAttribute('aria-pressed')).toBe('false');
    fireEvent.click(screen.getByRole('button', { name: /默认/ }));
    expect(screen.getByRole('button', { name: /默认/ }).getAttribute('aria-pressed')).toBe('true');
    await screen.findByText('旧词');
    fireEvent.change(screen.getByRole('textbox', { name: '搜索敏感词' }), { target: { value: '报价' } });
    const callsBeforeQuery = list.mock.calls.length;
    expect(callsBeforeQuery).toBeGreaterThan(0);
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(list.mock.calls.some(([input]) => input.groupId === 2 && input.keywords === '报价' && input.page === 1 && input.perPage === 20)).toBe(true));
  });

  it('uses confirmation before mutating a real word', async () => {
    const setEnabled = vi.fn<SensitiveWordApi['setEnabled']>().mockResolvedValue({ version: 'word-v2', idempotent: false });
    const api = apiFixture({ setEnabled });
    renderPage(api);
    fireEvent.click(await screen.findByRole('button', { name: '敏感词配置' }));
    fireEvent.click(await screen.findByRole('button', { name: '停用 旧词' }));
    expect(setEnabled).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));
    await waitFor(() => expect(setEnabled).toHaveBeenCalledWith(expect.objectContaining({ id: 1, enabled: false, version: 'word-v1' })));
  });

  it('shows empty state without inventing records', async () => {
    const api = apiFixture({ matches: vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, perPage: 20 }) });
    renderPage(api);
    expect(await screen.findByRole('heading', { name: '暂无敏感词命中记录' })).toBeTruthy();
    expect(screen.getByRole('button', { name: '查询' })).toBeTruthy();
  });
});
