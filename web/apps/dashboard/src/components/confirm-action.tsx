import { Popconfirm } from 'antd';
import type { ReactNode } from 'react';

export function ConfirmAction({
  title,
  description,
  onConfirm,
  children,
}: {
  title: string;
  description?: string;
  onConfirm: () => void;
  children: ReactNode;
}) {
  return (
    <Popconfirm title={title} description={description} okText="确认" cancelText="取消" okButtonProps={{ danger: true }} onConfirm={onConfirm}>
      {children}
    </Popconfirm>
  );
}
