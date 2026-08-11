/* @vitest-environment jsdom */
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { RouterProvider } from 'react-router';

import { ActivationPage } from './activation-page';
import { createDashboardRouter } from '../../app/router';

afterEach(cleanup);

function renderPage(overrides: Partial<Parameters<typeof ActivationPage>[0]> = {}) {
  const props: Parameters<typeof ActivationPage>[0] = {
    activationToken: 'one-time-activation-token',
    activate: vi.fn(() => Promise.resolve()),
    navigate: vi.fn(),
    ...overrides,
  };
  render(<ActivationPage {...props} />);
  return props;
}

describe('ActivationPage', () => {
  it('completes activation with the URL token and never echoes the raw token', async () => {
    const props = renderPage();
    fireEvent.change(screen.getByLabelText('新密码'), { target: { value: 'new-dashboard-password' } });
    fireEvent.change(screen.getByLabelText('确认密码'), { target: { value: 'new-dashboard-password' } });
    fireEvent.click(screen.getByRole('button', { name: '激活' }));

    await waitFor(() => expect(props.activate).toHaveBeenCalledWith({
      activationToken: 'one-time-activation-token',
      password: 'new-dashboard-password',
    }));
    expect(props.navigate).toHaveBeenCalledWith('/login');
    expect(screen.queryByText('one-time-activation-token')).toBeNull();
  });

  it('shows a retryable validation error and does not call the API for mismatched passwords', async () => {
    const activate = vi.fn(() => Promise.resolve());
    renderPage({ activate });
    fireEvent.change(screen.getByLabelText('新密码'), { target: { value: 'new-dashboard-password' } });
    fireEvent.change(screen.getByLabelText('确认密码'), { target: { value: 'different-password' } });
    fireEvent.click(screen.getByRole('button', { name: '激活' }));

    expect((await screen.findByRole('alert')).textContent).toContain('两次输入的密码不一致');
    expect(activate).not.toHaveBeenCalled();
  });

  it('keeps the form usable at a 390px viewport', () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 });
    renderPage();
    expect(screen.getByRole('main').className).toContain('activation-page');
    expect(screen.getByRole('button', { name: '激活' })).not.toBeNull();
  });

  it('exposes activation through the real Dashboard router with a URL token', async () => {
    const activate = vi.fn(() => Promise.resolve());
    const router = createDashboardRouter({
      activate,
      getSession: () => null,
      initialEntries: ['/activate?token=router-activation-token'],
      loadInitialData: () => Promise.resolve(),
    });
    render(<RouterProvider router={router} />);

    expect(screen.getByRole('heading', { name: '激活账号' })).not.toBeNull();
    fireEvent.change(screen.getByLabelText('新密码'), { target: { value: 'router-password' } });
    fireEvent.change(screen.getByLabelText('确认密码'), { target: { value: 'router-password' } });
    fireEvent.click(screen.getByRole('button', { name: '激活' }));

    await waitFor(() => expect(activate).toHaveBeenCalledWith({
      activationToken: 'router-activation-token',
      password: 'router-password',
    }));
  });
});
