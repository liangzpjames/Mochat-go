export type RiskRecord = {
  id: number;
  behavior: string;
  riskLevel: 'low' | 'medium' | 'high' | string;
  conversationType: string;
  conversationId: string;
  messageId: string;
  triggerMessage: string;
  relatedUser: Record<string, unknown>;
  aiSummary: string;
  auditStatus: string;
  occurredAt: string;
};

export type RiskRecordSummary = { total: number; pending: number; highRisk: number; processed: number };
export type RiskRecordPage = { items: RiskRecord[]; total: number; page: number; perPage: 20; summary: RiskRecordSummary | null };
export type RiskRecordAudit = { id: number; action: string; remark: string; createdAt: string; actorName: string };
export type RiskRecordDetail = { record: RiskRecord; audits: RiskRecordAudit[]; conversationAvailable: boolean };
export type RiskRuleStrategy = { id?: number; behavior: string; pattern: string; notifyType: 'none' | string; riskLevel: 'low' | 'medium' | 'high' };
export type RiskRule = { id: number; name: string; status: 'enabled' | 'disabled' | string; subject: string; whitelist: string[]; aiInsightEnabled: boolean; triggerCount: number; strategies: RiskRuleStrategy[] };
export type RiskRulePage = { items: RiskRule[]; total: number; page: number; perPage: 20 };
export type ScannerStatus = { enabled: boolean; state: 'ready' | 'never_run' | 'failed' | 'disabled'; lastAttemptAt: string; lastSuccessAt: string; lastFailureAt: string; lastError: string };
export type RiskRecordFilters = {
  riskLevel?: string;
  behavior?: string;
  auditStatus?: string;
  conversationType?: string;
  employeeIds?: number[];
  occurredFrom?: string;
  occurredTo?: string;
  page: number;
};
export type RiskRuleInput = {
  id?: number;
  name: string;
  status: 'enabled' | 'disabled';
  subject: 'employee' | 'customer' | 'both';
  whitelist: string[];
  aiInsightEnabled: boolean;
  strategies: RiskRuleStrategy[];
};

type Client = { request<T = unknown>(input: RequestInfo | URL, init?: RequestInit): Promise<T> };
type QueryValue = string | number | boolean;

function number(value: unknown, fallback = 0): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : Number.isFinite(Number(value)) ? Number(value) : fallback;
}
function string(value: unknown): string { return typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' ? String(value) : ''; }
function record(value: unknown): Record<string, unknown> { return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {}; }
function relatedUser(value: unknown): Record<string, unknown> {
  const objectValue = record(value);
  if (Object.keys(objectValue).length > 0) return objectValue;
  if (!Array.isArray(value)) return {};
  return value.reduce<Record<string, unknown>>((result, item) => {
    const person = record(item);
    const role = string(person.role);
    const name = string(person.userName ?? person.name);
    const id = number(person.userId ?? person.id);
    if (role === 'employee') {
      if (name) result.employeeName = name;
      if (id > 0) result.employeeId = id;
    } else if (role === 'customer') {
      if (name) result.customerName = name;
      if (id > 0) result.customerId = id;
    } else if (role === 'room' && name) {
      result.roomName = name;
    }
    return result;
  }, {});
}
function queryString(values: readonly [string, QueryValue][]): string {
  const query = new URLSearchParams();
  for (const [key, value] of values) query.append(key, String(value));
  return query.toString();
}
function parseRiskRecord(value: unknown): RiskRecord {
  const row = record(value);
  return {
    id: number(row.id), behavior: string(row.behavior), riskLevel: string(row.riskLevel), conversationType: string(row.conversationType),
    conversationId: string(row.conversationId), messageId: string(row.messageId), triggerMessage: string(row.triggerMessage),
    relatedUser: relatedUser(row.relatedUser), aiSummary: string(row.aiSummary), auditStatus: string(row.auditStatus), occurredAt: string(row.occurredAt),
  };
}
function parseRecordPage(value: unknown): RiskRecordPage {
  const source = record(value);
  const items = Array.isArray(source.items) ? source.items.map(parseRiskRecord) : [];
  const summary = source.summary && typeof source.summary === 'object' ? record(source.summary) : null;
  return {
    items, total: number(source.total, items.length), page: number(source.page, 1), perPage: 20,
    summary: summary ? { total: number(summary.total), pending: number(summary.pending), highRisk: number(summary.highRisk), processed: number(summary.processed) } : null,
  };
}
function parseRule(value: unknown): RiskRule {
  const row = record(value);
  const strategies = Array.isArray(row.strategies) ? row.strategies.map((item) => {
    const strategy = record(item);
    const riskLevel = string(strategy.riskLevel);
    const normalizedRiskLevel: RiskRuleStrategy['riskLevel'] = riskLevel === 'high' || riskLevel === 'low' ? riskLevel : 'medium';
    return { id: number(strategy.id), behavior: string(strategy.behavior), pattern: string(strategy.pattern), notifyType: string(strategy.notifyType), riskLevel: normalizedRiskLevel };
  }) : [];
  return { id: number(row.id), name: string(row.name), status: string(row.status), subject: string(row.subject), whitelist: Array.isArray(row.whitelist) ? row.whitelist.map(string) : [], aiInsightEnabled: row.aiInsightEnabled === true, triggerCount: number(row.triggerCount), strategies };
}
function json(method: 'POST' | 'PUT' | 'DELETE', body: unknown): RequestInit { return { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }; }

export type RiskBehaviorApi = {
  records(input: RiskRecordFilters): Promise<RiskRecordPage>;
  recordDetail(id: number): Promise<RiskRecordDetail>;
  rules(input: { name?: string; page: number }): Promise<RiskRulePage>;
  audit(input: { ids: number[]; action: 'confirmed' | 'ignored'; remark: string }): Promise<unknown>;
  createRule(input: RiskRuleInput): Promise<unknown>;
  updateRule(input: RiskRuleInput): Promise<unknown>;
  setRuleEnabled(input: { id: number; status: 'enabled' | 'disabled' }): Promise<unknown>;
  removeRule(id: number): Promise<unknown>;
  scannerStatus(): Promise<ScannerStatus>;
};

export function createRiskBehaviorApi(client: Client): RiskBehaviorApi {
  return {
    async records(input) {
      const values: [string, QueryValue][] = [];
      if (input.riskLevel) values.push(['riskLevel', input.riskLevel]);
      if (input.behavior) values.push(['behavior', input.behavior]);
      if (input.auditStatus) values.push(['auditStatus', input.auditStatus]);
      if (input.conversationType) values.push(['conversationType', input.conversationType]);
      for (const employeeId of input.employeeIds ?? []) values.push(['employeeIds', employeeId]);
      if (input.occurredFrom) values.push(['occurredFrom', input.occurredFrom]);
      if (input.occurredTo) values.push(['occurredTo', input.occurredTo]);
      values.push(['page', input.page], ['perPage', 20]);
      return parseRecordPage(await client.request(`/risk/records?${queryString(values)}`));
    },
    async recordDetail(id) {
      const value = record(await client.request(`/risk/records/detail?id=${encodeURIComponent(id)}`));
      return { record: parseRiskRecord(value.record), audits: Array.isArray(value.audits) ? value.audits.map((item) => record(item) as RiskRecordAudit) : [], conversationAvailable: value.conversationAvailable === true };
    },
    async rules(input) {
      const values: [string, QueryValue][] = [];
      if (input.name) values.push(['name', input.name]);
      values.push(['page', input.page], ['perPage', 20]);
      const source = record(await client.request(`/risk/rules?${queryString(values)}`));
      const items = Array.isArray(source.items) ? source.items.map(parseRule) : [];
      return { items, total: number(source.total, items.length), page: number(source.page, input.page), perPage: 20 };
    },
    async audit(input) { return client.request('/risk/records/audit', json('POST', input)); },
    async createRule(input) { return client.request('/risk/rules', json('POST', input)); },
    async updateRule(input) { return client.request('/risk/rules', json('PUT', input)); },
    async setRuleEnabled(input) { return client.request('/risk/rules/status', json('PUT', input)); },
    async removeRule(id) { return client.request(`/risk/rules?id=${encodeURIComponent(id)}`, json('DELETE', {})); },
    async scannerStatus() { return client.request('/risk/scanner-status'); },
  };
}
