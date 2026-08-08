import { Popconfirm } from 'antd';
import { cloneElement } from 'react';
import type { ReactElement } from 'react';

export function ConfirmAction({
  title,
  description,
  onConfirm,
  children,
}: {
  title: string;
  description?: string;
  onConfirm: () => void;
  children: ReactElement<{ onClick?: unknown }>;
}) {
  const confirmationTrigger = cloneElement(children, { onClick: undefined });

  return (
    <Popconfirm
      title={title}
      description={description}
      okText="确认"
      cancelText="取消"
      okButtonProps={{ 'aria-label': '确认', danger: true }}
      cancelButtonProps={{ 'aria-label': '取消' }}
      onConfirm={onConfirm}
    >
      {confirmationTrigger}
    </Popconfirm>
  );
}
