import { describe, expect, it, vi } from 'vitest';
import { createUserAdminApi } from './user-admin-api';

describe('user admin api', () => {
  it('uses the audited read contracts', async () => {
    const request = vi.fn(() => Promise.resolve([]));
    const api = createUserAdminApi({ request });
    await api.list({ phone: '13800000000', status: 1, page: 2, perPage: 20 });
    expect(request).toHaveBeenLastCalledWith('/user/index?phone=13800000000&status=1&page=2&perPage=20');
    await api.detail(4); expect(request).toHaveBeenLastCalledWith('/user/show?userId=4');
    await api.departments('13800000000');
    expect(request).toHaveBeenLastCalledWith('/workDepartment/selectByPhone?phone=13800000000');
    await api.roles(); expect(request).toHaveBeenLastCalledWith('/role/select');
  });

  it('uses the audited write contracts', async () => {
    const request = vi.fn(() => Promise.resolve(undefined));
    const api = createUserAdminApi({ request });
    const values = { userName: '张三', phone: '13800000000', gender: 1, roleId: 3, status: 1 };
    await api.create({ ...values, password: 'abc123', confirmPass: 'abc123' });
    expect(request).toHaveBeenLastCalledWith('/user/store', expect.objectContaining({ method: 'POST' }));
    await api.update(4, values);
    expect(request).toHaveBeenLastCalledWith('/user/update', expect.objectContaining({
      method: 'PUT', body: JSON.stringify({ ...values, userId: 4 }),
    }));
    await api.updateStatus([4, 5], 1);
    expect(request).toHaveBeenLastCalledWith('/user/statusUpdate', expect.objectContaining({
      method: 'PUT', body: JSON.stringify({ userId: '4,5', status: 1 }),
    }));
    await api.resetPassword(4, 'new123');
    expect(request).toHaveBeenLastCalledWith('/user/passwordReset', expect.objectContaining({
      method: 'PUT', body: JSON.stringify({ id: 4, newPassword: 'new123' }),
    }));
  });
});
