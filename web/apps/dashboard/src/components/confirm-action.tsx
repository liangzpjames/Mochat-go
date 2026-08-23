import { Popconfirm } from 'antd';
import { cloneElement } from 'react';
import type { ReactElement } from 'react';

export function ConfirmAction({
  title,
  description,
  onConfirm,
  danger = true,
  children,
}: {
  title: string;
  description?: string | undefined;
  onConfirm: () => void | Promise<unknown>;
  danger?: boolean;
  children: ReactElement<{ onClick?: unknown }>;
}) {
  const confirmationTrigger = cloneElement(children, { onClick: undefined });

  return (
    <Popconfirm
      title={title}
      description={description}
      okText="确认"
      cancelText="取消"
      okButtonProps={{ 'aria-label': '确认', danger }}
      cancelButtonProps={{ 'aria-label': '取消' }}
      onConfirm={onConfirm as () => void}
    >
      {confirmationTrigger}
    </Popconfirm>
  );
}
