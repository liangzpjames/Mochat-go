import { describe, expect, it, vi } from 'vitest';

import { MobileApiError } from '@mochat/mobile-foundation';

import {
  loadEmployeeContacts,
  loadEmployeeTasks,
  loadWorkbenchSummary,
} from './workbench-api';

describe('employee workbench API contracts', () => {
  it('parses a complete employee summary without inventing values', async () => {
    const request = vi.fn().mockResolvedValue({
      employee: { id: 7, name: '员工甲', avatar: null, departmentNames: ['销售部'], corpName: '示例企业' },
      customers: { total: 12, addedToday: 2, taggedTotal: 4, ownedRoomTotal: 1 },
      tasks: { contactSopPending: 3, roomSopPending: 2, batchAddPending: 1 },
    });
    await expect(loadWorkbenchSummary(request)).resolves.toMatchObject({
      employee: { id: 7, name: '员工甲' },
      customers: { total: 12 },
      tasks: { contactSopPending: 3 },
    });
    expect(request).toHaveBeenCalledWith('/workbench/summary', { method: 'GET' });
  });

  it('rejects a malformed summary instead of rendering fake zeroes', async () => {
    const request = vi.fn().mockResolvedValue({ employee: null, customers: {}, tasks: {} });
    await expect(loadWorkbenchSummary(request)).rejects.toBeInstanceOf(MobileApiError);
  });

  it('loads an employee-scoped contact page with encoded filters', async () => {
    const request = vi.fn().mockResolvedValue({
      page: 1, perPage: 20, total: 1, totalPage: 1,
      items: [{ id: 11, wxExternalUserid: 'external-11', name: '客户甲', avatar: null, remark: '重点', status: 1, addedAt: '2026-08-23 09:00:00', tags: ['高意向'] }],
    });
    const result = await loadEmployeeContacts(request, { keyword: '客户 A', page: 1, perPage: 20 });
    expect(result.items[0]?.tags).toEqual(['高意向']);
    expect(request).toHaveBeenCalledWith('/workContact/index?keyword=%E5%AE%A2%E6%88%B7+A&page=1&perPage=20', { method: 'GET' });
  });

  it('rejects unknown task kinds before a request is sent', async () => {
    const request = vi.fn();
    await expect(loadEmployeeTasks(request, { kind: 'archive' as 'contactSop', state: 'pending', page: 1, perPage: 20 })).rejects.toBeInstanceOf(MobileApiError);
    expect(request).not.toHaveBeenCalled();
  });

  it('parses a task page with only supported task kinds', async () => {
    const request = vi.fn().mockResolvedValue({
      page: 1, perPage: 20, total: 1, totalPage: 1,
      items: [{ id: 31, kind: 'roomSop', title: '群 SOP', subjectName: '客户群甲', scheduledAt: '2026-08-23 10:00:00', state: 'pending' }],
    });
    const result = await loadEmployeeTasks(request, { kind: 'roomSop', state: 'pending', page: 1, perPage: 20 });
    expect(result.items[0]?.kind).toBe('roomSop');
    expect(request).toHaveBeenCalledWith('/workbench/tasks?kind=roomSop&state=pending&page=1&perPage=20', { method: 'GET' });
  });
});
