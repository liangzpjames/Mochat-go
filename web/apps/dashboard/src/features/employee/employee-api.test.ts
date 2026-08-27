import { describe, expect, it, vi } from 'vitest';

import { createEmployeeApi } from './employee-api';

describe('employee api', () => {
  it('uses the audited list query', async () => {
    const request = vi.fn(() => Promise.resolve({ list: [], page: {} }));
    const api = createEmployeeApi({ request });

    await api.list({ name: '张 三', status: 1, contactAuth: 2, page: 3, perPage: 20 });

    expect(request).toHaveBeenCalledWith(
      '/workEmployee/index?name=%E5%BC%A0+%E4%B8%89&status=1&contactAuth=2&page=3&perPage=20',
    );
  });

  it('loads conditions and synchronizes with the audited methods', async () => {
    const request = vi.fn(() => Promise.resolve({}));
    const api = createEmployeeApi({ request });

    await api.conditions();
    expect(request).toHaveBeenCalledWith('/workEmployee/searchCondition');
    await api.sync();
    expect(request).toHaveBeenLastCalledWith('/company/employee-sync', { method: 'POST' });
  });
});
