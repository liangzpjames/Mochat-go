export type SensitiveWordItem = { id: number; groupId: number; groupName: string; name: string; status: number; version: string; employeeHitCount: number; customerHitCount: number; createdAt: string };
export type SensitiveWordGroup = { id: number; name: string; version: string; wordCount: number; enabledCount: number };
export type SensitiveWordPage = { items: SensitiveWordItem[]; total: number; page: number; perPage: number };
export type SensitiveWordMatch = { id: number; sensitiveWordID: number; sensitiveWordName: string; source: number; sourceText: string; triggerName: string; triggerScenario: string; triggerTime: string; contentPreview: string; workRoomID: number };
export type SensitiveWordMatchPage = { items: SensitiveWordMatch[]; total: number; page: number; perPage: number };
export type SensitiveWordMatchDetail = { sender: string; messageType: string; sendTime: string; isTrigger: boolean; content: string };
export type SensitiveWordMutationResult = { version: string; idempotent: boolean };
export type SensitiveWordMatchFilters = { employeeIds: number[]; workRoomId: number; groupId: number; sensitiveWordId?: number; source?: number; scenario?: string; triggerStart: string; triggerEnd: string; page: number; perPage: number };
export type SensitiveWordScannerStatus = { enabled: boolean; state: 'ready' | 'never_run' | 'failed' | 'disabled'; lastAttemptAt?: string; lastSuccessAt?: string; lastFailureAt?: string; lastError?: string };
export type SensitiveWordFilterOption = { id: number; name: string; count: number };
export type SensitiveWordFilterOptions = { employees: readonly SensitiveWordFilterOption[]; rooms: readonly SensitiveWordFilterOption[] };

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

function string(value: unknown): string {
  return typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' ? String(value) : '';
}

function messageText(value: unknown): string {
  if (typeof value === 'string' || typeof value === 'number') return String(value);
  if (!value || typeof value !== 'object') return '';
  const row = value as Record<string, unknown>;
  for (const key of ['content', 'text', 'body', 'message', 'title']) {
    const text = string(row[key]).trim();
    if (text) return text;
  }
  return '';
}

function messageType(value: unknown): string {
  const type = number(value);
  return ({ 1: '文本', 2: '图片', 3: '语音', 4: '视频', 5: '文件', 6: '链接' } as Record<number, string>)[type] ?? (type > 0 ? '其他消息' : '未知消息');
}

function parseMatchDetail(value: unknown): SensitiveWordMatchDetail[] {
  const list = Array.isArray(value) ? value : [];
  return list.map((item) => {
    const row = (item ?? {}) as Record<string, unknown>;
    const isTrigger = row.isTrigger ?? row.IsTrigger;
    return {
      sender: string(row.sender ?? row.Sender),
      messageType: messageType(row.msgType ?? row.MsgType),
      sendTime: string(row.sendTime ?? row.SendTime),
      isTrigger: isTrigger === undefined ? true : Boolean(Number(isTrigger)),
      content: messageText(row.msgContent ?? row.MsgContent),
    };
  });
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
        groupName: string(row.groupName),
        name: string(row.name),
        status: number(row.status),
        version: string(row.version),
        employeeHitCount: number(row.employeeNum ?? row.employeeHitCount),
        customerHitCount: number(row.contactNum ?? row.customerHitCount),
        createdAt: string(row.createdAt),
      };
    }),
    total: number(record?.page?.total),
    page: 1,
    perPage: 20,
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
        sensitiveWordID: number(row.sensitiveWordId ?? row.sensitiveWordID),
        sensitiveWordName: string(row.sensitiveWordName),
        source: number(row.source),
        sourceText: string(row.sourceText),
        triggerName: string(row.triggerName),
        triggerScenario: string(row.triggerScenario),
        triggerTime: string(row.triggerTime),
        contentPreview: string(row.contentPreview),
        workRoomID: number(row.workRoomId ?? row.workRoomID),
      };
    }),
    total: number(record?.page?.total),
    page,
    perPage: 20,
  };
}

export type SensitiveWordApi = {
  list(input: { groupId: number; keywords: string; status?: number; page: number; perPage: number }): Promise<SensitiveWordPage>;
  groups(): Promise<SensitiveWordGroup[]>;
  create(input: { groupId: number; names: string[]; version: string; idempotencyKey: string }): Promise<SensitiveWordMutationResult>;
  setEnabled(input: { id: number; enabled: boolean; version: string; idempotencyKey: string }): Promise<SensitiveWordMutationResult>;
  move(input: { id: number; groupId: number; version: string; idempotencyKey: string }): Promise<SensitiveWordMutationResult>;
  remove(input: { id: number; version: string; idempotencyKey: string; confirmed: true }): Promise<SensitiveWordMutationResult>;
  createGroup(input: { names: string[]; version: string; idempotencyKey: string }): Promise<SensitiveWordMutationResult>;
  renameGroup(input: { id: number; name: string; version: string; idempotencyKey: string }): Promise<SensitiveWordMutationResult>;
  matches(input: SensitiveWordMatchFilters): Promise<SensitiveWordMatchPage>;
  matchDetail(id: number): Promise<readonly SensitiveWordMatchDetail[]>;
  monitorStatus(): Promise<SensitiveWordScannerStatus>;
  filterOptions(): Promise<SensitiveWordFilterOptions>;
};

export function createSensitiveWordApi(client: Client): SensitiveWordApi {
  return {
	  async list(input) {
      const query = new URLSearchParams({ groupId: String(input.groupId), keyWords: input.keywords });
      if (input.status !== undefined) query.set('status', String(input.status));
      query.set('page', String(input.page));
      query.set('perPage', '20');
      const value = await client.request(`/sensitiveWord/index?${query.toString()}`);
      const result = parsePage(value);
      return { ...result, page: input.page };
    },
    async groups() {
      const rows = await client.request<unknown>('/sensitiveWordGroup/select');
      return (Array.isArray(rows) ? rows : []).map((row) => {
        const value = row as Record<string, unknown>;
	  return { id: number(value.groupId ?? value.id), name: string(value.name), version: string(value.version), wordCount: number(value.wordCount), enabledCount: number(value.enabledCount) };
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
	  if (input.sensitiveWordId && input.sensitiveWordId > 0) query.set('sensitiveWordId', String(input.sensitiveWordId));
	  if (input.source && input.source > 0) query.set('source', String(input.source));
	  if (input.scenario) query.set('scenario', input.scenario);
	  if (input.triggerStart) query.set('triggerStart', input.triggerStart);
	  if (input.triggerEnd) query.set('triggerEnd', input.triggerEnd);
	  query.set('page', String(input.page));
	  query.set('perPage', '20');
	  return parseMatchPage(await client.request(`/sensitiveWordsMonitor/index?${query.toString()}`), input.page);
	},
    async matchDetail(id) { return parseMatchDetail(await client.request(`/sensitiveWordsMonitor/show?id=${id}`)); },
    async monitorStatus() { return client.request('/sensitiveWordsMonitor/status'); },
    async filterOptions() {
      const [staff, rooms] = await Promise.all([
        client.request('/workMessage/staffDirectory?mode=all&page=1&pageSize=50'),
        client.request('/workMessage/roomFilterOptions?kind=group'),
      ]);
      const staffRecord = staff as { employees?: unknown };
      const employeeRows = Array.isArray(staffRecord?.employees) ? staffRecord.employees : [];
      const employees = employeeRows.map((item) => {
        const row = item as Record<string, unknown>;
        return { id: number(row.id), name: string(row.name), count: number(row.conversationCount) };
      }).filter((item) => item.id > 0 && item.name !== '');
      const roomRecord = rooms as { groups?: unknown };
      const groupRows = Array.isArray(roomRecord?.groups) ? roomRecord.groups : [];
      const roomOptions = groupRows.map((item) => {
        const row = item as Record<string, unknown>;
        return { id: Number(row.value) || 0, name: string(row.label), count: number(row.count) };
      }).filter((item) => item.id > 0 && item.name !== '');
      return { employees, rooms: roomOptions };
    },
  };
}
