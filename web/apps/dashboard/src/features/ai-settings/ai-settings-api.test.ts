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
    expect(request).toHaveBeenCalledWith('/ai-settings/knowledge-bases?corpId=9', expect.objectContaining({
      method: 'POST', body: JSON.stringify({ name: '售后话术库', description: '服务团队', documentCount: 3, status: 1, corpId: 9 }),
    }));
  });

  it('uses the knowledge-base update contract', async () => {
    const request = vi.fn().mockResolvedValue({});
    await createAISettingsApi({ request }).updateKnowledgeBase(9, 'kb-1', { name: '售后话术库', description: '', documentCount: 0, status: 0 });
    expect(request).toHaveBeenCalledWith('/ai-settings/knowledge-bases/kb-1?corpId=9', expect.objectContaining({
      method: 'PUT', body: JSON.stringify({ name: '售后话术库', description: '', documentCount: 0, status: 0, corpId: 9 }),
    }));
  });

  it('uses the knowledge-base delete contract', async () => {
    const request = vi.fn().mockResolvedValue(undefined);
    await createAISettingsApi({ request }).deleteKnowledgeBase(9, 'kb-1');
    expect(request).toHaveBeenCalledWith('/ai-settings/knowledge-bases/kb-1?corpId=9', { method: 'DELETE' });
  });

  it('uses the agent read contract', async () => {
    const request = vi.fn().mockResolvedValue([]);
    await createAISettingsApi({ request }).listAgents(9);
    expect(request).toHaveBeenCalledWith('/ai-settings/agents?corpId=9');
  });

  it('uses the agent create contract', async () => {
    const request = vi.fn().mockResolvedValue({});
    await createAISettingsApi({ request }).createAgent(9, { name: '客服助手', description: '处理售后问题', knowledgeBaseIds: ['kb-1'], status: 1 });
    expect(request).toHaveBeenCalledWith('/ai-settings/agents?corpId=9', expect.objectContaining({
      method: 'POST', body: JSON.stringify({ name: '客服助手', description: '处理售后问题', knowledgeBaseIds: ['kb-1'], status: 1, corpId: 9 }),
    }));
  });

  it('persists an explicit disabled agent status', async () => {
    const request = vi.fn().mockResolvedValue({});
    await createAISettingsApi({ request }).updateAgent(9, 'agent-1', { name: '客服助手', description: '', knowledgeBaseIds: [], status: 0 });
    expect(request).toHaveBeenCalledWith('/ai-settings/agents/agent-1?corpId=9', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: '客服助手', description: '', knowledgeBaseIds: [], status: 0, corpId: 9 }),
    });
  });

  it('uses the agent delete contract', async () => {
    const request = vi.fn().mockResolvedValue(undefined);
    await createAISettingsApi({ request }).deleteAgent(9, 'agent-1');
    expect(request).toHaveBeenCalledWith('/ai-settings/agents/agent-1?corpId=9', { method: 'DELETE' });
  });
});
