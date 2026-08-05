import { formatMetric } from '../presentation/formatters';
import { metricLabel } from '../presentation/labels';
type Item = { label: string; value: unknown; unit?: string };
export function MetricCardGrid({ metrics, items }: { metrics?: Record<string, unknown>; items?: Item[] }) {
  const values: Item[] = items ?? Object.entries(metrics ?? {}).map(([key,value])=>({label:metricLabel(key),value}));
  return <div className="dashboard-stat-grid">{values.map((item)=><article key={item.label}><span>{item.label}</span><strong>{formatMetric(item.value,item.unit)}</strong></article>)}</div>;
}
