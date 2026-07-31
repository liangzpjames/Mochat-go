export type SensitiveWordItem = { id: number; groupId: number; groupName: string; name: string; status: number };
export type SensitiveWordGroup = { id: number; name: string };
export type SensitiveWordPage = { items: SensitiveWordItem[]; total: number; page: number; perPage: number };
export type SensitiveWordMatchPage = { items: Array<Record<string, unknown>>; total: number; page: number; perPage: number };

type Client = { request<T = unknown>(input: RequestInfo | URL, init?: RequestInit): Promise<T> };
type JsonInit = { method: string; headers: { 'Content-Type': string }; body?: string };

const json = (method: string, body: unknown): JsonInit => ({
  method,
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(body),
});

function number(value: unknown, fallback = 0): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback;
}

function parsePage(value: unknown): SensitiveWordPage {
  const record = value as { list?: unknown; page?: { total?: unknown; perPage?: unknown; totalPage?: unknown } };
  const list = Array.isArray(record?.list) ? record.list : [];
  return {
    items: list.map((item) => {
      const row = item as Record<string, unknown>;
      return {
        id: number(row.sensitiveWordId ?? row.id),
        groupId: number(row.groupId),
        groupName: String(row.groupName ?? ''),
        name: String(row.name ?? ''),
        status: number(row.status),
      };
    }),
    total: number(record?.page?.total),
    page: 1,
    perPage: number(record?.page?.perPage, 10),
  };
}

export type SensitiveWordApi = {
  list(input: { groupId: number; keywords: string; page: number; perPage: number }): Promise<SensitiveWordPage>;
  groups(): Promise<SensitiveWordGroup[]>;
  create(input: { groupId: number; names: string[]; idempotencyKey: string }): Promise<void>;
  setEnabled(input: { id: number; enabled: boolean; version?: string }): Promise<void>;
  move(input: { id: number; groupId: number; version?: string }): Promise<void>;
  remove(input: { id: number; version?: string }): Promise<void>;
  createGroup(input: { names: string[] }): Promise<void>;
  renameGroup(input: { id: number; name: string }): Promise<void>;
  matches(input: { page: number; perPage: number }): Promise<SensitiveWordMatchPage>;
  matchDetail(id: number): Promise<readonly Record<string, unknown>[]>;
};

export function createSensitiveWordApi(client: Client): SensitiveWordApi {
  return {
    async list(input) {
      const query = new URLSearchParams({ groupId: String(input.groupId), keyWords: input.keywords, page: String(input.page), perPage: String(input.perPage) });
      const value = await client.request(`/sensitiveWord/index?${query.toString()}`);
      const result = parsePage(value);
      return { ...result, page: input.page };
    },
    async groups() {
      const rows = await client.request<unknown>('/sensitiveWordGroup/select');
      return (Array.isArray(rows) ? rows : []).map((row) => {
        const value = row as Record<string, unknown>;
        return { id: number(value.groupId ?? value.id), name: String(value.name ?? '') };
      });
    },
    async create(input) { await client.request('/sensitiveWord/store', json('POST', { groupId: input.groupId, name: input.names.join(','), idempotencyKey: input.idempotencyKey })); },
    async setEnabled(input) { await client.request('/sensitiveWord/statusUpdate', json('PUT', { sensitiveWordId: input.id, status: input.enabled ? 1 : 2, version: input.version })); },
    async move(input) { await client.request('/sensitiveWord/move', json('PUT', { sensitiveWordId: input.id, groupId: input.groupId, version: input.version })); },
    async remove(input) { await client.request('/sensitiveWord/destroy', json('DELETE', { sensitiveWordId: input.id, version: input.version })); },
    async createGroup(input) { await client.request('/sensitiveWordGroup/store', json('POST', { name: input.names.join(',') })); },
    async renameGroup(input) { await client.request('/sensitiveWordGroup/update', json('PUT', { groupId: input.id, name: input.name })); },
    async matches(input) { const query = new URLSearchParams({ page: String(input.page), perPage: String(input.perPage) }); return client.request(`/sensitiveWordsMonitor/index?${query.toString()}`) as Promise<SensitiveWordMatchPage>; },
    async matchDetail(id) { return client.request(`/sensitiveWordsMonitor/show?id=${id}`) as Promise<readonly Record<string, unknown>[]>; },
  };
}
