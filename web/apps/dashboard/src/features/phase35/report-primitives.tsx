import type { ReportResult } from './report-types';

export function ReportPrimitives({ result }: { result?: ReportResult }) {
  const hasSummary = Object.keys(result?.summary ?? {}).length > 0;
  const hasItems = (result?.items ?? []).length > 0;
  const hasSeries = (result?.series ?? []).length > 0;
  const hasDimensions = Object.keys(result?.dimensions ?? {}).length > 0;
  const hasLimitations = (result?.limitations ?? []).length > 0;
  if (!result || (!hasSummary && !hasItems && !hasSeries && !hasDimensions && !hasLimitations)) {
    return <p>暂无数据。</p>;
  }

  return (
    <>
      <div className="dashboard-stat-grid">
        {Object.entries(result.summary ?? {}).map(([key, value]) => (
          <article key={key}>
            <span>{key}</span>
            <strong>{value === null ? '--' : value}</strong>
          </article>
        ))}
      </div>
      {hasSeries && <section aria-label="趋势"><h2>趋势</h2><table><thead><tr>{Object.keys(result.series?.[0] ?? {}).map((key) => <th key={key}>{key}</th>)}</tr></thead><tbody>{result.series?.map((row, index) => <tr key={index}>{Object.values(row).map((value, valueIndex) => <td key={valueIndex}>{String(value ?? '--')}</td>)}</tr>)}</tbody></table></section>}
      {hasDimensions && <section aria-label="分布"><h2>分布</h2><div className="dashboard-stat-grid">{Object.entries(result.dimensions ?? {}).map(([key, value]) => <article key={key}><span>{key}</span><strong>{value ?? '--'}</strong></article>)}</div></section>}
      {result.limitations?.map((limitation) => (
        <p role="status" key={limitation.provider}>{limitation.message}</p>
      ))}
      <table>
        {hasItems && <thead><tr>{Object.keys(result.items?.[0] ?? {}).map((key) => <th key={key}>{key}</th>)}</tr></thead>}
        <tbody>
          {(result.items ?? []).map((row, rowIndex) => (
            <tr key={rowIndex}>
              {Object.values(row).map((value, valueIndex) => (
                <td key={valueIndex}>{String(value ?? '--')}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
      {typeof result.total === 'number' && <p role="status">共 {result.total} 条，第 {result.page ?? 1} 页</p>}
    </>
  );
}
