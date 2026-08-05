import type { ReportResult } from './report-types';
import { MetricCardGrid } from './components/metric-card-grid';
import { ReportDetailTable } from './components/report-detail-table';
import { SimpleTrendChart } from './components/simple-trend-chart';
import { metricLabel } from './presentation/labels';
export function ReportPrimitives({ result }: { result?: ReportResult }) {
  if (!result) return <p role="status">暂无数据。</p>;
  const summary = result.summary ?? {};
  const items = result.items ?? [];
  if (!Object.keys(summary).length && !items.length && !result.series?.length && !Object.keys(result.dimensions ?? {}).length && !result.limitations?.length) return <p role="status">暂无数据。</p>;
  return <>
    <MetricCardGrid metrics={summary} />
    {(result.series?.length ?? 0) > 0 && <section aria-label="趋势"><h2>趋势</h2><SimpleTrendChart points={result.series!.map((point) => ({ at: String(point.at ?? point.day ?? ''), value: Number(point.value ?? 0) }))} /></section>}
    {Object.keys(result.dimensions ?? {}).length > 0 && <section aria-label="分布"><h2>分布</h2><div className="dashboard-stat-grid">{Object.entries(result.dimensions!).map(([key, value]) => <article key={key}><span>{metricLabel(key)}</span><strong>{String(value ?? '--')}</strong></article>)}</div></section>}
    {result.limitations?.map((limitation) => <p role="status" key={limitation.provider}>{limitation.message}</p>)}
    {items.length > 0 ? <ReportDetailTable items={items} {...(result.page === undefined ? {} : { page: result.page })} {...(result.pageSize === undefined ? {} : { pageSize: result.pageSize })} {...(result.total === undefined ? {} : { total: result.total })} /> : !result.limitations?.length && <p role="status">暂无数据。</p>}
  </>;
}
