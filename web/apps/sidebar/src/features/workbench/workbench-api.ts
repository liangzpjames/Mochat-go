import { MobileApiError } from '@mochat/mobile-foundation';

import type { BusinessRequest } from '../business-api';

export type WorkbenchSummary = {
  employee: {
    id: number;
    name: string;
    avatar: string | null;
    departmentNames: string[];
    corpName: string;
  };
  customers: {
    total: number;
    addedToday: number;
    taggedTotal: number;
    ownedRoomTotal: number;
  };
  tasks: {
    contactSopRecords: number;
    roomSopPending: number;
    batchAddPending: number;
  };
};

export type EmployeeContact = {
  id: number;
  wxExternalUserid: string;
  name: string;
  avatar: string | null;
  remark: string;
  status: number;
  addedAt: string;
  tags: string[];
};

export type ContactListFilter = { keyword: string; page: number; perPage: number };
export type ContactListPage = { page: number; perPage: number; total: number; totalPage: number; items: EmployeeContact[] };
export type EmployeeTaskKind = 'contactSop' | 'roomSop' | 'batchAdd';
export type EmployeeTaskState = 'pending' | 'done' | 'recorded';
export type EmployeeTask = { id: number; kind: EmployeeTaskKind; title: string; subjectName: string; scheduledAt: string; state: EmployeeTaskState };
export type TaskListFilter = { kind: EmployeeTaskKind; state: EmployeeTaskState; page: number; perPage: number };
export type TaskListPage = { page: number; perPage: number; total: number; totalPage: number; items: EmployeeTask[] };

function record(value: unknown): Record<string, unknown> | null {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null;
}

function fail(message: string): never {
  throw new MobileApiError('validation', message);
}

function integer(value: unknown, positive = false): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < (positive ? 1 : 0)) {
    fail('员工工作台响应格式无效。');
  }
  return value;
}

function text(value: unknown): string {
  if (typeof value !== 'string') fail('员工工作台响应格式无效。');
  return value;
}

function nullableText(value: unknown): string | null {
  if (value === null || value === '') return null;
  return text(value);
}

function stringList(value: unknown): string[] {
  if (!Array.isArray(value) || value.some((item) => typeof item !== 'string')) {
    fail('员工工作台响应格式无效。');
  }
  return value as string[];
}

export async function loadWorkbenchSummary(request: BusinessRequest): Promise<WorkbenchSummary> {
  const raw = record(await request<unknown>('/workbench/summary', { method: 'GET' }));
  const employee = record(raw?.employee);
  const customers = record(raw?.customers);
  const tasks = record(raw?.tasks);
  if (raw === null || employee === null || customers === null || tasks === null) {
    fail('员工工作台响应格式无效。');
  }
  return {
    employee: {
      id: integer(employee.id, true),
      name: text(employee.name),
      avatar: nullableText(employee.avatar),
      departmentNames: stringList(employee.departmentNames),
      corpName: text(employee.corpName),
    },
    customers: {
      total: integer(customers.total),
      addedToday: integer(customers.addedToday),
      taggedTotal: integer(customers.taggedTotal),
      ownedRoomTotal: integer(customers.ownedRoomTotal),
    },
    tasks: {
      contactSopRecords: integer(tasks.contactSopRecords),
      roomSopPending: integer(tasks.roomSopPending),
      batchAddPending: integer(tasks.batchAddPending),
    },
  };
}

function pageFields(raw: Record<string, unknown>): Pick<ContactListPage, 'page' | 'perPage' | 'total' | 'totalPage'> {
  return {
    page: integer(raw.page, true),
    perPage: integer(raw.perPage, true),
    total: integer(raw.total),
    totalPage: integer(raw.totalPage),
  };
}

function validPagination(page: number, perPage: number): void {
  if (!Number.isSafeInteger(page) || page <= 0 || !Number.isSafeInteger(perPage) || perPage <= 0 || perPage > 50) {
    fail('分页参数无效。');
  }
}

export async function loadEmployeeContacts(request: BusinessRequest, filter: ContactListFilter): Promise<ContactListPage> {
  validPagination(filter.page, filter.perPage);
  const query = new URLSearchParams({ keyword: filter.keyword.trim(), page: String(filter.page), perPage: String(filter.perPage) });
  const raw = record(await request<unknown>(`/workContact/index?${query.toString()}`, { method: 'GET' }));
  if (raw === null || !Array.isArray(raw.items)) fail('员工客户列表响应格式无效。');
  const items = raw.items.map((entry): EmployeeContact => {
    const item = record(entry);
    if (item === null) fail('员工客户列表响应格式无效。');
    return {
      id: integer(item.id, true),
      wxExternalUserid: text(item.wxExternalUserid),
      name: text(item.name),
      avatar: nullableText(item.avatar),
      remark: text(item.remark),
      status: integer(item.status),
      addedAt: text(item.addedAt),
      tags: stringList(item.tags),
    };
  });
  return { ...pageFields(raw), items };
}

function isTaskKind(value: string): value is EmployeeTaskKind {
  return value === 'contactSop' || value === 'roomSop' || value === 'batchAdd';
}

function isTaskState(value: string): value is EmployeeTaskState {
  return value === 'pending' || value === 'done' || value === 'recorded';
}

export async function loadEmployeeTasks(request: BusinessRequest, filter: TaskListFilter): Promise<TaskListPage> {
  validPagination(filter.page, filter.perPage);
  if (!isTaskKind(filter.kind) || !isTaskState(filter.state)
    || (filter.kind === 'contactSop' ? filter.state !== 'recorded' : filter.state === 'recorded')) {
    fail('任务筛选参数无效。');
  }
  const query = new URLSearchParams({ kind: filter.kind, state: filter.state, page: String(filter.page), perPage: String(filter.perPage) });
  const raw = record(await request<unknown>(`/workbench/tasks?${query.toString()}`, { method: 'GET' }));
  if (raw === null || !Array.isArray(raw.items)) fail('员工任务列表响应格式无效。');
  const items = raw.items.map((entry): EmployeeTask => {
    const item = record(entry);
    if (item === null) fail('员工任务列表响应格式无效。');
    const kind = text(item.kind);
    const state = text(item.state);
    if (!isTaskKind(kind) || !isTaskState(state)) fail('员工任务列表响应格式无效。');
    return {
      id: integer(item.id, true),
      kind,
      title: text(item.title),
      subjectName: text(item.subjectName),
      scheduledAt: text(item.scheduledAt),
      state,
    };
  });
  return { ...pageFields(raw), items };
}
