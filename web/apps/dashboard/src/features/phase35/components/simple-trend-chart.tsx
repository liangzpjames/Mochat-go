export function SimpleTrendChart({ points }: { points: Array<{ at?: string; value?: number }> }) {
  if (!points.length) return <p role="status">暂无趋势数据</p>;
  const max = Math.max(...points.map((point) => point.value ?? 0), 1);
  return <div aria-label="趋势图" className="phase35-trend-chart">{points.map((point, index) => <div className="phase35-trend-column" key={index} title={`${point.at ?? ''}: ${point.value ?? 0}`}><em>{point.value ?? 0}</em><div className="phase35-trend-bar" style={{ height: `${Math.max(4, ((point.value ?? 0) / max) * 100)}%` }} /><span className="phase35-trend-label">{String(point.at ?? '').slice(5, 10)}</span></div>)}</div>;
}
