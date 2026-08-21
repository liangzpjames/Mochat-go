export type TransferCustomer = {
  contactId: number;
  employeeId: number;
  contactWxId: string;
  employeeWxId: string;
  contactName: string;
  nickName: string;
  corpName: string;
  employeeName: string;
  tags: readonly string[];
  transferState: string;
  addTime: string;
  addWay: string;
};

export type TransferRoom = {
  roomId: number;
  chatId: string;
  roomName: string;
  owner: string;
  userNum: number;
  addNum: number;
  quitNum: number;
  createTime: string;
};

export type TransferLog = {
  mode: 1 | 2 | 3;
  name: string;
  employee: string;
  state: string;
  corpName: string;
  roomNum: number;
  createTime: string;
};

export type EmployeeOption = { id: number; name: string; wxUserId: string; status: number };
export type TransferResult = { errcode: number; errmsg?: string; [key: string]: unknown };

export type ContactTransferFilter = {
  contactName: string;
  employeeIds: readonly number[];
  addTimeStart: string;
  addTimeEnd: string;
  page?: number;
  perPage?: number;
};

export type ContactTransferRoomFilter = { roomName: string };
export type ContactTransferLogFilter = {
  mode: 1 | 2 | 3 | '';
  name: string;
  employeeWxId: string;
  createTimeStart: string;
  createTimeEnd: string;
};

export type TransferCustomerInput = {
  type: number;
  list: readonly { employeeWxId: string; contactWxId: string }[];
  takeoverUserId: string;
};
export type TransferRoomInput = { list: readonly string[]; takeoverUserId: string };

type Client = {
  request<T>(input: RequestInfo | URL, init?: RequestInit): Promise<T>;
};

function objectPayload(value: unknown): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return {};
  const source = value as Record<string, unknown>;
  if (typeof source.data === 'object' && source.data !== null) return source.data as Record<string, unknown>;
  return source;
}

function arrayPayload(value: unknown): unknown[] {
  if (Array.isArray(value)) return value;
  const source = objectPayload(value);
  return Array.isArray(source.list) ? source.list : Array.isArray(source.items) ? source.items : [];
}

function numberValue(value: unknown): number {
  const result = Number(value);
  return Number.isFinite(result) ? result : 0;
}

function stringValue(value: unknown): string { return typeof value === 'string' ? value : value == null ? '' : String(value); }

function transferCustomer(value: unknown): TransferCustomer {
  const source = typeof value === 'object' && value !== null ? value as Record<string, unknown> : {};
  return {
    contactId: numberValue(source.contactId), employeeId: numberValue(source.employeeId),
    contactWxId: stringValue(source.contactWxId), employeeWxId: stringValue(source.employeeWxId),
    contactName: stringValue(source.contactName ?? source.name), nickName: stringValue(source.nickName),
    corpName: stringValue(source.corpName), employeeName: stringValue(source.employeeName),
    tags: Array.isArray(source.tags) ? source.tags.map(stringValue) : [],
    transferState: stringValue(source.transferState), addTime: stringValue(source.addTime), addWay: stringValue(source.addWay),
  };
}

function transferRoom(value: unknown): TransferRoom {
  const source = typeof value === 'object' && value !== null ? value as Record<string, unknown> : {};
  return {
    roomId: numberValue(source.roomId), chatId: stringValue(source.chatId), roomName: stringValue(source.roomName ?? source.name),
    owner: stringValue(source.owner), userNum: numberValue(source.userNum), addNum: numberValue(source.addNum),
    quitNum: numberValue(source.quitNum), createTime: stringValue(source.createTime),
  };
}

function transferLog(value: unknown): TransferLog {
  const source = typeof value === 'object' && value !== null ? value as Record<string, unknown> : {};
  const mode = numberValue(source.mode);
  return {
    mode: mode === 2 || mode === 3 ? mode : 1, name: stringValue(source.name), employee: stringValue(source.employee),
    state: stringValue(source.state), corpName: stringValue(source.corpName), roomNum: numberValue(source.roomNum),
    createTime: stringValue(source.createTime),
  };
}

function queryString(entries: Record<string, string | number | readonly number[] | undefined>): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(entries)) {
    if (value === undefined) continue;
    if (Array.isArray(value)) {
      if (value.length > 0) params.set(key, JSON.stringify(value));
      continue;
    }
    params.set(key, String(value));
  }
  return params.toString();
}

function contactFilterQuery(filter: ContactTransferFilter): string {
  const entries: Record<string, string | number | readonly number[] | undefined> = {
    page: filter.page ?? 1,
    perPage: filter.perPage ?? 20,
  };
  if (filter.contactName.trim()) entries.contactName = filter.contactName.trim();
  if (filter.employeeIds.length > 0) entries.employeeId = filter.employeeIds;
  if (filter.addTimeStart) entries.addTimeStart = filter.addTimeStart;
  if (filter.addTimeEnd) entries.addTimeEnd = filter.addTimeEnd;
  return queryString(entries);
}

export function createContactTransferApi(client: Client) {
  const read = async <T>(endpoint: string, query: string, parse: (payload: unknown) => T): Promise<T> =>
    parse(await client.request(`${endpoint}${query ? `?${query}` : ''}`));
  return {
    async assigned(filter: ContactTransferFilter): Promise<readonly TransferCustomer[]> {
      return read('/contactTransfer/info', contactFilterQuery(filter), (payload) => arrayPayload(payload).map(transferCustomer));
    },
    async unassigned(filter: ContactTransferFilter): Promise<{ list: readonly TransferCustomer[]; lastTime: string }> {
      return read('/contactTransfer/unassignedList', contactFilterQuery(filter), (payload) => {
        const source = objectPayload(payload);
        return { list: arrayPayload(source).map(transferCustomer), lastTime: stringValue(source.lastTime) };
      });
    },
    async rooms(filter: ContactTransferRoomFilter): Promise<readonly TransferRoom[]> {
      return read('/contactTransfer/room', queryString(filter), (payload) => arrayPayload(payload).map(transferRoom));
    },
    async logs(filter: ContactTransferLogFilter): Promise<readonly TransferLog[]> {
      return read('/contactTransfer/log', queryString(filter), (payload) => arrayPayload(payload).map(transferLog));
    },
    async employees(keyword: string): Promise<readonly EmployeeOption[]> {
      const params = new URLSearchParams({ page: '1', perPage: '200' });
      if (keyword.trim()) params.set('name', keyword.trim());
      const rows = arrayPayload(await client.request(`/workEmployee/index?${params.toString()}`));
      return rows.flatMap((item) => {
        const source = typeof item === 'object' && item !== null ? item as Record<string, unknown> : {};
        const wxUserId = stringValue(source.wxUserId ?? source.wx_user_id ?? source.userid);
        const status = numberValue(source.status);
        if (!wxUserId || (status !== 0 && status !== 1)) return [];
        return [{ id: numberValue(source.id), name: stringValue(source.name), wxUserId, status }];
      });
    },
    async syncUnassigned(): Promise<void> {
      await client.request('/contactTransfer/sync', { method: 'POST' });
    },
    async transferCustomers(input: TransferCustomerInput): Promise<readonly TransferResult[]> {
      const result = await client.request<unknown>('/contactTransfer/index', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...input, list: input.list }),
      });
      return arrayPayload(result) as TransferResult[];
    },
    async transferRooms(input: TransferRoomInput): Promise<readonly TransferResult[]> {
      const result = await client.request<unknown>('/contactTransfer/room', {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input),
      });
      const failedByChatID = new Map(
        arrayPayload(result).flatMap((item) => {
          const source = typeof item === 'object' && item !== null ? item as Record<string, unknown> : {};
          const chatID = stringValue(source.chat_id ?? source.chatId);
          return chatID ? [[chatID, {
            errcode: numberValue(source.errcode) || 1,
            errmsg: stringValue(source.errmsg) || '群聊转接失败',
          } as TransferResult] as const] : [];
        }),
      );
      return input.list.map((chatID) => failedByChatID.get(chatID) ?? { errcode: 0 });
    },
  };
}

export type ContactTransferApi = ReturnType<typeof createContactTransferApi>;
