export type ConversationTargetType = 'employee' | 'customer' | 'room';

export type ConversationSearch = {
  keyword: string;
  conversationType: '' | ConversationTargetType;
  employeeIds: readonly string[];
  startAt: string;
  endAt: string;
  page: number;
  pageSize: number;
};

export type ConversationSummary = {
  id: string;
  employeeId: number;
  employeeName: string;
  employeeAvatar: string;
  targetType: ConversationTargetType;
  targetId: number;
  targetName: string;
  targetAvatar: string;
  lastMessage: string;
  sentAt: string;
};

export type ConversationPage = {
  list: readonly ConversationSummary[];
  total: number;
  page: number;
  pageSize: number;
};

export type ConversationMessage = {
  id: string;
  senderName: string;
  senderAvatar: string;
  direction: 'inbound' | 'outbound';
  type: number;
  content: Record<string, unknown>;
  sentAt: string;
};

export type ConversationDetail = {
  id: string;
  employeeId: number;
  employeeName: string;
  targetType: ConversationTargetType;
  targetId: number;
  targetName: string;
  messageTotal: number;
  truncated: boolean;
  window: 'latest';
  messages: readonly ConversationMessage[];
};

export type ConversationGlobalApi = {
  search(input: ConversationSearch): Promise<ConversationPage>;
  detail(id: string): Promise<ConversationDetail>;
};

type ApiClient = {
  request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown>;
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value);
}

function isTargetType(value: unknown): value is ConversationTargetType {
  return value === 'employee' || value === 'customer' || value === 'room';
}

function parseSummary(value: unknown): ConversationSummary | null {
  if (!isRecord(value)
    || typeof value.id !== 'string'
    || !isFiniteNumber(value.employeeId)
    || typeof value.employeeName !== 'string'
    || typeof value.employeeAvatar !== 'string'
    || !isTargetType(value.targetType)
    || !isFiniteNumber(value.targetId)
    || typeof value.targetName !== 'string'
    || typeof value.targetAvatar !== 'string'
    || typeof value.lastMessage !== 'string'
    || typeof value.sentAt !== 'string') {
    return null;
  }
  return {
    id: value.id,
    employeeId: value.employeeId,
    employeeName: value.employeeName,
    employeeAvatar: value.employeeAvatar,
    targetType: value.targetType,
    targetId: value.targetId,
    targetName: value.targetName,
    targetAvatar: value.targetAvatar,
    lastMessage: value.lastMessage,
    sentAt: value.sentAt,
  };
}

function parsePage(value: unknown): ConversationPage {
  if (!isRecord(value)
    || !Array.isArray(value.list)
    || !isFiniteNumber(value.total)
    || !isFiniteNumber(value.page)
    || !isFiniteNumber(value.pageSize)) {
    throw new Error('全局消息接口返回了无效数据');
  }
  const list = value.list.map(parseSummary);
  if (list.some((item) => item === null)) {
    throw new Error('全局消息接口返回了无效数据');
  }
  return {
    list: list as ConversationSummary[],
    total: value.total,
    page: value.page,
    pageSize: value.pageSize,
  };
}

function parseMessage(value: unknown): ConversationMessage | null {
  if (!isRecord(value)
    || typeof value.id !== 'string'
    || typeof value.senderName !== 'string'
    || typeof value.senderAvatar !== 'string'
    || (value.direction !== 'inbound' && value.direction !== 'outbound')
    || !isFiniteNumber(value.type)
    || !isRecord(value.content)
    || typeof value.sentAt !== 'string') {
    return null;
  }
  return {
    id: value.id,
    senderName: value.senderName,
    senderAvatar: value.senderAvatar,
    direction: value.direction,
    type: value.type,
    content: value.content,
    sentAt: value.sentAt,
  };
}

function parseDetail(value: unknown): ConversationDetail {
  if (!isRecord(value)
    || typeof value.id !== 'string'
    || !isFiniteNumber(value.employeeId)
    || typeof value.employeeName !== 'string'
    || !isTargetType(value.targetType)
    || !isFiniteNumber(value.targetId)
    || typeof value.targetName !== 'string'
    || !isFiniteNumber(value.messageTotal)
    || typeof value.truncated !== 'boolean'
    || value.window !== 'latest'
    || !Array.isArray(value.messages)) {
    throw new Error('会话详情接口返回了无效数据');
  }
  const messages = value.messages.map(parseMessage);
  if (messages.some((message) => message === null)) {
    throw new Error('会话详情接口返回了无效数据');
  }
  return {
    id: value.id,
    employeeId: value.employeeId,
    employeeName: value.employeeName,
    targetType: value.targetType,
    targetId: value.targetId,
    targetName: value.targetName,
    messageTotal: value.messageTotal,
    truncated: value.truncated,
    window: value.window,
    messages: messages as ConversationMessage[],
  };
}

function appendNonBlank(query: URLSearchParams, key: string, value: string) {
  if (value.trim() !== '') {
    query.set(key, value.trim());
  }
}

function appendAllNonBlank(query: URLSearchParams, key: string, values: readonly string[]) {
  values.forEach((value) => {
    if (value.trim() !== '') {
      query.append(key, value.trim());
    }
  });
}

export function createConversationGlobalApi(
  client: ApiClient,
): ConversationGlobalApi {
  return {
    async search(input) {
      const query = new URLSearchParams({
        view: 'global',
      });
      appendNonBlank(query, 'keyword', input.keyword);
      appendNonBlank(query, 'conversationType', input.conversationType);
      appendAllNonBlank(query, 'employeeIds', input.employeeIds);
      appendNonBlank(query, 'startAt', input.startAt);
      appendNonBlank(query, 'endAt', input.endAt);
      query.set('page', String(input.page));
      query.set('pageSize', String(input.pageSize));
      return parsePage(await client.request(`/workMessage/toUsers?${query.toString()}`));
    },
    async detail(id) {
      const query = new URLSearchParams({ id });
      return parseDetail(await client.request(`/workMessage/detail?${query.toString()}`));
    },
  };
}
