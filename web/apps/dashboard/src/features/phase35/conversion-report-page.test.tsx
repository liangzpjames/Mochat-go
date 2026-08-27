/* eslint-disable @typescript-eslint/no-unsafe-member-access -- mock call inspection verifies the runtime request payload */
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, expect, test, vi } from 'vitest';
import { ConversionReportPage } from './conversion-report-page';

afterEach(cleanup);

const summary = {
  lead: 10, contact: 8, contactRate: .8, opportunity: 4, opportunityRate: .5,
  won: 2, wonRate: .5, order: 1, orderRate: .5,
};

test('renders Chinese rates and opens a stage drill-down', async () => {
  const api = {
    read: vi.fn()
      .mockResolvedValueOnce({ summary, items: [], pagination: { page: 1, pageSize: 20, total: 0 } })
      .mockResolvedValueOnce({
        summary,
        items: [{ id: 'op-1', contactName: '张三', status: 'won', ownerName: '李四', day: '2026-08-01' }],
        pagination: { page: 1, pageSize: 20, total: 1 },
      }),
    write: vi.fn(),
  };
  render(<QueryClientProvider client={new QueryClient()}><ConversionReportPage api={api} /></QueryClientProvider>);
  expect(await screen.findByText('联系人转化率')).not.toBeNull();
  fireEvent.click(await screen.findByRole('button', { name: /商机/ }));
  const dialog = await screen.findByRole('dialog', { name: '商机阶段明细' });
  expect(dialog).not.toBeNull();
  expect(await screen.findByText('张三')).not.toBeNull();
  expect(screen.getByText('李四')).not.toBeNull();
  await waitFor(() => {
    const stageCalls = api.read.mock.calls.filter(([, params]) => params.stage === 'opportunity');
    expect(stageCalls.length).toBeGreaterThan(0);
  });
});

test('stage detail keeps pagination after drill-down', async () => {
  const api = {
    read: vi.fn((_endpoint: string, params: Record<string, unknown>) => {
      if (params.stage) {
        return Promise.resolve({
          summary,
          items: [{ id: 'op-1', contactName: '张三', status: 'won', ownerName: '李四', day: '2026-08-01' }],
          pagination: { page: Number(params.page ?? 1), pageSize: 20, total: 21 },
        });
      }
      return Promise.resolve({ summary, items: [], pagination: { page: 1, pageSize: 20, total: 0 } });
    }),
    write: vi.fn(),
  };
  render(<QueryClientProvider client={new QueryClient()}><ConversionReportPage api={api} /></QueryClientProvider>);
  fireEvent.click(await screen.findByRole('button', { name: /赢单/ }));
  const dialog = await screen.findByRole('dialog', { name: '赢单阶段明细' });
  expect(dialog).not.toBeNull();
  expect(await screen.findByText('张三')).not.toBeNull();
  const next = screen.getByRole('button', { name: '下一页' });
  expect((next as HTMLButtonElement).disabled).toBe(false);
  fireEvent.click(next);
  await waitFor(() => {
    const pageTwo = api.read.mock.calls.filter(([, params]) => params.stage === 'won' && params.page === 2);
    expect(pageTwo.length).toBeGreaterThan(0);
  });
});

test('uses a business notice instead of raw limitation details', async () => {
  const api = {
    read: vi.fn().mockResolvedValue({
      summary,
      limitations: [{ provider: 'conversion', message: '转化 Provider 已提供部分数据' }],
    }),
    write: vi.fn(),
  };
  render(<QueryClientProvider client={new QueryClient()}><ConversionReportPage api={api} /></QueryClientProvider>);

  expect(await screen.findByText('部分数据暂未同步完整，当前仅展示已获取的数据。')).not.toBeNull();
  expect(screen.queryByText(/Provider/i)).toBeNull();
});
