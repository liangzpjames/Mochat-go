import { ApiError } from '@mochat/api-client';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { SensitiveWordPage } from './sensitive-word-page';

vi.mock('../../app/access-context', () => ({
  useDashboardAccess: () => ({ corp: { id: '7' }, allowedActions: new Set(['/ai-insight/v2/sensitive-word@add']) }),
}));

afterEach(cleanup);

function renderPage(api: Parameters<typeof SensitiveWordPage>[0]['api']) {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <SensitiveWordPage api={api} />
    </QueryClientProvider>,
  );
}

describe('SensitiveWordPage', () => {
  it('loads words and creates a new word through the real API contract', async () => {
    const api = {
      list: vi.fn().mockResolvedValue({ items: [{ id: 1, groupId: 2, groupName: '默认', name: '旧词', status: 1 }], total: 1, page: 1, perPage: 10 }),
      groups: vi.fn().mockResolvedValue([{ id: 2, name: '默认' }]),
      create: vi.fn().mockResolvedValue(undefined),
      setEnabled: vi.fn(), move: vi.fn(), remove: vi.fn(), createGroup: vi.fn(), renameGroup: vi.fn(),
      matches: vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, perPage: 10 }),
      matchDetail: vi.fn(),
    };
    renderPage(api);

    expect(await screen.findByText('敏感词库')).toBeTruthy();
    fireEvent.change(await screen.findByRole('textbox', { name: '敏感词名称' }), { target: { value: '报价' } });
    fireEvent.change(screen.getByRole('combobox', { name: '敏感词分组' }), { target: { value: '2' } });
    await waitFor(() => expect(screen.getByRole('button', { name: '新增敏感词' })).toBeTruthy());
    fireEvent.click(screen.getByRole('button', { name: '新增敏感词' }));

    await waitFor(() => expect(api.create).toHaveBeenCalledWith(expect.objectContaining({ names: ['报价'] })));
  });

  it('uses shared filters and retries both PageState data sources', async () => {
    const api = {
      list: vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce({ items: [{ id: 1, groupId: 2, groupName: '默认', name: '旧词', status: 1 }], total: 1, page: 1, perPage: 10 }),
      groups: vi.fn().mockResolvedValue([{ id: 2, name: '默认' }]),
      create: vi.fn(),
      setEnabled: vi.fn(), move: vi.fn(), remove: vi.fn(), createGroup: vi.fn(), renameGroup: vi.fn(),
      matches: vi.fn(), matchDetail: vi.fn(),
    };
    const { container } = renderPage(api);

    await waitFor(() => expect(container.querySelector('.page-state-error')).not.toBeNull());
    fireEvent.click(container.querySelector('.page-state-retry')!);
    await waitFor(() => expect(api.list).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(api.groups).toHaveBeenCalledTimes(2));

    expect(container.querySelector('.dashboard-page-header')).not.toBeNull();
    expect(container.querySelector('.dashboard-filter-bar')).not.toBeNull();
    expect(container.querySelector('.dashboard-data-card')).not.toBeNull();
    expect(container.querySelector('.dashboard-table-scroll')).not.toBeNull();
  });

  it('renders empty and 403 query responses through PageState', async () => {
    const emptyApi = {
      list: vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, perPage: 10 }), groups: vi.fn().mockResolvedValue([]), create: vi.fn(),
      setEnabled: vi.fn(), move: vi.fn(), remove: vi.fn(), createGroup: vi.fn(), renameGroup: vi.fn(), matches: vi.fn(), matchDetail: vi.fn(),
    };
    const emptyView = renderPage(emptyApi);
    await waitFor(() => expect(emptyView.container.querySelector('.page-state-empty')).not.toBeNull());
    emptyView.unmount();

    const forbiddenApi = { ...emptyApi, list: vi.fn().mockRejectedValue(new ApiError('forbidden', '无权限', { status: 403 })) };
    const forbiddenView = renderPage(forbiddenApi);
    await waitFor(() => expect(forbiddenView.container.querySelector('.page-state-forbidden')).not.toBeNull());
  });
});
