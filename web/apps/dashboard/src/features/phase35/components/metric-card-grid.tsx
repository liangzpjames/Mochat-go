import { formatMetric } from '../presentation/formatters';
import { metricLabel } from '../presentation/labels';
export function MetricCardGrid({ metrics }: { metrics: Record<string, unknown> }) {
  return <div className="dashboard-stat-grid">{Object.entries(metrics).map(([key, value]) => <article key={key}><span>{metricLabel(key)}</span><strong>{formatMetric(value)}</strong></article>)}</div>;
}
