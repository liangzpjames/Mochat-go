import { describe, expect, it, vi } from 'vitest';

import { createCorpAdminApi } from './corp-admin-api';

describe('corp admin api', () => {
  it('uses the audited list and detail contracts', async () => {
    const request = vi.fn(() => Promise.resolve({ list: [], page: {} }));
    const api = createCorpAdminApi({ request });

    await api.list({ corpId: '7', corpName: '测试 企业', page: 2, perPage: 20 });
    expect(request).toHaveBeenCalledWith(
      '/corp/index?corpName=%E6%B5%8B%E8%AF%95+%E4%BC%81%E4%B8%9A&page=2&perPage=20',
    );

    await api.show(7);
    expect(request).toHaveBeenLastCalledWith('/corp/show?corpId=7');
  });

  it('uses the audited create and update request bodies', async () => {
    const request = vi.fn(() => Promise.resolve([]));
    const api = createCorpAdminApi({ request });
    const values = {
      corpName: '测试企业',
      wxCorpId: 'wx-7',
      employeeSecret: 'employee',
      contactSecret: 'contact',
    };

    await api.create(values);
    expect(request).toHaveBeenCalledWith('/corp/store', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(values),
    });

    await api.update({ corpId: 7, ...values });
    expect(request).toHaveBeenLastCalledWith('/corp/update', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ corpId: 7, ...values }),
    });
  });
});
