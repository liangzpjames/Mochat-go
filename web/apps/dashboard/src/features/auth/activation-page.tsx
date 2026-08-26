import { Button, Card, Input, Typography } from 'antd';
import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { useLocation, useNavigate } from 'react-router';

import type { DashboardActivationStatus } from './auth-api';

export type ActivationInput = {
  activationToken: string;
  password: string;
};

export type ActivationPageProps = {
  activationToken: string | null;
  activate: (input: ActivationInput) => Promise<void>;
  inspect: (token: string) => Promise<DashboardActivationStatus>;
  navigate: (path: string) => void;
};

export function ActivationPage({
  activationToken: initialToken,
  activate,
  inspect,
  navigate,
}: ActivationPageProps) {
  const [activationToken, setActivationToken] = useState(initialToken);
  const [password, setPassword] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [inspectionError, setInspectionError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [status, setStatus] = useState<DashboardActivationStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [completed, setCompleted] = useState(false);

  const inspectToken = useCallback(async () => {
    if (completed) return;
    if (activationToken === null || activationToken.trim() === '') {
      setStatus({
        status: 'invalid',
        tenantName: '',
        accountHint: '',
        expiresAt: 0,
        primaryAction: 'contact_admin',
      });
      return;
    }
    setLoading(true);
    setInspectionError(null);
    try {
      setStatus(await inspect(activationToken));
    } catch {
      setInspectionError('无法检查激活入口，请重试');
    } finally {
      setLoading(false);
    }
  }, [activationToken, completed, inspect]);

  useEffect(() => {
    void inspectToken();
  }, [inspectToken]);

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError(null);
    if (activationToken === null || activationToken.trim() === '') {
      setError('激活链接无效');
      return;
    }
    if (password.trim() === '' || password !== confirmation) {
      setError('两次输入的密码不一致');
      return;
    }
    setSubmitting(true);
    try {
      await activate({ activationToken, password });
      setActivationToken(null);
      setCompleted(true);
      setPassword('');
      setConfirmation('');
      setStatus({
        status: 'activated',
        tenantName: status?.tenantName ?? '',
        accountHint: status?.accountHint ?? '',
        expiresAt: 0,
        primaryAction: 'login',
      });
    } catch {
      setError('激活失败，请重试');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <main className="activation-page login-page">
      <section className="login-brand" aria-label="MoChat">
        <div className="login-brand-mark" aria-hidden="true">M</div>
        <p className="login-brand-name">MoChat</p>
        <Typography.Title level={1}>激活 Dashboard 访问权限</Typography.Title>
        <Typography.Paragraph>首次登录 Dashboard 前，请先设置新的登录密码。</Typography.Paragraph>
      </section>
      <Card className="login-card activation-card" variant="borderless">
        <div className="login-card-heading">
          <Typography.Text className="login-eyebrow">Dashboard 身份</Typography.Text>
          <Typography.Title level={2}>激活账号</Typography.Title>
        </div>
        {loading && <div aria-live="polite">正在检查激活入口…</div>}
        {inspectionError !== null && (
          <div className="login-server-error" role="alert">{inspectionError}</div>
        )}
        {!loading && inspectionError !== null && (
          <Button onClick={() => void inspectToken()}>重新检查</Button>
        )}
        {!loading && inspectionError === null && status?.status === 'valid' && (
          <form className="login-form" onSubmit={(event) => void submit(event)}>
            <label htmlFor="activation-password">新密码</label>
            <Input.Password
              autoComplete="new-password"
              id="activation-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
            <label htmlFor="activation-confirm-password">确认密码</label>
            <Input.Password
              autoComplete="new-password"
              id="activation-confirm-password"
              value={confirmation}
              onChange={(event) => setConfirmation(event.target.value)}
            />
            {error !== null && (
              <div className="login-server-error" role="alert">{error}</div>
            )}
            <Button
              aria-label="激活"
              block
              disabled={submitting}
              htmlType="submit"
              loading={submitting}
              size="large"
              type="primary"
            >
              激活
            </Button>
          </form>
        )}
        {!loading && inspectionError === null && status?.status === 'activated' && (
          <div className="activation-status">
            <Typography.Title level={3}>账号已激活</Typography.Title>
            <Button type="primary" onClick={() => navigate('/login')}>前往登录</Button>
          </div>
        )}
        {!loading && inspectionError === null && status !== null
          && ['expired', 'revoked', 'invalid'].includes(status.status) && (
          <div className="activation-status">
            <Typography.Title level={3}>
              {status.status === 'expired'
                ? '激活入口已过期'
                : status.status === 'revoked'
                  ? '激活入口已失效'
                  : '激活入口无效'}
            </Typography.Title>
            <p>联系平台管理员重新生成激活入口</p>
          </div>
        )}
      </Card>
    </main>
  );
}

export type RoutedActivationPageProps = Pick<ActivationPageProps, 'activate' | 'inspect'>;

export function RoutedActivationPage({ activate, inspect }: RoutedActivationPageProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const [activationToken] = useState(() => {
    const hash = new URLSearchParams(location.hash.replace(/^#/, ''));
    const query = new URLSearchParams(location.search);
    const token = hash.get('token') ?? query.get('token') ?? query.get('activationToken');
    window.history.replaceState({}, '', '/activate');
    return token;
  });
  return (
    <ActivationPage
      activationToken={activationToken}
      activate={activate}
      inspect={inspect}
      navigate={(path) => void navigate(path, { replace: true })}
    />
  );
}
