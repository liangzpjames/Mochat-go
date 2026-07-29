import type { ReactNode } from 'react';

export interface DataPageProps {
  title: string;
  page: number;
  perPage: number;
  total: number;
  onPageChange: (page: number) => void;
  onPerPageChange: (perPage: number) => void;
  children: ReactNode;
  actions?: ReactNode;
}

export function DataPage({
  title,
  page,
  perPage,
  total,
  onPageChange,
  onPerPageChange,
  children,
  actions,
}: DataPageProps) {
  const pageCount = Math.max(1, Math.ceil(total / perPage));

  return (
    <section aria-labelledby="data-page-title">
      <header>
        <h1 id="data-page-title">{title}</h1>
        {actions}
      </header>
      {children}
      <nav aria-label="分页">
        <button type="button" disabled={page <= 1} onClick={() => onPageChange(page - 1)}>
          上一页
        </button>
        <span>
          第 {page} / {pageCount} 页
        </span>
        <button
          type="button"
          disabled={page >= pageCount}
          onClick={() => onPageChange(page + 1)}
        >
          下一页
        </button>
        <label>
          每页
          <select
            aria-label="每页条数"
            value={perPage}
            onChange={(event) => onPerPageChange(Number(event.currentTarget.value))}
          >
            {[10, 20, 50, 100].map((size) => (
              <option key={size} value={size}>
                {size}
              </option>
            ))}
          </select>
        </label>
      </nav>
    </section>
  );
}
