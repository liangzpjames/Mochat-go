import type { ReactNode } from 'react';

export type PageStateKind =
  | 'loading'
  | 'empty'
  | 'forbidden'
  | 'not-found'
  | 'conflict'
  | 'invalid-transition'
  | 'error';

const stateCopy: Record<PageStateKind, { title: string; description: string }> = {
  loading: { title: '正在加载', description: '请稍候，正在读取最新数据。' },
  empty: { title: '暂无数据', description: '当前条件下还没有可展示的记录。' },
  forbidden: { title: '暂无权限查看此内容', description: '请联系管理员确认你的数据权限。' },
  'not-found': { title: '内容不存在', description: '记录可能已被删除，或你没有查看权限。' },
  conflict: { title: '数据已被其他操作更新', description: '请重新加载后再继续操作。' },
  'invalid-transition': { title: '当前状态不允许此操作', description: '请刷新页面确认最新状态。' },
  error: { title: '加载失败', description: '服务暂时不可用，请稍后重试。' },
};

export function PageState({
  state,
  onRetry,
  children,
}: {
  state: PageStateKind;
  onRetry?: () => void;
  children?: ReactNode;
}) {
  if (children !== undefined) return <>{children}</>;

  const copy = stateCopy[state];
  return (
    <section
      aria-busy={state === 'loading'}
      aria-live="polite"
      className={`page-state page-state-${state}`}
      role="status"
    >
      <h2>{copy.title}</h2>
      <p>{copy.description}</p>
      {onRetry !== undefined && state !== 'loading' && (
        <button className="page-state-retry" type="button" onClick={onRetry}>重新加载</button>
      )}
    </section>
  );
}
