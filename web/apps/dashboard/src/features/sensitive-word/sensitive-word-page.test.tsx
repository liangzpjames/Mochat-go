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
      list: vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, perPage: 10 }),
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
});
