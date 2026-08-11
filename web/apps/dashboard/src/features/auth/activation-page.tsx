import { Button, Card, Input, Typography } from 'antd';
import { useState, type FormEvent } from 'react';
import { useNavigate, useSearchParams } from 'react-router';

export type ActivationInput = {
  activationToken: string;
  password: string;
};

export type ActivationPageProps = {
  activationToken: string | null;
  activate: (input: ActivationInput) => Promise<void>;
  navigate: (path: string) => void;
};

export function ActivationPage({ activationToken, activate, navigate }: ActivationPageProps) {
  const [password, setPassword] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

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
      navigate('/login');
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '激活失败，请重试');
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
          {error !== null && <div className="login-server-error" role="alert">{error}</div>}
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
      </Card>
    </main>
  );
}

export type RoutedActivationPageProps = Pick<ActivationPageProps, 'activate'>;

export function RoutedActivationPage({ activate }: RoutedActivationPageProps) {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  return (
    <ActivationPage
      activationToken={searchParams.get('token') ?? searchParams.get('activationToken')}
      activate={activate}
      navigate={(path) => void navigate(path, { replace: true })}
    />
  );
}
