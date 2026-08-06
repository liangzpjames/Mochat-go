export type KnowledgeBaseItem = {
  id: string; corpId: number; name: string; description: string;
  documentCount: number; status: number; createdAt: string; updatedAt: string;
};
export type AgentItem = {
  id: string; corpId: number; name: string; description: string;
  knowledgeBaseIds: string[]; status: number; createdAt: string; updatedAt: string;
};

type Client = { request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown> };
const jsonRequest = (method: string, body: unknown): RequestInit => ({
  method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body),
});

export function createAISettingsApi(client: Client) {
  const knowledgeBasePath = (corpId: number, id?: string) =>
    `/ai-settings/knowledge-bases${id ? `/${id}` : ''}?corpId=${corpId}`;
  const agentPath = (corpId: number, id?: string) =>
    `/ai-settings/agents${id ? `/${id}` : ''}?corpId=${corpId}`;
  return {
    listKnowledgeBases(corpId: number): Promise<KnowledgeBaseItem[]> {
      return client.request(knowledgeBasePath(corpId)) as Promise<KnowledgeBaseItem[]>;
    },
    createKnowledgeBase(corpId: number, input: { name: string; description: string; documentCount: number; status: number }): Promise<KnowledgeBaseItem> {
      return client.request(knowledgeBasePath(corpId), jsonRequest('POST', { ...input, corpId })) as Promise<KnowledgeBaseItem>;
    },
    updateKnowledgeBase(corpId: number, id: string, input: { name: string; description: string; documentCount: number; status: number }): Promise<KnowledgeBaseItem> {
      return client.request(knowledgeBasePath(corpId, id), jsonRequest('PUT', { ...input, corpId })) as Promise<KnowledgeBaseItem>;
    },
    deleteKnowledgeBase(corpId: number, id: string): Promise<unknown> {
      return client.request(knowledgeBasePath(corpId, id), { method: 'DELETE' });
    },
    listAgents(corpId: number): Promise<AgentItem[]> {
      return client.request(agentPath(corpId)) as Promise<AgentItem[]>;
    },
    createAgent(corpId: number, input: { name: string; description: string; knowledgeBaseIds: string[]; status: number }): Promise<AgentItem> {
      return client.request(agentPath(corpId), jsonRequest('POST', { ...input, corpId })) as Promise<AgentItem>;
    },
    updateAgent(corpId: number, id: string, input: { name: string; description: string; knowledgeBaseIds: string[]; status: number }): Promise<AgentItem> {
      return client.request(agentPath(corpId, id), jsonRequest('PUT', { ...input, corpId })) as Promise<AgentItem>;
    },
    deleteAgent(corpId: number, id: string): Promise<unknown> {
      return client.request(agentPath(corpId, id), { method: 'DELETE' });
    },
  };
}

export type AISettingsApi = ReturnType<typeof createAISettingsApi>;
