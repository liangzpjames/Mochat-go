import { describe, expect, it, vi } from 'vitest';

import { createDepartmentApi } from './department-api';

describe('department api', () => {
  it('uses the audited tree list query', async () => {
    const request = vi.fn(() => Promise.resolve({ list: [], page: {} }));
    const api = createDepartmentApi({ request });

    await api.list({ name: '销售 部', parentName: '总部', page: 2, perPage: 20 });

    expect(request).toHaveBeenCalledWith(
      '/workDepartment/pageIndex?name=%E9%94%80%E5%94%AE+%E9%83%A8&parentName=%E6%80%BB%E9%83%A8&page=2&perPage=20',
    );
  });

  it('uses the audited member query', async () => {
    const request = vi.fn(() => Promise.resolve({ list: [], page: {} }));
    const api = createDepartmentApi({ request });

    await api.members({ departmentId: 10, page: 3, perPage: 10 });

    expect(request).toHaveBeenCalledWith(
      '/workDepartment/showEmployee?departmentId=10&page=3&perPage=10',
    );
  });
});
