import { QueryClient } from '@tanstack/react-query';
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { CorpProvider } from './corp-provider';
import type { CorpOption } from './corp-api';
import { DashboardSessionActionsProvider } from '../auth/session-actions';

const authorized: CorpOption = {
  id: '3',
  name: '迁移企业',
  authorized: true,
};

afterEach(cleanup);

function renderProvider(
  corps: CorpOption[],
  overrides: Partial<Parameters<typeof CorpProvider>[0]> = {},
  onLogout: () => Promise<void> = () => Promise.resolve(),
) {
  const queryClient = new QueryClient();
  const props: Parameters<typeof CorpProvider>[0] = {
    bindCorp: vi.fn(() => Promise.resolve()),
    children: <div>业务内容</div>,
    initialCorpId: null,
    loadCorps: vi.fn(() => Promise.resolve(corps)),
    loadMenuAccess: vi.fn(() => Promise.resolve({
      firstRoute: '/workContact/index',
    })),
    navigate: vi.fn(),
    persistCorpId: vi.fn(),
    queryClient,
    ...overrides,
  };
  render(
    <DashboardSessionActionsProvider
      onLogout={onLogout}
      userId="7"
    >
      <CorpProvider {...props} />
    </DashboardSessionActionsProvider>,
  );
  return props;
}

describe('CorpProvider', () => {
  it('shows an empty state when the user has no enterprises', async () => {
    const onLogout = vi.fn(() => Promise.resolve());
    renderProvider([], {}, onLogout);

    expect(await screen.findByRole('heading', { name: '暂无可用企业' }))
      .not.toBeNull();
    expect(screen.getByText('当前账号：7')).not.toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '退出登录' }));
    await waitFor(() => expect(onLogout).toHaveBeenCalledOnce());
  });

  it('automatically selects one authorized enterprise', async () => {
    const props = renderProvider([authorized]);

    await waitFor(() => expect(props.bindCorp).toHaveBeenCalledWith('3'));
    expect(props.persistCorpId).toHaveBeenCalledWith('3');
    expect(props.navigate).toHaveBeenCalledWith('/workContact/index');
  });

  it('uses loader-provided enterprises without fetching them twice', async () => {
    const loadCorps = vi.fn(() => Promise.resolve([]));
    const props = renderProvider([], {
      initialCorps: [authorized],
      loadCorps,
    });

    await waitFor(() => expect(props.bindCorp).toHaveBeenCalledWith('3'));
    expect(loadCorps).not.toHaveBeenCalled();
  });

  it('requires an explicit choice for multiple enterprises and disables unauthorized ones', async () => {
    const props = renderProvider([
      authorized,
      { id: '4', name: '未授权企业', authorized: false },
    ]);

    expect(
      (await screen.findByRole('button', { name: '迁移企业' }))
        .hasAttribute('disabled'),
    ).toBe(false);
    expect(
      screen.getByRole('button', { name: '未授权企业' })
        .hasAttribute('disabled'),
    ).toBe(true);
    expect(props.bindCorp).not.toHaveBeenCalled();
  });

  it('switches enterprise in the required cache-safe order', async () => {
    const order: string[] = [];
    const queryClient = new QueryClient();
    const cancelQueries = vi.spyOn(queryClient, 'cancelQueries').mockImplementation(() => {
      order.push('cancel');
      return Promise.resolve();
    });
    const removeQueries = vi.spyOn(queryClient, 'removeQueries').mockImplementation(() => {
      order.push('remove');
    });
    const props = renderProvider(
      [authorized, { id: '4', name: '新企业', authorized: true }],
      {
        initialCorpId: '3',
        queryClient,
        persistCorpId: vi.fn(() => order.push('persist')),
        loadMenuAccess: vi.fn(() => {
          order.push('menu');
          return Promise.resolve({ firstRoute: '/new/home' });
        }),
        navigate: vi.fn(() => order.push('navigate')),
      },
    );

    fireEvent.click(await screen.findByRole('button', { name: '新企业' }));
    await waitFor(() => expect(props.navigate).toHaveBeenCalledWith('/new/home'));

    expect(order).toEqual(['cancel', 'persist', 'remove', 'menu', 'navigate']);
    expect(cancelQueries).toHaveBeenCalledWith({
      queryKey: ['corp', '3'],
    });
    expect(removeQueries).toHaveBeenCalledWith({
      queryKey: ['corp', '3'],
    });
  });

  it('prevents overlapping enterprise switches', async () => {
    let releaseBind: (() => void) | undefined;
    const bindCorp = vi.fn(() => new Promise<void>((resolve) => {
      releaseBind = resolve;
    }));
    renderProvider(
      [authorized, { id: '4', name: '新企业', authorized: true }],
      { bindCorp, initialCorpId: '3' },
    );

    const next = await screen.findByRole('button', { name: '新企业' });
    fireEvent.click(next);
    await waitFor(() => expect(bindCorp).toHaveBeenCalledOnce());
    expect(next.hasAttribute('disabled')).toBe(true);
    expect(screen.queryByText('业务内容')).toBeNull();
    fireEvent.click(next);
    expect(bindCorp).toHaveBeenCalledOnce();
    releaseBind?.();
  });

  it('keeps UI state aligned with a persisted corp when menu loading fails', async () => {
    renderProvider(
      [authorized, { id: '4', name: '新企业', authorized: true }],
      {
        initialCorpId: '3',
        loadMenuAccess: vi.fn(() => Promise.reject(new Error('菜单加载失败'))),
      },
    );

    fireEvent.click(await screen.findByRole('button', { name: '新企业' }));

    expect((await screen.findByRole('alert')).textContent).toContain('菜单加载失败');
    expect(screen.queryByText('业务内容')).toBeNull();
    expect(
      screen.getByRole('button', { name: '新企业' }).hasAttribute('disabled'),
    ).toBe(true);
  });
});
