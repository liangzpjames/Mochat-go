import { describe, expect, it, vi } from 'vitest';

import { createDashboardOverviewApi } from './dashboard-overview-api';

describe('createDashboardOverviewApi', () => {
  it('serializes the corp and local-calendar date range and returns real response values', async () => {
    const response = {
      cards: [
        { key: 'contacts', label: '客户总数', value: 137 },
      ],
      trend: [
        {
          date: '2026-07-30',
          addContactNum: 12,
          addIntoRoomNum: 8,
          lossContactNum: 2,
          quitRoomNum: 1,
        },
      ],
      updatedAt: '2026-07-31 09:30:00',
    };
    const request = vi.fn(() => Promise.resolve(response));
    const api = createDashboardOverviewApi({ request });

    await expect(api.load({
      corpId: 'corp 7',
      from: '2026-07-01',
      to: '2026-07-31',
    })).resolves.toEqual(response);

    expect(request).toHaveBeenCalledWith(
      '/corpData/index?corpId=corp+7&from=2026-07-01&to=2026-07-31',
    );
  });

  it('rejects an incompatible legacy response instead of crashing the page', async () => {
    const request = vi.fn(() => Promise.resolve({
      weChatContactNum: 137,
      updateTime: '2026-07-31 09:30:00',
    }));
    const api = createDashboardOverviewApi({ request });

    await expect(api.load({
      corpId: '7',
      from: '2026-07-01',
      to: '2026-07-31',
    })).rejects.toThrow('数据概览接口尚未启用');
  });
});
