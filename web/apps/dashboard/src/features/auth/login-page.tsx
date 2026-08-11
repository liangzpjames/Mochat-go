import { zodResolver } from '@hookform/resolvers/zod';
import type { Session } from '@mochat/auth';
import { Button, Card, Input, Typography } from 'antd';
import { useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { useNavigate, useSearchParams } from 'react-router';
import { z } from 'zod';

import type {
  DashboardAuthPending,
  DashboardAuthResult,
  LoginInput,
  MFAInput,
} from './auth-api';

const loginSchema = z.object({
  phone: z.string().trim().min(1, '请输入手机号'),
  password: z.string().min(1, '请输入密码'),
});

function safeReturnTo(returnTo: string | null): string {
  if (
    returnTo !== null &&
    returnTo.startsWith('/') &&
    !returnTo.startsWith('//') &&
    !returnTo.startsWith('/\\') &&
    !returnTo.startsWith('/login')
  ) {
    return returnTo;
  }
  return '/index';
}

export type LoginPageProps = {
  authenticate: (input: LoginInput) => Promise<DashboardAuthResult>;
  completeMFA?: (input: MFAInput) => Promise<DashboardAuthResult>;
  navigate: (path: string) => void;
  returnTo: string | null;
  setSession: (session: Session) => void;
};

function isSession(result: DashboardAuthResult): result is Session {
  return 'token' in result;
}

function pendingTitle(pending: DashboardAuthPending): string {
  switch (pending.kind) {
    case 'mfa-enrollment':
      return 'Set up MFA';
    case 'mfa':
      return 'Verify MFA';
    case 'password-change':
      return 'Change password';
  }
}

export function LoginPage({
  authenticate,
  completeMFA,
  navigate,
  returnTo,
  setSession,
}: LoginPageProps) {
  const [serverError, setServerError] = useState<string | null>(null);
  const [pending, setPending] = useState<DashboardAuthPending | null>(null);
  const [code, setCode] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [passwordConfirmation, setPasswordConfirmation] = useState('');
  const [pendingSubmitting, setPendingSubmitting] = useState(false);
  const {
    formState: { errors, isSubmitting },
    handleSubmit,
    control,
  } = useForm<LoginInput>({
    defaultValues: { phone: '', password: '' },
    resolver: zodResolver(loginSchema),
  });

  const finishResult = (result: DashboardAuthResult) => {
    if (isSession(result)) {
      setSession(result);
      navigate(safeReturnTo(returnTo));
      return;
    }
    setPending(result);
    setServerError(null);
    setCode('');
    setNewPassword('');
    setPasswordConfirmation('');
  };

  const submit = handleSubmit(async (input) => {
    setServerError(null);
    try {
      finishResult(await authenticate(input));
    } catch (error) {
      setServerError(error instanceof Error ? error.message : '登录失败');
    }
  });

  const submitPending = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (pending === null) return;
    setServerError(null);
    if (completeMFA === undefined) {
      setServerError('Dashboard authentication is not configured');
      return;
    }
    let input: MFAInput;
    if (pending.kind === 'password-change') {
      if (newPassword.trim() === '' || newPassword !== passwordConfirmation) {
        setServerError('Passwords do not match');
        return;
      }
      input = {
        passwordChangeToken: pending.passwordChangeToken,
        newPassword,
      };
    } else {
      if (code.trim() === '') {
        setServerError('Enter the verification code');
        return;
      }
      input = {
        challengeToken: pending.kind === 'mfa-enrollment'
          ? pending.enrollmentToken
          : pending.challengeToken,
        code: code.trim(),
      };
    }
    setPendingSubmitting(true);
    try {
      finishResult(await completeMFA(input));
    } catch (error) {
      setServerError(error instanceof Error ? error.message : 'Authentication failed');
    } finally {
      setPendingSubmitting(false);
    }
  };

  if (pending !== null) {
    return (
      <main className="login-page dashboard-auth-pending">
        <section className="login-brand" aria-label="MoChat product introduction">
          <div className="login-brand-mark" aria-hidden="true">M</div>
          <p className="login-brand-name">MoChat</p>
          <Typography.Title level={1}>Dashboard access</Typography.Title>
          <Typography.Paragraph>
            Complete every required authentication step before a session is created.
          </Typography.Paragraph>
        </section>
        <Card className="login-card" variant="borderless">
          <div className="login-card-heading">
            <Typography.Text className="login-eyebrow">Dashboard identity</Typography.Text>
            <Typography.Title level={2}>{pendingTitle(pending)}</Typography.Title>
          </div>
          <form className="login-form" onSubmit={(event) => void submitPending(event)}>
            {pending.kind === 'mfa-enrollment' && (
              <>
                <Typography.Paragraph>
                  Save this one-time enrollment secret and add it to your authenticator.
                </Typography.Paragraph>
                <div className="login-enrollment-secret" aria-label="One-time enrollment secret">
                  {pending.enrollmentSecret}
                </div>
                <Typography.Text type="secondary">{pending.otpAuthURL}</Typography.Text>
              </>
            )}
            {pending.kind !== 'password-change' && (
              <>
                <label htmlFor="dashboard-mfa-code">Verification code</label>
                <Input
                  autoComplete="one-time-code"
                  id="dashboard-mfa-code"
                  inputMode="numeric"
                  value={code}
                  onChange={(event) => setCode(event.target.value)}
                />
              </>
            )}
            {pending.kind === 'password-change' && (
              <>
                <label htmlFor="dashboard-new-password">New password</label>
                <Input.Password
                  autoComplete="new-password"
                  id="dashboard-new-password"
                  value={newPassword}
                  onChange={(event) => setNewPassword(event.target.value)}
                />
                <label htmlFor="dashboard-confirm-password">Confirm password</label>
                <Input.Password
                  autoComplete="new-password"
                  id="dashboard-confirm-password"
                  value={passwordConfirmation}
                  onChange={(event) => setPasswordConfirmation(event.target.value)}
                />
              </>
            )}
            {serverError !== null && (
              <div className="login-server-error" role="alert">
                {serverError}
              </div>
            )}
            <Button
              aria-label={pendingSubmitting ? 'Working…' : pending.kind === 'mfa-enrollment' ? 'Verify enrollment' : pending.kind === 'mfa' ? 'Verify MFA' : 'Change password'}
              block
              disabled={pendingSubmitting}
              htmlType="submit"
              loading={pendingSubmitting}
              size="large"
              type="primary"
            >
              {pendingSubmitting ? 'Working…' : pending.kind === 'mfa-enrollment' ? 'Verify enrollment' : pending.kind === 'mfa' ? 'Verify MFA' : 'Change password'}
            </Button>
          </form>
        </Card>
      </main>
    );
  }

  return (
    <main className="login-page">
      <section className="login-brand" aria-label="MoChat 产品介绍">
        <div className="login-brand-mark" aria-hidden="true">M</div>
        <p className="login-brand-name">MoChat</p>
        <Typography.Title level={1}>企业客户运营平台</Typography.Title>
        <Typography.Paragraph>
          统一管理企业、客户与运营能力，安全进入你的工作空间。
        </Typography.Paragraph>
      </section>
      <Card className="login-card" variant="borderless">
        <div className="login-card-heading">
          <Typography.Text className="login-eyebrow">
            企业管理后台
          </Typography.Text>
          <Typography.Title level={2}>登录</Typography.Title>
          <Typography.Paragraph>
            使用管理员手机号和密码继续
          </Typography.Paragraph>
        </div>
        <form
          className="login-form"
          onSubmit={(event) => void submit(event)}
        >
          <label htmlFor="login-phone">手机号</label>
          <Controller
            control={control}
            name="phone"
            render={({ field }) => (
              <Input
                {...field}
                autoComplete="username"
                id="login-phone"
                placeholder="请输入手机号"
                size="large"
              />
            )}
          />
          {errors.phone?.message && (
            <div className="login-field-error">{errors.phone.message}</div>
          )}
          <label htmlFor="login-password">密码</label>
          <Controller
            control={control}
            name="password"
            render={({ field }) => (
              <Input.Password
                {...field}
                autoComplete="current-password"
                id="login-password"
                placeholder="请输入密码"
                size="large"
              />
            )}
          />
          {errors.password?.message && (
            <div className="login-field-error">{errors.password.message}</div>
          )}
          {serverError !== null && (
            <div className="login-server-error" role="alert">
              {serverError}
            </div>
          )}
          <Button
            aria-label={isSubmitting ? '登录中…' : '登录'}
            block
            disabled={isSubmitting}
            htmlType="submit"
            loading={isSubmitting}
            size="large"
            type="primary"
          >
            {isSubmitting ? '登录中…' : '登录'}
          </Button>
        </form>
      </Card>
    </main>
  );
}

export type RoutedLoginPageProps = Pick<
  LoginPageProps,
  'authenticate' | 'completeMFA' | 'setSession'
>;

export function RoutedLoginPage(props: RoutedLoginPageProps) {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  return (
    <LoginPage
      {...props}
      navigate={(path) => void navigate(path, { replace: true })}
      returnTo={searchParams.get('returnTo')}
    />
  );
}
