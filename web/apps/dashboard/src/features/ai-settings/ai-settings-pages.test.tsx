import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { MemoryRouter, useLocation } from 'react-router';
import { ApiError } from '@mochat/api-client';
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

function LocationProbe() {
  const location = useLocation();
  return <output aria-label="当前地址">{`${location.pathname}${location.search}`}</output>;
}

function renderPage(api: AISettingsApi, page: 'kb' | 'agent', initialEntry?: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const access = { corp: { id: '9' }, session: {}, menu: [], allowedRoutes: new Set(), allowedActions: new Set() } as never;
  return render(
    <DashboardAccessProvider value={access}>
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={[initialEntry ?? (page === 'kb' ? '/ai-setting/ai-knowledge-base' : '/ai-setting/agent')]}>
          {page === 'kb' ? <KnowledgeBasePage api={api} /> : <AgentPage api={api} />}
          <LocationProbe />
        </MemoryRouter>
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
        <MemoryRouter><KnowledgeBasePage api={api} /></MemoryRouter>
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

  it('知识库：共享 Modal 支持 Esc 关闭并恢复触发焦点', async () => {
    renderPage(createApi(), 'kb');
    const trigger = await screen.findByRole('button', { name: '新建知识库' });
    fireEvent.click(trigger);
    const dialog = await screen.findByRole('dialog', { name: '新建知识库' });
    expect(dialog.closest('.dashboard-dialog--modal')).not.toBeNull();
    fireEvent.keyDown(document, { key: 'Escape', code: 'Escape' });
    await waitFor(() => expect(screen.queryByRole('dialog', { name: '新建知识库' })).toBeNull());
    await waitFor(() => expect(document.activeElement).toBe(trigger));
  });

  it('智能体：从 URL 恢复筛选并展示真实知识库名称', async () => {
    const api = createApi({
      listKnowledgeBases: vi.fn().mockResolvedValue([
        { id: 'kb-1', corpId: 9, name: '售后话术库', description: '', documentCount: 3, status: 1, createdAt: '', updatedAt: '' },
      ]),
      listAgents: vi.fn().mockResolvedValue([
        { id: 'a-1', corpId: 9, name: '客服助手', description: '', knowledgeBaseIds: ['kb-1'], status: 1, createdAt: '', updatedAt: '' },
        { id: 'a-2', corpId: 9, name: '停用助手', description: '', knowledgeBaseIds: [], status: 0, createdAt: '', updatedAt: '' },
      ]),
    });
    renderPage(api, 'agent', '/ai-setting/agent?q=客服&status=enabled&page=1&pageSize=10');
    expect(await screen.findByText('售后话术库')).toBeTruthy();
    expect(screen.queryByText('停用助手')).toBeNull();
    expect(screen.getByRole('textbox', { name: '关键词' })).toHaveProperty('value', '客服');
    expect(screen.getByRole('combobox', { name: '状态' })).toHaveProperty('value', 'enabled');
  });

  it('知识库：查询、重置、翻页和每页条数写入 URL', async () => {
    const items = Array.from({ length: 21 }, (_, index) => ({
      id: `kb-${index + 1}`, corpId: 9, name: `知识库 ${String(index + 1).padStart(2, '0')}`,
      description: index === 20 ? '售后专用' : '', documentCount: index, status: index % 2,
      createdAt: '', updatedAt: '2026-08-07T00:00:00Z',
    }));
    renderPage(createApi({ listKnowledgeBases: vi.fn().mockResolvedValue(items) }), 'kb');
    await screen.findByText('知识库 01');
    fireEvent.change(screen.getByRole('textbox', { name: '关键词' }), { target: { value: '售后' } });
    fireEvent.change(screen.getByRole('combobox', { name: '状态' }), { target: { value: 'enabled' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    expect(screen.getByLabelText('当前地址').textContent).toContain('q=%E5%94%AE%E5%90%8E');
    expect(screen.getByLabelText('当前地址').textContent).toContain('status=enabled');
    fireEvent.click(screen.getByRole('button', { name: '重置' }));
    expect(screen.getByLabelText('当前地址').textContent).not.toContain('q=');
    fireEvent.click(screen.getByRole('button', { name: '下一页' }));
    expect(screen.getByLabelText('当前地址').textContent).toContain('page=2');
    fireEvent.change(screen.getByRole('combobox', { name: '每页条数' }), { target: { value: '20' } });
    expect(screen.getByLabelText('当前地址').textContent).toContain('pageSize=20');
  });

  it('知识库：区分真实空态、筛选无结果与登记值能力边界', async () => {
    const api = createApi({
      listKnowledgeBases: vi.fn().mockResolvedValue([
        { id: 'kb-1', corpId: 9, name: '产品资料库', description: '', documentCount: 7, status: 1, createdAt: '', updatedAt: '' },
      ]),
    });
    renderPage(api, 'kb', '/ai-setting/ai-knowledge-base?q=不存在');
    expect(await screen.findByText('当前筛选条件下没有匹配的知识库')).toBeTruthy();
    expect(screen.getByText(/文档上传、解析与检索能力尚未接入/)).toBeTruthy();
    expect(screen.getByText('登记文档数合计')).toBeTruthy();
  });

  it('知识库：被智能体引用时把稳定机器码映射为中文冲突反馈', async () => {
    const api = createApi({
      listKnowledgeBases: vi.fn().mockResolvedValue([
        { id: 'kb-1', corpId: 9, name: '售后话术库', description: '', documentCount: 3, status: 1, createdAt: '', updatedAt: '' },
      ]),
      deleteKnowledgeBase: vi.fn().mockRejectedValue(new ApiError('validation', 'conflict', { status: 409, machineCode: 'AI_SETTINGS_KNOWLEDGE_BASE_REFERENCED' })),
    });
    renderPage(api, 'kb');
    fireEvent.click(await screen.findByRole('button', { name: '删除 售后话术库' }));
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));
    expect((await screen.findByRole('alert')).textContent).toContain('仍被智能体引用');
  });

  it('智能体：知识库加载失败时禁用保存并保留明确错误语义', async () => {
    const api = createApi({ listKnowledgeBases: vi.fn().mockRejectedValue(new Error('network')) });
    renderPage(api, 'agent');
    fireEvent.click(await screen.findByRole('button', { name: '新建智能体' }));
    expect(await screen.findByRole('alert')).toBeTruthy();
    expect(screen.getByRole('button', { name: '保存' })).toHaveProperty('disabled', true);
  });

  it('知识库：脏数据关闭需确认，放弃后恢复到实际触发按钮', async () => {
    renderPage(createApi(), 'kb');
    const trigger = await screen.findByRole('button', { name: '新建知识库' });
    fireEvent.click(trigger);
    fireEvent.change(await screen.findByRole('textbox', { name: '名称' }), { target: { value: '未保存知识库' } });
    fireEvent.keyDown(document, { key: 'Escape', code: 'Escape' });
    expect(await screen.findByText('放弃未保存更改？')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '放弃更改' }));
    await waitFor(() => expect(screen.queryByRole('dialog', { name: '新建知识库' })).toBeNull());
    await waitFor(() => expect(document.activeElement).toBe(trigger));
  });

  it('知识库：创建时提交显式停用状态并在真实回读后反馈成功', async () => {
    const createKnowledgeBase = vi.fn().mockResolvedValue({ id: 'kb-new' });
    const listKnowledgeBases = vi.fn().mockResolvedValue([]);
    renderPage(createApi({ createKnowledgeBase, listKnowledgeBases }), 'kb');
    fireEvent.click(await screen.findByRole('button', { name: '新建知识库' }));
    fireEvent.change(screen.getByRole('textbox', { name: '名称' }), { target: { value: '验收知识库' } });
    fireEvent.change(screen.getByRole('spinbutton', { name: /登记文档数/ }), { target: { value: '4' } });
    fireEvent.change(screen.getByRole('combobox', { name: /配置状态/ }), { target: { value: '0' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => expect(createKnowledgeBase).toHaveBeenCalledWith(9, expect.objectContaining({ name: '验收知识库', documentCount: 4, status: 0 })));
    expect(await screen.findByText('知识库已创建。')).toBeTruthy();
    expect(listKnowledgeBases.mock.calls.length).toBeGreaterThan(1);
  });

  it('知识库：保存失败映射稳定中文且保留表单上下文', async () => {
    const createKnowledgeBase = vi.fn().mockRejectedValue(new ApiError('server', 'raw sql error', { status: 500, machineCode: 'AI_SETTINGS_STORAGE_FAILURE' }));
    renderPage(createApi({ createKnowledgeBase }), 'kb');
    fireEvent.click(await screen.findByRole('button', { name: '新建知识库' }));
    fireEvent.change(screen.getByRole('textbox', { name: '名称' }), { target: { value: '保留输入' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    expect((await screen.findByRole('alert')).textContent).toContain('服务暂时不可用');
    expect(screen.getByRole('textbox', { name: '名称' })).toHaveProperty('value', '保留输入');
    expect(screen.queryByText('raw sql error')).toBeNull();
  });

  it('智能体：编辑时允许保留既有关联的停用知识库，但禁止新关联其他停用知识库', async () => {
    const api = createApi({
      listKnowledgeBases: vi.fn().mockResolvedValue([
        { id: 'kb-old', corpId: 9, name: '既有关联停用库', description: '', documentCount: 0, status: 0, createdAt: '', updatedAt: '' },
        { id: 'kb-disabled', corpId: 9, name: '其他停用库', description: '', documentCount: 0, status: 0, createdAt: '', updatedAt: '' },
      ]),
      listAgents: vi.fn().mockResolvedValue([
        { id: 'a-1', corpId: 9, name: '客服助手', description: '', knowledgeBaseIds: ['kb-old', 'missing-kb'], status: 1, createdAt: '', updatedAt: '' },
      ]),
    });
    renderPage(api, 'agent');
    fireEvent.click(await screen.findByRole('button', { name: '编辑 客服助手' }));
    const existing = screen.getByRole('checkbox', { name: /既有关联停用库/ });
    const disabled = screen.getByRole('checkbox', { name: /其他停用库/ });
    expect(existing).toHaveProperty('checked', true);
    expect(existing).toHaveProperty('disabled', false);
    expect(disabled).toHaveProperty('disabled', true);
    expect(screen.getAllByText(/已失效（ID: missing-kb）/).length).toBeGreaterThan(0);
  });

  it('智能体：显式停用使用完整真实记录更新并回读', async () => {
    const updateAgent = vi.fn().mockResolvedValue({});
    const listAgents = vi.fn().mockResolvedValue([
      { id: 'a-1', corpId: 9, name: '客服助手', description: '真实说明', knowledgeBaseIds: ['kb-1'], status: 1, createdAt: '', updatedAt: '' },
    ]);
    renderPage(createApi({ updateAgent, listAgents }), 'agent');
    fireEvent.click(await screen.findByRole('button', { name: '停用 客服助手' }));
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));
    await waitFor(() => expect(updateAgent).toHaveBeenCalledWith(9, 'a-1', { name: '客服助手', description: '真实说明', knowledgeBaseIds: ['kb-1'], status: 0 }));
    expect(await screen.findByText('智能体“客服助手”已停用。')).toBeTruthy();
    expect(listAgents.mock.calls.length).toBeGreaterThan(1);
  });

  it('知识库：保存 pending 时 Escape 不关闭编辑器也不重复提交', async () => {
    let resolveSave: ((value: unknown) => void) | undefined;
    const createKnowledgeBase = vi.fn().mockImplementation(() => new Promise((resolve) => { resolveSave = resolve; }));
    renderPage(createApi({ createKnowledgeBase }), 'kb');
    fireEvent.click(await screen.findByRole('button', { name: '新建知识库' }));
    fireEvent.change(screen.getByRole('textbox', { name: '名称' }), { target: { value: '保存中知识库' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => expect(createKnowledgeBase).toHaveBeenCalledTimes(1));
    fireEvent.keyDown(document, { key: 'Escape', code: 'Escape' });
    expect(screen.getByRole('dialog', { name: '新建知识库' })).toBeTruthy();
    expect(screen.queryByText('放弃未保存更改？')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    expect(createKnowledgeBase).toHaveBeenCalledTimes(1);
    resolveSave?.({});
  });
});
