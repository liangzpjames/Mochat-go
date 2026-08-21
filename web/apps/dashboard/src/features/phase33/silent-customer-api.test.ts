import { describe, expect, it, vi } from 'vitest';

import { createSilentCustomerApi } from './silent-customer-api';

describe('silent customer api', () => {
  it('sends staff and date filters only after explicit query', async () => {
    const read = vi.fn().mockResolvedValue({ items: [{ id: 3, customerName: '林芳', status: 'followed', silentDays: 28 }] });
    const api = createSilentCustomerApi({ read, write: vi.fn() });

    const result = await api.records({ customer: '林', employeeId: 1002, minSilentDays: 20, maxSilentDays: 40, page: 1, perPage: 20 });

    expect(read).toHaveBeenCalledWith('/silent-customer/records', {
      customer: '林', employeeId: 1002, minSilentDays: 20, maxSilentDays: 40, page: 1, perPage: 20,
    });
    expect(result.items[0]).toMatchObject({ customerName: '林芳', status: 'followed', silentDays: 28 });
  });

  it('returns real staff options for assignment', async () => {
    const read = vi.fn().mockResolvedValue({ data: { employees: [{ id: 1002, name: '张伟', departments: ['销售'] }] } });
    const api = createSilentCustomerApi({ read, write: vi.fn() });

    await expect(api.staffOptions('张')).resolves.toEqual([{ id: 1002, name: '张伟', departments: ['销售'] }]);
    expect(read).toHaveBeenCalledWith('/workMessage/staffDirectory', { mode: 'all', keyword: '张', page: 1, pageSize: 50 });
  });
});
