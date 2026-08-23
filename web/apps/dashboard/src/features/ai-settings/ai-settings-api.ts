export type KnowledgeBaseItem = {
  id: string; corpId: number; name: string; description: string;
  documentCount: number; status: number; createdAt: string; updatedAt: string;
};
export type AgentItem = {
  id: string; corpId: number; name: string; description: string;
  systemKey?: string; knowledgeBaseIds: string[]; status: number; createdAt: string; updatedAt: string;
};
export type KnowledgeDocumentItem = {
  id: string; corpId: number; knowledgeBaseId: string; filename: string; extension: string;
  mimeType: string; sizeBytes: number; sha256: string; status: 'ready' | 'failed'; errorSummary: string;
  characterCount: number; chunkCount: number; createdAt: string; updatedAt: string;
};
export type KnowledgeBaseInput = {
  name: string; description: string; documentCount: number; status: number;
};
export type AgentInput = {
  name: string; description: string; knowledgeBaseIds: string[]; status: number;
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
  const documentPath = (corpId: number, knowledgeBaseId: string, documentId?: string) =>
    `/ai-settings/knowledge-bases/${knowledgeBaseId}/documents${documentId ? `/${documentId}` : ''}?corpId=${corpId}`;
  return {
    listKnowledgeBases(corpId: number): Promise<KnowledgeBaseItem[]> {
      return client.request(knowledgeBasePath(corpId)) as Promise<KnowledgeBaseItem[]>;
    },
    createKnowledgeBase(corpId: number, input: KnowledgeBaseInput): Promise<KnowledgeBaseItem> {
      return client.request(knowledgeBasePath(corpId), jsonRequest('POST', input)) as Promise<KnowledgeBaseItem>;
    },
    updateKnowledgeBase(corpId: number, id: string, input: KnowledgeBaseInput): Promise<KnowledgeBaseItem> {
      return client.request(knowledgeBasePath(corpId, id), jsonRequest('PUT', input)) as Promise<KnowledgeBaseItem>;
    },
    deleteKnowledgeBase(corpId: number, id: string): Promise<unknown> {
      return client.request(knowledgeBasePath(corpId, id), { method: 'DELETE' });
    },
    listAgents(corpId: number): Promise<AgentItem[]> {
      return client.request(agentPath(corpId)) as Promise<AgentItem[]>;
    },
    listKnowledgeDocuments(corpId: number, knowledgeBaseId: string): Promise<KnowledgeDocumentItem[]> {
      return client.request(documentPath(corpId, knowledgeBaseId)) as Promise<KnowledgeDocumentItem[]>;
    },
    uploadKnowledgeDocument(corpId: number, knowledgeBaseId: string, file: File): Promise<KnowledgeDocumentItem> {
      const body = new FormData();
      body.set('file', file);
      return client.request(documentPath(corpId, knowledgeBaseId), { method: 'POST', body }) as Promise<KnowledgeDocumentItem>;
    },
    deleteKnowledgeDocument(corpId: number, knowledgeBaseId: string, documentId: string): Promise<unknown> {
      return client.request(documentPath(corpId, knowledgeBaseId, documentId), { method: 'DELETE' });
    },
    createAgent(corpId: number, input: AgentInput): Promise<AgentItem> {
      return client.request(agentPath(corpId), jsonRequest('POST', input)) as Promise<AgentItem>;
    },
    updateAgent(corpId: number, id: string, input: AgentInput): Promise<AgentItem> {
      return client.request(agentPath(corpId, id), jsonRequest('PUT', input)) as Promise<AgentItem>;
    },
    deleteAgent(corpId: number, id: string): Promise<unknown> {
      return client.request(agentPath(corpId, id), { method: 'DELETE' });
    },
  };
}

export type AISettingsApi = ReturnType<typeof createAISettingsApi>;
