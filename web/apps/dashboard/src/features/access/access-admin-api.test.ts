/* eslint-disable @typescript-eslint/no-base-to-string */
import { describe, expect, it, vi } from 'vitest';
import { createDashboardAccessAdminApi } from './access-admin-api';

describe('dashboard access administration API', () => {
  it('uses only /access endpoints and sends expectedVersion on mutations', async () => {
    const request = vi.fn((path: RequestInfo | URL, init?: RequestInit) => Promise.resolve({ path, init }));
    const api = createDashboardAccessAdminApi({ request });
    await api.employees({ page: 1, perPage: 50 });
    await api.provisionEmployeeAccount(4, {
      loginIdentifier: '13800000004',
      roleIds: [2],
      directPermissions: [{ code: 'dashboard.index', scope: 'self' }],
    });
    await api.updateEmployeeAccountStatus(4, 2);
    await api.resetEmployeePassword(4);
    await api.users({ page: 1, perPage: 20 });
    await api.replaceUser(7, { roleIds: [2, 3], directPermissions: [], expectedVersion: 4 });
    await api.createRole({ name: '销售', remark: '', status: 1, permissions: [] });
    await api.updateRoleStatus(2, { status: 2, expectedVersion: 5 });
    await api.deleteRole(2, 6);
    expect(request.mock.calls.map(([path]) => typeof path === 'string' ? path : path.toString())).toEqual([
      '/access/employees?page=1&perPage=50', '/access/employees/4/account',
      '/access/employees/4/account/status', '/access/employees/4/account/reset-password',
      '/access/users?page=1&perPage=20', '/access/users/7', '/access/roles',
      '/access/roles/2/status', '/access/roles/2',
    ]);
    expect(request.mock.calls[1]?.[1]).toMatchObject({ method: 'POST' });
    expect(JSON.parse(String(request.mock.calls[1]?.[1]?.body))).toEqual({
      loginIdentifier: '13800000004',
      roleIds: [2],
      directPermissions: [{ code: 'dashboard.index', scope: 'self' }],
    });
    expect(request.mock.calls[2]?.[1]).toMatchObject({ method: 'PUT' });
  });
});
