import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, expect, it, vi } from 'vitest';
import { TagPage } from './tag-page';

vi.mock('../../app/access-context', () => ({ useDashboardAccess: () => ({ corp: { id: '7' } }) }));

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
});
