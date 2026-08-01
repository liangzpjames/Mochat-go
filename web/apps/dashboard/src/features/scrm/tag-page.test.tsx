import { ApiError } from '@mochat/api-client';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { TagPage } from './tag-page';

vi.mock('../../app/access-context', () => ({ useDashboardAccess: () => ({ corp: { id: '7' } }) }));
afterEach(cleanup);

describe('TagPage', () => {
  it('creates a tag and binds it to selected contacts', async () => {
    const create = vi.fn().mockResolvedValue({ id: 't2', name: '重点' });
    const bind = vi.fn().mockResolvedValue({});
    const api = { listTags: vi.fn().mockResolvedValue({ items: [{ id: 't1', name: '普通', version: 1 }], nextCursor: '' }), createTag: create, renameTag: vi.fn(), bindTags: bind };
    render(<QueryClientProvider client={new QueryClient()}><TagPage api={api} /></QueryClientProvider>);
    await screen.findByText('普通');
    fireEvent.change(screen.getByLabelText('新标签'), { target: { value: '重点' } });
    fireEvent.click(screen.getByRole('button', { name: '创建标签' }));
    await waitFor(() => expect(create).toHaveBeenCalledWith({ corpId: 7, name: '重点', idempotencyKey: expect.any(String) }));
    fireEvent.click(screen.getByRole('button', { name: '绑定到联系人' }));
    await waitFor(() => expect(bind).toHaveBeenCalledWith({ corpId: 7, tagId: 't1', contactIds: ['c1'], idempotencyKey: expect.any(String) }));
  });

  it('uses shared actions and retries the PageState query failure', async () => {
    const api = {
      listTags: vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce({ items: [{ id: 't1', name: '普通', version: 1 }], nextCursor: '' }),
      createTag: vi.fn(),
      bindTags: vi.fn(),
    };
    const { container } = render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><TagPage api={api} /></QueryClientProvider>);

    await waitFor(() => expect(container.querySelector('.page-state-error')).not.toBeNull());
    fireEvent.click(container.querySelector('.page-state-retry')!);
    await waitFor(() => expect(api.listTags).toHaveBeenCalledTimes(2));

    expect(container.querySelector('.dashboard-page-header')).not.toBeNull();
    expect(container.querySelector('.dashboard-table-actions')).not.toBeNull();
    expect(container.querySelector('.dashboard-data-card')).not.toBeNull();
  });

  it('renders empty and 403 query responses through PageState', async () => {
    const emptyApi = { listTags: vi.fn().mockResolvedValue({ items: [], nextCursor: '' }), createTag: vi.fn(), bindTags: vi.fn() };
    const emptyView = render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><TagPage api={emptyApi} /></QueryClientProvider>);
    await waitFor(() => expect(emptyView.container.querySelector('.page-state-empty')).not.toBeNull());
    emptyView.unmount();

    const forbiddenApi = { listTags: vi.fn().mockRejectedValue(new ApiError('forbidden', '无权限', { status: 403 })), createTag: vi.fn(), bindTags: vi.fn() };
    const forbiddenView = render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><TagPage api={forbiddenApi} /></QueryClientProvider>);
    await waitFor(() => expect(forbiddenView.container.querySelector('.page-state-forbidden')).not.toBeNull());
  });
});
