import type { Session } from '@mochat/auth';
import { ApiError } from '@mochat/api-client';
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { LoginPage } from './login-page';
import type { DashboardAuthPending } from './auth-api';

const session: Session = {
  token: 'jwt',
  userId: '',
  expiresAt: Date.now() + 60_000,
};

afterEach(cleanup);

function renderLogin(overrides: Partial<Parameters<typeof LoginPage>[0]> = {}) {
  const props: Parameters<typeof LoginPage>[0] = {
    authenticate: vi.fn(() => Promise.resolve(session)),
    navigate: vi.fn(),
    returnTo: null,
    setSession: vi.fn(),
    ...overrides,
  };
  render(<LoginPage {...props} />);
  return props;
}

function submitCredentials() {
  fireEvent.change(screen.getByLabelText('手机号'), {
    target: { value: '13800138000' },
  });
  fireEvent.change(screen.getByLabelText('密码'), {
    target: { value: 'secret' },
  });
  fireEvent.click(screen.getByRole('button', { name: '登录' }));
}

describe('LoginPage', () => {
  it('renders the branded Ant Design login surface', () => {
    renderLogin();

    expect(screen.getByRole('main').className).toContain('login-page');
    expect(screen.getByText('企业管理后台')).toBeTruthy();
    expect(screen.getByLabelText('手机号').className).toContain('ant-input');
    expect(screen.getByRole('button', { name: '登录' }).className).toContain(
      'ant-btn',
    );
  });

  it('validates empty fields before sending a request', async () => {
    const props = renderLogin();

    fireEvent.click(screen.getByRole('button', { name: '登录' }));

    expect(await screen.findByText('请输入手机号')).not.toBeNull();
    expect(screen.getByText('请输入密码')).not.toBeNull();
    expect(props.authenticate).not.toHaveBeenCalled();
  });

  it('shows a server validation error without exposing the password', async () => {
    renderLogin({
      authenticate: vi.fn(() => Promise.reject(
        new ApiError('validation', '手机号或密码错误'),
      )),
    });

    submitCredentials();

    expect((await screen.findByRole('alert')).textContent).toContain('手机号或密码错误');
    expect(screen.queryByText('secret')).toBeNull();
  });

  it('restores a safe same-origin deep link after login', async () => {
    const props = renderLogin({
      returnTo: '/workContact/index?tab=active#top',
    });

    submitCredentials();

    await waitFor(() => expect(props.setSession).toHaveBeenCalledWith(session));
    expect(props.navigate).toHaveBeenCalledWith('/workContact/index?tab=active#top');
  });

  it('falls back to data overview for a login-page return target', async () => {
    const props = renderLogin({ returnTo: '/login?next=/data/customer' });

    submitCredentials();

    await waitFor(() => expect(props.navigate).toHaveBeenCalledWith('/index'));
  });

  it('rejects an external return target and opens data overview', async () => {
    const props = renderLogin({ returnTo: 'https://evil.example/steal' });

    submitCredentials();

    await waitFor(() => expect(props.navigate).toHaveBeenCalledWith('/index'));
  });

  it('rejects a backslash network-path return target', async () => {
    const props = renderLogin({ returnTo: '/\\evil.example/steal' });

    submitCredentials();

    await waitFor(() => expect(props.navigate).toHaveBeenCalledWith('/index'));
  });

  it('disables duplicate submission while authentication is pending', async () => {
    let resolveLogin: ((value: Session) => void) | undefined;
    const pending = new Promise<Session>((resolve) => {
      resolveLogin = resolve;
    });
    const props = renderLogin({
      authenticate: vi.fn(() => pending),
    });

    submitCredentials();
    const button = screen.getByRole('button', { name: '登录中…' });
    expect(button.hasAttribute('disabled')).toBe(true);
    await waitFor(() => expect(props.authenticate).toHaveBeenCalledOnce());
    fireEvent.click(button);
    expect(props.authenticate).toHaveBeenCalledOnce();
    resolveLogin?.(session);
    await waitFor(() => expect(props.navigate).toHaveBeenCalled());
  });

  it('keeps the first enrollment challenge out of the session until TOTP and password setup finish', async () => {
    const enrollment: DashboardAuthPending = {
      kind: 'mfa-enrollment',
      enrollmentToken: 'enrollment-token',
      enrollmentSecret: 'one-time-secret',
      otpAuthURL: 'otpauth://dashboard/test',
      expiresAt: Date.now() + 300_000,
    };
    const passwordChange: DashboardAuthPending = {
      kind: 'password-change',
      passwordChangeToken: 'password-change-token',
      expiresAt: Date.now() + 600_000,
    };
    const completeMFA = vi.fn()
      .mockResolvedValueOnce(passwordChange)
      .mockResolvedValueOnce(session);
    const props = renderLogin({
      authenticate: vi.fn(() => Promise.resolve(enrollment)),
      completeMFA,
    });

    submitCredentials();
    await waitFor(() => expect(screen.getByText('one-time-secret')).not.toBeNull());
    expect(props.setSession).not.toHaveBeenCalled();
    expect(props.navigate).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText('Verification code'), { target: { value: '123456' } });
    fireEvent.click(screen.getByRole('button', { name: 'Verify enrollment' }));
    await waitFor(() => expect(completeMFA).toHaveBeenCalledWith({
      challengeToken: 'enrollment-token',
      code: '123456',
    }));
    expect(props.setSession).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'rotated-password' } });
    fireEvent.change(screen.getByLabelText('Confirm password'), { target: { value: 'rotated-password' } });
    fireEvent.click(screen.getByRole('button', { name: 'Change password' }));
    await waitFor(() => expect(completeMFA).toHaveBeenCalledWith({
      passwordChangeToken: 'password-change-token',
      newPassword: 'rotated-password',
    }));
    await waitFor(() => expect(props.setSession).toHaveBeenCalledWith(session));
    expect(props.navigate).toHaveBeenCalledWith('/index');
  });

  it('completes a normal MFA challenge and only then enters the Dashboard', async () => {
    const challenge: DashboardAuthPending = {
      kind: 'mfa',
      challengeToken: 'login-challenge-token',
      expiresAt: Date.now() + 300_000,
    };
    const completeMFA = vi.fn(() => Promise.resolve(session));
    const props = renderLogin({
      authenticate: vi.fn(() => Promise.resolve(challenge)),
      completeMFA,
    });

    submitCredentials();
    await waitFor(() => expect(screen.getByLabelText('Verification code')).not.toBeNull());
    expect(props.setSession).not.toHaveBeenCalled();
    fireEvent.change(screen.getByLabelText('Verification code'), { target: { value: '654321' } });
    fireEvent.click(screen.getByRole('button', { name: 'Verify MFA' }));

    await waitFor(() => expect(completeMFA).toHaveBeenCalledWith({
      challengeToken: 'login-challenge-token',
      code: '654321',
    }));
    await waitFor(() => expect(props.setSession).toHaveBeenCalledWith(session));
  });

  it('keeps an MFA error retryable without creating a session', async () => {
    const challenge: DashboardAuthPending = {
      kind: 'mfa',
      challengeToken: 'retryable-challenge-token',
      expiresAt: Date.now() + 300_000,
    };
    const completeMFA = vi.fn()
      .mockRejectedValueOnce(new ApiError('unauthorized', 'Invalid MFA code', {
        status: 401,
        machineCode: 'MFA_CHALLENGE_INVALID',
      }))
      .mockResolvedValueOnce(session);
    const props = renderLogin({
      authenticate: vi.fn(() => Promise.resolve(challenge)),
      completeMFA,
    });

    submitCredentials();
    await waitFor(() => expect(screen.getByLabelText('Verification code')).not.toBeNull());
    fireEvent.change(screen.getByLabelText('Verification code'), { target: { value: '000000' } });
    fireEvent.click(screen.getByRole('button', { name: 'Verify MFA' }));
    expect((await screen.findByRole('alert')).textContent).toContain('Invalid MFA code');
    expect(props.setSession).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText('Verification code'), { target: { value: '123456' } });
    fireEvent.click(screen.getByRole('button', { name: 'Verify MFA' }));
    await waitFor(() => expect(props.setSession).toHaveBeenCalledWith(session));
  });

  it('keeps the enrollment screen usable at a 390px viewport', async () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 });
    const enrollment: DashboardAuthPending = {
      kind: 'mfa-enrollment',
      enrollmentToken: 'narrow-enrollment-token',
      enrollmentSecret: 'narrow-secret',
      otpAuthURL: 'otpauth://dashboard/narrow',
      expiresAt: Date.now() + 300_000,
    };
    renderLogin({ authenticate: vi.fn(() => Promise.resolve(enrollment)) });
    submitCredentials();
    await waitFor(() => expect(screen.getByRole('main').className).toContain('login-page'));
    expect(screen.getByLabelText('Verification code')).not.toBeNull();
    expect(screen.getByRole('button', { name: 'Verify enrollment' })).not.toBeNull();
  });
});
