export function SimpleTrendChart({ points }: { points: Array<{ at?: string; value?: number }> }) {
  if (!points.length) return <p role="status">暂无趋势数据</p>;
  const max = Math.max(...points.map((point) => point.value ?? 0), 1);
  return <div aria-label="趋势图" className="phase35-trend-chart">{points.map((point, index) => <div key={index} title={`${point.at ?? ''}: ${point.value ?? 0}`} style={{ height: `${Math.max(4, ((point.value ?? 0) / max) * 100)}%` }} />)}</div>;
}
