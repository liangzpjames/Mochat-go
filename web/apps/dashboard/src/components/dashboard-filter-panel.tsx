import { Button } from 'antd';
import type { FormEvent, ReactNode } from 'react';

export function DashboardFilterPanel({
  children,
  onSubmit,
  onReset,
  submitLabel = '查询',
  resetLabel = '重置',
  extraActions,
  pending = false,
  className,
}: {
  children: ReactNode;
  onSubmit: () => void;
  onReset?: () => void;
  submitLabel?: string;
  resetLabel?: string;
  extraActions?: ReactNode;
  pending?: boolean;
  className?: string;
}) {
  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onSubmit();
  }

  return (
    <form
      aria-label="业务筛选"
      className={['dashboard-filter-panel', className].filter(Boolean).join(' ')}
      role="search"
      onSubmit={handleSubmit}
    >
      <div className="dashboard-filter-panel__fields">{children}</div>
      <div className="dashboard-filter-panel__actions">
        <Button aria-label={submitLabel} htmlType="submit" type="primary" loading={pending}>{submitLabel}</Button>
        {onReset !== undefined && <Button aria-label={resetLabel} htmlType="button" onClick={onReset}>{resetLabel}</Button>}
        {extraActions}
      </div>
    </form>
  );
}
