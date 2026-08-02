import { zodResolver } from '@hookform/resolvers/zod';
import type { Session } from '@mochat/auth';
import { Button, Card, Input, Typography } from 'antd';
import { useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { useNavigate, useSearchParams } from 'react-router';
import { z } from 'zod';

import type { LoginInput } from './auth-api';

const loginSchema = z.object({
  phone: z.string().trim().min(1, '请输入手机号'),
  password: z.string().min(1, '请输入密码'),
});

function safeReturnTo(returnTo: string | null): string {
  void returnTo;
  return '/index';
}

export type LoginPageProps = {
  authenticate: (input: LoginInput) => Promise<Session>;
  navigate: (path: string) => void;
  returnTo: string | null;
  setSession: (session: Session) => void;
};

export function LoginPage({
  authenticate,
  navigate,
  returnTo,
  setSession,
}: LoginPageProps) {
  const [serverError, setServerError] = useState<string | null>(null);
  const {
    formState: { errors, isSubmitting },
    handleSubmit,
    control,
  } = useForm<LoginInput>({
    defaultValues: { phone: '', password: '' },
    resolver: zodResolver(loginSchema),
  });

  const submit = handleSubmit(async (input) => {
    setServerError(null);
    try {
      const session = await authenticate(input);
      setSession(session);
      navigate(safeReturnTo(returnTo));
    } catch (error) {
      setServerError(error instanceof Error ? error.message : '登录失败');
    }
  });

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
  'authenticate' | 'setSession'
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
