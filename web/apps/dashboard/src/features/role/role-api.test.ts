import { describe, expect, it, vi } from 'vitest';

import { createRoleApi } from './role-api';

describe('role api', () => {
  it('uses the audited read contracts', async () => {
    const request = vi.fn(() => Promise.resolve({ list: [], page: {} }));
    const api = createRoleApi({ request });

    await api.list({ name: '管理员', page: 2, perPage: 20 });
    expect(request).toHaveBeenLastCalledWith('/role/index?name=%E7%AE%A1%E7%90%86%E5%91%98&page=2&perPage=20');
    await api.detail(7);
    expect(request).toHaveBeenLastCalledWith('/role/show?roleId=7');
    await api.members({ roleId: 7, page: 3, perPage: 10 });
    expect(request).toHaveBeenLastCalledWith('/role/showEmployee?roleId=7&page=3&perPage=10');
  });

  it('uses the audited write contracts including copy', async () => {
    const request = vi.fn(() => Promise.resolve(undefined));
    const api = createRoleApi({ request });
    const role = { name: '主管', remarks: '销售主管', dataPermission: 1 };

    await api.create(role);
    expect(request).toHaveBeenLastCalledWith('/role/store', expect.objectContaining({
      method: 'POST', body: JSON.stringify(role),
    }));
    await api.copy(7, role);
    expect(request).toHaveBeenLastCalledWith('/role/store', expect.objectContaining({
      method: 'POST', body: JSON.stringify({ ...role, roleId: 7 }),
    }));
    await api.update(7, role);
    expect(request).toHaveBeenLastCalledWith('/role/update', expect.objectContaining({
      method: 'PUT', body: JSON.stringify({ ...role, roleId: 7 }),
    }));
    await api.updateStatus(7, 2);
    expect(request).toHaveBeenLastCalledWith('/role/statusUpdate', expect.objectContaining({
      method: 'PUT', body: JSON.stringify({ roleId: 7, status: 2 }),
    }));
    await api.remove(7);
    expect(request).toHaveBeenLastCalledWith('/role/destroy', expect.objectContaining({
      method: 'DELETE', body: JSON.stringify({ roleId: 7 }),
    }));
  });
});
