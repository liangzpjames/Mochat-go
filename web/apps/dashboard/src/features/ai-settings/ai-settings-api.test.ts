import { describe, expect, it, vi } from 'vitest';
import { createAISettingsApi } from './ai-settings-api';

describe('AI 设置 API', () => {
  it('uses the knowledge-base read contract', async () => {
    const request = vi.fn().mockResolvedValue([]);
    await createAISettingsApi({ request }).listKnowledgeBases(9);
    expect(request).toHaveBeenCalledWith('/ai-settings/knowledge-bases?corpId=9');
  });

  it('uses the knowledge-base create contract', async () => {
    const request = vi.fn().mockResolvedValue({});
    await createAISettingsApi({ request }).createKnowledgeBase(9, { name: '售后话术库', description: '服务团队', documentCount: 3, status: 1 });
    expect(request).toHaveBeenCalledWith('/ai-settings/knowledge-bases?corpId=9', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: '售后话术库', description: '服务团队', documentCount: 3, status: 1 }),
    });
  });

  it('uses the knowledge-base update contract', async () => {
    const request = vi.fn().mockResolvedValue({});
    await createAISettingsApi({ request }).updateKnowledgeBase(9, 'kb-1', { name: '售后话术库', description: '', documentCount: 0, status: 0 });
    expect(request).toHaveBeenCalledWith('/ai-settings/knowledge-bases/kb-1?corpId=9', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: '售后话术库', description: '', documentCount: 0, status: 0 }),
    });
  });

  it('uses the knowledge-base delete contract', async () => {
    const request = vi.fn().mockResolvedValue(undefined);
    await createAISettingsApi({ request }).deleteKnowledgeBase(9, 'kb-1');
    expect(request).toHaveBeenCalledWith('/ai-settings/knowledge-bases/kb-1?corpId=9', { method: 'DELETE' });
  });

  it('uses real multipart document contracts without overriding the content type', async () => {
    const request = vi.fn().mockResolvedValue([]);
    const api = createAISettingsApi({ request });
    await api.listKnowledgeDocuments(9, 'kb-1');
    expect(request).toHaveBeenLastCalledWith('/ai-settings/knowledge-bases/kb-1/documents?corpId=9');
    const file = new File(['退款制度'], '退款制度.md', { type: 'text/markdown' });
    await api.uploadKnowledgeDocument(9, 'kb-1', file);
    const calls = request.mock.calls as unknown as Array<[RequestInfo | URL, RequestInit?]>;
    const init = calls.at(-1)?.[1];
    expect(init).toMatchObject({ method: 'POST' });
    expect(init?.headers).toBeUndefined();
    expect(init?.body).toBeInstanceOf(FormData);
    expect((init?.body as FormData).get('file')).toBe(file);
    await api.deleteKnowledgeDocument(9, 'kb-1', 'doc-1');
    expect(request).toHaveBeenLastCalledWith('/ai-settings/knowledge-bases/kb-1/documents/doc-1?corpId=9', { method: 'DELETE' });
  });

  it('parses the two fixed assistants from the real contract', async () => {
    const request = vi.fn().mockResolvedValue([
      {
        id: 'agent-session',
        corpId: 9,
        name: '会话分析助手',
        systemKey: 'session-analysis',
        description: '客户洞察与员工质检',
        knowledgeBaseIds: ['kb-session'],
        knowledgeBaseCount: 1,
        readyDocumentCount: 5,
        status: 1,
        createdAt: '',
        updatedAt: '',
        sessionAnalysisRule: {
          id: 12,
          name: '会话分析规则',
          customerAnalysisPrompt: '识别购买意向',
          employeeQaPrompt: '检查服务质量',
          conversationTypes: ['direct'],
          lookbackDays: 14,
          minimumMessages: 3,
          currentVersion: 2,
          updatedAt: '2026-08-24T00:00:00Z',
        },
      },
      {
        id: 'agent-smart',
        corpId: 9,
        name: '智能分析助手',
        systemKey: 'smart-analysis',
        description: '识别业务信号',
        knowledgeBaseIds: ['kb-smart'],
        knowledgeBaseCount: 2,
        readyDocumentCount: 8,
        status: 1,
        createdAt: '',
        updatedAt: '',
        smartAnalysisRule: {
          id: 34,
          name: '智能分析规则',
          objective: '识别复购和流失风险',
          conversationTypes: ['direct', 'group'],
          lookbackDays: 7,
          minimumMessages: 4,
          currentVersion: 5,
          updatedAt: '2026-08-24T00:00:00Z',
        },
      },
    ]);

    const agents = await createAISettingsApi({ request }).listAgents(9);
    expect(agents).toHaveLength(2);
    expect(agents[0]).toMatchObject({
      id: 'agent-session',
      systemKey: 'session-analysis',
      sessionAnalysisRule: {
        customerAnalysisPrompt: '识别购买意向',
        employeeQaPrompt: '检查服务质量',
      },
    });
    expect(agents[1]).toMatchObject({
      id: 'agent-smart',
      systemKey: 'smart-analysis',
      smartAnalysisRule: {
        objective: '识别复购和流失风险',
        conversationTypes: ['direct', 'group'],
      },
    });
  });

  it('fails honestly when one fixed assistant is missing', async () => {
    const request = vi.fn().mockResolvedValue([
      {
        id: 'agent-session',
        corpId: 9,
        name: '会话分析助手',
        systemKey: 'session-analysis',
        description: '客户洞察与员工质检',
        knowledgeBaseIds: [],
        knowledgeBaseCount: 0,
        readyDocumentCount: 0,
        status: 1,
        createdAt: '',
        updatedAt: '',
        sessionAnalysisRule: {
          id: 12,
          name: '会话分析规则',
          customerAnalysisPrompt: '识别购买意向',
          employeeQaPrompt: '检查服务质量',
          conversationTypes: ['direct'],
          lookbackDays: 14,
          minimumMessages: 3,
          currentVersion: 2,
          updatedAt: '2026-08-24T00:00:00Z',
        },
      },
    ]);

    await expect(createAISettingsApi({ request }).listAgents(9)).rejects.toThrow(/系统助手返回异常/);
  });

  it('fails honestly when rule types are crossed or duplicated', async () => {
    const request = vi.fn().mockResolvedValue([
      {
        id: 'agent-session',
        corpId: 9,
        name: '会话分析助手',
        systemKey: 'session-analysis',
        description: '客户洞察与员工质检',
        knowledgeBaseIds: [],
        knowledgeBaseCount: 0,
        readyDocumentCount: 0,
        status: 1,
        createdAt: '',
        updatedAt: '',
        sessionAnalysisRule: {
          id: 12,
          name: '会话分析规则',
          customerAnalysisPrompt: '识别购买意向',
          employeeQaPrompt: '检查服务质量',
          conversationTypes: ['direct'],
          lookbackDays: 14,
          minimumMessages: 3,
          currentVersion: 2,
          updatedAt: '2026-08-24T00:00:00Z',
        },
        smartAnalysisRule: {
          id: 99,
          name: '错误规则',
          objective: '不该出现在会话助手',
          conversationTypes: ['group'],
          lookbackDays: 7,
          minimumMessages: 4,
          currentVersion: 1,
          updatedAt: '2026-08-24T00:00:00Z',
        },
      },
      {
        id: 'agent-smart',
        corpId: 9,
        name: '智能分析助手',
        systemKey: 'smart-analysis',
        description: '识别业务信号',
        knowledgeBaseIds: [],
        knowledgeBaseCount: 0,
        readyDocumentCount: 0,
        status: 1,
        createdAt: '',
        updatedAt: '',
        smartAnalysisRule: {
          id: 34,
          name: '智能分析规则',
          objective: '识别复购和流失风险',
          conversationTypes: ['direct', 'group'],
          lookbackDays: 7,
          minimumMessages: 4,
          currentVersion: 5,
          updatedAt: '2026-08-24T00:00:00Z',
        },
      },
    ]);

    await expect(createAISettingsApi({ request }).listAgents(9)).rejects.toThrow(/系统助手返回异常/);
  });

  it('updates the session-analysis assistant with a mutually exclusive session rule payload', async () => {
    const request = vi.fn().mockResolvedValue({});
    await createAISettingsApi({ request }).updateAgent(9, 'agent-session', {
      name: '会话分析助手',
      description: '客户洞察与员工质检',
      knowledgeBaseIds: ['kb-session'],
      status: 0,
      sessionAnalysisRule: {
        customerAnalysisPrompt: '识别复购意向',
        employeeQaPrompt: '检查异议处理',
        conversationTypes: ['direct'],
        lookbackDays: 14,
        minimumMessages: 3,
      },
    });
    expect(request).toHaveBeenCalledWith('/ai-settings/agents/agent-session?corpId=9', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: '会话分析助手',
        description: '客户洞察与员工质检',
        knowledgeBaseIds: ['kb-session'],
        status: 0,
        sessionAnalysisRule: {
          customerAnalysisPrompt: '识别复购意向',
          employeeQaPrompt: '检查异议处理',
          conversationTypes: ['direct'],
          lookbackDays: 14,
          minimumMessages: 3,
        },
      }),
    });
  });

  it('updates the smart-analysis assistant with a mutually exclusive smart rule payload', async () => {
    const request = vi.fn().mockResolvedValue({});
    await createAISettingsApi({ request }).updateAgent(9, 'agent-smart', {
      name: '智能分析助手',
      description: '识别业务信号',
      knowledgeBaseIds: ['kb-smart'],
      status: 1,
      smartAnalysisRule: {
        objective: '识别商机与流失风险',
        conversationTypes: ['direct', 'group'],
        lookbackDays: 21,
        minimumMessages: 5,
      },
    });
    expect(request).toHaveBeenCalledWith('/ai-settings/agents/agent-smart?corpId=9', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: '智能分析助手',
        description: '识别业务信号',
        knowledgeBaseIds: ['kb-smart'],
        status: 1,
        smartAnalysisRule: {
          objective: '识别商机与流失风险',
          conversationTypes: ['direct', 'group'],
          lookbackDays: 21,
          minimumMessages: 5,
        },
      }),
    });
  });

  it('uses the agent delete contract', async () => {
    const request = vi.fn().mockResolvedValue(undefined);
    await createAISettingsApi({ request }).deleteAgent(9, 'agent-1');
    expect(request).toHaveBeenCalledWith('/ai-settings/agents/agent-1?corpId=9', { method: 'DELETE' });
  });
});
