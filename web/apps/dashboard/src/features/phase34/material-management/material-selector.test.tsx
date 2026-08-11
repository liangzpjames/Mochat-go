import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../../app/access-context';
import type { AccessContext } from '../../../app/access-loader';
import type { BusinessWorkbenchApi } from '../../business-workbench/business-workbench-page';
import { MaterialSelector } from './material-selector';

const access: AccessContext = {
  session: { token: 'token', userId: '1', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set(['/acquisition/material-management']),
  allowedActions: new Set(),
};

afterEach(() => { cleanup(); vi.restoreAllMocks(); });

function view(api: BusinessWorkbenchApi, onChange = vi.fn()) {
  return {
    onChange,
    ...render(
      <MemoryRouter>
        <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
          <DashboardAccessProvider value={access}>
            <MaterialSelector api={api} scene="group_send" value={null} onChange={onChange} />
          </DashboardAccessProvider>
        </QueryClientProvider>
      </MemoryRouter>,
    ),
  };
}

describe('MaterialSelector', () => {
  it('loads tenant-visible materials for a scene and returns the selected record', async () => {
    const read = vi.fn().mockResolvedValue({ list: [{ id: 21, name: '欢迎话术', type: 'text', preview: '你好，欢迎咨询', scopeType: 'public' }] });
    const api: BusinessWorkbenchApi = { read, write: vi.fn() };
    const onChange = vi.fn();

    view(api, onChange);

    const selector = await screen.findByRole('combobox', { name: '引用素材' });
    expect(read).toHaveBeenCalledWith('/materialSelector/index', { scene: 'group_send' });
    expect(await screen.findByRole('option', { name: '欢迎话术' })).toBeTruthy();

    fireEvent.change(selector, { target: { value: '21' } });

    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ id: 21, preview: '你好，欢迎咨询' }));
  });
});
