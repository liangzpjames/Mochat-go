/* eslint-disable @typescript-eslint/unbound-method */
import { App } from 'antd';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import { PasswordPage, type PasswordPageApi } from './password-page';

const access: AccessContext = {
  session: { token: 'Bearer token', userId: '1', expiresAt: null },
  corp: { id: '7', name: '测试企业', authorized: true },
  menu: [],
  allowedRoutes: new Set(['/passwordUpdate/index']),
  allowedActions: new Set(['/passwordUpdate/index@save']),
};

afterEach(cleanup);
beforeEach(() => {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    value: vi.fn(() => ({
      addEventListener: vi.fn(),
      addListener: vi.fn(),
      matches: false,
      removeEventListener: vi.fn(),
      removeListener: vi.fn(),
    })),
  });
});

function renderPage(
  api: PasswordPageApi,
  onUpdated = vi.fn(),
  allowedActions = access.allowedActions,
) {
  render(
    <App>
      <DashboardAccessProvider value={{ ...access, allowedActions }}>
        <PasswordPage api={api} onUpdated={onUpdated} />
      </DashboardAccessProvider>
    </App>,
  );
  return { onUpdated };
}

function api(overrides: Partial<PasswordPageApi> = {}): PasswordPageApi {
  return {
    update: vi.fn(() => Promise.resolve()),
    ...overrides,
  };
}

function fillForm(values = {
  oldPassword: 'old123',
  newPassword: 'new456',
  againNewPassword: 'new456',
}) {
  fireEvent.change(screen.getByLabelText('旧密码'), {
    target: { value: values.oldPassword },
  });
  fireEvent.change(screen.getByLabelText('新密码'), {
    target: { value: values.newPassword },
  });
  fireEvent.change(screen.getByLabelText('确认新密码'), {
    target: { value: values.againNewPassword },
  });
}

describe('PasswordPage', () => {
  it('renders password inputs and keeps save when no legacy action contract exists', () => {
    renderPage(api(), vi.fn(), new Set());

    expect(screen.getByLabelText('旧密码').getAttribute('type')).toBe('password');
    expect(screen.getByLabelText('新密码').getAttribute('type')).toBe('password');
    expect(screen.getByLabelText('确认新密码').getAttribute('type')).toBe('password');
    expect(screen.getByRole('button', { name: /保\s*存/ })).not.toBeNull();
  });

  it('rejects non-alphanumeric and mismatched passwords before submission', async () => {
    const client = api();
    renderPage(client);

    fillForm({
      oldPassword: 'old-123',
      newPassword: 'new456',
      againNewPassword: 'different7',
    });
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));

    expect(await screen.findByText('密码只能包含字母和数字')).not.toBeNull();
    expect(await screen.findByText('两次输入的新密码不一致')).not.toBeNull();
    expect(client.update).not.toHaveBeenCalled();
  });

  it('submits the audited fields and hands off logout after success', async () => {
    const client = api();
    const { onUpdated } = renderPage(client);

    fillForm();
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));

    await waitFor(() => expect(client.update).toHaveBeenCalledWith({
      oldPassword: 'old123',
      newPassword: 'new456',
      againNewPassword: 'new456',
    }));
    expect(onUpdated).toHaveBeenCalledOnce();
  });

  it('shows a business error and keeps the user on the form', async () => {
    const client = api({
      update: vi.fn(() => Promise.reject(new Error('旧密码错误'))),
    });
    const { onUpdated } = renderPage(client);

    fillForm();
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));

    expect((await screen.findByRole('alert')).textContent).toContain('旧密码错误');
    expect(onUpdated).not.toHaveBeenCalled();
  });
});
