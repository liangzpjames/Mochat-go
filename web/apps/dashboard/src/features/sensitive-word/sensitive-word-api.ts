export type SensitiveWordItem = { id: number; groupId: number; groupName: string; name: string; status: number; version: string };
export type SensitiveWordGroup = { id: number; name: string; version: string };
export type SensitiveWordPage = { items: SensitiveWordItem[]; total: number; page: number; perPage: number };
export type SensitiveWordMatch = { id: number; sensitiveWordName: string; source: number; sourceText: string; triggerName: string; triggerScenario: string; triggerTime: string };
export type SensitiveWordMatchPage = { items: SensitiveWordMatch[]; total: number; page: number; perPage: number };
export type SensitiveWordMutationResult = { version: string; idempotent: boolean };
export type SensitiveWordMatchFilters = { employeeIds: number[]; workRoomId: number; groupId: number; triggerStart: string; triggerEnd: string; page: number; perPage: number };

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
		version: String(row.version ?? ''),
      };
    }),
    total: number(record?.page?.total),
    page: 1,
    perPage: number(record?.page?.perPage, 10),
  };
}

function parseMatchPage(value: unknown, page: number): SensitiveWordMatchPage {
  const record = value as { list?: unknown; page?: { total?: unknown; perPage?: unknown } };
  const list = Array.isArray(record?.list) ? record.list : [];
  return {
	items: list.map((item) => {
	  const row = item as Record<string, unknown>;
	  return {
		id: number(row.sensitiveWordsMonitorId ?? row.sensitiveWordMonitorId ?? row.id),
		sensitiveWordName: String(row.sensitiveWordName ?? ''),
		source: number(row.source),
		sourceText: String(row.sourceText ?? ''),
		triggerName: String(row.triggerName ?? ''),
		triggerScenario: String(row.triggerScenario ?? ''),
		triggerTime: String(row.triggerTime ?? ''),
	  };
	}),
	total: number(record?.page?.total),
	page,
	perPage: number(record?.page?.perPage, 10),
  };
}

export type SensitiveWordApi = {
  list(input: { groupId: number; keywords: string; page: number; perPage: number }): Promise<SensitiveWordPage>;
  groups(): Promise<SensitiveWordGroup[]>;
  create(input: { groupId: number; names: string[]; version: string; idempotencyKey: string }): Promise<SensitiveWordMutationResult>;
  setEnabled(input: { id: number; enabled: boolean; version: string; idempotencyKey: string }): Promise<SensitiveWordMutationResult>;
  move(input: { id: number; groupId: number; version: string; idempotencyKey: string }): Promise<SensitiveWordMutationResult>;
  remove(input: { id: number; version: string; idempotencyKey: string; confirmed: true }): Promise<SensitiveWordMutationResult>;
  createGroup(input: { names: string[]; version: string; idempotencyKey: string }): Promise<SensitiveWordMutationResult>;
  renameGroup(input: { id: number; name: string; version: string; idempotencyKey: string }): Promise<SensitiveWordMutationResult>;
  matches(input: SensitiveWordMatchFilters): Promise<SensitiveWordMatchPage>;
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
	  return { id: number(value.groupId ?? value.id), name: String(value.name ?? ''), version: String(value.version ?? '') };
	});
	},
	async create(input) { return client.request('/sensitiveWord/store', json('POST', { groupId: input.groupId, name: input.names.join(','), version: input.version, idempotencyKey: input.idempotencyKey })); },
	async setEnabled(input) { return client.request('/sensitiveWord/statusUpdate', json('PUT', { sensitiveWordId: input.id, status: input.enabled ? 1 : 2, version: input.version, idempotencyKey: input.idempotencyKey })); },
	async move(input) { return client.request('/sensitiveWord/move', json('PUT', { sensitiveWordId: input.id, groupId: input.groupId, version: input.version, idempotencyKey: input.idempotencyKey })); },
	async remove(input) { return client.request('/sensitiveWord/destroy', json('DELETE', { sensitiveWordId: input.id, version: input.version, idempotencyKey: input.idempotencyKey, confirmed: input.confirmed })); },
	async createGroup(input) { return client.request('/sensitiveWordGroup/store', json('POST', { name: input.names.join(','), version: input.version, idempotencyKey: input.idempotencyKey })); },
	async renameGroup(input) { return client.request('/sensitiveWordGroup/update', json('PUT', { groupId: input.id, name: input.name, version: input.version, idempotencyKey: input.idempotencyKey })); },
	async matches(input) {
	  const query = new URLSearchParams();
	  if (input.employeeIds.length > 0) query.set('employeeId', input.employeeIds.join(','));
	  if (input.workRoomId > 0) query.set('workRoomId', String(input.workRoomId));
	  if (input.groupId > 0) query.set('intelligentGroupId', String(input.groupId));
	  if (input.triggerStart) query.set('triggerStart', input.triggerStart);
	  if (input.triggerEnd) query.set('triggerEnd', input.triggerEnd);
	  query.set('page', String(input.page));
	  query.set('perPage', String(input.perPage));
	  return parseMatchPage(await client.request(`/sensitiveWordsMonitor/index?${query.toString()}`), input.page);
	},
    async matchDetail(id) { return client.request(`/sensitiveWordsMonitor/show?id=${id}`) as Promise<readonly Record<string, unknown>[]>; },
  };
}
