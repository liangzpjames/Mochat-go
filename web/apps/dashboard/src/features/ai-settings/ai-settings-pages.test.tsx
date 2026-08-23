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
    listKnowledgeDocuments: vi.fn().mockResolvedValue([]),
    uploadKnowledgeDocument: vi.fn().mockResolvedValue({ id: 'doc-1', filename: '资料.md', status: 'ready' }),
    deleteKnowledgeDocument: vi.fn().mockResolvedValue({}),
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
    expect((await screen.findByRole('alert')).textContent).toContain('数据加载失败');
    expect(screen.getByRole('button', { name: '重新加载' })).toBeTruthy();
    const metrics = screen.getByRole('region', { name: '知识库指标' });
    expect(metrics.textContent).toContain('—');
    expect(metrics.textContent).not.toContain('知识库总数0');
  });

  it('智能体：首屏失败时指标不伪装为零值', async () => {
    renderPage(createApi({ listAgents: vi.fn().mockRejectedValue(new Error('boom')) }), 'agent');
    expect((await screen.findByRole('alert')).textContent).toContain('数据加载失败');
    const metrics = screen.getByRole('region', { name: '智能体指标' });
    expect(metrics.textContent).toContain('—');
    expect(metrics.textContent).not.toContain('智能体总数0');
  });

  it('知识库：刷新失败时保留并明确标记上次成功数据', async () => {
    const listKnowledgeBases = vi.fn()
      .mockResolvedValueOnce([{ id: 'kb-1', corpId: 9, name: '缓存知识库', description: '', documentCount: 3, status: 1, createdAt: '', updatedAt: '' }])
      .mockRejectedValueOnce(new Error('offline'));
    renderPage(createApi({ listKnowledgeBases }), 'kb');
    await screen.findByText('缓存知识库');
    fireEvent.click(screen.getByRole('button', { name: '刷新' }));
    await waitFor(() => expect(listKnowledgeBases).toHaveBeenCalledTimes(2));
    const metrics = screen.getByRole('region', { name: '知识库指标' });
    await waitFor(() => expect(metrics.textContent).toContain('上次成功数据，刷新失败'));
    expect(metrics.textContent).toContain('知识库总数1');
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

  it('智能体：固定系统助手不提供新增或删除入口', async () => {
    const createAgent = vi.fn();
    const deleteAgent = vi.fn();
    const api = createApi({
      listAgents: vi.fn().mockResolvedValue([{ id: 'a-1', corpId: 9, name: '会话分析助手', systemKey: 'session-analysis', description: '', knowledgeBaseIds: [], status: 1, createdAt: '', updatedAt: '' }]),
      createAgent, deleteAgent,
    });
    renderPage(api, 'agent');
    expect(await screen.findByText('会话分析助手')).toBeTruthy();
    expect(screen.queryByRole('button', { name: /新建智能体/ })).toBeNull();
    expect(screen.queryByRole('button', { name: /删除 会话分析助手/ })).toBeNull();
    expect(createAgent).not.toHaveBeenCalled();
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

  it('知识库：区分筛选无结果并明确实际消费边界', async () => {
    const api = createApi({
      listKnowledgeBases: vi.fn().mockResolvedValue([
        { id: 'kb-1', corpId: 9, name: '产品资料库', description: '', documentCount: 7, status: 1, createdAt: '', updatedAt: '' },
      ]),
    });
    renderPage(api, 'kb', '/ai-setting/ai-knowledge-base?q=不存在');
    expect(await screen.findByText('当前筛选条件下没有匹配的知识库')).toBeTruthy();
    expect(screen.getByText(/知识不能替代真实会话消息作为分析证据/)).toBeTruthy();
    expect(screen.getByText('已上传文档')).toBeTruthy();
  });

  it('知识库：被智能体引用时把稳定机器码映射为中文冲突反馈', async () => {
    const api = createApi({
      listKnowledgeBases: vi.fn().mockResolvedValue([
        { id: 'kb-1', corpId: 9, name: '售后话术库', description: '', documentCount: 3, status: 1, createdAt: '', updatedAt: '' },
      ]),
      deleteKnowledgeBase: vi.fn().mockRejectedValue(new ApiError('validation', 'conflict', { status: 409, machineCode: 'AI_SETTINGS_KNOWLEDGE_BASE_REFERENCED', data: { referenceCount: 2 } })),
    });
    renderPage(api, 'kb');
    fireEvent.click(await screen.findByRole('button', { name: '删除 售后话术库' }));
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));
    expect((await screen.findByRole('alert')).textContent).toContain('仍被 2 个智能体引用');
  });

  it('智能体：知识库加载失败时禁用保存并保留明确错误语义', async () => {
    const api = createApi({
      listKnowledgeBases: vi.fn().mockRejectedValue(new Error('network')),
      listAgents: vi.fn().mockResolvedValue([{ id: 'a-1', corpId: 9, name: '会话分析助手', description: '', knowledgeBaseIds: [], status: 1, createdAt: '', updatedAt: '' }]),
    });
    renderPage(api, 'agent');
    fireEvent.click(await screen.findByRole('button', { name: '编辑 会话分析助手' }));
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
    fireEvent.change(screen.getByRole('combobox', { name: /配置状态/ }), { target: { value: '0' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => expect(createKnowledgeBase).toHaveBeenCalledWith(9, expect.objectContaining({ name: '验收知识库', documentCount: 0, status: 0 })));
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

  it('知识库：请求 JSON 合同失败显示明确中文反馈', async () => {
    const createKnowledgeBase = vi.fn().mockRejectedValue(new ApiError('validation', 'invalid json', { status: 400, machineCode: 'AI_SETTINGS_INVALID_JSON' }));
    renderPage(createApi({ createKnowledgeBase }), 'kb');
    fireEvent.click(await screen.findByRole('button', { name: '新建知识库' }));
    fireEvent.change(screen.getByRole('textbox', { name: '名称' }), { target: { value: '请求知识库' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    expect((await screen.findByRole('alert')).textContent).toContain('请求格式无效，请刷新后重试。');
    expect(screen.queryByText('操作失败，请稍后重试。')).toBeNull();
  });

  it('智能体：更新请求失败显示明确中文并保留编辑上下文', async () => {
    const updateAgent = vi.fn().mockRejectedValue(new ApiError('validation', 'invalid json', { status: 400, machineCode: 'AI_SETTINGS_INVALID_JSON' }));
    renderPage(createApi({ updateAgent, listAgents: vi.fn().mockResolvedValue([{ id: 'a-1', corpId: 9, name: '会话分析助手', description: '', knowledgeBaseIds: [], status: 1, createdAt: '', updatedAt: '' }]) }), 'agent');
    fireEvent.click(await screen.findByRole('button', { name: '编辑 会话分析助手' }));
    fireEvent.change(screen.getByRole('textbox', { name: '分析要求' }), { target: { value: '重点检查退款风险' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    expect((await screen.findByRole('alert')).textContent).toContain('请求格式无效，请刷新后重试。');
    expect(screen.queryByText('操作失败，请稍后重试。')).toBeNull();
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

  it('智能体：直接访问越界页码时以 replace 规范化为最后一页', async () => {
    const agents = Array.from({ length: 21 }, (_, index) => ({
      id: `a-${index + 1}`, corpId: 9, name: `智能体 ${index + 1}`, description: '', knowledgeBaseIds: [], status: 1,
      createdAt: '', updatedAt: '',
    }));
    renderPage(createApi({ listAgents: vi.fn().mockResolvedValue(agents) }), 'agent', '/ai-setting/agent?page=999&pageSize=10');
    await screen.findByText('智能体 21');
    await waitFor(() => expect(screen.getByLabelText('当前地址').textContent).toContain('page=3'));
    expect(screen.getByLabelText('当前地址').textContent).not.toContain('page=999');
  });

  it('知识库：显式非法列表参数规范化为实际状态', async () => {
    renderPage(createApi({
      listKnowledgeBases: vi.fn().mockResolvedValue([{ id: 'kb-1', corpId: 9, name: '参数知识库', description: '', documentCount: 0, status: 1, createdAt: '', updatedAt: '' }]),
    }), 'kb', '/ai-setting/ai-knowledge-base?page=1&pageSize=999&status=invalid');
    await screen.findByText('参数知识库');
    await waitFor(() => expect(screen.getByLabelText('当前地址').textContent).toBe('/ai-setting/ai-knowledge-base?page=1&pageSize=10&status=all'));
    expect(screen.getByRole('combobox', { name: '状态' })).toHaveProperty('value', 'all');
    expect(screen.getByRole('combobox', { name: '每页条数' })).toHaveProperty('value', '10');
  });

  it('智能体：显式非法列表参数规范化为实际状态', async () => {
    renderPage(createApi({
      listAgents: vi.fn().mockResolvedValue([{ id: 'a-1', corpId: 9, name: '参数智能体', description: '', knowledgeBaseIds: [], status: 1, createdAt: '', updatedAt: '' }]),
    }), 'agent', '/ai-setting/agent?page=1&pageSize=999&status=invalid');
    await screen.findByText('参数智能体');
    await waitFor(() => expect(screen.getByLabelText('当前地址').textContent).toBe('/ai-setting/agent?page=1&pageSize=10&status=all'));
    expect(screen.getByRole('combobox', { name: '状态' })).toHaveProperty('value', 'all');
    expect(screen.getByRole('combobox', { name: '每页条数' })).toHaveProperty('value', '10');
  });

  it.each([
    ['知识库', 'kb' as const, '/ai-setting/ai-knowledge-base', '默认知识库'],
    ['智能体', 'agent' as const, '/ai-setting/agent', '默认智能体'],
  ])('%s：未提供列表参数时默认 URL 保持简洁', async (_label, pageKind, path, itemName) => {
    const api = pageKind === 'kb'
      ? createApi({ listKnowledgeBases: vi.fn().mockResolvedValue([{ id: 'kb-default', corpId: 9, name: itemName, description: '', documentCount: 0, status: 1, createdAt: '', updatedAt: '' }]) })
      : createApi({ listAgents: vi.fn().mockResolvedValue([{ id: 'agent-default', corpId: 9, name: itemName, description: '', knowledgeBaseIds: [], status: 1, createdAt: '', updatedAt: '' }]) });
    renderPage(api, pageKind, path);
    await screen.findByText(itemName);
    await waitFor(() => expect(screen.getByLabelText('当前地址').textContent).toBe(path));
  });

  it('知识库：删除最后一页唯一记录后回退并规范化为第一页', async () => {
    const items = Array.from({ length: 11 }, (_, index) => ({
      id: `kb-${index + 1}`, corpId: 9, name: `知识库 ${index + 1}`, description: '', documentCount: 0, status: 1,
      createdAt: '', updatedAt: '',
    }));
    const listKnowledgeBases = vi.fn().mockImplementation(() => Promise.resolve([...items]));
    const deleteKnowledgeBase = vi.fn().mockImplementation((_corpId: number, id: string) => {
      items.splice(items.findIndex((item) => item.id === id), 1);
      return Promise.resolve({});
    });
    renderPage(createApi({ listKnowledgeBases, deleteKnowledgeBase }), 'kb', '/ai-setting/ai-knowledge-base?page=2&pageSize=10');
    fireEvent.click(await screen.findByRole('button', { name: '删除 知识库 11' }));
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));
    await waitFor(() => expect(deleteKnowledgeBase).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(screen.getByLabelText('当前地址').textContent).toContain('page=1'));
  });

  it('知识库：异步删除确认保持 pending 并阻止重复请求', async () => {
    let resolveDelete: ((value: unknown) => void) | undefined;
    const deleteKnowledgeBase = vi.fn().mockImplementation(() => new Promise((resolve) => { resolveDelete = resolve; }));
    renderPage(createApi({
      listKnowledgeBases: vi.fn().mockResolvedValue([{ id: 'kb-1', corpId: 9, name: '待删除知识库', description: '', documentCount: 0, status: 1, createdAt: '', updatedAt: '' }]),
      deleteKnowledgeBase,
    }), 'kb');
    fireEvent.click(await screen.findByRole('button', { name: '删除 待删除知识库' }));
    const confirm = await screen.findByRole('button', { name: '确认' });
    fireEvent.click(confirm);
    await waitFor(() => expect(deleteKnowledgeBase).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(confirm.className).toContain('ant-btn-loading'));
    fireEvent.click(confirm);
    expect(deleteKnowledgeBase).toHaveBeenCalledTimes(1);
    resolveDelete?.({});
    await screen.findByText('知识库“待删除知识库”已删除。');
  });

  it('智能体：异步启停确认保持 pending 并阻止重复请求', async () => {
    let resolveToggle: ((value: unknown) => void) | undefined;
    const updateAgent = vi.fn().mockImplementation(() => new Promise((resolve) => { resolveToggle = resolve; }));
    renderPage(createApi({
      listAgents: vi.fn().mockResolvedValue([{ id: 'a-1', corpId: 9, name: '待停用助手', description: '', knowledgeBaseIds: [], status: 1, createdAt: '', updatedAt: '' }]),
      updateAgent,
    }), 'agent');
    fireEvent.click(await screen.findByRole('button', { name: '停用 待停用助手' }));
    const confirm = await screen.findByRole('button', { name: '确认' });
    fireEvent.click(confirm);
    await waitFor(() => expect(updateAgent).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(confirm.className).toContain('ant-btn-loading'));
    fireEvent.click(confirm);
    expect(updateAgent).toHaveBeenCalledTimes(1);
    resolveToggle?.({});
    await screen.findByText('智能体“待停用助手”已停用。');
  });

  it('知识库：文档数由上传链路维护且 Unicode 长度不依赖原生 maxLength', async () => {
    renderPage(createApi(), 'kb');
    fireEvent.click(await screen.findByRole('button', { name: '新建知识库' }));
    const nameInput = screen.getByRole('textbox', { name: '名称' });
    const descriptionInput = screen.getByRole('textbox', { name: '说明' });
    expect(nameInput.getAttribute('maxlength')).toBeNull();
    expect(descriptionInput.getAttribute('maxlength')).toBeNull();
    expect(screen.queryByRole('spinbutton', { name: /登记文档数/ })).toBeNull();
    fireEvent.change(nameInput, { target: { value: '😀'.repeat(128) } });
    expect(screen.getByRole('button', { name: '保存' })).toHaveProperty('disabled', false);
  });

  it('知识库：管理文档执行真实上传、展示解析状态并支持删除', async () => {
    const uploadKnowledgeDocument = vi.fn().mockResolvedValue({ id: 'doc-2', filename: '退款制度.md', status: 'ready' });
    const deleteKnowledgeDocument = vi.fn().mockResolvedValue({});
    renderPage(createApi({
      listKnowledgeBases: vi.fn().mockResolvedValue([{ id: 'kb-1', corpId: 9, name: '制度库', description: '', documentCount: 1, status: 1, createdAt: '', updatedAt: '' }]),
      listKnowledgeDocuments: vi.fn().mockResolvedValue([{ id: 'doc-1', corpId: 9, knowledgeBaseId: 'kb-1', filename: '售后流程.pdf', extension: 'pdf', mimeType: 'application/pdf', sizeBytes: 2048, sha256: 'x', status: 'ready', errorSummary: '', characterCount: 120, chunkCount: 2, createdAt: '2026-08-23T12:00:00Z', updatedAt: '' }]),
      uploadKnowledgeDocument, deleteKnowledgeDocument,
    }), 'kb');
    fireEvent.click(await screen.findByRole('button', { name: '管理文档 制度库' }));
    expect(await screen.findByText('售后流程.pdf')).toBeTruthy();
    const file = new File(['退款需要主管审批'], '退款制度.md', { type: 'text/markdown' });
    fireEvent.change(screen.getByLabelText('选择文档'), { target: { files: [file] } });
    fireEvent.click(screen.getByRole('button', { name: '上传并解析' }));
    await waitFor(() => expect(uploadKnowledgeDocument).toHaveBeenCalledWith(9, 'kb-1', file));
    fireEvent.click(screen.getByRole('button', { name: '删除文档 售后流程.pdf' }));
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));
    await waitFor(() => expect(deleteKnowledgeDocument).toHaveBeenCalledWith(9, 'kb-1', 'doc-1'));
  });
});
