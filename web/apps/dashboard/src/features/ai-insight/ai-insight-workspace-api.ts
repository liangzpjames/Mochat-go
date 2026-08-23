export type InsightStatus = 'pending' | 'running' | 'succeeded' | 'failed';
export type ConversationType = 'direct' | 'group';
export type SessionInsightFilters = { page: number; employeeId?: number | undefined; conversationType?: ConversationType | undefined; targetId?: string | undefined; keyword?: string | undefined; status?: InsightStatus | undefined; startDate?: string | undefined; endDate?: string | undefined };
export type SmartInsightFilters = SessionInsightFilters;
export type SessionInsightRow = { id: number; conversationKey: string; employee: Person; target: Person & { type: ConversationType }; sourceWindow: SourceWindow; status: InsightStatus; summary: string; errorSummary: string; analysisAt: string; result?: Record<string, unknown>; provider: string; model: string; promptVersion: string };
export type SmartInsightRow = SessionInsightRow & { rule?: { id: number; name: string; version: number } };
export type Person = { id: number; name: string; avatar: string };
export type SourceWindow = { startedAt: string; endedAt: string; messageCount: number; fingerprint: string };
export type InsightPage<T> = { page: number; pageSize: 20; total: number; items: T[] };
export type InsightDetail<T extends object> = T & { messages: SourceMessage[]; conversationUrl: string };
export type SourceMessage = { id: string; time: string; direction: 'inbound' | 'outbound'; senderName: string; content: string };
export type InsightRunStatus = { provider: { state: string; source?: string; code?: string; message?: string }; assistant?: { name: string; enabled: boolean; knowledgeBaseCount: number; readyDocumentCount: number; updatedAt: string }; run?: { status: InsightStatus; candidateCount: number; successCount: number; failureCount: number; backlogCount: number; errorSummary: string; createdAt: string } };
export type AiInsightWorkspaceApi = {
  sessionRecords(filters: SessionInsightFilters): Promise<InsightPage<SessionInsightRow>>;
  sessionDetail(id: number): Promise<InsightDetail<SessionInsightRow>>;
  sessionStatus(): Promise<InsightRunStatus>;
  sessionExportUrl(filters: SessionInsightFilters): string;
  smartRecords(filters: SmartInsightFilters): Promise<InsightPage<SmartInsightRow>>;
  smartDetail(id: number): Promise<InsightDetail<SmartInsightRow>>;
  smartStatus(): Promise<InsightRunStatus>;
};

type Client = { request<T = unknown>(input: RequestInfo | URL, init?: RequestInit): Promise<T> };
function record(value: unknown): Record<string, unknown> { return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {}; }
function text(value: unknown): string { return typeof value === 'string' || typeof value === 'number' ? String(value) : ''; }
function number(value: unknown): number { return typeof value === 'number' && Number.isFinite(value) ? value : Number(value) || 0; }
function person(value: unknown, field: string): Person { const source = record(value); const item = { id: number(source.id), name: text(source.name), avatar: text(source.avatar) }; if (item.id <= 0 || item.name.trim() === '') throw new Error(`${field}资料不完整`); return item; }
function status(value: unknown): InsightStatus { const item = text(value); if (!['pending', 'running', 'succeeded', 'failed'].includes(item)) throw new Error(`未知分析状态：${item}`); return item as InsightStatus; }
function type(value: unknown): ConversationType { const item = text(value); if (item !== 'direct' && item !== 'group') throw new Error(`未知会话类型：${item}`); return item; }
function parseRow(value: unknown, smart: boolean): SessionInsightRow | SmartInsightRow { const source = record(value); const id = number(source.id); const conversationKey = text(source.conversationKey); const window = record(source.sourceWindow); const sourceWindow: SourceWindow = { startedAt: text(window.startedAt), endedAt: text(window.endedAt), messageCount: number(window.messageCount), fingerprint: text(window.fingerprint) }; if (id <= 0 || conversationKey.trim() === '' || sourceWindow.messageCount <= 0 || sourceWindow.fingerprint.trim() === '') throw new Error('AI 洞察列表接口返回了无效数据'); const employee = person(source.employee, '员工'); const targetSource = record(source.target); const target = { ...person(targetSource, '对象'), type: type(targetSource.type) }; const item: SessionInsightRow = { id, conversationKey, employee, target, sourceWindow, status: status(source.status), summary: text(source.summary), errorSummary: text(source.errorSummary), analysisAt: text(source.analysisAt), result: record(source.result), provider: text(source.provider), model: text(source.model), promptVersion: text(source.promptVersion) }; if (!smart) return item; const rule = record(source.rule); const ruleId = number(rule.id); if (ruleId <= 0 || text(rule.name).trim() === '') throw new Error('智能分析结果缺少规则快照'); return { ...item, rule: { id: ruleId, name: text(rule.name), version: number(rule.version) } }; }
function parsePage<T>(value: unknown, parser: (value: unknown) => T): InsightPage<T> { const source = record(value); const rawItems = Array.isArray(source.items) ? source.items : []; return { page: Math.max(1, number(source.page)), pageSize: 20, total: Math.max(0, number(source.total)), items: rawItems.map(parser) }; }
function parseDetail<T extends object>(value: unknown, parser: (value: unknown) => T): InsightDetail<T> { const source = record(value); const base = parser(source); const messages = Array.isArray(source.messages) ? source.messages.map((entry) => { const item = record(entry); const rawDirection = text(item.direction); if (rawDirection !== 'inbound' && rawDirection !== 'outbound') throw new Error('详情消息方向无效'); const direction: SourceMessage['direction'] = rawDirection; return { id: text(item.id), time: text(item.time), direction, senderName: text(item.senderName), content: text(item.content) }; }) : []; const conversationUrl = text(source.conversationUrl); if (!conversationUrl) throw new Error('详情缺少会话跳转地址'); return Object.assign(base, { messages, conversationUrl }); }
function query(values: Record<string, string | number | boolean | undefined>): string { const params = new URLSearchParams(); Object.entries(values).forEach(([key, value]) => { if (value !== undefined && value !== '' && value !== 0) params.set(key, String(value)); }); return params.toString(); }
function path(filters: SessionInsightFilters, suffix: string): string { const params = { page: filters.page, employeeId: filters.employeeId, conversationType: filters.conversationType, targetId: filters.targetId, keyword: filters.keyword, status: filters.status, startDate: filters.startDate, endDate: filters.endDate }; return `/ai-insight/session-analysis/${suffix}?${query(params)}`; }
function smartPath(filters: SmartInsightFilters, suffix: string): string { const params = { page: filters.page, employeeId: filters.employeeId, conversationType: filters.conversationType, targetId: filters.targetId, keyword: filters.keyword, status: filters.status, startDate: filters.startDate, endDate: filters.endDate }; return `/ai-insight/smart-analysis/${suffix}?${query(params)}`; }

export function createAiInsightWorkspaceApi(client: Client): AiInsightWorkspaceApi {
  return {
    async sessionRecords(filters) { return parsePage(await client.request(path(filters, 'records')), (value) => parseRow(value, false) as SessionInsightRow); },
    async sessionDetail(id) { return parseDetail(await client.request(`/ai-insight/session-analysis/detail?id=${encodeURIComponent(id)}`), (value) => parseRow(value, false) as SessionInsightRow); },
    async sessionStatus() { return await client.request<InsightRunStatus>('/ai-insight/session-analysis/status'); },
    sessionExportUrl(filters) { return path(filters, 'export'); },
    async smartRecords(filters) { return parsePage(await client.request(smartPath(filters, 'records')), (value) => parseRow(value, true) as SmartInsightRow); },
    async smartDetail(id) { return parseDetail(await client.request(`/ai-insight/smart-analysis/detail?id=${encodeURIComponent(id)}`), (value) => parseRow(value, true) as SmartInsightRow); },
    async smartStatus() { return await client.request<InsightRunStatus>('/ai-insight/smart-analysis/status'); },
  };
}
