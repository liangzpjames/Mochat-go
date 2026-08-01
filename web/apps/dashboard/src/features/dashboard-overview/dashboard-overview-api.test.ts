import { describe, expect, it, vi } from 'vitest';

import { createDashboardOverviewApi, type DashboardOverviewApi } from './dashboard-overview-api';

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
      page: 1,
      pageSize: 20,
      total: 1,
    };
    const request = vi.fn(() => Promise.resolve(response));
    const api = createDashboardOverviewApi({ request });

    await expect(api.load({
      corpId: 'corp 7',
      startDate: '2026-07-01',
      endDate: '2026-07-31',
      employeeIds: [],
      departmentIds: [],
      period: 'day',
      page: 1,
      pageSize: 20,
    })).resolves.toEqual(response);

    expect(request).toHaveBeenCalledWith(
      '/corpData/index?corpId=corp+7&startDate=2026-07-01&endDate=2026-07-31&employeeIds=&departmentIds=&period=day&page=1&pageSize=20',
    );
  });

  it('serializes the complete overview query, including filter arrays and pagination', async () => {
    const request = vi.fn(() => Promise.resolve({ cards: [], trend: [], updatedAt: '' }));
    const api = createDashboardOverviewApi({ request });

    await api.load({
      corpId: '7',
      startDate: '2026-07-01',
      endDate: '2026-07-31',
      employeeIds: ['9', '12'],
      departmentIds: ['3', '8'],
      period: 'week',
      page: 2,
      pageSize: 20,
    });

    expect(request).toHaveBeenCalledWith(
      '/corpData/index?corpId=7&startDate=2026-07-01&endDate=2026-07-31&employeeIds=9&employeeIds=12&departmentIds=3&departmentIds=8&period=week&page=2&pageSize=20',
    );
  });

  it('exports with precisely the same serialized query as the loaded overview', async () => {
    const request = vi.fn(() => Promise.resolve({ cards: [], trend: [], updatedAt: '' }));
    const api = createDashboardOverviewApi({ request }) as DashboardOverviewApi & {
      exportCsv(input: unknown): Promise<Blob>;
    };
    const input = {
      corpId: '7',
      startDate: '2026-07-01',
      endDate: '2026-07-31',
      employeeIds: ['9'],
      departmentIds: ['3'],
      period: 'month',
      page: 1,
      pageSize: 10,
    };

    await api.exportCsv(input);

    expect(request).toHaveBeenCalledWith(
      '/corpData/index?corpId=7&startDate=2026-07-01&endDate=2026-07-31&employeeIds=9&departmentIds=3&period=month&page=1&pageSize=10',
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
      startDate: '2026-07-01',
      endDate: '2026-07-31',
      employeeIds: [],
      departmentIds: [],
      period: 'day',
      page: 1,
      pageSize: 20,
    })).rejects.toThrow('数据概览接口尚未启用');
  });
});
