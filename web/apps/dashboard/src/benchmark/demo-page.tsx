import { useMemo, useState } from 'react';

import type { DemoPageConfig, DemoRow } from './demo-fixtures';

const pageSize = 2;

export function DemoPage({ config }: { config: DemoPageConfig }) {
  const [query, setQuery] = useState('');
  const [page, setPage] = useState(1);
  const [detail, setDetail] = useState<DemoRow | null>(null);
  const rows = useMemo(
    () => config.rows.filter((row) => row.name.includes(query.trim())),
    [config.rows, query],
  );
  const pageCount = Math.max(1, Math.ceil(rows.length / pageSize));
  const currentPage = Math.min(page, pageCount);
  const pageRows = rows.slice((currentPage - 1) * pageSize, currentPage * pageSize);

  return (
    <section className="benchmark-demo-page" aria-label={`${config.title}演示页面`}>
      <header className="benchmark-demo-header">
        <div>
          <p className="benchmark-demo-breadcrumb">工作台 / 演示页面</p>
          <h1>{config.title}</h1>
          <p>{config.evidence}</p>
          <p>稳定演示 fixture，仅用于界面预览；不连接 API，也不会保存数据。</p>
        </div>
        <span className="benchmark-demo-badge">演示数据</span>
      </header>

      <div className="benchmark-demo-toolbar">
        <p className="benchmark-demo-unobserved">未观测交互，仅演示：{config.unobserved.interactions.join('、')}</p>
        <label>
          <span className="sr-only">{config.searchLabel}</span>
          <input
            aria-label={config.searchLabel}
            type="search"
            value={query}
            placeholder="搜索名称"
            onChange={(event) => {
              setQuery(event.target.value);
              setPage(1);
            }}
          />
        </label>
        <span className="benchmark-demo-count">共 {rows.length} 条演示记录</span>
      </div>

      <div className="benchmark-demo-table-wrap">
        <table className="benchmark-demo-table">
          <thead><tr>{config.columns.map((column) => <th key={column.key}>{column.title}<small>未观测字段，仅演示</small></th>)}<th>操作<small>未观测交互，仅演示</small></th></tr></thead>
          <tbody>
            {pageRows.map((row) => (
              <tr key={row.id}>
                <td>{row.name}</td>
                <td><span className={`benchmark-demo-status benchmark-demo-status-${row.status}`}>{row.status}</span><small className="benchmark-demo-unobserved">未观测状态，仅演示</small></td>
                <td>{row.updatedAt}</td>
                <td><button type="button" onClick={() => setDetail(row)} aria-label={`查看${row.name}详情`}>查看详情</button></td>
              </tr>
            ))}
          </tbody>
        </table>
        {rows.length === 0 && <p className="benchmark-demo-empty">暂无匹配的演示数据</p>}
      </div>

      {rows.length > 0 && (
        <nav className="benchmark-demo-pagination" aria-label="分页">
          {Array.from({ length: pageCount }, (_, index) => index + 1).map((number) => (
            <button key={number} type="button" aria-label={`第 ${number} 页`} aria-current={currentPage === number ? 'page' : undefined} onClick={() => setPage(number)}>{number}</button>
          ))}
        </nav>
      )}

      {detail !== null && (
        <div className="benchmark-demo-drawer-backdrop" role="presentation" onClick={() => setDetail(null)}>
          <aside className="benchmark-demo-drawer" role="dialog" aria-modal="true" aria-label={`${config.title}详情`} onClick={(event) => event.stopPropagation()}>
            <header><h2>{config.title}详情</h2><button type="button" aria-label="关闭详情" onClick={() => setDetail(null)}>×</button></header>
            <dl>{detail.detail.map((item) => <div key={item.label}><dt>{item.label}</dt><dd>{item.value}</dd></div>)}</dl>
            <p>演示数据，仅用于界面预览</p>
          </aside>
        </div>
      )}
    </section>
  );
}
