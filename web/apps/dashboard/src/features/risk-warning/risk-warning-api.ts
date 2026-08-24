import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';

export type RiskLevel = 'unclassified' | 'low' | 'medium' | 'high';
export type AuditStatus = 'pending' | 'confirmed' | 'ignored' | 'closed';
export type RuleStatus = 'enabled' | 'disabled';

export type Page<T> = {
  items: T[];
  total: number;
  page: number;
  perPage: number;
};

export type TimeoutRecord = {
  id: number;
  timeoutSeconds: number;
  triggerMessage: string;
  messageType: string;
  customerId: string;
  customerName: string;
  customerAvatar: string;
  employeeId: number;
  employeeName: string;
  conversationType: string;
  riskLevel: string;
  ruleId: number;
  ruleName: string;
  auditStatus: string;
  assignedEmployeeId: number;
  occurredAt: string;
};

export type TimeoutRule = {
  id: number;
  name: string;
  status: string;
  monitorTarget: string;
  conversationScopes: string[];
  triggerCount: number;
  strategies: Array<{ timeoutMinutes: number; riskLevel: string; notifyType: string }>;
  createdAt: string;
};

export type CustomerLossRecord = {
  id: number;
  contactId: number;
  customerName: string;
  customerAvatar: string;
  employeeId: number;
  employeeName: string;
  lossType: string;
  riskLevel: string;
  ruleName: string;
  auditStatus: string;
  lastMessage: string;
  occurredAt: string;
  tags: string[];
  source: string;
};

type Row = Record<string, unknown>;

function object(value: unknown): Row {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Row : {};
}

function number(value: unknown, fallback = 0): number {
  const parsed = typeof value === 'number' ? value : Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function string(value: unknown): string {
  return typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' ? String(value) : '';
}

function rows(value: unknown): { list: Row[]; total: number; page: number; perPage: number } {
  const source = object(value);
  const body = object(source.data);
  const root = Object.keys(body).length > 0 ? body : source;
  const listValue = [root.items, root.list, root.rows].find(Array.isArray);
  const list = Array.isArray(listValue) ? listValue.map(object) : [];
  const pageObject = object(root.page);
  return {
    list,
    total: number(root.total ?? pageObject.total, list.length),
    page: number(root.pageNumber ?? pageObject.page, 1),
    perPage: number(root.perPage ?? pageObject.perPage, 20),
  };
}

function parseTimeout(value: unknown): TimeoutRecord {
  const row = object(value);
  const customerID = string(row.customerId ?? row.customerID);
  return {
    id: number(row.id),
    timeoutSeconds: number(row.timeoutSeconds),
    triggerMessage: string(row.triggerMessage),
    messageType: string(row.messageType),
    customerId: customerID,
    customerName: string(row.customerName) || (customerID ? `客户 ${customerID}` : '客户资料暂缺'),
    customerAvatar: string(row.customerAvatar ?? row.avatar),
    employeeId: number(row.employeeId),
    employeeName: string(row.employeeName) || '责任员工暂缺',
    conversationType: string(row.conversationType) || 'single',
    riskLevel: string(row.riskLevel) || 'low',
    ruleId: number(row.ruleId),
    ruleName: string(row.ruleName) || '规则已删除',
    auditStatus: string(row.auditStatus) || 'pending',
    assignedEmployeeId: number(row.assignedEmployeeId),
    occurredAt: string(row.occurredAt ?? row.createdAt),
  };
}

function parseRule(value: unknown): TimeoutRule {
  const row = object(value);
  const strategies = Array.isArray(row.strategies) ? row.strategies.map((value) => {
    const strategy = object(value);
    return { timeoutMinutes: number(strategy.timeoutMinutes), riskLevel: string(strategy.riskLevel) || 'low', notifyType: string(strategy.notifyType) || 'none' };
  }) : [];
  return {
    id: number(row.id), name: string(row.name) || '未命名规则', status: string(row.status) || 'disabled',
    monitorTarget: string(row.monitorTarget) || 'all', conversationScopes: Array.isArray(row.conversationScopes) ? row.conversationScopes.map(string) : [],
    triggerCount: number(row.triggerCount), strategies, createdAt: string(row.createdAt),
  };
}

function parseLoss(value: unknown): CustomerLossRecord {
  const row = object(value);
  const contactId = number(row.contactId ?? row.customerId);
  const lossType = string(row.lossType ?? row.type) || 'employee_removed_customer';
  const tags = Array.isArray(row.tag) ? row.tag.map(string).filter(Boolean) : Array.isArray(row.tags) ? row.tags.map(string).filter(Boolean) : [];
  return {
    id: number(row.id), contactId, customerName: string(row.customerName ?? row.name) || (contactId ? `客户 ${contactId}` : '客户资料暂缺'),
    customerAvatar: string(row.customerAvatar ?? row.avatar), employeeId: number(row.employeeId), employeeName: string(row.employeeName) || '责任员工暂缺',
    lossType, riskLevel: string(row.riskLevel) || 'unclassified', ruleName: string(row.ruleName) || '未匹配规则',
    auditStatus: string(row.auditStatus) || 'pending', lastMessage: string(row.lastMessage), occurredAt: string(row.occurredAt ?? row.deletedAt), tags,
    source: string(row.source) || 'legacy_backfill',
  };
}

export type TimeoutFilter = { customer?: string | undefined; riskLevel?: string | undefined; conversationType?: string | undefined; auditStatus?: string | undefined; page: number; perPage: number };
export type CustomerLossFilter = { employeeId?: string | undefined; page: number; perPage: number };

export type RiskWarningApi = {
  timeoutRecords(filter: TimeoutFilter): Promise<Page<TimeoutRecord>>;
  timeoutRules(input: { name?: string | undefined; page: number; perPage: number }): Promise<Page<TimeoutRule>>;
  timeoutSettings(): Promise<Row>;
  write(endpoint: string, values: Row, method?: 'POST' | 'PUT' | 'DELETE'): Promise<unknown>;
  customerLossRecords(filter: CustomerLossFilter): Promise<Page<CustomerLossRecord>>;
};

export function createRiskWarningApi(api: BusinessWorkbenchApi): RiskWarningApi {
  return {
    async timeoutRecords(filter) {
      const result = rows(await api.read('/timeout-warning/records', Object.fromEntries(Object.entries(filter).filter(([, value]) => value !== undefined)) as Record<string, string | number>));
      return { items: result.list.map(parseTimeout), total: result.total, page: result.page || filter.page, perPage: result.perPage || filter.perPage };
    },
    async timeoutRules(input) {
      const result = rows(await api.read('/timeout-warning/rules', Object.fromEntries(Object.entries(input).filter(([, value]) => value !== undefined)) as Record<string, string | number>));
      return { items: result.list.map(parseRule), total: result.total, page: result.page || input.page, perPage: result.perPage || input.perPage };
    },
    async timeoutSettings() { return object(await api.read('/timeout-warning/settings', {})); },
    write(endpoint, values, method = 'POST') { return api.write(endpoint, values, method); },
    async customerLossRecords(filter) {
      const result = rows(await api.read('/workContact/lossContact', Object.fromEntries(Object.entries(filter).filter(([, value]) => value !== undefined)) as Record<string, string | number>));
      return { items: result.list.map(parseLoss), total: result.total, page: result.page || filter.page, perPage: result.perPage || filter.perPage };
    },
  };
}
