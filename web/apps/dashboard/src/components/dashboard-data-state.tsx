import type { ReactNode } from 'react';

export type DashboardDataStateKind = 'loading' | 'empty' | 'error' | 'forbidden' | 'provider-unavailable';

const defaultCopy: Record<DashboardDataStateKind, { title: string; description: string }> = {
  loading: { title: '正在加载', description: '请稍候，正在读取最新数据。' },
  empty: { title: '暂无数据', description: '当前条件下还没有可展示的记录。' },
  error: { title: '加载失败', description: '服务暂时不可用，请稍后重试。' },
  forbidden: { title: '暂无权限查看此内容', description: '请联系管理员确认你的数据权限。' },
  'provider-unavailable': { title: '服务能力尚未开通', description: '完成相关配置后即可使用此功能。' },
};

export function DashboardDataState({
  state,
  title,
  description,
  action,
  className,
}: {
  state: DashboardDataStateKind;
  title?: ReactNode;
  description?: ReactNode;
  action?: ReactNode;
  className?: string;
}) {
  const copy = defaultCopy[state];
  const role = state === 'error' || state === 'forbidden' ? 'alert' : 'status';
  return (
    <section
      aria-busy={state === 'loading'}
      className={['dashboard-data-state', `dashboard-data-state--${state}`, className].filter(Boolean).join(' ')}
      role={role}
    >
      <h2>{title ?? copy.title}</h2>
      <p>{description ?? copy.description}</p>
      {action !== undefined && <div className="dashboard-data-state__action">{action}</div>}
    </section>
  );
}
