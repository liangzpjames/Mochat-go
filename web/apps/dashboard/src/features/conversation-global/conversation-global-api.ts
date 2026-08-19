export type ConversationTargetType = 'employee' | 'customer' | 'room';

export type ConversationSearch = {
  keyword: string;
  conversationType: '' | ConversationTargetType;
  employeeIds: readonly string[];
  startAt: string;
  endAt: string;
  bucket?: 'all' | 'timeout' | 'risk' | 'today' | 'focused';
  messageTypes?: readonly string[];
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
  conversationId?: string;
  lastMessageType?: number;
  lastDirection?: 'inbound' | 'outbound';
  messageTotal?: number;
  riskCount?: number;
  timeoutCount?: number;
  focused?: boolean;
  archiveSource?: string;
  archiveSourceId?: string;
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
  archiveSource?: string;
  archiveSourceId?: string;
};

export type StaffDirectoryMode = 'all' | 'focused' | 'archived' | 'departed';

export type StaffDirectoryInput = {
  mode: StaffDirectoryMode;
  keyword: string;
  departmentId: number | null;
  page: number;
  pageSize: 50;
};

export type StaffDepartment = {
  id: number;
  parentId: number;
  name: string;
  employeeCount: number;
  children: readonly StaffDepartment[];
};

export type StaffDirectoryEmployee = {
  id: number;
  name: string;
  avatar: string;
  status: 0 | 1 | 2 | 4 | 5;
  departmentIds: readonly number[];
  archived: boolean;
  conversationCount: number;
  focusedConversationCount: number;
  lastConversationAt: string;
};

export type ConversationCapability = { key: string; available: boolean; reason?: string };

export type StaffDirectoryPage = {
  departments: readonly StaffDepartment[];
  employees: readonly StaffDirectoryEmployee[];
  counts: { all: number; focused: number; archived: number; departed: number };
  page: number;
  pageSize: 50;
  total: number;
  limitations: readonly { key: string; reason: string }[];
  capabilities: readonly ConversationCapability[];
};

export type StaffDetailInput = {
  conversationId: string;
  keyword: string;
  messageTypes: readonly string[];
  date: string;
  pageSize: 50;
  before?: string;
};

export type StaffConversationStats = {
  communicationDays: number | null;
  messageTotal: number | null;
  inboundTotal: number | null;
  outboundTotal: number | null;
};

export type StaffConversationDetail = {
  conversationId: string;
  employeeId: number;
  employeeName: string;
  targetType: ConversationTargetType;
  targetId: number;
  targetName: string;
  focused: boolean;
  stats: StaffConversationStats;
  messages: readonly ConversationMessage[];
  nextBefore: string;
  hasMore: boolean;
  capabilities: readonly ConversationCapability[];
};

export type CustomerDirectoryMode = 'all' | 'focused' | 'active' | 'lost';
export type CustomerProfileStatus = 'available' | 'missing' | 'deleted';
export type CustomerConversationMode = 'direct' | 'group';
export type CustomerDirectoryInput = { mode: CustomerDirectoryMode; keyword: string; page: number; pageSize: 50 };
export type CustomerDirectoryCustomer = { id: number; name: string; avatar: string; profileStatus: CustomerProfileStatus; activeRelationCount: number; lostRelationCount: number; directConversationCount: number; groupConversationCount: number; focusedConversationCount: number; lastConversationAt: string };
export type CustomerDirectoryPage = { customers: readonly CustomerDirectoryCustomer[]; counts: { all: number; focused: number; active: number; lost: number }; page: number; pageSize: 50; total: number; limitations: readonly { key: string; reason: string }[]; capabilities: readonly ConversationCapability[] };
export type CustomerConversationInput = { customerId: number; mode: CustomerConversationMode; page: number; pageSize: 20 };
export type CustomerConversationSummary = ConversationSummary & { relationStatus?: 'active' | 'lost' | 'unknown'; membershipStatus?: 'active' | 'left' };
export type CustomerConversationPage = { customer: { id: number; name: string; avatar: string; profileStatus: CustomerProfileStatus }; mode: CustomerConversationMode; list: readonly CustomerConversationSummary[]; total: number; page: number; pageSize: 20; capabilities: readonly ConversationCapability[] };
export type CustomerDetailInput = { customerId: number; conversationId: string; keyword: string; messageTypes: readonly string[]; date: string; pageSize: 50; before?: string };
export type CustomerConversationProfile = { id: number; name: string; avatar: string; profileStatus: CustomerProfileStatus };
export type CustomerConversationDetail = StaffConversationDetail & { customerId: number; customerName: string; profile: CustomerConversationProfile };

// messageText 将归档消息内容还原为可读文本：内容为 JSON 时优先取 text 字段，
// 避免在会话详情里直接展示 {"text":"..."} 原始串。
export function messageText(message: ConversationMessage): string {
  const record = message.content ?? {};
  // 当前接口形态：{ text: "..." }
  if (typeof record.text === 'string' && record.text.trim() !== '') {
    return record.text;
  }
  const raw = record.content;
  if (typeof raw === 'string') {
    const trimmed = raw.trim();
    if (trimmed === '') return '';
    try {
      const parsed: unknown = JSON.parse(trimmed);
      if (typeof parsed === 'string' && parsed.trim() !== '') {
        return parsed;
      }
      if (
        parsed !== null &&
        typeof parsed === 'object' &&
        typeof (parsed as { text?: unknown }).text === 'string' &&
        String((parsed as { text: unknown }).text).trim() !== ''
      ) {
        return String((parsed as { text: unknown }).text);
      }
    } catch {
      // 非 JSON 文本直接展示
    }
    return raw;
  }
  return JSON.stringify(message.content);
}

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

export type ConversationMetricValue = {
  value: number | null;
  changeRate: number | null;
  status: 'available' | 'unavailable';
  reason?: string;
};

export type ConversationOverview = {
  metrics: Record<string, ConversationMetricValue>;
  capabilities: readonly {
    key: string;
    available: boolean;
    reason?: string;
  }[];
};

export type ConversationGlobalApi = {
  search(input: ConversationSearch): Promise<ConversationPage>;
  overview?(input: Omit<ConversationSearch, 'page' | 'pageSize'>): Promise<ConversationOverview>;
  detail(id: string): Promise<ConversationDetail>;
  setFocus?(id: string): Promise<void>;
  removeFocus?(id: string): Promise<void>;
  employees?(input: { keyword: string }): Promise<readonly ConversationEmployee[]>;
  staffDirectory?(input: StaffDirectoryInput): Promise<StaffDirectoryPage>;
  staffDetail?(input: StaffDetailInput): Promise<StaffConversationDetail>;
  customerDirectory?(input: CustomerDirectoryInput): Promise<CustomerDirectoryPage>;
  customerConversations?(input: CustomerConversationInput): Promise<CustomerConversationPage>;
  customerDetail?(input: CustomerDetailInput): Promise<CustomerConversationDetail>;
};

export type ConversationEmployee = {
  id: number;
  name: string;
  avatar: string;
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
  const summary: ConversationSummary = {
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
  if (typeof value.conversationId === 'string') summary.conversationId = value.conversationId;
  if (isFiniteNumber(value.lastMessageType)) summary.lastMessageType = value.lastMessageType;
  if (value.lastDirection === 'inbound' || value.lastDirection === 'outbound') summary.lastDirection = value.lastDirection;
  if (isFiniteNumber(value.messageTotal)) summary.messageTotal = value.messageTotal;
  if (isFiniteNumber(value.riskCount)) summary.riskCount = value.riskCount;
  if (isFiniteNumber(value.timeoutCount)) summary.timeoutCount = value.timeoutCount;
  if (typeof value.focused === 'boolean') summary.focused = value.focused;
  if (typeof value.archiveSource === 'string') summary.archiveSource = value.archiveSource;
  if (typeof value.archiveSourceId === 'string') summary.archiveSourceId = value.archiveSourceId;
  return summary;
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
  const message: ConversationMessage = {
    id: value.id,
    senderName: value.senderName,
    senderAvatar: value.senderAvatar,
    direction: value.direction,
    type: value.type,
    content: value.content,
    sentAt: value.sentAt,
  };
  if (typeof value.archiveSource === 'string') message.archiveSource = value.archiveSource;
  if (typeof value.archiveSourceId === 'string') message.archiveSourceId = value.archiveSourceId;
  return message;
}

function parseCapability(value: unknown): ConversationCapability | null {
  if (!isRecord(value) || typeof value.key !== 'string' || typeof value.available !== 'boolean') return null;
  const capability: ConversationCapability = { key: value.key, available: value.available };
  if (typeof value.reason === 'string') capability.reason = value.reason;
  return capability;
}

function parseStaffDepartment(value: unknown): StaffDepartment | null {
  if (!isRecord(value) || !isFiniteNumber(value.id) || !isFiniteNumber(value.parentId)
    || typeof value.name !== 'string' || !isFiniteNumber(value.employeeCount) || !Array.isArray(value.children)) return null;
  const children = value.children.map(parseStaffDepartment);
  if (children.some((item) => item === null)) return null;
  return { id: value.id, parentId: value.parentId, name: value.name, employeeCount: value.employeeCount, children: children as StaffDepartment[] };
}

function parseStaffDirectory(value: unknown): StaffDirectoryPage {
  if (!isRecord(value) || !Array.isArray(value.departments) || !Array.isArray(value.employees)
    || !isRecord(value.counts) || !isFiniteNumber(value.page) || value.pageSize !== 50 || !isFiniteNumber(value.total)
    || !Array.isArray(value.limitations) || !Array.isArray(value.capabilities)) {
    throw new Error('员工目录接口返回了无效数据');
  }
  const departments = value.departments.map(parseStaffDepartment);
  const employees = value.employees.map((item): StaffDirectoryEmployee | null => {
    if (!isRecord(item) || !isFiniteNumber(item.id) || typeof item.name !== 'string' || typeof item.avatar !== 'string'
      || ![0, 1, 2, 4, 5].includes(Number(item.status)) || !Array.isArray(item.departmentIds)
      || item.departmentIds.some((id) => !isFiniteNumber(id)) || typeof item.archived !== 'boolean'
      || !isFiniteNumber(item.conversationCount) || !isFiniteNumber(item.focusedConversationCount) || typeof item.lastConversationAt !== 'string') return null;
    return {
      id: item.id, name: item.name, avatar: item.avatar, status: item.status as StaffDirectoryEmployee['status'],
      departmentIds: item.departmentIds as number[], archived: item.archived,
      conversationCount: item.conversationCount, focusedConversationCount: item.focusedConversationCount, lastConversationAt: item.lastConversationAt,
    };
  });
  const counts = value.counts;
  if (departments.some((item) => item === null) || employees.some((item) => item === null)
    || !isFiniteNumber(counts.all) || !isFiniteNumber(counts.focused) || !isFiniteNumber(counts.archived) || !isFiniteNumber(counts.departed)) {
    throw new Error('员工目录接口返回了无效数据');
  }
  const limitations = value.limitations.map((item) => isRecord(item) && typeof item.key === 'string' && typeof item.reason === 'string' ? { key: item.key, reason: item.reason } : null);
  const capabilities = value.capabilities.map(parseCapability);
  if (limitations.some((item) => item === null) || capabilities.some((item) => item === null)) throw new Error('员工目录接口返回了无效数据');
  return {
    departments: departments as StaffDepartment[], employees: employees as StaffDirectoryEmployee[],
    counts: { all: counts.all, focused: counts.focused, archived: counts.archived, departed: counts.departed },
    page: value.page, pageSize: 50, total: value.total,
    limitations: limitations as StaffDirectoryPage['limitations'], capabilities: capabilities as ConversationCapability[],
  };
}

function nullableFinite(value: unknown): value is number | null {
  return value === null || isFiniteNumber(value);
}

function parseStaffDetail(value: unknown): StaffConversationDetail {
  if (!isRecord(value) || typeof value.conversationId !== 'string' || value.conversationId.trim() === ''
    || !isFiniteNumber(value.employeeId) || typeof value.employeeName !== 'string' || !isTargetType(value.targetType)
    || !isFiniteNumber(value.targetId) || typeof value.targetName !== 'string' || typeof value.focused !== 'boolean'
    || !isRecord(value.stats) || !Array.isArray(value.messages) || typeof value.nextBefore !== 'string'
    || typeof value.hasMore !== 'boolean' || !Array.isArray(value.capabilities)) {
    throw new Error('员工会话详情接口返回了无效数据');
  }
  const { stats } = value;
  if (!nullableFinite(stats.communicationDays) || !nullableFinite(stats.messageTotal)
    || !nullableFinite(stats.inboundTotal) || !nullableFinite(stats.outboundTotal)) throw new Error('员工会话详情接口返回了无效数据');
  const messages = value.messages.map(parseMessage);
  const capabilities = value.capabilities.map(parseCapability);
  if (messages.some((item) => item === null) || capabilities.some((item) => item === null)) throw new Error('员工会话详情接口返回了无效数据');
  return {
    conversationId: value.conversationId, employeeId: value.employeeId, employeeName: value.employeeName,
    targetType: value.targetType, targetId: value.targetId, targetName: value.targetName, focused: value.focused,
    stats: { communicationDays: stats.communicationDays, messageTotal: stats.messageTotal, inboundTotal: stats.inboundTotal, outboundTotal: stats.outboundTotal },
    messages: messages as ConversationMessage[], nextBefore: value.nextBefore, hasMore: value.hasMore,
    capabilities: capabilities as ConversationCapability[],
  };
}

function isProfileStatus(value: unknown): value is CustomerProfileStatus {
  return value === 'available' || value === 'missing' || value === 'deleted';
}

function parseCustomerDirectory(value: unknown): CustomerDirectoryPage {
  if (!isRecord(value) || !Array.isArray(value.customers) || !isRecord(value.counts)
    || !isFiniteNumber(value.page) || value.pageSize !== 50 || !isFiniteNumber(value.total)
    || !Array.isArray(value.limitations) || !Array.isArray(value.capabilities)) {
    throw new Error('客户目录接口返回了无效数据');
  }
  const customers = value.customers.map((item): CustomerDirectoryCustomer | null => {
    if (!isRecord(item) || !isFiniteNumber(item.id) || typeof item.name !== 'string' || typeof item.avatar !== 'string'
      || !isProfileStatus(item.profileStatus) || !isFiniteNumber(item.activeRelationCount)
      || !isFiniteNumber(item.lostRelationCount) || !isFiniteNumber(item.directConversationCount)
      || !isFiniteNumber(item.groupConversationCount) || !isFiniteNumber(item.focusedConversationCount)
      || typeof item.lastConversationAt !== 'string') return null;
    return {
      id: item.id, name: item.name, avatar: item.avatar, profileStatus: item.profileStatus,
      activeRelationCount: item.activeRelationCount, lostRelationCount: item.lostRelationCount,
      directConversationCount: item.directConversationCount, groupConversationCount: item.groupConversationCount,
      focusedConversationCount: item.focusedConversationCount, lastConversationAt: item.lastConversationAt,
    };
  });
  const counts = value.counts;
  const limitations = value.limitations.map((item) => isRecord(item) && typeof item.key === 'string' && typeof item.reason === 'string' ? { key: item.key, reason: item.reason } : null);
  const capabilities = value.capabilities.map(parseCapability);
  if (customers.some((item) => item === null) || limitations.some((item) => item === null) || capabilities.some((item) => item === null)
    || !isFiniteNumber(counts.all) || !isFiniteNumber(counts.focused) || !isFiniteNumber(counts.active) || !isFiniteNumber(counts.lost)) {
    throw new Error('客户目录接口返回了无效数据');
  }
  return {
    customers: customers as CustomerDirectoryCustomer[],
    counts: { all: counts.all, focused: counts.focused, active: counts.active, lost: counts.lost },
    page: value.page, pageSize: 50, total: value.total,
    limitations: limitations as CustomerDirectoryPage['limitations'], capabilities: capabilities as ConversationCapability[],
  };
}

function parseCustomerConversations(value: unknown): CustomerConversationPage {
  if (!isRecord(value) || !isRecord(value.customer) || !isFiniteNumber(value.customer.id)
    || typeof value.customer.name !== 'string' || typeof value.customer.avatar !== 'string'
    || !isProfileStatus(value.customer.profileStatus) || !isCustomerConversationMode(value.mode)
    || !Array.isArray(value.list) || !isFiniteNumber(value.total) || !isFiniteNumber(value.page)
    || value.pageSize !== 20 || !Array.isArray(value.capabilities)) throw new Error('客户会话列表接口返回了无效数据');
  const list = value.list.map((item): CustomerConversationSummary | null => {
    const summary = parseSummary(item);
    if (!summary || !isRecord(item) || typeof item.conversationId !== 'string' || item.conversationId.trim() === ''
      || (item.relationStatus !== undefined && !isRelationStatus(item.relationStatus))
      || (item.membershipStatus !== undefined && !isMembershipStatus(item.membershipStatus))) return null;
    const result: CustomerConversationSummary = { ...summary };
    if (isRecord(item) && isRelationStatus(item.relationStatus)) result.relationStatus = item.relationStatus;
    if (isRecord(item) && isMembershipStatus(item.membershipStatus)) result.membershipStatus = item.membershipStatus;
    return result;
  });
  const capabilities = value.capabilities.map(parseCapability);
  if (list.some((item) => item === null) || capabilities.some((item) => item === null)) throw new Error('客户会话列表接口返回了无效数据');
  return {
    customer: { id: value.customer.id, name: value.customer.name, avatar: value.customer.avatar, profileStatus: value.customer.profileStatus },
    mode: value.mode, list: list as CustomerConversationSummary[], total: value.total, page: value.page, pageSize: 20,
    capabilities: capabilities as ConversationCapability[],
  };
}

function isCustomerConversationMode(value: unknown): value is CustomerConversationMode { return value === 'direct' || value === 'group'; }
function isRelationStatus(value: unknown): value is NonNullable<CustomerConversationSummary['relationStatus']> { return value === 'active' || value === 'lost' || value === 'unknown'; }
function isMembershipStatus(value: unknown): value is NonNullable<CustomerConversationSummary['membershipStatus']> { return value === 'active' || value === 'left'; }

function parseCustomerDetail(value: unknown, expectedCustomerId: number, expectedConversationId: string): CustomerConversationDetail {
  if (!isRecord(value) || !isFiniteNumber(value.customerId) || value.customerId !== expectedCustomerId
    || typeof value.customerName !== 'string' || value.customerName.trim() === ''
    || typeof value.conversationId !== 'string' || value.conversationId.trim() === '' || value.conversationId !== expectedConversationId
    || !isRecord(value.profile) || !isFiniteNumber(value.profile.id) || value.profile.id !== value.customerId
    || typeof value.profile.name !== 'string' || value.profile.name.trim() === '' || typeof value.profile.avatar !== 'string'
    || !isProfileStatus(value.profile.profileStatus) || (value.targetType !== 'customer' && value.targetType !== 'room')) {
    throw new Error('客户会话详情接口返回了无效数据');
  }
  let detail: StaffConversationDetail;
  try {
    detail = parseStaffDetail(value);
  } catch {
    throw new Error('客户会话详情接口返回了无效数据');
  }
  return {
    ...detail,
    customerId: value.customerId,
    customerName: value.customerName,
    profile: { id: value.profile.id, name: value.profile.name, avatar: value.profile.avatar, profileStatus: value.profile.profileStatus },
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

function parseEmployees(value: unknown): readonly ConversationEmployee[] {
  if (!Array.isArray(value)) {
    throw new Error('员工列表接口返回了无效数据');
  }
  const employees = value.map((item) => {
    if (!isRecord(item) || !isFiniteNumber(item.id) || typeof item.name !== 'string' || typeof item.avatar !== 'string') {
      return null;
    }
    return { id: item.id, name: item.name, avatar: item.avatar };
  });
  if (employees.some((item) => item === null)) {
    throw new Error('员工列表接口返回了无效数据');
  }
  return employees as ConversationEmployee[];
}

function parseOverview(value: unknown): ConversationOverview {
  if (!isRecord(value) || !isRecord(value.metrics) || !Array.isArray(value.capabilities)) {
    throw new Error('全局消息概览接口返回了无效数据');
  }
  const metrics: Record<string, ConversationMetricValue> = {};
  for (const [key, raw] of Object.entries(value.metrics)) {
    if (!isRecord(raw)
      || (raw.value !== null && !isFiniteNumber(raw.value))
      || (raw.changeRate !== null && raw.changeRate !== undefined && !isFiniteNumber(raw.changeRate))
      || (raw.status !== 'available' && raw.status !== 'unavailable')) {
      throw new Error('全局消息概览接口返回了无效数据');
    }
    const metric: ConversationMetricValue = {
      value: raw.value,
      changeRate: raw.changeRate === undefined ? null : raw.changeRate,
      status: raw.status,
    };
    if (typeof raw.reason === 'string') metric.reason = raw.reason;
    metrics[key] = metric;
  }
  const capabilities = value.capabilities.map((raw) => {
    if (!isRecord(raw) || typeof raw.key !== 'string' || typeof raw.available !== 'boolean') {
      return null;
    }
    return { key: raw.key, available: raw.available, reason: typeof raw.reason === 'string' ? raw.reason : undefined };
  });
  if (capabilities.some((item) => item === null)) {
    throw new Error('全局消息概览接口返回了无效数据');
  }
  return { metrics, capabilities: capabilities as ConversationOverview['capabilities'] };
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
      appendNonBlank(query, 'bucket', input.bucket ?? '');
      appendAllNonBlank(query, 'messageTypes', input.messageTypes ?? []);
      query.set('page', String(input.page));
      query.set('pageSize', String(input.pageSize));
      return parsePage(await client.request(`/workMessage/toUsers?${query.toString()}`));
    },
    async overview(input) {
      const query = new URLSearchParams({ view: 'global' });
      appendNonBlank(query, 'keyword', input.keyword);
      appendNonBlank(query, 'conversationType', input.conversationType);
      appendAllNonBlank(query, 'employeeIds', input.employeeIds);
      appendNonBlank(query, 'startAt', input.startAt);
      appendNonBlank(query, 'endAt', input.endAt);
      appendNonBlank(query, 'bucket', input.bucket ?? '');
      appendAllNonBlank(query, 'messageTypes', input.messageTypes ?? []);
      return parseOverview(await client.request(`/workMessage/globalOverview?${query.toString()}`));
    },
    async detail(id) {
      const query = new URLSearchParams({ id });
      return parseDetail(await client.request(`/workMessage/detail?${query.toString()}`));
    },
    async setFocus(id) {
      await client.request('/workMessage/focus', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ conversationId: id }),
      });
    },
    async removeFocus(id) {
      await client.request('/workMessage/focus', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ conversationId: id }),
      });
    },
    async employees(input) {
      const query = new URLSearchParams({ page: '1', perPage: '100' });
      appendNonBlank(query, 'name', input.keyword);
      return parseEmployees(await client.request(`/workMessage/fromUsers?${query.toString()}`));
    },
    async staffDirectory(input) {
      const query = new URLSearchParams({ mode: input.mode });
      appendNonBlank(query, 'keyword', input.keyword);
      if (input.departmentId !== null) query.set('departmentId', String(input.departmentId));
      query.set('page', String(input.page));
      query.set('pageSize', String(input.pageSize));
      return parseStaffDirectory(await client.request(`/workMessage/staffDirectory?${query.toString()}`));
    },
    async staffDetail(input) {
      const query = new URLSearchParams({ conversationId: input.conversationId });
      appendNonBlank(query, 'keyword', input.keyword);
      appendAllNonBlank(query, 'messageTypes', input.messageTypes);
      appendNonBlank(query, 'date', input.date);
      query.set('pageSize', String(input.pageSize));
      appendNonBlank(query, 'before', input.before ?? '');
      return parseStaffDetail(await client.request(`/workMessage/staffDetail?${query.toString()}`));
    },
    async customerDirectory(input) {
      const query = new URLSearchParams({ mode: input.mode });
      appendNonBlank(query, 'keyword', input.keyword);
      query.set('page', String(input.page));
      query.set('pageSize', '50');
      return parseCustomerDirectory(await client.request(`/workMessage/customerDirectory?${query.toString()}`));
    },
    async customerConversations(input) {
      const query = new URLSearchParams({ customerId: String(input.customerId), mode: input.mode, page: String(input.page), pageSize: '20' });
      return parseCustomerConversations(await client.request(`/workMessage/customerConversations?${query.toString()}`));
    },
    async customerDetail(input) {
      const query = new URLSearchParams({ customerId: String(input.customerId), conversationId: input.conversationId });
      appendNonBlank(query, 'keyword', input.keyword);
      appendAllNonBlank(query, 'messageTypes', input.messageTypes);
      appendNonBlank(query, 'date', input.date);
      query.set('pageSize', '50');
      appendNonBlank(query, 'before', input.before ?? '');
      return parseCustomerDetail(await client.request(`/workMessage/customerDetail?${query.toString()}`), input.customerId, input.conversationId);
    },
  };
}
