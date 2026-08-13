import * as React from "react";

type Props = {
  page: number;
  pageSize: number;
  total: number;
  onPageChange: (page: number) => void;
  onPageSizeChange?: (pageSize: number) => void;
  pageSizeOptions?: number[];
  ariaLabel?: string;
};

function visiblePages(page: number, totalPages: number) {
  if (totalPages <= 5) {
    return Array.from({ length: totalPages }, (_, index) => index + 1);
  }
  const start = Math.min(Math.max(1, page - 2), totalPages - 4);
  return Array.from({ length: 5 }, (_, index) => start + index);
}

export function DashboardPagination({
  page,
  pageSize,
  total,
  onPageChange,
  onPageSizeChange,
  pageSizeOptions = [10, 20, 50, 100],
  ariaLabel = "分页",
}: Props) {
  const totalPages = Math.max(1, Math.ceil(total / Math.max(1, pageSize)));
  const currentPage = Math.min(Math.max(1, page), totalPages);
  const pages = visiblePages(currentPage, totalPages);

  return (
    <nav className="dashboard-pagination" aria-label={ariaLabel}>
      <div className="dashboard-pagination-summary">
        <span>共 {total} 条</span>
        <span>第 {currentPage}/{totalPages} 页</span>
        {onPageSizeChange ? (
          <label>
            <span>每页</span>
            <select
              aria-label="每页条数"
              value={pageSize}
              onChange={(event) => onPageSizeChange(Number(event.target.value))}
            >
              {pageSizeOptions.map((option) => (
                <option key={option} value={option}>{option} 条</option>
              ))}
            </select>
          </label>
        ) : null}
      </div>
      <div className="dashboard-pagination-pages">
        <button
          type="button"
          aria-label="上一页"
          disabled={currentPage <= 1}
          onClick={() => onPageChange(currentPage - 1)}
        >
          ‹
        </button>
        {pages[0] !== 1 ? <span aria-hidden="true">…</span> : null}
        {pages.map((item) => (
          <button
            type="button"
            key={item}
            aria-label={`第 ${item} 页`}
            aria-current={item === currentPage ? "page" : undefined}
            onClick={() => onPageChange(item)}
          >
            {item}
          </button>
        ))}
        {pages[pages.length - 1] !== totalPages ? <span aria-hidden="true">…</span> : null}
        <button
          type="button"
          aria-label="下一页"
          disabled={currentPage >= totalPages}
          onClick={() => onPageChange(currentPage + 1)}
        >
          ›
        </button>
      </div>
    </nav>
  );
}

type CursorProps = {
  cursor: string;
  nextCursor: string | null | undefined;
  onCursorChange: (cursor: string) => void;
  ariaLabel?: string;
};

export function DashboardCursorPagination({
  cursor,
  nextCursor,
  onCursorChange,
  ariaLabel = "分页",
}: CursorProps) {
  const [history, setHistory] = React.useState<string[]>([]);

  React.useEffect(() => {
    if (!cursor) setHistory([]);
  }, [cursor]);

  const previous = () => {
    const nextHistory = history.slice(0, -1);
    const previousCursor = history[history.length - 1] ?? "";
    setHistory(nextHistory);
    onCursorChange(previousCursor);
  };
  const next = () => {
    if (!nextCursor) return;
    setHistory((current) => [...current, cursor]);
    onCursorChange(nextCursor);
  };

  return (
    <nav className="dashboard-pagination" aria-label={ariaLabel}>
      <div className="dashboard-pagination-summary">
        <span>第 {history.length + 1} 页</span>
      </div>
      <div className="dashboard-pagination-pages">
        <button type="button" aria-label="上一页" disabled={history.length === 0} onClick={previous}>‹</button>
        <button type="button" aria-label={`第 ${history.length + 1} 页`} aria-current="page">
          {history.length + 1}
        </button>
        <button type="button" aria-label="下一页" disabled={!nextCursor} onClick={next}>›</button>
      </div>
    </nav>
  );
}
