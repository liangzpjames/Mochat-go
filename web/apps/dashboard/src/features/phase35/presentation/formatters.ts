export function formatMetric(value: unknown, unit = ''): string {
  if (value === null || value === undefined || value === '') return '--';
  if (typeof value === 'number') return `${new Intl.NumberFormat('zh-CN').format(value)}${unit}`;
  return `${String(value)}${unit}`;
}
export function formatDate(value: unknown): string {
  if (!value) return '--';
  const date = new Date(String(value));
  return Number.isNaN(date.valueOf()) ? String(value) : new Intl.DateTimeFormat('zh-CN').format(date);
}
