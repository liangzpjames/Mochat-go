import type { ReactNode } from 'react';

type Props = { loading?: boolean; error?: unknown; empty?: boolean; emptyContent?: ReactNode; forbidden?: boolean; limitations?: string[]; onRetry?: () => void; children: ReactNode };

export function Phase35DataState({ loading, error, empty, emptyContent, forbidden, limitations, onRetry, children }: Props) {
  if (loading) return <p role="status" aria-live="polite">正在加载…</p>;
  if (forbidden) return <p role="alert">暂无权限查看此页面</p>;
  if (error) return <div role="alert"><p>数据加载失败，请稍后重试</p>{onRetry && <button type="button" onClick={onRetry}>重新加载</button>}</div>;
  if (empty) return <>{emptyContent ?? <p role="status">暂无数据</p>}</>;
  return <>{limitations?.map((item) => <p role="status" key={item}>数据限制：{item}</p>)}{children}</>;
}
