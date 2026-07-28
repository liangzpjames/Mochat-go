import { zodResolver } from '@hookform/resolvers/zod';
import type { Session } from '@mochat/auth';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { useNavigate, useSearchParams } from 'react-router';
import { z } from 'zod';

import type { LoginInput } from './auth-api';

const loginSchema = z.object({
  phone: z.string().trim().min(1, '请输入手机号'),
  password: z.string().min(1, '请输入密码'),
});

function safeReturnTo(returnTo: string | null): string {
  if (returnTo === null || returnTo.includes('\\')) {
    return '/';
  }
  try {
    const base = new URL('https://dashboard.local/');
    const target = new URL(returnTo, base);
    if (target.origin !== base.origin || !returnTo.startsWith('/')) {
      return '/';
    }
    return `${target.pathname}${target.search}${target.hash}`;
  } catch {
    return '/';
  }
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
    register,
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
    <main>
      <h1>登录</h1>
      <form onSubmit={(event) => void submit(event)}>
        <label>
          手机号
          <input autoComplete="username" {...register('phone')} />
        </label>
        {errors.phone?.message && <div>{errors.phone.message}</div>}
        <label>
          密码
          <input
            autoComplete="current-password"
            type="password"
            {...register('password')}
          />
        </label>
        {errors.password?.message && <div>{errors.password.message}</div>}
        {serverError !== null && <div role="alert">{serverError}</div>}
        <button disabled={isSubmitting} type="submit">
          {isSubmitting ? '登录中…' : '登录'}
        </button>
      </form>
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
