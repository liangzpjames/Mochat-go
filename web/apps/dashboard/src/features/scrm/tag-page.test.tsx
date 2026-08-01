import { ApiError } from '@mochat/api-client';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { TagPage } from './tag-page';

vi.mock('../../app/access-context', () => ({ useDashboardAccess: () => ({ corp: { id: '7' } }) }));
afterEach(cleanup);

const catalog = {
  groups: [
    { id: 'g1', name: '客户等级', version: 1, tagCount: 1 },
    { id: 'g2', name: '地域', version: 2, tagCount: 0 },
  ],
  tags: [{ id: 't1', groupId: 'g1', name: '普通', version: 3, usageCount: 2 }],
};

function api(overrides: Record<string, unknown> = {}) {
  return {
    listTagCatalog: vi.fn().mockResolvedValue(catalog),
    createTagGroup: vi.fn().mockResolvedValue(catalog.groups[0]),
    renameTagGroup: vi.fn().mockResolvedValue({ ...catalog.groups[0], name: '客户分层', version: 2 }),
    createTag: vi.fn().mockResolvedValue(catalog.tags[0]),
    renameTag: vi.fn().mockResolvedValue({ ...catalog.tags[0], name: '重点', version: 4 }),
    moveTag: vi.fn().mockResolvedValue({ ...catalog.tags[0], groupId: 'g2', version: 4 }),
    deleteTag: vi.fn().mockResolvedValue({ affectedResourceCount: 2 }),
    maintainTagContacts: vi.fn().mockResolvedValue({ ...catalog.tags[0], usageCount: 2, version: 4 }),
    ...overrides,
  };
}

function view(value = api(), entry = '/customer/tags') {
  return render(<MemoryRouter initialEntries={[entry]}><QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })}><TagPage api={value} /></QueryClientProvider></MemoryRouter>);
}

describe('TagPage', () => {
  it('restores group and keyword filters from URL and can reset them', async () => {
    const value = api();
    view(value, '/customer/tags?groupId=g1&keyword=%E6%99%AE%E9%80%9A');
    await screen.findByText('普通');
    expect(value.listTagCatalog).toHaveBeenCalledWith({ corpId: 7, groupId: 'g1', keyword: '普通' });
    fireEvent.click(screen.getByRole('button', { name: '重置' }));
    await waitFor(() => expect(value.listTagCatalog).toHaveBeenLastCalledWith({ corpId: 7, groupId: undefined, keyword: undefined }));
  });

  it('creates and renames groups, and creates renames and moves tags', async () => {
    const value = api();
    view(value);
    await screen.findByText('普通');
    fireEvent.change(screen.getByLabelText('新标签组'), { target: { value: '生命周期' } });
    fireEvent.click(screen.getByRole('button', { name: '新增标签组' }));
    await waitFor(() => expect(value.createTagGroup).toHaveBeenCalledWith(expect.objectContaining({ corpId: 7, name: '生命周期', idempotencyKey: expect.any(String) })));
    fireEvent.click(screen.getByRole('button', { name: '改名标签组 客户等级' }));
    await waitFor(() => expect(value.renameTagGroup).toHaveBeenCalledWith(expect.objectContaining({ groupId: 'g1', version: 1 })));
    fireEvent.change(screen.getByLabelText('新标签'), { target: { value: '重点' } });
    fireEvent.click(screen.getByRole('button', { name: '新增标签' }));
    await waitFor(() => expect(value.createTag).toHaveBeenCalledWith(expect.objectContaining({ corpId: 7, groupId: 'g1', name: '重点' })));
    fireEvent.click(screen.getByRole('button', { name: '改名标签 普通' }));
    fireEvent.click(screen.getByRole('button', { name: '移动标签 普通' }));
    await waitFor(() => expect(value.moveTag).toHaveBeenCalledWith(expect.objectContaining({ tagId: 't1', groupId: 'g2', version: 3 })));
  });

  it('binds and unbinds contacts in one batch and refreshes usage count', async () => {
    const value = api();
    view(value);
    await screen.findByText('使用 2');
    fireEvent.change(screen.getByLabelText('绑定联系人'), { target: { value: 'c1,c2' } });
    fireEvent.change(screen.getByLabelText('解绑联系人'), { target: { value: 'c3' } });
    fireEvent.click(screen.getByRole('button', { name: '维护联系人 普通' }));
    await waitFor(() => expect(value.maintainTagContacts).toHaveBeenCalledWith(expect.objectContaining({ tagId: 't1', addContactIds: ['c1', 'c2'], removeContactIds: ['c3'], version: 3 })));
    expect(value.listTagCatalog).toHaveBeenCalledTimes(2);
  });

  it('confirms deletion with affectedResourceCount and deletes with version', async () => {
    const value = api();
    view(value);
    await screen.findByText('普通');
    fireEvent.click(screen.getByRole('button', { name: '删除标签 普通' }));
    expect(await screen.findByText('将影响 2 个联系人')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '确认删除' }));
    await waitFor(() => expect(value.deleteTag).toHaveBeenCalledWith(expect.objectContaining({ tagId: 't1', version: 3, idempotencyKey: expect.any(String) })));
  });

  it('uses PageState for query and mutation failures with retry', async () => {
    const value = api({ listTagCatalog: vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(catalog) });
    const first = view(value);
    await waitFor(() => expect(first.container.querySelector('.page-state-error')).not.toBeNull());
    fireEvent.click(first.container.querySelector('.page-state-retry')!);
    await screen.findByText('普通');
    first.unmount();

    const conflict = api({ moveTag: vi.fn().mockRejectedValue(new ApiError('validation', '版本冲突', { status: 409 })) });
    const second = view(conflict);
    await screen.findByText('普通');
    fireEvent.click(screen.getByRole('button', { name: '移动标签 普通' }));
    await waitFor(() => expect(second.container.querySelector('.page-state-conflict')).not.toBeNull());
  });
});
