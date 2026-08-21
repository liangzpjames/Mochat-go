import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';

export type Page<T> = { items: T[]; total: number; page: number; perPage: number };

export type KeywordLibrary = {
  id: number; name: string; description: string; matchMode: string; status: string;
  draftVersion: number; publishedVersion: number; entryCount: number; updatedAt: string;
};

export type KeywordEntry = { id: number; libraryId: number; keyword: string; status: string; updatedAt: string };

export type MessageInterceptRule = {
  id: number; name: string; libraryId: number; libraryName: string; libraryVersion: number;
  conversationScopes: string[]; decision: string; status: string; triggerCount: number; updatedAt: string;
};

export type MessageInterceptRecord = {
  id: number; ruleId: number; ruleName: string; libraryId: number; libraryName: string;
  libraryVersion: number; conversationType: string; conversationId: string; messageId: string;
  senderId: string; senderName: string; messageContent: string; matchedKeywords: string[];
  decision: string; explanation: string; auditStatus: string; occurredAt: string;
};

export type MessageInterceptFilters = {
  keyword?: string; ruleId?: number; conversationType?: string; decision?: string;
  auditStatus?: string; startAt?: string; endAt?: string; page: number; perPage: number;
};

type ApiRow = Record<string, unknown>;

function row(value: unknown): ApiRow {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as ApiRow : {};
}

function number(value: unknown, fallback = 0): number {
  const parsed = typeof value === 'number' ? value : Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function string(value: unknown): string {
  return typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' ? String(value) : '';
}

function strings(value: unknown): string[] {
  return Array.isArray(value) ? value.map(string).filter(Boolean) : [];
}

function normalizeScope(value: string): string {
  if (value === 'customer') return 'single';
  if (value === 'room') return 'group';
  return value;
}

function body(value: unknown): ApiRow {
  const source = row(value);
  return row(source.data) && Object.keys(row(source.data)).length ? row(source.data) : source;
}

function page<T>(value: unknown, parse: (value: unknown) => T, fallbackPage: number, fallbackPerPage: number): Page<T> {
  const source = body(value);
  const list = [source.items, source.list, source.rows].find(Array.isArray);
  const pageValue = row(source.page);
  return {
    items: (Array.isArray(list) ? list : []).map(parse),
    total: number(source.total ?? pageValue.total, Array.isArray(list) ? list.length : 0),
    page: number(source.pageNumber ?? pageValue.page, fallbackPage),
    perPage: number(source.perPage ?? pageValue.perPage, fallbackPerPage),
  };
}

function parseLibrary(value: unknown): KeywordLibrary {
  const source = row(value);
  return {
    id: number(source.id), name: string(source.name), description: string(source.description),
    matchMode: string(source.matchMode) || 'contains', status: string(source.status) || 'disabled',
    draftVersion: number(source.draftVersion), publishedVersion: number(source.publishedVersion),
    entryCount: number(source.entryCount), updatedAt: string(source.updatedAt),
  };
}

function parseEntry(value: unknown): KeywordEntry {
  const source = row(value);
  return { id: number(source.id), libraryId: number(source.libraryId), keyword: string(source.keyword), status: string(source.status) || 'disabled', updatedAt: string(source.updatedAt) };
}

function parseRule(value: unknown): MessageInterceptRule {
  const source = row(value);
  return {
    id: number(source.id), name: string(source.name), libraryId: number(source.libraryId), libraryName: string(source.libraryName),
    libraryVersion: number(source.libraryVersion), conversationScopes: strings(source.conversationScopes).map(normalizeScope),
    decision: string(source.decision) || 'blocked', status: string(source.status) || 'disabled',
    triggerCount: number(source.triggerCount), updatedAt: string(source.updatedAt),
  };
}

function parseRecord(value: unknown): MessageInterceptRecord {
  const source = row(value);
  return {
    id: number(source.id), ruleId: number(source.ruleId), ruleName: string(source.ruleName), libraryId: number(source.libraryId),
    libraryName: string(source.libraryName), libraryVersion: number(source.libraryVersion), conversationType: normalizeScope(string(source.conversationType)),
    conversationId: string(source.conversationId), messageId: string(source.messageId), senderId: string(source.senderId),
    senderName: string(source.senderName), messageContent: string(source.messageContent), matchedKeywords: strings(source.matchedKeywords),
    decision: string(source.decision), explanation: string(source.explanation), auditStatus: string(source.auditStatus) || 'pending',
    occurredAt: string(source.occurredAt),
  };
}

function queryValues(values: Record<string, unknown>): Record<string, string | number> {
  return Object.fromEntries(Object.entries(values).filter(([, value]) => value !== undefined && value !== '' && value !== null)) as Record<string, string | number>;
}

export type MessageInterceptApi = {
  libraries(filter: { name?: string; status?: string; page: number; perPage: number }): Promise<Page<KeywordLibrary>>;
  entries(filter: { libraryId: number; keyword?: string; status?: string; page: number; perPage: number }): Promise<Page<KeywordEntry>>;
  rules(filter: { name?: string; status?: string; decision?: string; page: number; perPage: number }): Promise<Page<MessageInterceptRule>>;
  records(filter: MessageInterceptFilters): Promise<Page<MessageInterceptRecord>>;
  write(endpoint: string, values: Record<string, unknown>, method?: 'POST' | 'PUT' | 'DELETE'): Promise<unknown>;
};

export function createMessageInterceptApi(api: BusinessWorkbenchApi): MessageInterceptApi {
  return {
    async libraries(filter) { return page(await api.read('/keyword-library/libraries', queryValues(filter)), parseLibrary, filter.page, filter.perPage); },
    async entries(filter) { return page(await api.read('/keyword-library/entries', queryValues(filter)), parseEntry, filter.page, filter.perPage); },
    async rules(filter) { return page(await api.read('/message-intercept/rules', queryValues(filter)), parseRule, filter.page, filter.perPage); },
    async records(filter) { return page(await api.read('/message-intercept/records', queryValues(filter)), parseRecord, filter.page, filter.perPage); },
    write(endpoint, values, method = 'POST') { return api.write(endpoint, values, method); },
  };
}
