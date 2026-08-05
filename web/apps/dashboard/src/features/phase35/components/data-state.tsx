import type { ReactNode } from 'react';
export function Phase35DataState({ loading, error, empty, forbidden, limitations, children }: { loading?: boolean; error?: unknown; empty?: boolean; forbidden?: boolean; limitations?: string[]; children: ReactNode }) {
  if (loading) return <p role="status" aria-live="polite">正在加载…</p>;
  if (forbidden) return <p role="alert">暂无权限查看此页面</p>;
  if (error) return <p role="alert">数据加载失败，请稍后重试</p>;
  if (empty) return <p role="status">暂无数据</p>;
  return <>{limitations?.map((item) => <p role="status" key={item}>数据限制：{item}</p>)}{children}</>;
}
