export function displayValue(value: unknown): string {
  return value === null || value === undefined || value === '' ? '--' : String(value);
}

export function formatDateTime(value: string): string {
  if (value.trim() === '') return '--';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date).replaceAll('/', '-');
}

export function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '--';
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}
