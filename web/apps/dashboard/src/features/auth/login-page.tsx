import { zodResolver } from '@hookform/resolvers/zod';
import { ApiError } from '@mochat/api-client';
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
      return '设置多因素认证';
    case 'mfa':
      return '验证多因素认证';
    case 'password-change':
      return '修改登录密码';
  }
}

function dashboardAuthErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    switch (error.machineCode) {
      case 'INVALID_CREDENTIALS':
        return '手机号或密码错误';
      case 'MFA_CHALLENGE_INVALID':
        return '验证码无效或已过期，请重试';
      case 'PASSWORD_CHANGE_INVALID':
        return '密码修改信息无效，请重试';
      case 'SESSION_INVALID':
        return '登录状态已失效，请重新登录';
      case 'TENANT_ACCESS_DENIED':
        return '当前账号暂无 Dashboard 访问权限';
      case 'AUTH_UNAVAILABLE':
        return '认证服务暂不可用，请稍后重试';
      case 'INVALID_REQUEST':
        return '请求格式有误，请重试';
      default:
        if (error.kind === 'network') return '网络异常，请重试';
        if (/[一-鿿]/u.test(error.message)) return error.message;
        return '认证失败，请重试';
    }
  }
  if (error instanceof Error && /[一-鿿]/u.test(error.message)) {
    return error.message;
  }
  return '认证失败，请重试';
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
      setServerError(dashboardAuthErrorMessage(error));
    }
  });

  const submitPending = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (pending === null) return;
    setServerError(null);
    if (completeMFA === undefined) {
      setServerError('认证服务未配置');
      return;
    }
    let input: MFAInput;
    if (pending.kind === 'password-change') {
      if (newPassword.trim() === '' || newPassword !== passwordConfirmation) {
        setServerError('两次输入的密码不一致');
        return;
      }
      input = {
        passwordChangeToken: pending.passwordChangeToken,
        newPassword,
      };
    } else {
      if (code.trim() === '') {
        setServerError('请输入验证码');
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
      setServerError(dashboardAuthErrorMessage(error));
    } finally {
      setPendingSubmitting(false);
    }
  };

  if (pending !== null) {
    return (
      <main className="login-page dashboard-auth-pending">
        <section className="login-brand" aria-label="MoChat 产品介绍">
          <div className="login-brand-mark" aria-hidden="true">M</div>
          <p className="login-brand-name">MoChat</p>
          <Typography.Title level={1}>Dashboard 访问</Typography.Title>
          <Typography.Paragraph>
            请完成全部认证步骤，验证通过后才会创建登录会话。
          </Typography.Paragraph>
        </section>
        <Card className="login-card" variant="borderless">
          <div className="login-card-heading">
            <Typography.Text className="login-eyebrow">Dashboard 身份</Typography.Text>
            <Typography.Title level={2}>{pendingTitle(pending)}</Typography.Title>
          </div>
          <form className="login-form" onSubmit={(event) => void submitPending(event)}>
            {pending.kind === 'mfa-enrollment' && (
              <>
                <Typography.Paragraph>
                  请保存本次一次性绑定密钥，并将其添加到验证器应用。
                </Typography.Paragraph>
                <div className="login-enrollment-secret" aria-label="一次性绑定密钥">
                  {pending.enrollmentSecret}
                </div>
                <Typography.Text type="secondary">{pending.otpAuthURL}</Typography.Text>
              </>
            )}
            {pending.kind !== 'password-change' && (
              <>
                <label htmlFor="dashboard-mfa-code">验证码</label>
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
                <label htmlFor="dashboard-new-password">新密码</label>
                <Input.Password
                  autoComplete="new-password"
                  id="dashboard-new-password"
                  value={newPassword}
                  onChange={(event) => setNewPassword(event.target.value)}
                />
                <label htmlFor="dashboard-confirm-password">确认密码</label>
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
              aria-label={pendingSubmitting ? '提交中…' : pending.kind === 'password-change' ? '修改密码' : '验证多因素认证'}
              block
              disabled={pendingSubmitting}
              htmlType="submit"
              loading={pendingSubmitting}
              size="large"
              type="primary"
            >
              {pendingSubmitting ? '提交中…' : pending.kind === 'password-change' ? '修改密码' : '验证多因素认证'}
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
