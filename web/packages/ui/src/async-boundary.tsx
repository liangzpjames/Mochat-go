import type { ReactNode } from 'react';

import { ErrorState } from './error-state';
import { LoadingState } from './loading-state';

export type PageState<T> =
  | { status: 'loading' }
  | { status: 'error'; message: string; retry: () => void }
  | { status: 'ready'; rows: T[] };

export interface AsyncBoundaryProps<T> {
  state: PageState<T>;
  children?: (rows: T[]) => ReactNode;
  emptyLabel?: string;
}

export function AsyncBoundary<T>({
  state,
  children,
  emptyLabel = '暂无数据',
}: AsyncBoundaryProps<T>) {
  if (state.status === 'loading') {
    return <LoadingState label="正在加载" />;
  }

  if (state.status === 'error') {
    return (
      <div>
        <ErrorState title="加载失败" message={state.message} />
        <button type="button" onClick={state.retry}>
          重试
        </button>
      </div>
    );
  }

  if (state.rows.length === 0) {
    return <p role="status">{emptyLabel}</p>;
  }

  return children ? children(state.rows) : null;
}
