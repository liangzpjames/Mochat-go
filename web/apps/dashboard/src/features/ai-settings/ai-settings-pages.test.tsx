import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { MemoryRouter, useLocation } from 'react-router';
import { ApiError } from '@mochat/api-client';
import { KnowledgeBasePage } from './knowledge-base-page';
import { AgentPage } from './agent-page';
import type { AgentItem, AISettingsApi } from './ai-settings-api';
import { DashboardAccessProvider } from '../../app/access-context';

function createApi(overrides: Partial<AISettingsApi> = {}): AISettingsApi {
  return {
    listKnowledgeBases: vi.fn().mockResolvedValue([
      { id: 'kb-session', corpId: 9, name: '会话知识库', description: '', documentCount: 2, status: 1, createdAt: '', updatedAt: '' },
      { id: 'kb-smart', corpId: 9, name: '智能知识库', description: '', documentCount: 4, status: 1, createdAt: '', updatedAt: '' },
    ]),
    createKnowledgeBase: vi.fn().mockResolvedValue({ id: 'kb-1', corpId: 9, name: 'x', description: '', documentCount: 0, status: 1, createdAt: '', updatedAt: '' }),
    updateKnowledgeBase: vi.fn(),
    deleteKnowledgeBase: vi.fn().mockResolvedValue({}),
    listKnowledgeDocuments: vi.fn().mockResolvedValue([]),
    uploadKnowledgeDocument: vi.fn().mockResolvedValue({ id: 'doc-1', filename: '资料.md', status: 'ready' }),
    deleteKnowledgeDocument: vi.fn().mockResolvedValue({}),
    listAgents: vi.fn().mockResolvedValue([sessionAgent(), smartAgent()]),
    createAgent: vi.fn().mockResolvedValue({}),
    updateAgent: vi.fn().mockResolvedValue({}),
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

function sessionAgent(overrides: Partial<AgentItem> = {}): AgentItem {
  return {
    id: 'agent-session',
    corpId: 9,
    name: '会话分析助手',
    systemKey: 'session-analysis',
    description: '用于客户洞察与员工质检',
    knowledgeBaseIds: ['kb-session'],
    knowledgeBaseCount: 1,
    readyDocumentCount: 3,
    status: 1,
    createdAt: '',
    updatedAt: '2026-08-24T00:00:00Z',
    sessionAnalysisRule: {
      id: 11,
      name: '会话分析规则',
      customerAnalysisPrompt: '识别购买意向',
      employeeQaPrompt: '检查服务质量',
      conversationTypes: ['direct'],
      lookbackDays: 14,
      minimumMessages: 3,
      currentVersion: 2,
      updatedAt: '2026-08-24T00:00:00Z',
    },
    ...overrides,
  } as AgentItem;
}

function smartAgent(overrides: Partial<AgentItem> = {}): AgentItem {
  return {
    id: 'agent-smart',
    corpId: 9,
    name: '智能分析助手',
    systemKey: 'smart-analysis',
    description: '用于识别业务信号',
    knowledgeBaseIds: ['kb-smart'],
    knowledgeBaseCount: 1,
    readyDocumentCount: 4,
    status: 1,
    createdAt: '',
    updatedAt: '2026-08-24T00:00:00Z',
    smartAnalysisRule: {
      id: 22,
      name: '智能分析规则',
      objective: '识别商机与流失风险',
      conversationTypes: ['direct'],
      lookbackDays: 7,
      minimumMessages: 4,
      currentVersion: 5,
      updatedAt: '2026-08-24T00:00:00Z',
    },
    ...overrides,
  } as AgentItem;
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
  it('分析助手页展示双助手卡片与固定说明，不再出现默认术语', async () => {
    renderPage(createApi(), 'agent');
    const sessionHeading = await screen.findByRole('heading', { name: '会话分析助手' });
    const sessionCard = sessionHeading.closest('.ai-assistant-card') as HTMLElement;
    expect(sessionHeading).toBeTruthy();
    expect(screen.getByRole('heading', { name: '智能分析助手' })).toBeTruthy();
    expect(screen.getByText('分别配置会话分析与智能分析使用的固定系统助手')).toBeTruthy();
    expect(screen.getByText(/AI 设置管配置、AI 洞察看结果/)).toBeTruthy();
    expect(within(sessionCard).getByText('会话范围')).toBeTruthy();
    expect(within(sessionCard).getByText('客户单聊')).toBeTruthy();
    expect(within(sessionCard).getByText('回看 14 天 · 至少 3 条消息')).toBeTruthy();
    expect(within(sessionCard).queryByText('关联知识库摘要')).toBeNull();
    expect(within(sessionCard).queryByText('文档就绪摘要')).toBeNull();
    expect(screen.queryByText('默认智能体')).toBeNull();
    expect(screen.queryByText('默认智能分析规则')).toBeNull();
    expect(screen.queryByText('同一个助手服务两页')).toBeNull();
  });

  it('分析助手首屏失败时保持诚实错误态，不补助手卡片', async () => {
    renderPage(createApi({ listAgents: vi.fn().mockRejectedValue(new Error('boom')) }), 'agent');
    expect((await screen.findByRole('alert')).textContent).toContain('数据加载失败');
    expect(screen.queryByRole('heading', { name: '会话分析助手' })).toBeNull();
    expect(screen.queryByRole('heading', { name: '智能分析助手' })).toBeNull();
  });

  it('会话分析助手会话范围支持整卡点击、窗口编辑，并只提交互斥的会话规则字段', async () => {
    const updateAgent = vi.fn().mockResolvedValue({});
    renderPage(createApi({ updateAgent }), 'agent');

    fireEvent.click(await screen.findByRole('button', { name: '编辑配置 会话分析助手' }));
    const dialog = screen.getByRole('dialog', { name: '配置会话分析助手' });
    const groupCard = within(dialog).getByText('客户群聊').closest('.ai-conversation-scope-option') as HTMLElement;
    const directCheckbox = within(dialog).getByRole('checkbox', { name: '客户单聊' });
    const groupCheckbox = within(dialog).getByRole('checkbox', { name: '客户群聊' });
    fireEvent.click(groupCard);
    expect(groupCheckbox).toHaveProperty('checked', true);
    fireEvent.click(directCheckbox);
    expect(directCheckbox).toHaveProperty('checked', false);
    expect(groupCheckbox).toHaveProperty('checked', true);
    expect(groupCard.className).toContain('ai-conversation-scope-option--selected');
    fireEvent.change(screen.getByRole('textbox', { name: '客户分析提示词' }), { target: { value: '重点识别复购机会' } });
    fireEvent.change(screen.getByRole('textbox', { name: '员工质检提示词' }), { target: { value: '检查异议和未解决问题' } });
    fireEvent.change(screen.getByRole('spinbutton', { name: '回看天数' }), { target: { value: '21' } });
    fireEvent.change(screen.getByRole('spinbutton', { name: '最少消息数' }), { target: { value: '5' } });
    fireEvent.change(screen.getByRole('combobox', { name: '运行状态' }), { target: { value: '0' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(updateAgent).toHaveBeenCalledWith(9, 'agent-session', {
      name: '会话分析助手',
      description: '用于客户洞察与员工质检',
      knowledgeBaseIds: ['kb-session'],
      status: 0,
      sessionAnalysisRule: {
        customerAnalysisPrompt: '重点识别复购机会',
        employeeQaPrompt: '检查异议和未解决问题',
        conversationTypes: ['group'],
        lookbackDays: 21,
        minimumMessages: 5,
      },
    }));
    expect(updateAgent.mock.calls[0]?.[2]).not.toHaveProperty('smartAnalysisRule');
    expect(await screen.findByText(/会话分析助手配置已保存/)).toBeTruthy();
  });

  it('两个分析助手编辑器使用宽屏横向信息布局', async () => {
    renderPage(createApi(), 'agent');

    fireEvent.click(await screen.findByRole('button', { name: '编辑配置 会话分析助手' }));
    const sessionDialog = screen.getByRole('dialog', { name: '配置会话分析助手' });
    expect(sessionDialog.querySelector('.ai-assistant-editor-form')).toBeTruthy();
    expect(sessionDialog.querySelectorAll('.ai-assistant-editor-section--primary')).toHaveLength(3);
    expect(sessionDialog.querySelector('.ai-assistant-editor-section--prompts')).toBeTruthy();
    expect(sessionDialog.querySelector('.ai-assistant-prompt-grid')).toBeTruthy();
    fireEvent.click(within(sessionDialog).getByRole('button', { name: '取消' }));
    await waitFor(() => expect(screen.queryByRole('dialog', { name: '配置会话分析助手' })).toBeNull());

    fireEvent.click(await screen.findByRole('button', { name: '编辑配置 智能分析助手' }));
    const smartDialog = screen.getByRole('textbox', { name: '智能分析目标' }).closest('[role="dialog"]') as HTMLElement;
    expect(smartDialog.querySelector('.ai-assistant-editor-form')).toBeTruthy();
    expect(smartDialog.querySelectorAll('.ai-assistant-editor-section--primary')).toHaveLength(3);
  });

  it('会话分析助手的小数规则窗口在前端即视为无效，不发起保存', async () => {
    const updateAgent = vi.fn().mockResolvedValue({});
    renderPage(createApi({ updateAgent }), 'agent');

    fireEvent.click(await screen.findByRole('button', { name: '编辑配置 会话分析助手' }));
    fireEvent.change(screen.getByRole('spinbutton', { name: '回看天数' }), { target: { value: '7.5' } });
    fireEvent.change(screen.getByRole('spinbutton', { name: '最少消息数' }), { target: { value: '4.2' } });

    const save = screen.getByRole('button', { name: '保存' });
    expect(save).toHaveProperty('disabled', true);
    fireEvent.click(save);
    expect(updateAgent).not.toHaveBeenCalled();
  });

  it('智能分析助手会话范围支持整卡点击、键盘切换和互斥保存', async () => {
    const updateAgent = vi.fn().mockResolvedValue({});
    renderPage(createApi({ updateAgent }), 'agent');

    fireEvent.click(await screen.findByRole('button', { name: '编辑配置 智能分析助手' }));
    const groupCard = screen.getByText('客户群聊').closest('.ai-conversation-scope-option') as HTMLElement;
    const groupCheckbox = screen.getByRole('checkbox', { name: '客户群聊' });
    groupCard.focus();
    fireEvent.keyDown(groupCard, { key: 'Enter', code: 'Enter' });
    expect(groupCheckbox).toHaveProperty('checked', true);
    expect(groupCard.className).toContain('ai-conversation-scope-option--selected');

    fireEvent.change(screen.getByRole('textbox', { name: '智能分析目标' }), { target: { value: '识别高意向和流失信号' } });
    fireEvent.change(screen.getByRole('spinbutton', { name: '回看天数' }), { target: { value: '21' } });
    fireEvent.change(screen.getByRole('spinbutton', { name: '最少消息数' }), { target: { value: '5' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(updateAgent).toHaveBeenCalledWith(9, 'agent-smart', {
      name: '智能分析助手',
      description: '用于识别业务信号',
      knowledgeBaseIds: ['kb-smart'],
      status: 1,
      smartAnalysisRule: {
        objective: '识别高意向和流失信号',
        conversationTypes: ['direct', 'group'],
        lookbackDays: 21,
        minimumMessages: 5,
      },
    }));
    expect(updateAgent.mock.calls[0]?.[2]).not.toHaveProperty('sessionAnalysisRule');
  });

  it('智能分析助手的小数规则窗口在前端即视为无效，不发起保存', async () => {
    const updateAgent = vi.fn().mockResolvedValue({});
    renderPage(createApi({ updateAgent }), 'agent');

    fireEvent.click(await screen.findByRole('button', { name: '编辑配置 智能分析助手' }));
    fireEvent.change(screen.getByRole('spinbutton', { name: '回看天数' }), { target: { value: '7.5' } });
    fireEvent.change(screen.getByRole('spinbutton', { name: '最少消息数' }), { target: { value: '4.2' } });

    const save = screen.getByRole('button', { name: '保存' });
    expect(save).toHaveProperty('disabled', true);
    fireEvent.click(save);
    expect(updateAgent).not.toHaveBeenCalled();
  });

  it('两次保存互不串改，切换编辑器不会带入另一张卡的草稿', async () => {
    const updateAgent = vi.fn().mockResolvedValue({});
    renderPage(createApi({ updateAgent }), 'agent');

    fireEvent.click(await screen.findByRole('button', { name: '编辑配置 会话分析助手' }));
    fireEvent.change(screen.getByRole('textbox', { name: '客户分析提示词' }), { target: { value: '识别二次购买意向' } });
    fireEvent.click(screen.getByText('客户群聊').closest('.ai-conversation-scope-option') as HTMLElement);
    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => expect(updateAgent).toHaveBeenCalledTimes(1));

    fireEvent.click(await screen.findByRole('button', { name: '编辑配置 智能分析助手' }));
    expect(screen.queryByRole('textbox', { name: '客户分析提示词' })).toBeNull();
    expect(screen.getByRole('textbox', { name: '智能分析目标' })).toHaveProperty('value', '识别商机与流失风险');
    expect(screen.getByRole('checkbox', { name: '客户群聊' })).toHaveProperty('checked', false);
    fireEvent.change(screen.getByRole('textbox', { name: '智能分析目标' }), { target: { value: '识别沉默客户' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(updateAgent).toHaveBeenCalledTimes(2));
    expect(updateAgent.mock.calls[0]?.[1]).toBe('agent-session');
    expect(updateAgent.mock.calls[1]?.[1]).toBe('agent-smart');
    expect(updateAgent.mock.calls[1]?.[2]).not.toHaveProperty('sessionAnalysisRule');
  });

  it('刷新按钮会同时刷新助手与知识库，并回读真实结果', async () => {
    const listAgents = vi.fn()
      .mockResolvedValueOnce([sessionAgent(), smartAgent()])
      .mockResolvedValueOnce([sessionAgent({ description: '刷新后的会话说明' }), smartAgent({ description: '刷新后的智能说明' })]);
    const listKnowledgeBases = vi.fn()
      .mockResolvedValueOnce([
        { id: 'kb-session', corpId: 9, name: '会话知识库', description: '', documentCount: 2, status: 1, createdAt: '', updatedAt: '' },
        { id: 'kb-smart', corpId: 9, name: '智能知识库', description: '', documentCount: 4, status: 1, createdAt: '', updatedAt: '' },
      ])
      .mockResolvedValueOnce([
        { id: 'kb-session', corpId: 9, name: '新的会话知识库', description: '', documentCount: 2, status: 1, createdAt: '', updatedAt: '' },
        { id: 'kb-smart', corpId: 9, name: '新的智能知识库', description: '', documentCount: 4, status: 1, createdAt: '', updatedAt: '' },
      ]);
    renderPage(createApi({ listAgents, listKnowledgeBases }), 'agent');

    await screen.findByText('用于客户洞察与员工质检');
    fireEvent.click(screen.getByRole('button', { name: '刷新 会话分析助手' }));

    await waitFor(() => expect(listAgents).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(listKnowledgeBases).toHaveBeenCalledTimes(2));
    expect(await screen.findByText('刷新后的会话说明')).toBeTruthy();
    expect(screen.getAllByText('新的会话知识库')).toHaveLength(1);
  });

  it('更新失败时保留输入并映射稳定机器码', async () => {
    const updateAgent = vi.fn().mockRejectedValue(new ApiError('validation', 'invalid json', { status: 400, machineCode: 'AI_SETTINGS_INVALID_JSON' }));
    renderPage(createApi({ updateAgent }), 'agent');

    fireEvent.click(await screen.findByRole('button', { name: '编辑配置 智能分析助手' }));
    fireEvent.change(screen.getByRole('textbox', { name: '智能分析目标' }), { target: { value: '失败后保留输入' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    expect((await screen.findByRole('alert')).textContent).toContain('请求格式无效，请刷新后重试。');
    expect(screen.getByRole('textbox', { name: '智能分析目标' })).toHaveProperty('value', '失败后保留输入');
  });

  it('编辑器允许保留既有关联的停用知识库，但阻止新增其他停用知识库', async () => {
    renderPage(createApi({
      listKnowledgeBases: vi.fn().mockResolvedValue([
        { id: 'kb-smart', corpId: 9, name: '既有关联停用库', description: '', documentCount: 0, status: 0, createdAt: '', updatedAt: '' },
        { id: 'kb-disabled', corpId: 9, name: '其他停用库', description: '', documentCount: 0, status: 0, createdAt: '', updatedAt: '' },
      ]),
      listAgents: vi.fn().mockResolvedValue([
        sessionAgent(),
        smartAgent({ knowledgeBaseIds: ['kb-smart', 'missing-kb'] }),
      ]),
    }), 'agent');

    fireEvent.click(await screen.findByRole('button', { name: '编辑配置 智能分析助手' }));
    const existing = screen.getByRole('checkbox', { name: /既有关联停用库/ });
    const disabled = screen.getByRole('checkbox', { name: /其他停用库/ });
    expect(existing).toHaveProperty('checked', true);
    expect(existing).toHaveProperty('disabled', false);
    expect(disabled).toHaveProperty('disabled', true);
    expect(screen.getAllByText(/已失效（ID: missing-kb）/).length).toBeGreaterThan(0);
  });

  it('未保存更改时 Escape 会触发二次确认并允许放弃', async () => {
    renderPage(createApi(), 'agent');

    fireEvent.click(await screen.findByRole('button', { name: '编辑配置 智能分析助手' }));
    fireEvent.change(screen.getByRole('textbox', { name: '智能分析目标' }), { target: { value: '准备放弃的修改' } });
    fireEvent.keyDown(document, { key: 'Escape', code: 'Escape' });

    expect(await screen.findByText('放弃未保存更改？')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '放弃更改' }));
    await waitFor(() => expect(screen.queryByRole('dialog', { name: '配置智能分析助手' })).toBeNull());
  });

  it('知识库筛选区刷新按钮使用统一 secondary action 样式类', async () => {
    renderPage(createApi(), 'kb');
    const refresh = await screen.findByRole('button', { name: '刷新' });
    expect(refresh.className).toContain('dashboard-secondary-action');
  });

  it('新建知识库不选文档时只创建知识库', async () => {
    const createKnowledgeBase = vi.fn().mockResolvedValue({ id: 'kb-plain', corpId: 9, name: '售后知识库', description: '', documentCount: 0, status: 1, createdAt: '', updatedAt: '' });
    const uploadKnowledgeDocument = vi.fn();
    const api = createApi({ createKnowledgeBase, uploadKnowledgeDocument });
    renderPage(api, 'kb');

    fireEvent.click(await screen.findByRole('button', { name: '新建知识库' }));
    fireEvent.change(screen.getByRole('textbox', { name: '名称' }), { target: { value: '售后知识库' } });
    expect(screen.getByLabelText('初始文档（可选）')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(createKnowledgeBase).toHaveBeenCalledTimes(1));
    expect(uploadKnowledgeDocument).not.toHaveBeenCalled();
    expect(await screen.findByText('知识库已创建。')).toBeTruthy();
  });

  it('新建知识库可选择文档并使用新知识库 ID 上传', async () => {
    const uploadKnowledgeDocument = vi.fn().mockResolvedValue({ id: 'doc-created', filename: '产品资料.md', status: 'ready' });
    const api = createApi({
      createKnowledgeBase: vi.fn().mockResolvedValue({ id: 'kb-created', corpId: 9, name: '产品资料库', description: '', documentCount: 0, status: 1, createdAt: '', updatedAt: '' }),
      uploadKnowledgeDocument,
    });
    renderPage(api, 'kb');
    const file = new File(['# 产品资料'], '产品资料.md', { type: 'text/markdown' });

    fireEvent.click(await screen.findByRole('button', { name: '新建知识库' }));
    fireEvent.change(screen.getByRole('textbox', { name: '名称' }), { target: { value: '产品资料库' } });
    fireEvent.change(screen.getByLabelText('初始文档（可选）'), { target: { files: [file] } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(uploadKnowledgeDocument).toHaveBeenCalledWith(9, 'kb-created', file));
    expect(await screen.findByText(/知识库“产品资料库”已创建，文档“产品资料\.md”已解析/)).toBeTruthy();
  });

  it('知识库创建成功但初始文档上传失败时保留知识库并提供重试指引', async () => {
    const deleteKnowledgeBase = vi.fn();
    const api = createApi({
      createKnowledgeBase: vi.fn().mockResolvedValue({ id: 'kb-partial', corpId: 9, name: '部分成功库', description: '', documentCount: 0, status: 1, createdAt: '', updatedAt: '' }),
      uploadKnowledgeDocument: vi.fn().mockRejectedValue(new ApiError('validation', 'too large', { status: 400, machineCode: 'AI_SETTINGS_DOCUMENT_TOO_LARGE' })),
      deleteKnowledgeBase,
    });
    renderPage(api, 'kb');
    const file = new File(['content'], '过大资料.md', { type: 'text/markdown' });

    fireEvent.click(await screen.findByRole('button', { name: '新建知识库' }));
    fireEvent.change(screen.getByRole('textbox', { name: '名称' }), { target: { value: '部分成功库' } });
    fireEvent.change(screen.getByLabelText('初始文档（可选）'), { target: { files: [file] } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    const feedback = await screen.findByRole('alert');
    expect(feedback.textContent).toContain('知识库“部分成功库”已创建');
    expect(feedback.textContent).toContain('管理文档');
    expect(deleteKnowledgeBase).not.toHaveBeenCalled();
    expect(screen.queryByRole('dialog', { name: '新建知识库' })).toBeNull();
  });

  it('新建知识库只选择文档也会触发未保存确认', async () => {
    renderPage(createApi(), 'kb');
    const file = new File(['draft'], '草稿.md', { type: 'text/markdown' });

    fireEvent.click(await screen.findByRole('button', { name: '新建知识库' }));
    fireEvent.change(screen.getByLabelText('初始文档（可选）'), { target: { files: [file] } });
    fireEvent.keyDown(document, { key: 'Escape', code: 'Escape' });

    expect(await screen.findByText('放弃未保存更改？')).toBeTruthy();
  });
});
