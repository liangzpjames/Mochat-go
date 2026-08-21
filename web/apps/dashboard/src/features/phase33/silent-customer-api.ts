import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';
import type { Page } from './message-intercept-api';

export type SilentCustomerRule = { id: number; name: string; silentDays: number; status: string; triggerCount: number; updatedAt: string };
export type SilentCustomerRecord = {
  id: number; ruleId: number; ruleName: string; customerId: string; customerName: string;
  employeeId: number; employeeName: string; lastInteractionAt: string; silentDays: number;
  status: string; assignedEmployeeId: number; assignedEmployeeName: string; followUpNote: string; updatedAt: string;
};
export type StaffOption = { id: number; name: string; departments: string[] };
export type SilentCustomerFilters = {
  customer?: string; employeeId?: number; assignedEmployeeId?: number; status?: string;
  minSilentDays?: number; maxSilentDays?: number; snapshotDate?: string; page: number; perPage: number;
};

type ApiRow = Record<string, unknown>;
function row(value: unknown): ApiRow { return value && typeof value === 'object' && !Array.isArray(value) ? value as ApiRow : {}; }
function number(value: unknown, fallback = 0): number { const parsed = typeof value === 'number' ? value : Number(value); return Number.isFinite(parsed) ? parsed : fallback; }
function string(value: unknown): string { return typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' ? String(value) : ''; }
function body(value: unknown): ApiRow { const source = row(value); return Object.keys(row(source.data)).length ? row(source.data) : source; }
function page<T>(value: unknown, parse: (value: unknown) => T, fallbackPage: number, fallbackPerPage: number): Page<T> {
  const source = body(value); const list = [source.items, source.list, source.rows].find(Array.isArray); const pageValue = row(source.page);
  return { items: (Array.isArray(list) ? list : []).map(parse), total: number(source.total ?? pageValue.total, Array.isArray(list) ? list.length : 0), page: number(source.pageNumber ?? pageValue.page, fallbackPage), perPage: number(source.perPage ?? pageValue.perPage, fallbackPerPage) };
}
function parseRecord(value: unknown): SilentCustomerRecord {
  const source = row(value);
  return { id: number(source.id), ruleId: number(source.ruleId), ruleName: string(source.ruleName), customerId: string(source.customerId), customerName: string(source.customerName), employeeId: number(source.employeeId), employeeName: string(source.employeeName), lastInteractionAt: string(source.lastInteractionAt), silentDays: number(source.silentDays), status: string(source.status) || 'pending', assignedEmployeeId: number(source.assignedEmployeeId), assignedEmployeeName: string(source.assignedEmployeeName), followUpNote: string(source.followUpNote), updatedAt: string(source.updatedAt) };
}
function parseRule(value: unknown): SilentCustomerRule { const source = row(value); return { id: number(source.id), name: string(source.name), silentDays: number(source.silentDays), status: string(source.status) || 'disabled', triggerCount: number(source.triggerCount), updatedAt: string(source.updatedAt) }; }
function queryValues(values: Record<string, unknown>): Record<string, string | number> { return Object.fromEntries(Object.entries(values).filter(([, value]) => value !== undefined && value !== '' && value !== null)) as Record<string, string | number>; }

export type SilentCustomerApi = {
  records(filter: SilentCustomerFilters): Promise<Page<SilentCustomerRecord>>;
  rules(filter: { name?: string; status?: string; page: number; perPage: number }): Promise<Page<SilentCustomerRule>>;
  staffOptions(keyword: string): Promise<StaffOption[]>;
  write(endpoint: string, values: Record<string, unknown>, method?: 'POST' | 'PUT' | 'DELETE'): Promise<unknown>;
};

export function createSilentCustomerApi(api: BusinessWorkbenchApi): SilentCustomerApi {
  return {
    async records(filter) { return page(await api.read('/silent-customer/records', queryValues(filter)), parseRecord, filter.page, filter.perPage); },
    async rules(filter) { return page(await api.read('/silent-customer/rules', queryValues(filter)), parseRule, filter.page, filter.perPage); },
    async staffOptions(keyword) {
      const source = body(await api.read('/workMessage/staffDirectory', { mode: 'all', keyword, page: 1, pageSize: 50 }));
      const items = Array.isArray(source.employees) ? source.employees : Array.isArray(source.items) ? source.items : [];
      return items.map((value): StaffOption => { const item = row(value); return { id: number(item.id), name: string(item.name), departments: Array.isArray(item.departments) ? item.departments.map(string).filter(Boolean) : [] }; }).filter((item) => item.id > 0 && item.name !== '');
    },
    write(endpoint, values, method = 'POST') { return api.write(endpoint, values, method); },
  };
}
