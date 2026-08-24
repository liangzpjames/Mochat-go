export type InsightStatus = 'pending' | 'running' | 'succeeded' | 'failed';
export type ConversationType = 'direct' | 'group';
export type DerivedInsightView = 'emotion' | 'employee-score' | 'communication-keyword';
export type EmotionLabel = 'positive' | 'neutral' | 'negative' | 'mixed' | 'unknown';
export type SessionInsightFilters = { page: number; employeeId?: number | undefined; customerName?: string | undefined; conversationType?: ConversationType | undefined; targetId?: string | undefined; keyword?: string | undefined; status?: InsightStatus | undefined; startDate?: string | undefined; endDate?: string | undefined };
export type SmartInsightFilters = SessionInsightFilters;
export type DerivedInsightFilters = {
  page: number;
  employeeId?: number | undefined;
  customerName?: string | undefined;
  status?: InsightStatus | undefined;
  startDate?: string | undefined;
  endDate?: string | undefined;
  emotion?: EmotionLabel | undefined;
  minScore?: number | undefined;
  maxScore?: number | undefined;
  keyword?: string | undefined;
};
export type SessionInsightRow = { id: number; conversationKey: string; employee: Person; target: ConversationTarget; sourceWindow: SourceWindow; status: InsightStatus; summary: string; errorSummary: string; analysisAt: string; result?: Record<string, unknown>; provider: string; model: string; promptVersion: string };
export type SmartInsightRow = SessionInsightRow & { rule?: { id: number; name: string; version: number } };
export type Person = { id: number; name: string; avatar: string };
export type ConversationTarget = { id: string; name: string; avatar: string; type: ConversationType };
export type EmployeeFilterOptions = { employees: Person[] };
export type SourceWindow = { startedAt: string; endedAt: string; messageCount: number; fingerprint: string };
export type InsightPage<T> = { page: number; pageSize: 20; total: number; items: T[] };
export type InsightDetail<T extends object> = T & { messages: SourceMessage[]; conversationUrl: string };
export type SourceMessage = { id: string; time: string; direction: 'inbound' | 'outbound'; senderName: string; content: string };
export type InsightRunStatus = { provider: { state: string; source?: string; code?: string; message?: string }; assistant?: { name: string; enabled: boolean; knowledgeBaseCount: number; readyDocumentCount: number; updatedAt: string }; run?: { status: InsightStatus; candidateCount: number; successCount: number; failureCount: number; backlogCount: number; errorSummary: string; createdAt: string } };
type BaseAiInsightWorkspaceApi = {
  sessionRecords(filters: SessionInsightFilters): Promise<InsightPage<SessionInsightRow>>;
  sessionDetail(id: number): Promise<InsightDetail<SessionInsightRow>>;
  sessionStatus(): Promise<InsightRunStatus>;
  sessionFilterOptions(employeeKeyword?: string, limit?: number): Promise<EmployeeFilterOptions>;
  sessionExportUrl(filters: SessionInsightFilters): string;
  smartRecords(filters: SmartInsightFilters): Promise<InsightPage<SmartInsightRow>>;
  smartDetail(id: number): Promise<InsightDetail<SmartInsightRow>>;
  smartStatus(): Promise<InsightRunStatus>;
  smartFilterOptions(employeeKeyword?: string, limit?: number): Promise<EmployeeFilterOptions>;
};
type DerivedInsightApiMethods = {
  derivedRecords(view: DerivedInsightView, filters: DerivedInsightFilters): Promise<InsightPage<SessionInsightRow>>;
  derivedDetail(view: DerivedInsightView, id: number): Promise<InsightDetail<SessionInsightRow>>;
  derivedStatus(view: DerivedInsightView): Promise<InsightRunStatus>;
  derivedFilterOptions(view: DerivedInsightView, employeeKeyword?: string, limit?: number): Promise<EmployeeFilterOptions>;
  derivedExportUrl(view: DerivedInsightView, filters: DerivedInsightFilters): string;
};
export type AiInsightWorkspaceApi = BaseAiInsightWorkspaceApi & DerivedInsightApiMethods;
export type AiInsightExportDownloader = (view: DerivedInsightView, filters: DerivedInsightFilters) => Promise<{ blob: Blob; filename: string }>;

type Client = {
  request<T = unknown>(input: RequestInfo | URL, init?: RequestInit): Promise<T>;
  download?: (input: RequestInfo | URL, init?: RequestInit) => Promise<{ blob: Blob; filename: string }>;
};

function record(value: unknown): Record<string, unknown> { return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {}; }
function text(value: unknown): string { return typeof value === 'string' || typeof value === 'number' ? String(value) : ''; }
function number(value: unknown): number { return typeof value === 'number' && Number.isFinite(value) ? value : Number(value) || 0; }
function person(value: unknown, field: string): Person { const source = record(value); const item = { id: number(source.id), name: text(source.name), avatar: text(source.avatar) }; if (item.id <= 0 || item.name.trim() === '') throw new Error(`${field}资料不完整`); return item; }
function target(value: unknown): ConversationTarget { const source = record(value); const item: ConversationTarget = { id: text(source.id), name: text(source.name), avatar: text(source.avatar), type: conversationType(source.type) }; if (!item.id.trim() || item.id === '0' || !item.name.trim()) throw new Error('对象资料不完整'); return item; }
function status(value: unknown): InsightStatus { const item = text(value); if (!['pending', 'running', 'succeeded', 'failed'].includes(item)) throw new Error(`未知分析状态：${item}`); return item as InsightStatus; }
function conversationType(value: unknown): ConversationType { const item = text(value); if (item !== 'direct' && item !== 'group') throw new Error(`未知会话类型：${item}`); return item; }

function parseRow(value: unknown, smart: boolean): SessionInsightRow | SmartInsightRow {
  const source = record(value);
  const id = number(source.id);
  const conversationKey = text(source.conversationKey);
  const window = record(source.sourceWindow);
  const sourceWindow: SourceWindow = { startedAt: text(window.startedAt), endedAt: text(window.endedAt), messageCount: number(window.messageCount), fingerprint: text(window.fingerprint) };
  if (id <= 0 || conversationKey.trim() === '' || sourceWindow.messageCount <= 0 || sourceWindow.fingerprint.trim() === '') throw new Error('AI 洞察列表接口返回了无效数据');
  const employee = person(source.employee, '员工');
  const parsedTarget = target(source.target);
  const item: SessionInsightRow = { id, conversationKey, employee, target: parsedTarget, sourceWindow, status: status(source.status), summary: text(source.summary), errorSummary: text(source.errorSummary), analysisAt: text(source.analysisAt), result: record(source.result), provider: text(source.provider), model: text(source.model), promptVersion: text(source.promptVersion) };
  if (!smart) return item;
  const rule = record(source.rule);
  const ruleId = number(rule.id);
  if (ruleId <= 0 || text(rule.name).trim() === '') throw new Error('智能分析结果缺少规则快照');
  return { ...item, rule: { id: ruleId, name: text(rule.name), version: number(rule.version) } };
}

function parseDerivedRow(value: unknown, view: DerivedInsightView): SessionInsightRow {
  const source = record(value);
  const parsed = parseRow(source, false) as SessionInsightRow;
  if (parsed.status === 'failed' && source.result === undefined) {
    const failed: SessionInsightRow = { ...parsed };
    delete failed.result;
    return failed;
  }
  const result = record(source.result);
  if (parsed.status === 'succeeded') {
    const customer = record(result.customer);
    if (view === 'emotion') {
      const label = text(record(customer.emotion).label);
      if (!['positive', 'neutral', 'negative', 'mixed', 'unknown'].includes(label)) throw new Error(`未知客户情绪：${label}`);
    } else if (view === 'employee-score') {
      const score = record(result.employeeQa).score;
      if (typeof score !== 'number' || !Number.isFinite(score)) throw new Error('员工评分缺失或不是数值');
      if (score < 0 || score > 100) throw new Error('员工评分超出 0-100');
    } else {
      const keywords = customer.keywords;
      if (!Array.isArray(keywords) || !keywords.every((entry) => typeof entry === 'string')) throw new Error('客户关键词必须是字符串数组');
    }
  }
  return { ...parsed, result };
}

function parsePage<T>(value: unknown, parser: (value: unknown) => T): InsightPage<T> { const source = record(value); const rawItems = Array.isArray(source.items) ? source.items : []; return { page: Math.max(1, number(source.page)), pageSize: 20, total: Math.max(0, number(source.total)), items: rawItems.map(parser) }; }
function parseDetail<T extends object>(value: unknown, parser: (value: unknown) => T): InsightDetail<T> { const source = record(value); const base = parser(source); const messages = Array.isArray(source.messages) ? source.messages.map((entry) => { const item = record(entry); const rawDirection = text(item.direction); if (rawDirection !== 'inbound' && rawDirection !== 'outbound') throw new Error('详情消息方向无效'); const direction: SourceMessage['direction'] = rawDirection; return { id: text(item.id), time: text(item.time), direction, senderName: text(item.senderName), content: text(item.content) }; }) : []; const conversationUrl = text(source.conversationUrl); if (!conversationUrl) throw new Error('详情缺少会话跳转地址'); return Object.assign(base, { messages, conversationUrl }); }
function parseEmployeeFilterOptions(value: unknown): EmployeeFilterOptions { const source = record(value); if (!Array.isArray(source.employees)) throw new Error('员工筛选选项返回了无效数据'); try { return { employees: source.employees.map((entry) => person(entry, '员工')) }; } catch { throw new Error('员工筛选选项返回了无效数据'); } }
function parseRunStatus(value: unknown): InsightRunStatus { const source = record(value); const providerSource = record(source.provider); const providerState = text(providerSource.state); if (!providerState) throw new Error('AI 服务状态接口返回了无效数据'); const provider: InsightRunStatus['provider'] = { state: providerState }; for (const key of ['source', 'code', 'message'] as const) { const item = text(providerSource[key]); if (item) Object.assign(provider, { [key]: item }); } const result: InsightRunStatus = { provider }; const runSource = record(source.run); if (Object.keys(runSource).length > 0) { result.run = { status: status(runSource.status), candidateCount: Math.max(0, number(runSource.candidateCount)), successCount: Math.max(0, number(runSource.successCount)), failureCount: Math.max(0, number(runSource.failureCount)), backlogCount: Math.max(0, number(runSource.backlogCount)), errorSummary: text(runSource.errorSummary), createdAt: text(runSource.createdAt) }; } return result; }
function query(values: Record<string, string | number | boolean | undefined>, preserveZero = false): string { const params = new URLSearchParams(); Object.entries(values).forEach(([key, value]) => { if (value !== undefined && value !== '' && (preserveZero || value !== 0)) params.set(key, String(value)); }); return params.toString(); }
function withQuery(base: string, values: Record<string, string | number | boolean | undefined>, preserveZero = false): string { const serialized = query(values, preserveZero); return serialized ? `${base}?${serialized}` : base; }
function path(filters: SessionInsightFilters, suffix: string): string { const params = { page: filters.page, employeeId: filters.employeeId, customerName: filters.customerName, conversationType: filters.conversationType, targetId: filters.targetId, keyword: filters.keyword, status: filters.status, startDate: filters.startDate, endDate: filters.endDate }; return `/ai-insight/session-analysis/${suffix}?${query(params)}`; }
function smartPath(filters: SmartInsightFilters, suffix: string): string { const params = { page: filters.page, employeeId: filters.employeeId, customerName: filters.customerName, conversationType: filters.conversationType, targetId: filters.targetId, keyword: filters.keyword, status: filters.status, startDate: filters.startDate, endDate: filters.endDate }; return `/ai-insight/smart-analysis/${suffix}?${query(params)}`; }
function derivedPath(view: DerivedInsightView, filters: DerivedInsightFilters, suffix: string): string {
  const common = { page: suffix === 'export' && filters.page === 1 ? undefined : filters.page, employeeId: filters.employeeId, customerName: filters.customerName, status: filters.status, startDate: filters.startDate, endDate: filters.endDate };
  const specialized = view === 'emotion' ? { emotion: filters.emotion } : view === 'employee-score' ? { minScore: filters.minScore, maxScore: filters.maxScore } : { keyword: filters.keyword };
  return withQuery(`/ai-insight/${view}/${suffix}`, { ...common, ...specialized }, true);
}
function filterOptionsPath(prefix: 'session-analysis' | 'smart-analysis' | DerivedInsightView, employeeKeyword?: string, limit?: number): string { const values = { employeeKeyword, limit }; return prefix === 'session-analysis' || prefix === 'smart-analysis' ? `/ai-insight/${prefix}/filter-options?${query(values)}` : withQuery(`/ai-insight/${prefix}/filter-options`, values); }

export function createAiInsightExportDownloader(client: Client): AiInsightExportDownloader {
  return async (view, filters) => {
    if (!client.download) throw new Error('当前客户端未接入认证导出能力');
    return client.download(derivedPath(view, filters, 'export'));
  };
}

export function createAiInsightWorkspaceApi(client: Client): AiInsightWorkspaceApi {
  return {
    async sessionRecords(filters) { return parsePage(await client.request(path(filters, 'records')), (value) => parseRow(value, false) as SessionInsightRow); },
    async sessionDetail(id) { return parseDetail(await client.request(`/ai-insight/session-analysis/detail?id=${encodeURIComponent(id)}`), (value) => parseRow(value, false) as SessionInsightRow); },
    async sessionStatus() { return await client.request<InsightRunStatus>('/ai-insight/session-analysis/status'); },
    async sessionFilterOptions(employeeKeyword, limit) { return parseEmployeeFilterOptions(await client.request(filterOptionsPath('session-analysis', employeeKeyword, limit))); },
    sessionExportUrl(filters) { return path(filters, 'export'); },
    async smartRecords(filters) { return parsePage(await client.request(smartPath(filters, 'records')), (value) => parseRow(value, true) as SmartInsightRow); },
    async smartDetail(id) { return parseDetail(await client.request(`/ai-insight/smart-analysis/detail?id=${encodeURIComponent(id)}`), (value) => parseRow(value, true) as SmartInsightRow); },
    async smartStatus() { return await client.request<InsightRunStatus>('/ai-insight/smart-analysis/status'); },
    async smartFilterOptions(employeeKeyword, limit) { return parseEmployeeFilterOptions(await client.request(filterOptionsPath('smart-analysis', employeeKeyword, limit))); },
    async derivedRecords(view, filters) { return parsePage(await client.request(derivedPath(view, filters, 'records')), (value) => parseDerivedRow(value, view)); },
    async derivedDetail(view, id) { if (!Number.isInteger(id) || id <= 0) throw new Error('详情 ID 必须为正数'); return parseDetail(await client.request(`/ai-insight/${view}/detail?id=${encodeURIComponent(id)}`), (value) => parseDerivedRow(value, view)); },
    async derivedStatus(view) { return parseRunStatus(await client.request(`/ai-insight/${view}/status`)); },
    async derivedFilterOptions(view, employeeKeyword, limit) { return parseEmployeeFilterOptions(await client.request(filterOptionsPath(view, employeeKeyword, limit))); },
    derivedExportUrl(view, filters) { return derivedPath(view, filters, 'export'); },
  };
}
