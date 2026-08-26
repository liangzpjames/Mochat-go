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
	inspect: vi.fn(() => Promise.resolve({ status: 'valid' as const, tenantName: '安全租户', accountHint: '138****8000', expiresAt: 1780000000, primaryAction: 'activate' as const })),
    navigate: vi.fn(),
    ...overrides,
  };
  render(<ActivationPage {...props} />);
  return props;
}

describe('ActivationPage', () => {
  it('completes activation with the URL token and never echoes the raw token', async () => {
    const props = renderPage();
	await screen.findByLabelText('新密码');
    fireEvent.change(screen.getByLabelText('新密码'), { target: { value: 'new-dashboard-password' } });
    fireEvent.change(screen.getByLabelText('确认密码'), { target: { value: 'new-dashboard-password' } });
    fireEvent.click(screen.getByRole('button', { name: '激活' }));

    await waitFor(() => expect(props.activate).toHaveBeenCalledWith({
      activationToken: 'one-time-activation-token',
      password: 'new-dashboard-password',
    }));
	expect(await screen.findByRole('button', { name: '前往登录' })).not.toBeNull();
	expect(props.navigate).not.toHaveBeenCalled();
    expect(screen.queryByText('one-time-activation-token')).toBeNull();
  });

  it('shows a retryable validation error and does not call the API for mismatched passwords', async () => {
    const activate = vi.fn(() => Promise.resolve());
    renderPage({ activate });
	await screen.findByLabelText('新密码');
    fireEvent.change(screen.getByLabelText('新密码'), { target: { value: 'new-dashboard-password' } });
    fireEvent.change(screen.getByLabelText('确认密码'), { target: { value: 'different-password' } });
    fireEvent.click(screen.getByRole('button', { name: '激活' }));

    expect((await screen.findByRole('alert')).textContent).toContain('两次输入的密码不一致');
    expect(activate).not.toHaveBeenCalled();
		expect(screen.getByLabelText('新密码')).not.toBeNull();
		expect(screen.getByRole('button', { name: '激活' })).not.toBeNull();
  });

  it('keeps the form usable at a 390px viewport', async () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 });
	renderPage();
    expect(screen.getByRole('main').className).toContain('activation-page');
	  expect(await screen.findByRole('button', { name: '激活' })).not.toBeNull();
  });

  it('exposes activation through the real Dashboard router with a URL token', async () => {
    const activate = vi.fn(() => Promise.resolve());
	const inspectActivation = vi.fn(() => Promise.resolve({ status: 'valid' as const, tenantName: '安全租户', accountHint: '138****8000', expiresAt: 1780000000, primaryAction: 'activate' as const }));
    const router = createDashboardRouter({
      activate,
	  inspectActivation,
      getSession: () => null,
      initialEntries: ['/activate?token=router-activation-token'],
      loadInitialData: () => Promise.resolve(),
    });
    render(<RouterProvider router={router} />);

	expect(screen.getByRole('heading', { name: '激活账号' })).not.toBeNull();
	await screen.findByLabelText('新密码');
    fireEvent.change(screen.getByLabelText('新密码'), { target: { value: 'router-password' } });
    fireEvent.change(screen.getByLabelText('确认密码'), { target: { value: 'router-password' } });
    fireEvent.click(screen.getByRole('button', { name: '激活' }));

    await waitFor(() => expect(activate).toHaveBeenCalledWith({
      activationToken: 'router-activation-token',
      password: 'router-password',
    }));
  });

	it('cleans fragment before inspection and never touches web storage', async () => {
		window.history.replaceState({}, '', '/activate#token=fragment-secret');
		const replaceState = vi.spyOn(window.history, 'replaceState');
		const local = vi.spyOn(Storage.prototype, 'setItem');
		const inspectActivation = vi.fn(() => {
			expect(window.location.pathname + window.location.search + window.location.hash).toBe('/activate');
			return Promise.resolve({ status: 'expired' as const, tenantName: '安全租户', accountHint: '138****8000', expiresAt: 1, primaryAction: 'contact_admin' as const });
		});
		const router = createDashboardRouter({ activate: vi.fn(), inspectActivation, getSession: () => null, initialEntries: ['/activate#token=fragment-secret'], loadInitialData: () => Promise.resolve() });
		render(<RouterProvider router={router} />);
		await screen.findByText('激活入口已过期');
		expect(inspectActivation).toHaveBeenCalledWith('fragment-secret');
		expect(replaceState).toHaveBeenCalledWith({}, '', '/activate');
		expect(local).not.toHaveBeenCalled();
	});

	it.each([
		['activated', '前往登录'], ['expired', '联系平台管理员重新生成激活入口'], ['revoked', '联系平台管理员重新生成激活入口'], ['invalid', '联系平台管理员重新生成激活入口'],
	] as const)('renders %s activation state', async (status, expected) => {
		renderPage({ inspect: () => Promise.resolve({ status, tenantName: '', accountHint: '', expiresAt: 0, primaryAction: status === 'activated' ? 'login' : 'contact_admin' }) });
		expect(await screen.findByText(expected)).not.toBeNull();
	});

	it('shows a safe inspection error and retries without exposing the token', async () => {
		const inspect = vi.fn()
			.mockRejectedValueOnce(new Error('backend included one-time-activation-token'))
			.mockResolvedValueOnce({ status: 'valid', tenantName: '安全租户', accountHint: '138****8000', expiresAt: 1780000000, primaryAction: 'activate' });
		renderPage({ inspect });
		expect((await screen.findByRole('alert')).textContent).toBe('无法检查激活入口，请重试');
		expect(screen.queryByText(/one-time-activation-token/)).toBeNull();
		fireEvent.click(screen.getByRole('button', { name: '重新检查' }));
		expect(await screen.findByLabelText('新密码')).not.toBeNull();
		expect(inspect).toHaveBeenCalledTimes(2);
	});
});
