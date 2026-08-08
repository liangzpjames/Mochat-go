import { describe, expect, it, vi } from 'vitest';

import { createDashboardOverviewApi } from './dashboard-overview-api';

const reportResponse = {
  summary: {
    customer: 137, lead: 58, contact: 46, opportunity: 30, won: 15, order: 12, behavior: 41, employee: 0,
    contactRate: 0.3358, opportunityRate: 0.6522, wonRate: 0.5, orderRate: 0.8,
  },
  series: [{ at: '2026-07-30T00:00:00Z', value: 12 }],
  items: [{ id: 'c1', day: '2026-07-30', ownerId: 0, ownerName: '' }],
  pagination: { page: 1, pageSize: 20, total: 1 },
  freshness: { provider: 'scrm', status: 'available', dataThrough: '2026-07-31T09:30:00Z' },
  limitations: [{ provider: 'conversation_archive', code: 'provider_unavailable', message: '会话归档表不可用' }],
};

describe('createDashboardOverviewApi', () => {
  it('loads the unified reporting overview and parses cards, trend and limitations', async () => {
    const request = vi.fn(() => Promise.resolve(reportResponse));
    const api = createDashboardOverviewApi({ request });

    const result = await api.load({
      corpId: '7',
      startDate: '2026-07-01',
      endDate: '2026-08-01',
      employeeIds: [],
      departmentIds: [],
      page: 1,
      pageSize: 20,
    });

    expect(result).toEqual(expect.objectContaining({
      cards: [
        { key: 'customer', label: '客户总数', value: 137 },
        { key: 'lead', label: '线索总数', value: 58 },
        { key: 'order', label: '订单总数', value: 12 },
        { key: 'behavior', label: '行为事件', value: 41 },
      ],
      trend: [{ date: '2026-07-30', addCustomerNum: 12 }],
      summary: expect.objectContaining({ customer: 137, employee: 0 }),
      limitations: reportResponse.limitations,
      updatedAt: '2026-07-31 09:30:00',
      page: 1,
      pageSize: 20,
      total: 1,
    }));
    expect(request).toHaveBeenCalledWith(
      '/reports/overview?corpId=7&timezone=Asia%2FShanghai&startAt=2026-07-01T00%3A00%3A00%2B08%3A00&endAt=2026-08-01T00%3A00%3A00%2B08%3A00&page=1&pageSize=20',
    );
  });

  it('serializes employee and department filters and pagination', async () => {
    const request = vi.fn(() => Promise.resolve({ summary: {}, series: [], pagination: { page: 2, pageSize: 20, total: 0 }, freshness: { provider: 'scrm', status: 'available' }, limitations: [] }));
    const api = createDashboardOverviewApi({ request });

    await api.load({
      corpId: '7',
      startDate: '2026-07-01',
      endDate: '2026-08-01',
      employeeIds: ['9', '12'],
      departmentIds: ['3', '8'],
      page: 2,
      pageSize: 20,
    });

    expect(request).toHaveBeenCalledWith(
      '/reports/overview?corpId=7&timezone=Asia%2FShanghai&startAt=2026-07-01T00%3A00%3A00%2B08%3A00&endAt=2026-08-01T00%3A00%3A00%2B08%3A00&employeeIds=9&employeeIds=12&departmentIds=3&departmentIds=8&page=2&pageSize=20',
    );
  });

  it('exports csv with the same serialized query and a single trend column', async () => {
    const request = vi.fn(() => Promise.resolve(reportResponse));
    const api = createDashboardOverviewApi({ request });
    const blob = await api.exportCsv({
      corpId: '7',
      startDate: '2026-07-01',
      endDate: '2026-08-01',
      employeeIds: ['9'],
      departmentIds: ['3'],
      page: 1,
      pageSize: 20,
    });
    expect(blob.type).toBe('text/csv;charset=utf-8');
    expect(request).toHaveBeenCalledWith(
      '/reports/overview?corpId=7&timezone=Asia%2FShanghai&startAt=2026-07-01T00%3A00%3A00%2B08%3A00&endAt=2026-08-01T00%3A00%3A00%2B08%3A00&employeeIds=9&departmentIds=3&page=1&pageSize=20',
    );
  });

  it('rejects an incompatible legacy response instead of crashing', async () => {
    const request = vi.fn(() => Promise.resolve({ weChatContactNum: 137, updateTime: '2026-07-31 09:30:00' }));
    const api = createDashboardOverviewApi({ request });
    await expect(api.load({
      corpId: '7',
      startDate: '2026-07-01',
      endDate: '2026-08-01',
      employeeIds: [],
      departmentIds: [],
      page: 1,
      pageSize: 20,
    })).rejects.toThrow('数据概览接口返回了无效数据');
  });
});
