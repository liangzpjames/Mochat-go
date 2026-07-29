import { Alert, Button, Card, Form, Input } from 'antd';
import { useState } from 'react';

import { useDashboardAccess } from '../../app/access-context';
import type { PasswordUpdateInput } from './password-api';

export type PasswordPageApi = {
  update(input: PasswordUpdateInput): Promise<void>;
};

const alphaNumeric = /^[A-Za-z0-9]+$/;

export function PasswordPage({
  api,
  onUpdated,
}: {
  api: PasswordPageApi;
  onUpdated: () => void;
}) {
  const access = useDashboardAccess();
  const [form] = Form.useForm<PasswordUpdateInput>();
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const canSave = access.allowedActions.has('/passwordUpdate/index@save');

  const submit = async (values: PasswordUpdateInput) => {
    setSubmitting(true);
    setError(null);
    try {
      await api.update(values);
      onUpdated();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '密码修改失败');
    } finally {
      setSubmitting(false);
    }
  };

  const passwordRules = [
    { required: true, message: '请输入密码' },
    {
      pattern: alphaNumeric,
      message: '密码只能包含字母和数字',
    },
  ];

  return (
    <Card title="修改密码">
      <p>为当前登录账号设置新密码。修改成功后需要重新登录。</p>
      {error !== null && <Alert title={error} role="alert" type="error" />}
      <Form
        form={form}
        layout="vertical"
        onFinish={(values) => void submit(values)}
        style={{ maxWidth: 480 }}
      >
        <Form.Item label="旧密码" name="oldPassword" rules={passwordRules}>
          <Input.Password autoComplete="current-password" />
        </Form.Item>
        <Form.Item label="新密码" name="newPassword" rules={passwordRules}>
          <Input.Password autoComplete="new-password" />
        </Form.Item>
        <Form.Item
          dependencies={['newPassword']}
          label="确认新密码"
          name="againNewPassword"
          rules={[
            ...passwordRules,
            ({ getFieldValue }) => ({
              validator(_, value: string) {
                return value === getFieldValue('newPassword')
                  ? Promise.resolve()
                  : Promise.reject(new Error('两次输入的新密码不一致'));
              },
            }),
          ]}
        >
          <Input.Password autoComplete="new-password" />
        </Form.Item>
        {canSave && (
          <Button htmlType="submit" loading={submitting} type="primary">
            保存
          </Button>
        )}
      </Form>
    </Card>
  );
}
