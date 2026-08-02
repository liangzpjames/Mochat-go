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

const session: Session = {
  token: 'jwt',
  userId: '',
  corpId: null,
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

  it('stores the session and opens data overview instead of a stale same-origin path', async () => {
    const props = renderLogin({
      returnTo: '/workContact/index?tab=active#top',
    });

    submitCredentials();

    await waitFor(() => expect(props.setSession).toHaveBeenCalledWith(session));
    expect(props.navigate).toHaveBeenCalledWith('/index');
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
});
