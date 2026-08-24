export type KnowledgeBaseItem = {
  id: string; corpId: number; name: string; description: string;
  documentCount: number; status: number; createdAt: string; updatedAt: string;
};
export type ConversationType = 'direct' | 'group';
export type SmartAnalysisRule = {
  id: number; name: string; objective: string; conversationTypes: ConversationType[];
  lookbackDays: number; minimumMessages: number; currentVersion: number; updatedAt: string;
};
export type SessionAnalysisRule = {
  id: number; name: string; customerAnalysisPrompt: string; employeeQaPrompt: string; conversationTypes: ConversationType[];
  lookbackDays: number; minimumMessages: number; currentVersion: number; updatedAt: string;
};
type AgentCommon = {
  id: string; corpId: number; name: string; description: string;
  knowledgeBaseIds: string[]; status: number; createdAt: string; updatedAt: string;
  knowledgeBaseCount: number; readyDocumentCount: number;
};
export type SessionAnalysisAgentItem = AgentCommon & {
  systemKey: 'session-analysis';
  sessionAnalysisRule: SessionAnalysisRule;
  smartAnalysisRule?: never;
};
export type SmartAnalysisAgentItem = AgentCommon & {
  systemKey: 'smart-analysis';
  smartAnalysisRule: SmartAnalysisRule;
  sessionAnalysisRule?: never;
};
export type AgentItem = SessionAnalysisAgentItem | SmartAnalysisAgentItem;
export type KnowledgeDocumentItem = {
  id: string; corpId: number; knowledgeBaseId: string; filename: string; extension: string;
  mimeType: string; sizeBytes: number; sha256: string; status: 'ready' | 'failed'; errorSummary: string;
  characterCount: number; chunkCount: number; createdAt: string; updatedAt: string;
};
export type KnowledgeBaseInput = {
  name: string; description: string; documentCount: number; status: number;
};
type AgentInputBase = {
  name: string; description: string; knowledgeBaseIds: string[]; status: number;
};
export type AgentInput =
  | (AgentInputBase & {
      sessionAnalysisRule: Pick<SessionAnalysisRule, 'customerAnalysisPrompt' | 'employeeQaPrompt' | 'conversationTypes' | 'lookbackDays' | 'minimumMessages'>;
      smartAnalysisRule?: never;
    })
  | (AgentInputBase & {
      smartAnalysisRule: Pick<SmartAnalysisRule, 'objective' | 'conversationTypes' | 'lookbackDays' | 'minimumMessages'>;
      sessionAnalysisRule?: never;
    });

type Client = { request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown> };
const jsonRequest = (method: string, body: unknown): RequestInit => ({
  method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body),
});

const agentParseError = '系统助手返回异常，请刷新后重试。';

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function asString(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function asNumber(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0;
}

function parseConversationTypes(value: unknown): ConversationType[] {
  if (!Array.isArray(value)) throw new Error(agentParseError);
  const types = value.filter((item): item is ConversationType => item === 'direct' || item === 'group');
  if (types.length !== value.length) throw new Error(agentParseError);
  return types;
}

function parseCommonAgent(value: Record<string, unknown>): AgentCommon {
  const knowledgeBaseIds = Array.isArray(value.knowledgeBaseIds)
    ? value.knowledgeBaseIds.filter((item): item is string => typeof item === 'string')
    : [];
  if (Array.isArray(value.knowledgeBaseIds) && knowledgeBaseIds.length !== value.knowledgeBaseIds.length) {
    throw new Error(agentParseError);
  }
  return {
    id: asString(value.id),
    corpId: asNumber(value.corpId),
    name: asString(value.name),
    description: asString(value.description),
    knowledgeBaseIds,
    status: asNumber(value.status),
    createdAt: asString(value.createdAt),
    updatedAt: asString(value.updatedAt),
    knowledgeBaseCount: asNumber(value.knowledgeBaseCount),
    readyDocumentCount: asNumber(value.readyDocumentCount),
  };
}

function parseSessionRule(value: unknown): SessionAnalysisRule {
  if (!isRecord(value)) throw new Error(agentParseError);
  return {
    id: asNumber(value.id),
    name: asString(value.name),
    customerAnalysisPrompt: asString(value.customerAnalysisPrompt),
    employeeQaPrompt: asString(value.employeeQaPrompt),
    conversationTypes: parseConversationTypes(value.conversationTypes),
    lookbackDays: asNumber(value.lookbackDays),
    minimumMessages: asNumber(value.minimumMessages),
    currentVersion: asNumber(value.currentVersion),
    updatedAt: asString(value.updatedAt),
  };
}

function parseSmartRule(value: unknown): SmartAnalysisRule {
  if (!isRecord(value)) throw new Error(agentParseError);
  return {
    id: asNumber(value.id),
    name: asString(value.name),
    objective: asString(value.objective),
    conversationTypes: parseConversationTypes(value.conversationTypes),
    lookbackDays: asNumber(value.lookbackDays),
    minimumMessages: asNumber(value.minimumMessages),
    currentVersion: asNumber(value.currentVersion),
    updatedAt: asString(value.updatedAt),
  };
}

function parseAgent(value: unknown): AgentItem {
  if (!isRecord(value)) throw new Error(agentParseError);
  const common = parseCommonAgent(value);
  if (!common.id || !common.name) throw new Error(agentParseError);
  if (value.systemKey === 'session-analysis') {
    if ('smartAnalysisRule' in value || !('sessionAnalysisRule' in value)) throw new Error(agentParseError);
    return { ...common, systemKey: 'session-analysis', sessionAnalysisRule: parseSessionRule(value.sessionAnalysisRule) };
  }
  if (value.systemKey === 'smart-analysis') {
    if ('sessionAnalysisRule' in value || !('smartAnalysisRule' in value)) throw new Error(agentParseError);
    return { ...common, systemKey: 'smart-analysis', smartAnalysisRule: parseSmartRule(value.smartAnalysisRule) };
  }
  throw new Error(agentParseError);
}

function parseAgentsResponse(value: unknown): AgentItem[] {
  if (!Array.isArray(value)) throw new Error(agentParseError);
  const items = value.map(parseAgent);
  const keys = new Set(items.map((item) => item.systemKey));
  if (items.length !== 2 || keys.size !== 2 || !keys.has('session-analysis') || !keys.has('smart-analysis')) {
    throw new Error(agentParseError);
  }
  return items;
}

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
    async listAgents(corpId: number): Promise<AgentItem[]> {
      return parseAgentsResponse(await client.request(agentPath(corpId)));
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
