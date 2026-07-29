import { describe, expect, it, vi } from 'vitest';
import { createMenuAdminApi } from './menu-admin-api';

describe('menu admin api', () => {
  it('uses all audited read contracts', async () => {
    const request = vi.fn(() => Promise.resolve([]));
    const api = createMenuAdminApi({ request });
    await api.list({ name: '客户', page: 2, perPage: 20 });
    expect(request).toHaveBeenLastCalledWith('/menu/index?name=%E5%AE%A2%E6%88%B7&page=2&perPage=20');
    await api.options(); expect(request).toHaveBeenLastCalledWith('/menu/select');
    await api.detail(4); expect(request).toHaveBeenLastCalledWith('/menu/show?menuId=4');
    await api.usedIcons(); expect(request).toHaveBeenLastCalledWith('/menu/iconIndex');
  });

  it('uses all audited write contracts', async () => {
    const request = vi.fn(() => Promise.resolve(undefined));
    const api = createMenuAdminApi({ request });
    const values = { level: 3, name: '客户', icon: '', linkUrl: '/customer', linkType: 1,
      firstMenuId: 1, secondMenuId: 2 };
    await api.create(values);
    expect(request).toHaveBeenLastCalledWith('/menu/store', expect.objectContaining({ method: 'POST', body: JSON.stringify(values) }));
    await api.update(4, values);
    expect(request).toHaveBeenLastCalledWith('/menu/update', expect.objectContaining({ method: 'PUT', body: JSON.stringify({ ...values, menuId: 4 }) }));
    await api.updateStatus(4, 2);
    expect(request).toHaveBeenLastCalledWith('/menu/statusUpdate', expect.objectContaining({ method: 'PUT', body: JSON.stringify({ menuId: 4, status: 2 }) }));
    await api.remove(4);
    expect(request).toHaveBeenLastCalledWith('/menu/destroy', expect.objectContaining({ method: 'DELETE', body: JSON.stringify({ menuId: 4 }) }));
  });
});
