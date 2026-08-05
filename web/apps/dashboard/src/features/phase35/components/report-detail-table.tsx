import { metricLabel } from '../presentation/labels';
import { formatDate, formatMetric } from '../presentation/formatters';
export function ReportDetailTable({ items, page = 1, pageSize = 20, total = 0, onPageChange }: { items: Record<string, unknown>[]; page?: number; pageSize?: number; total?: number; onPageChange?: (page: number) => void }) {
  const keys = Object.keys(items[0] ?? {});
  return <><table><thead><tr>{keys.map((key) => <th key={key}>{metricLabel(key)}</th>)}</tr></thead><tbody>{items.map((row, index) => <tr key={String(row.id ?? index)}>{keys.map((key) => <td key={key}>{key.toLowerCase().includes('at') ? formatDate(row[key]) : formatMetric(row[key])}</td>)}</tr>)}</tbody></table><div className="dashboard-pagination"><span>共 {total} 条，第 {page} 页</span><button type="button" disabled={page <= 1} onClick={() => onPageChange?.(page - 1)}>上一页</button><button type="button" disabled={page * pageSize >= total} onClick={() => onPageChange?.(page + 1)}>下一页</button></div></>;
}
