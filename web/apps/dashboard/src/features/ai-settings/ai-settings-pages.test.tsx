import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { KnowledgeBasePage } from './knowledge-base-page';
import { AgentPage } from './agent-page';
import type { AISettingsApi } from './ai-settings-api';
import { DashboardAccessProvider } from '../../app/access-context';

function createApi(overrides: Partial<AISettingsApi> = {}): AISettingsApi {
  return {
    listKnowledgeBases: vi.fn().mockResolvedValue([]),
    createKnowledgeBase: vi.fn().mockResolvedValue({ id: 'kb-1', corpId: 9, name: 'x', description: '', documentCount: 0, status: 1, createdAt: '', updatedAt: '' }),
    updateKnowledgeBase: vi.fn(),
    deleteKnowledgeBase: vi.fn().mockResolvedValue({}),
    listAgents: vi.fn().mockResolvedValue([]),
    createAgent: vi.fn().mockResolvedValue({ id: 'a-1', corpId: 9, name: 'x', description: '', knowledgeBaseIds: [], status: 1, createdAt: '', updatedAt: '' }),
    updateAgent: vi.fn(),
    deleteAgent: vi.fn().mockResolvedValue({}),
    ...overrides,
  };
}

afterEach(cleanup);
beforeAll(() => { globalThis.ResizeObserver = class { observe() {} unobserve() {} disconnect() {} }; });

function renderPage(api: AISettingsApi, page: 'kb' | 'agent') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const access = { corp: { id: '9' }, session: {}, menu: [], allowedRoutes: new Set(), allowedActions: new Set() } as never;
  return render(
    <DashboardAccessProvider value={access}>
      <QueryClientProvider client={client}>
        {page === 'kb' ? <KnowledgeBasePage api={api} /> : <AgentPage api={api} />}
      </QueryClientProvider>
    </DashboardAccessProvider>,
  );
}

describe('AI 设置页面', () => {
  it('知识库：数据加载并渲染行', async () => {
    const api = createApi({
      listKnowledgeBases: vi.fn().mockResolvedValue([
        { id: 'kb-1', corpId: 9, name: '售后话术库', description: 'test', documentCount: 3, status: 1, createdAt: '', updatedAt: '2026-08-07T00:00:00Z' },
      ]),
    });
    renderPage(api, 'kb');
    expect(await screen.findByText('售后话术库')).toBeTruthy();
    expect(screen.getAllByText('3').length).toBeGreaterThan(0);
  });

  it('知识库：空态', async () => {
    renderPage(createApi(), 'kb');
    expect(await screen.findByText(/暂无知识库/)).toBeTruthy();
  });

  it('知识库：错误态与重试', async () => {
    const refetch = vi.fn().mockRejectedValue(new Error('boom'));
    const api = createApi({ listKnowledgeBases: refetch });
    renderPage(api, 'kb');
    expect(await screen.findByText(/数据加载失败/)).toBeTruthy();
    expect(screen.getByRole('button', { name: '重新加载' })).toBeTruthy();
  });

  it('知识库：受限态（未授权 corp 不请求）', () => {
    const listKnowledgeBases = vi.fn().mockResolvedValue([]);
    const api = createApi({ listKnowledgeBases });
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { container } = render(
      <QueryClientProvider client={client}>
        <KnowledgeBasePage api={api} />
      </QueryClientProvider>,
    );
    expect(container.textContent).toContain('知识库');
    expect(listKnowledgeBases).not.toHaveBeenCalled();
  });

  it('智能体：数据加载并渲染行，创建校验名称必填', async () => {
    const api = createApi({
      listAgents: vi.fn().mockResolvedValue([
        { id: 'a-1', corpId: 9, name: '智能客服', description: '', knowledgeBaseIds: ['kb-1'], status: 1, createdAt: '', updatedAt: '2026-08-07T00:00:00Z' },
      ]),
    });
    renderPage(api, 'agent');
    expect(await screen.findByText('智能客服')).toBeTruthy();
  });

  it('知识库：确认前不删除指定对象', async () => {
    const deleteKnowledgeBase = vi.fn().mockResolvedValue({});
    const api = createApi({
      listKnowledgeBases: vi.fn().mockResolvedValue([{ id: 'kb-1', corpId: 9, name: '售后话术库', description: '', documentCount: 3, status: 1, createdAt: '', updatedAt: '' }]),
      deleteKnowledgeBase,
    });
    renderPage(api, 'kb');
    fireEvent.click(await screen.findByRole('button', { name: '删除 售后话术库' }));
    expect(deleteKnowledgeBase).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));
    await waitFor(() => expect(deleteKnowledgeBase).toHaveBeenCalledTimes(1));
  });

  it('智能体：取消删除时请求数为零', async () => {
    const deleteAgent = vi.fn().mockResolvedValue({});
    const api = createApi({
      listAgents: vi.fn().mockResolvedValue([{ id: 'a-1', corpId: 9, name: '智能客服', description: '', knowledgeBaseIds: [], status: 1, createdAt: '', updatedAt: '' }]),
      deleteAgent,
    });
    renderPage(api, 'agent');
    fireEvent.click(await screen.findByRole('button', { name: '删除 智能客服' }));
    fireEvent.click(await screen.findByRole('button', { name: '取消' }));
    expect(deleteAgent).not.toHaveBeenCalled();
  });
});
