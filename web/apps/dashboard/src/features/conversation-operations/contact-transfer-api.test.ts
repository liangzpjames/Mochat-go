import { describe, expect, it, vi } from 'vitest';

import { createContactTransferApi } from './contact-transfer-api';

describe('contact transfer api', () => {
  it('uses the real transfer endpoints and parses business responses', async () => {
    const customer = {
      contactId: 31,
      employeeId: 21,
      contactWxId: 'external-31',
      employeeWxId: 'employee-21',
      contactName: '重点客户',
      nickName: '客户A',
      corpName: '客户公司',
      employeeName: '员工A',
      tags: ['高意向'],
      transferState: '等待接替',
      addTime: '2026-08-20 10:00:00',
      addWay: '客户添加',
    };
    const request = vi.fn()
      .mockResolvedValueOnce({ list: [customer], lastTime: '2026-08-20 10:00:00' })
      .mockResolvedValueOnce([{ errcode: 0 }, { errcode: 40096, errmsg: 'customer refused' }]);
    const api = createContactTransferApi({ request });
    await expect(api.unassigned({ contactName: '', employeeIds: [], addTimeStart: '', addTimeEnd: '' })).resolves.toMatchObject({ lastTime: '2026-08-20 10:00:00' });
    await expect(api.transferCustomers({ type: 1, list: [{ employeeWxId: 'zhang', contactWxId: 'wm1' }], takeoverUserId: 'li' })).resolves.toEqual([{ errcode: 0 }, { errcode: 40096, errmsg: 'customer refused' }]);
    expect(request).toHaveBeenNthCalledWith(1, '/contactTransfer/unassignedList?page=1&perPage=20');
    expect(request).toHaveBeenNthCalledWith(2, '/contactTransfer/index', expect.objectContaining({ method: 'POST' }));
  });

  it('uses POST for unassigned synchronization', async () => {
    const request = vi.fn().mockResolvedValue({});
    await createContactTransferApi({ request }).syncUnassigned();
    expect(request).toHaveBeenCalledWith('/contactTransfer/sync', expect.objectContaining({ method: 'POST' }));
  });

  it('expands failed group responses into results aligned with the submitted rooms', async () => {
    const request = vi.fn().mockResolvedValue([{ chat_id: 'room-failed', errcode: 90001, errmsg: '群主不允许转接' }]);
    const api = createContactTransferApi({ request });

    await expect(api.transferRooms({ list: ['room-ok', 'room-failed'], takeoverUserId: 'li' })).resolves.toEqual([
      { errcode: 0 },
      { errcode: 90001, errmsg: '群主不允许转接' },
    ]);
  });
});
