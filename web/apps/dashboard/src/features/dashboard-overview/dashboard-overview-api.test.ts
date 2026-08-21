/* eslint-disable @typescript-eslint/no-unsafe-assignment -- malformed API fixtures intentionally exercise unknown payload normalization */
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
  aiMetrics: { analysisCount: 5, employeeNegativeEmotion: null, customerNegativeEmotion: null, riskBehavior: null, sensitiveWords: null },
  quality: {
    sensitiveWords: 2, riskBehavior: 4, customerLoss: 0, timeoutWarning: 3,
    trend: [{ date: '2026-08-15', sensitiveWords: 1, riskBehavior: 2, customerLoss: 0, timeoutWarning: 2 }],
  },
  employeeRanking: [{ employeeId: 1001, employeeName: '张伟', sessions: 1, messages: 13 }],
  trajectory: [{ id: 'customer:2001', targetType: 'customer', targetId: '2001', employeeName: '张伟', messageCount: 13, latestAt: '2026-08-16 13:50:08' }],
};

describe('createDashboardOverviewApi', () => {
  it('loads the unified reporting overview and parses cards, trend and limitations', async () => {
    const request = vi.fn(() => Promise.resolve(reportResponse));
    const api = createDashboardOverviewApi({ request });

    const result = await api.load({
      corpId: '7',
      startDate: '2026-07-01',
      endDate: '2026-08-01',
      trendStartDate: '',
      trendEndDate: '',
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
      updatedAt: '2026-07-31 17:30:00',
      page: 1,
      pageSize: 20,
      total: 1,
      aiMetrics: reportResponse.aiMetrics,
      quality: reportResponse.quality,
      employeeRanking: reportResponse.employeeRanking,
      trajectory: reportResponse.trajectory,
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
      trendStartDate: '',
      trendEndDate: '',
      employeeIds: ['9', '12'],
      departmentIds: ['3', '8'],
      page: 2,
      pageSize: 20,
    });

    expect(request).toHaveBeenCalledWith(
      '/reports/overview?corpId=7&timezone=Asia%2FShanghai&startAt=2026-07-01T00%3A00%3A00%2B08%3A00&endAt=2026-08-01T00%3A00%3A00%2B08%3A00&employeeIds=9&employeeIds=12&departmentIds=3&departmentIds=8&page=2&pageSize=20',
    );
  });

  it('preserves missing metrics as null while retaining real zero values', async () => {
    const request = vi.fn(() => Promise.resolve({
      ...reportResponse,
      summary: { ...reportResponse.summary, customer: null },
      conversation: {
        customer: { sessions: 0, employeeMessages: undefined, customerMessages: null },
        room: { sessions: 1, employeeMessages: 0, customerMessages: 0 },
        trend: [{ date: '2026-08-16', customerSessions: 0 }],
      },
    }));
    const api = createDashboardOverviewApi({ request });

    const result = await api.load({
      corpId: '7', startDate: '2026-07-01', endDate: '2026-08-01', trendStartDate: '', trendEndDate: '',
      employeeIds: [], departmentIds: [], page: 1, pageSize: 20,
    });

    expect(result.summary.customer).toBeNull();
    expect(result.summary.employee).toBe(0);
    expect(result.conversation?.customer.sessions).toBe(0);
    expect(result.conversation?.customer.employeeMessages).toBeNull();
    expect(result.conversation?.trend[0]?.customerSessions).toBe(0);
    expect(result.conversation?.trend[0]?.customerEmployeeMessages).toBeNull();
  });

  it('preserves real overview modules and explicit null gaps from the response', async () => {
    const request = vi.fn(() => Promise.resolve(reportResponse));
    const api = createDashboardOverviewApi({ request });

    const result = await api.load({
      corpId: '7', startDate: '2026-07-01', endDate: '2026-08-01', trendStartDate: '', trendEndDate: '',
      employeeIds: [], departmentIds: [], page: 1, pageSize: 20,
    });

    expect(result.aiMetrics?.analysisCount).toBe(5);
    expect(result.aiMetrics?.customerNegativeEmotion).toBeNull();
    expect(result.quality).toEqual(reportResponse.quality);
    expect(result.employeeRanking?.[0]).toEqual(reportResponse.employeeRanking[0]);
    expect(result.trajectory?.[0]).toEqual(reportResponse.trajectory[0]);
  });

  it('exports csv with the same serialized query and a single trend column', async () => {
    const request = vi.fn(() => Promise.resolve(reportResponse));
    const api = createDashboardOverviewApi({ request });
    const blob = await api.exportCsv({
      corpId: '7',
      startDate: '2026-07-01',
      endDate: '2026-08-01',
      trendStartDate: '',
      trendEndDate: '',
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
      trendStartDate: '',
      trendEndDate: '',
      employeeIds: [],
      departmentIds: [],
      page: 1,
      pageSize: 20,
    })).rejects.toThrow('数据概览接口返回了无效数据');
  });
});
