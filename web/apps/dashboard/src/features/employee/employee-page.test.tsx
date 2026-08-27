/* eslint-disable @typescript-eslint/unbound-method */
import { App } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import { EmployeePage, type EmployeePageApi } from './employee-page';

const access: AccessContext = {
  session: { token: 'token', userId: '1', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set(['/workEmployee/index']),
  allowedActions: new Set(['/workEmployee/index@search', '/workEmployee/index@sync']),
};

beforeEach(() => {
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    value: vi.fn(() => ({ matches: false, addListener: vi.fn(), removeListener: vi.fn(),
      addEventListener: vi.fn(), removeEventListener: vi.fn() })),
  });
  Object.defineProperty(window, 'getComputedStyle', {
    configurable: true,
    value: () => ({ getPropertyValue: () => '' }) as unknown as CSSStyleDeclaration,
  });
});
afterEach(cleanup);

function api(): EmployeePageApi {
  return {
    list: vi.fn(() => Promise.resolve({
      list: [{ id: 21, name: '张三', thumbAvatar: '', statusName: '已激活',
        contactAuthName: '是', gender: '男', applyNums: 1, addNums: 2,
        messageNums: 3, sendMessageNums: 4, replyMessageRatio: '0.75',
        averageReply: 30, invalidContact: 0 }],
      page: { page: 1, perPage: 10, total: 1, totalPage: 1 },
    })),
    conditions: vi.fn(() => Promise.resolve({
      status: [{ id: 1, name: '已激活' }],
      contactAuth: [{ id: 1, name: '是' }],
      syncTime: '2026-07-28 10:00:00',
    })),
    sync: vi.fn(() => Promise.resolve()),
  };
}

function renderPage(client: EmployeePageApi, allowedActions = access.allowedActions) {
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: {
      queries: { retry: false }, mutations: { retry: false },
    } })}>
      <App><DashboardAccessProvider value={{ ...access, allowedActions }}>
        <EmployeePage api={client} />
      </DashboardAccessProvider></App>
    </QueryClientProvider>,
  );
}

describe('EmployeePage', () => {
  it('loads the employee list and search conditions', async () => {
    const client = api();
    renderPage(client);
    expect(await screen.findByText('张三')).not.toBeNull();
    expect(screen.getByText(/2026-07-28 10:00:00/)).not.toBeNull();
    expect(client.list).toHaveBeenCalledWith({
      name: '', status: null, contactAuth: null, page: 1, perPage: 10,
    });
  });

  it('keeps page actions available when no legacy action contract exists', async () => {
    renderPage(api(), new Set());
    await screen.findByText('张三');
    expect(screen.getByRole('button', { name: '条件筛选' })).not.toBeNull();
    expect(screen.getByRole('button', { name: '立即同步人员' })).not.toBeNull();
  });

  it('applies the member name filter', async () => {
    const client = api();
    renderPage(client);
    await screen.findByText('张三');
    fireEvent.click(screen.getByRole('button', { name: '条件筛选' }));
    fireEvent.change(screen.getByLabelText('成员姓名'), { target: { value: '李四' } });
    fireEvent.click(screen.getByRole('button', { name: /确\s*定/ }));
    await waitFor(() => expect(client.list).toHaveBeenLastCalledWith({
      name: '李四', status: null, contactAuth: null, page: 1, perPage: 10,
    }));
  });

  it('refreshes the list and conditions after synchronization', async () => {
    const client = api();
    renderPage(client);
    await screen.findByText('张三');
    fireEvent.click(screen.getByRole('button', { name: '立即同步人员' }));
    expect(client.sync).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole('button', { name: '确认' }));
    await waitFor(() => expect(client.sync).toHaveBeenCalledOnce());
    await waitFor(() => expect(client.list).toHaveBeenCalledTimes(2));
    expect(client.conditions).toHaveBeenCalledTimes(2);
    expect(await screen.findByText('人员同步任务已提交')).not.toBeNull();
  });
});
